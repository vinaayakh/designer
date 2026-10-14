package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vinaayakh/designer/internal/agent"
	"github.com/vinaayakh/designer/internal/obs"
)

const (
	idleStop     = 15 * time.Minute // ticket 013
	resumeWithin = time.Hour        // prompt cache lifetime on subscription logins
)

// Turn is one exchange with Claude Code.
type Turn struct {
	Kind        string // chat | design | gap | fix | reply
	Expect      string // the submit tool a task must call ("" for chat)
	Submitted   bool
	Interrupted bool
	Started     time.Time
	spec        *taskSpec
	resent      bool
	switched    bool // model was switched for this task
	text        strings.Builder
	model       string
	firstToken  time.Duration // until the first streamed text
	tools       int
	msgChars    int
}

// taskSpec can rebuild its message, so a crashed turn can be resent with
// fresh context.
type taskSpec struct {
	kind, expect, model string
	text                string // the user's chat message, kept with entries proposed from it
	build               func() (string, error)
}

var ErrBusy = errors.New("the Assistant is still working on the last message")

// Status is shown in the sidebar.
type Status struct {
	Running   bool    `json:"running"`
	Busy      bool    `json:"busy"`
	Task      string  `json:"task,omitempty"`
	CostUSD   float64 `json:"costUsd"`
	Version   string  `json:"claudeVersion,omitempty"`
	Warning   string  `json:"warning,omitempty"`
	CanRetry  bool    `json:"canRetry"`
	SessionID string  `json:"sessionId,omitempty"`
}

func (a *App) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.statusLocked()
}

func (a *App) statusLocked() Status {
	s := Status{Running: a.proc != nil, Busy: a.turn != nil, CostUSD: a.sess.CostUSD, Version: a.claudeVer, Warning: a.verWarn, CanRetry: a.lastTask != nil && a.turn == nil, SessionID: a.sess.SessionID}
	if a.turn != nil {
		s.Task = a.turn.Kind
	}
	return s
}

func (a *App) emitStatusLocked() { a.emit("assistant.status", a.statusLocked()) }

// Warm starts Claude Code early (sidebar opened, user started typing) to hide
// its ~4 s startup.
func (a *App) Warm() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ensureLocked()
}

func (a *App) workDir() string {
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	h := sha256.Sum256([]byte(a.P.Dir))
	return filepath.Join(cache, "designer", "sessions", hex.EncodeToString(h[:8]))
}

func (a *App) ensureLocked() error {
	if a.proc != nil {
		return nil
	}
	if a.Base == "" {
		return errors.New("the runtime isn't listening yet")
	}
	if a.claudeVer == "" {
		out, err := agent.Version(context.Background(), a.Bin)
		if err != nil {
			return err
		}
		v, warn, err := agent.CheckVersion(out)
		if err != nil {
			return err
		}
		a.claudeVer, a.verWarn = v, warn
	}
	resume := ""
	if a.sess.SessionID != "" && time.Since(a.sess.LastTurn) < resumeWithin {
		resume = a.sess.SessionID
	} else {
		a.resetSessionLocked()
	}
	return a.startLocked(resume)
}

// resetSessionLocked forgets everything the AI saw: the next message carries
// full context again.
func (a *App) resetSessionLocked() {
	fb := a.sess.Feedback
	cost := a.sess.CostUSD
	a.sess = newSessionState()
	a.sess.Feedback = fb
	a.sess.CostUSD = cost
	a.saveSession()
}

func (a *App) startLocked(resume string) error {
	mcpCfg, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{
		"designer": map[string]any{"type": "http", "url": a.Base + "/mcp", "headers": map[string]string{"Authorization": "Bearer " + a.Token}},
	}})
	wd := a.workDir()
	opts := agent.Options{
		Binary:       a.Bin,
		WorkDir:      wd,
		SystemPrompt: a.systemPrompt,
		Model:        a.settings.Model,
		MCPConfig:    string(mcpCfg),
		Settings:     hookSettings(a.Base+"/hook", a.Token),
		Tools:        []string{"WebSearch", "WebFetch"},
		AllowedTools: append([]string{"mcp__designer__*", "WebSearch"}, webFetchRules()...),
		ResumeID:     resume,
		LogPath:      filepath.Join(a.P.StateDir(), "assistant.log"),
	}
	t0 := time.Now()
	p, err := agent.Start(context.Background(), opts)
	if err != nil {
		slog.Error("Claude Code failed to start", "err", err, "resume", resume != "")
		return err
	}
	slog.Info("Claude Code started", "pid", p.PID(), "model", modelOrDefault(a.settings.Model), "resume", resume != "", "ms", float64(time.Since(t0).Microseconds())/1000)
	a.proc = p
	a.procGen++
	go a.loop(p, a.procGen, resume != "")
	a.resetIdleLocked()
	a.emitStatusLocked()
	return nil
}

func (a *App) resetIdleLocked() {
	if a.idle != nil {
		a.idle.Stop()
	}
	a.idle = time.AfterFunc(idleStop, func() { a.Stop("idle for 15 minutes") })
}

// Stop ends Claude Code (idle, no tabs, or shutdown). The session resumes later.
func (a *App) Stop(reason string) {
	a.mu.Lock()
	p := a.proc
	if p == nil {
		a.mu.Unlock()
		return
	}
	slog.Info("Claude Code stopping", "reason", reason, "pid", p.PID())
	a.proc = nil
	a.procGen++ // the old loop's exit is now expected
	if a.turn != nil {
		a.failTurnLocked("The Assistant stopped: " + reason)
	}
	if a.idle != nil {
		a.idle.Stop()
	}
	a.emitStatusLocked()
	a.mu.Unlock()
	go p.Close()
}

func (a *App) loop(p *agent.Process, gen int, resumed bool) {
	gotInit := false
	for ev := range p.Events {
		if ev.Type() == "system" && ev.Subtype() == "init" {
			gotInit = true
		}
		a.handle(ev, gen)
	}
	a.onExit(p, gen, resumed, gotInit)
}

func (a *App) handle(ev agent.Event, gen int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if gen != a.procGen {
		return
	}
	switch ev.Type() {
	case "system":
		if ev.Subtype() == "init" {
			if id, ok := ev["session_id"].(string); ok && id != "" && id != a.sess.SessionID {
				a.sess.SessionID = id
				a.saveSession()
			}
		}
	case "stream_event":
		e, _ := ev["event"].(map[string]any)
		if e["type"] == "content_block_delta" {
			d, _ := e["delta"].(map[string]any)
			if d["type"] == "text_delta" {
				t, _ := d["text"].(string)
				if a.turn != nil {
					if a.turn.firstToken == 0 {
						a.turn.firstToken = time.Since(a.turn.Started)
					}
					a.turn.text.WriteString(t)
				}
				a.emit("assistant.delta", map[string]any{"text": t})
			}
		}
	case "assistant":
		msg, _ := ev["message"].(map[string]any)
		content, _ := msg["content"].([]any)
		for _, c := range content {
			cm, _ := c.(map[string]any)
			if cm["type"] == "tool_use" {
				name, _ := cm["name"].(string)
				if a.turn != nil {
					a.turn.tools++
				}
				slog.Debug("assistant tool call", "tool", name, "turn", turnKind(a.turn))
				a.emit("assistant.tool", map[string]any{"name": strings.TrimPrefix(name, "mcp__designer__"), "input": cm["input"]})
			}
		}
	case "result":
		a.finishTurnLocked(ev)
	}
}

func (a *App) finishTurnLocked(ev agent.Event) {
	a.logTurnLocked(a.turn, ev)
	if c, ok := ev["total_cost_usd"].(float64); ok && c > 0 {
		a.sess.CostUSD = c
	}
	a.sess.LastTurn = time.Now()
	a.saveSession()
	t := a.turn
	a.turn = nil
	a.resetIdleLocked()
	isErr, _ := ev["is_error"].(bool)
	done := map[string]any{"costUsd": a.sess.CostUSD, "isError": isErr, "subtype": ev.Subtype()}
	if t != nil {
		done["kind"] = t.Kind
		if t.switched && a.proc != nil {
			_ = a.proc.SetModel(modelOrDefault(a.settings.Model))
		}
		switch {
		case t.Interrupted:
			done["interrupted"] = true
			a.reviewNotRevisedLocked(t, "")
		case t.Kind == "revise" && !t.Submitted:
			// No new revision: the AI is asking the user something.
			a.reviewNotRevisedLocked(t, strings.TrimSpace(t.text.String()))
		case t.Expect != "" && !t.Submitted:
			msg := "The Assistant finished without submitting a result."
			if r, ok := ev["result"].(string); ok && r != "" {
				msg += " It said: " + truncate(r, 400)
			}
			done["failed"] = msg
		case isErr:
			r, _ := ev["result"].(string)
			done["failed"] = "Claude Code reported an error: " + truncate(r, 400)
		default:
			if t.spec != nil && t.Expect != "" {
				a.lastTask = nil // succeeded; nothing to retry
			}
		}
	}
	a.emit("assistant.done", done)
	a.emitStatusLocked()
	if t != nil && t.Kind == "step" {
		a.nextDocumentLocked(t.Submitted && !t.Interrupted && !isErr)
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func (a *App) failTurnLocked(reason string) {
	t := a.turn
	a.turn = nil
	kind := ""
	if t != nil {
		kind = t.Kind
		obs.M.ObserveSlow("turn", kind, time.Since(t.Started), true, false)
	}
	slog.Error("assistant turn failed", "kind", kind, "reason", reason)
	a.emit("assistant.done", map[string]any{"kind": kind, "failed": reason, "costUsd": a.sess.CostUSD})
}

// onExit handles the process ending. An expected stop does nothing. A crash
// resumes once and resends the in-flight turn; if that fails too, a fresh
// session starts with full context (ticket 013).
func (a *App) onExit(p *agent.Process, gen int, resumed, gotInit bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if gen != a.procGen {
		return
	}
	a.proc = nil
	reason := "Claude Code exited"
	if p.Err != nil {
		reason += ": " + p.Err.Error()
	}
	slog.Error("Claude Code exited unexpectedly", "err", p.Err, "turn", turnKind(a.turn), "resumed", resumed, "got_init", gotInit)
	t := a.turn
	if resumed && !gotInit {
		// The saved session couldn't be resumed: start fresh.
		a.resetSessionLocked()
		a.emit("assistant.notice", map[string]any{"text": "Started a new Assistant session (the previous one couldn't be resumed)."})
		if t != nil {
			a.resendLocked(t, "")
		}
		a.emitStatusLocked()
		return
	}
	if t == nil {
		a.emitStatusLocked()
		return
	}
	if !t.resent && a.sess.SessionID != "" {
		t.resent = true
		a.resendLocked(t, a.sess.SessionID)
		return
	}
	a.resetSessionLocked()
	a.emit("assistant.notice", map[string]any{"text": "The Assistant restarted with a fresh session. " + reason})
	if t.spec != nil && !t.Submitted {
		t.resent = true
		a.resendLocked(t, "")
		return
	}
	a.failTurnLocked(reason)
	a.emitStatusLocked()
}

func (a *App) resendLocked(t *Turn, resume string) {
	a.turn = nil
	if err := a.startLocked(resume); err != nil {
		a.failTurnLocked(err.Error())
		return
	}
	if t.spec == nil {
		a.failTurnLocked("The Assistant restarted; please send your message again.")
		return
	}
	msg, err := t.spec.build()
	if err != nil {
		a.failTurnLocked(err.Error())
		return
	}
	a.turn = t
	t.Started = time.Now()
	if err := a.proc.SendUser(msg); err != nil {
		a.failTurnLocked(err.Error())
	}
	a.emitStatusLocked()
}

// run starts a turn.
func (a *App) run(spec *taskSpec) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.turn != nil {
		return ErrBusy
	}
	if err := a.ensureLocked(); err != nil {
		return err
	}
	build := time.Now()
	msg, err := spec.build()
	if err != nil {
		slog.Error("building the task message failed", "kind", spec.kind, "err", err)
		return err
	}
	model := spec.model
	if model == "" {
		model = modelOrDefault(a.settings.Model)
	}
	t := &Turn{Kind: spec.kind, Expect: spec.expect, Started: time.Now(), spec: spec, model: model, msgChars: len(msg)}
	slog.Info("assistant turn started", "kind", spec.kind, "model", model, "message_chars", len(msg), "build_ms", float64(time.Since(build).Microseconds())/1000)
	if spec.model != "" && spec.model != a.settings.Model {
		if err := a.proc.SetModel(spec.model); err == nil {
			t.switched = true
		}
	}
	if spec.expect != "" {
		a.lastTask = spec
	}
	a.turn = t
	if err := a.proc.SendUser(msg); err != nil {
		a.turn = nil
		return err
	}
	if a.idle != nil {
		a.idle.Stop()
	}
	a.emit("assistant.started", map[string]any{"kind": spec.kind})
	a.emitStatusLocked()
	return nil
}

// Interrupt cancels the current turn.
func (a *App) Interrupt() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.turn == nil || a.proc == nil {
		return nil
	}
	a.turn.Interrupted = true
	return a.proc.Interrupt()
}

// Retry reruns the last task that failed.
func (a *App) Retry() error {
	a.mu.Lock()
	spec := a.lastTask
	a.mu.Unlock()
	if spec == nil {
		return errors.New("nothing to retry")
	}
	return a.run(spec)
}

// markSubmitted is called by the submit tools.
func (a *App) markSubmittedLocked(tool string) {
	if a.turn != nil && a.turn.Expect == tool {
		a.turn.Submitted = true
	}
}

// Chat sends the user's message with the context the AI needs.
func (a *App) Chat(text, view string, selection []string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("empty message")
	}
	return a.run(&taskSpec{kind: "chat", text: text, build: func() (string, error) {
		ctx, err := a.contextLocked(view, selection)
		if err != nil {
			return "", err
		}
		return ctx + "\n\nUser: " + text, nil
	}})
}

// ResetSession starts a brand-new Assistant Session (no memory of earlier chat).
func (a *App) ResetSession() {
	a.Stop("new session")
	a.mu.Lock()
	a.resetSessionLocked()
	a.sess.SessionID = ""
	a.saveSession()
	a.emitStatusLocked()
	a.mu.Unlock()
}

func (a *App) queueFeedbackLocked(s string) {
	a.sess.Feedback = append(a.sess.Feedback, s)
	a.saveSession()
}

// reviewNotRevisedLocked handles a revise turn that ended without a new
// revision: its reply becomes a question for the user, and review resumes.
func (a *App) reviewNotRevisedLocked(t *Turn, question string) {
	if t.Kind != "revise" {
		return
	}
	p := a.pendingLocked()
	if p == nil || !p.Awaiting {
		return
	}
	if question == "" {
		question = "The Assistant didn't send a revision. Tell it how to continue, or reject the whole Proposal."
	}
	p.Awaiting, p.Question = false, question
	if err := a.P.WriteState(pendingFile, p); err == nil {
		a.emit("proposal", p)
	}
}
