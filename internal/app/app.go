// Package app is the Designer runtime for one open Project: the Assistant
// Session (Claude Code), its MCP tools and hook, Proposals, Conversion and
// Gap Checks.
package app

import (
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vinaayakh/designer/internal/agent"
	"github.com/vinaayakh/designer/internal/brief"
	"github.com/vinaayakh/designer/internal/catalog"
	"github.com/vinaayakh/designer/internal/kb"
	"github.com/vinaayakh/designer/internal/project"
	"github.com/vinaayakh/designer/internal/scene"
)

//go:embed prompt.md
var promptTemplate string

// Settings are per install (not per Project).
type Settings struct {
	Model           string `json:"model"`           // "" = Claude Code's default; opus | sonnet | haiku
	ConversionModel string `json:"conversionModel"` // used for Design and Gap Check
}

// sessionState is persisted in .designer/session.json so a restart can
// --resume with the same Aliases and know what the AI has already seen.
type sessionState struct {
	SessionID string                    `json:"sessionId"`
	LastTurn  time.Time                 `json:"lastTurn"`
	Aliases   *scene.Aliases            `json:"aliases"`
	Sent      map[string]scene.Snapshot `json:"sent"` // view -> what the AI last saw
	SentBrief string                    `json:"sentBrief"`
	SentVer   int                       `json:"sentVersion"`
	Feedback  []string                  `json:"feedback"` // queued for the next message
	CostUSD   float64                   `json:"costUsd"`
}

func newSessionState() *sessionState {
	return &sessionState{Aliases: scene.NewAliases(), Sent: map[string]scene.Snapshot{}}
}

type App struct {
	P     *project.Project
	Cat   *catalog.Catalog
	KB    *kb.KB
	Bin   string // Claude Code binary
	Token string // bearer token for /mcp and /hook
	Base  string // http://127.0.0.1:port, set once the server listens

	settingsPath string
	systemPrompt string

	mu        sync.Mutex
	settings  Settings
	sess      *sessionState
	proc      *agent.Process
	procGen   int
	turn      *Turn
	idle      *time.Timer
	lastTask  *taskSpec // for Retry
	gap       *gapSpec  // the Gap Check in progress
	doc       *docSpec  // the document task in progress
	clients   int
	noClients *time.Timer
	claudeVer string
	verWarn   string

	hub hub
}

// New opens the runtime for a Project.
func New(p *project.Project, cat *catalog.Catalog, k *kb.KB) (*App, error) {
	tok := make([]byte, 24)
	if _, err := rand.Read(tok); err != nil {
		return nil, err
	}
	a := &App{P: p, Cat: cat, KB: k, Bin: "claude", Token: hex.EncodeToString(tok)}
	a.hub.subs = map[chan []byte]struct{}{}
	a.systemPrompt = strings.Replace(promptTemplate, "{{KINDS}}", cat.KindsPrompt(), 1)

	cfg, _ := os.UserConfigDir()
	a.settingsPath = filepath.Join(cfg, "designer", "settings.json")
	a.settings = Settings{ConversionModel: "opus"}
	if b, err := os.ReadFile(a.settingsPath); err == nil {
		_ = json.Unmarshal(b, &a.settings)
	}

	a.sess = newSessionState()
	if err := p.ReadState("session.json", a.sess); err != nil {
		a.sess = newSessionState()
	}
	if a.sess.Aliases == nil {
		a.sess.Aliases = scene.NewAliases()
	}
	if a.sess.Sent == nil {
		a.sess.Sent = map[string]scene.Snapshot{}
	}
	return a, nil
}

func (a *App) authorized(r *http.Request) bool {
	if r.Header.Get("Origin") != "" {
		return false // never reachable from a browser page
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(got), []byte(a.Token)) == 1
}

func (a *App) saveSession() {
	if err := a.P.WriteState("session.json", a.sess); err != nil {
		fmt.Fprintln(os.Stderr, "designer: saving session:", err)
	}
}

func (a *App) Settings() Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings
}

// SetSettings saves per-install settings; a model change applies mid-session.
func (a *App) SetSettings(s Settings) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	modelChanged := s.Model != a.settings.Model
	a.settings = s
	if err := os.MkdirAll(filepath.Dir(a.settingsPath), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	if err := os.WriteFile(a.settingsPath, b, 0o644); err != nil {
		return err
	}
	if modelChanged && a.proc != nil && a.turn == nil {
		_ = a.proc.SetModel(modelOrDefault(s.Model))
	}
	return nil
}

func modelOrDefault(m string) string {
	if m == "" {
		return "default"
	}
	return m
}

// ---- Views

const (
	ViewRough      = "rough"
	ViewProduction = "production"
)

// Graph returns the meaning of a view: the Rough Draft, or the latest
// Production version (with its number).
func (a *App) Graph(view string) (*scene.Graph, int, error) {
	if view == ViewProduction {
		n, err := a.P.Latest()
		if err != nil {
			return nil, 0, err
		}
		if n == 0 {
			return nil, 0, fmt.Errorf("there is no Production Diagram yet")
		}
		_, data, err := a.P.Version(n)
		if err != nil {
			return nil, 0, err
		}
		g, err := scene.Parse(data, true)
		return g, n, err
	}
	data, err := a.P.RoughDraft()
	if err != nil {
		return nil, 0, err
	}
	g, err := scene.Parse(data, false)
	return g, 0, err
}

// DefaultView is Production once it exists.
func (a *App) DefaultView() string {
	if n, _ := a.P.Latest(); n > 0 {
		return ViewProduction
	}
	return ViewRough
}

func (a *App) Brief() (*brief.Brief, error) { return a.P.Brief() }

// ---- Events to the browser (Server-Sent Events)

type hub struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func (a *App) emit(typ string, data any) {
	b, err := json.Marshal(map[string]any{"type": typ, "data": data})
	if err != nil {
		return
	}
	a.hub.mu.Lock()
	defer a.hub.mu.Unlock()
	for ch := range a.hub.subs {
		select {
		case ch <- b:
		default: // a slow client misses events rather than blocking the runtime
		}
	}
}

// Subscribe registers a browser tab. Stop the Assistant 30 s after the last
// tab disconnects (ticket 013).
func (a *App) Subscribe() (chan []byte, func()) {
	ch := make(chan []byte, 256)
	a.hub.mu.Lock()
	a.hub.subs[ch] = struct{}{}
	a.hub.mu.Unlock()
	a.mu.Lock()
	a.clients++
	if a.noClients != nil {
		a.noClients.Stop()
		a.noClients = nil
	}
	a.mu.Unlock()
	return ch, func() {
		a.hub.mu.Lock()
		delete(a.hub.subs, ch)
		a.hub.mu.Unlock()
		a.mu.Lock()
		a.clients--
		if a.clients == 0 {
			a.noClients = time.AfterFunc(30*time.Second, func() { a.Stop("no browser tab is open") })
		}
		a.mu.Unlock()
	}
}

// Notify tells every tab that files changed (after saves from another tab
// or from the runtime).
func (a *App) Notify(what string, data any) { a.emit(what, data) }
