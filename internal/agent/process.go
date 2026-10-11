// Package agent runs Claude Code as a long-running background process,
// talking line-delimited JSON over stdin/stdout (tickets 002, 012, 013).
package agent

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Event is one line of Claude Code's stream-json output.
type Event map[string]any

func (e Event) Type() string    { s, _ := e["type"].(string); return s }
func (e Event) Subtype() string { s, _ := e["subtype"].(string); return s }

type Options struct {
	Binary       string // "claude"
	WorkDir      string // empty, runtime-owned directory
	SystemPrompt string
	Model        string // "" = Claude Code's default
	MCPConfig    string // JSON
	Settings     string // JSON (hooks)
	AllowedTools []string
	Tools        []string
	ResumeID     string
	LogPath      string
}

// Args builds the launch command from ticket 013.
func (o Options) Args() []string {
	args := []string{"-p",
		"--input-format", "stream-json", "--output-format", "stream-json", "--verbose",
		"--include-partial-messages",
		"--system-prompt", o.SystemPrompt,
		"--permission-mode", "dontAsk",
		"--mcp-config", o.MCPConfig, "--strict-mcp-config",
		"--setting-sources", "",
		"--disable-slash-commands",
		"--settings", o.Settings,
	}
	if len(o.Tools) > 0 {
		args = append(args, "--tools", join(o.Tools))
	}
	if len(o.AllowedTools) > 0 {
		args = append(args, "--allowedTools", join(o.AllowedTools))
	}
	if o.Model != "" {
		args = append(args, "--model", o.Model)
	}
	if o.ResumeID != "" {
		args = append(args, "--resume", o.ResumeID)
	}
	return args
}

func join(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += ","
		}
		out += x
	}
	return out
}

type Process struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	mu     sync.Mutex // guards stdin
	Events chan Event
	Done   chan struct{}
	Err    error
	logw   io.WriteCloser
}

// Start launches Claude Code.
func Start(ctx context.Context, o Options) (*Process, error) {
	bin := o.Binary
	if bin == "" {
		bin = "claude"
	}
	if err := os.MkdirAll(o.WorkDir, 0o755); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin, o.Args()...)
	cmd.Dir = o.WorkDir
	cmd.Env = append(os.Environ(), "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var logw io.WriteCloser = nopCloser{io.Discard}
	if o.LogPath != "" {
		if f, err := os.OpenFile(o.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			logw = f
		}
	}
	cmd.Stderr = logw
	p := &Process{cmd: cmd, stdin: stdin, Events: make(chan Event, 256), Done: make(chan struct{}), logw: logw}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting Claude Code: %w", err)
	}
	go p.read(stdout)
	return p, nil
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

func (p *Process) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		line := sc.Bytes()
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			fmt.Fprintf(p.logw, "%s non-JSON: %s\n", stamp(), line)
			continue
		}
		if ev.Type() != "stream_event" {
			fmt.Fprintf(p.logw, "%s <<< %s\n", stamp(), truncate(string(line), 2000))
		}
		p.Events <- ev
	}
	p.Err = p.cmd.Wait()
	close(p.Events)
	close(p.Done)
	p.logw.Close()
}

func stamp() string { return time.Now().Format("15:04:05.000") }

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// send writes one message. It is always produced by json.Marshal, because a
// malformed line kills the process (ticket 012).
func (p *Process) send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.Done:
		return errors.New("Claude Code is not running")
	default:
	}
	fmt.Fprintf(p.logw, "%s >>> %s\n", stamp(), truncate(string(b), 2000))
	_, err = p.stdin.Write(append(b, '\n'))
	return err
}

// PID is Claude Code's process ID (0 once it has exited).
func (p *Process) PID() int {
	select {
	case <-p.Done:
		return 0
	default:
	}
	if p.cmd.Process == nil {
		return 0
	}
	return p.cmd.Process.Pid
}

// SendUser sends a user message (one turn).
func (p *Process) SendUser(text string) error {
	return p.send(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": text}})
}

func reqID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Interrupt stops the current turn; the process stays usable.
func (p *Process) Interrupt() error {
	return p.send(map[string]any{"type": "control_request", "request_id": reqID(), "request": map[string]any{"subtype": "interrupt"}})
}

// SetModel switches the model mid-session.
func (p *Process) SetModel(model string) error {
	req := map[string]any{"subtype": "set_model"}
	if model != "" {
		req["model"] = model
	}
	return p.send(map[string]any{"type": "control_request", "request_id": reqID(), "request": req})
}

// Close ends the process: closing stdin lets Claude Code exit cleanly.
func (p *Process) Close() {
	p.mu.Lock()
	_ = p.stdin.Close()
	p.mu.Unlock()
	select {
	case <-p.Done:
	case <-time.After(5 * time.Second):
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		<-p.Done
	}
}

// Version runs `claude --version`.
func Version(ctx context.Context, bin string) (string, error) {
	if bin == "" {
		bin = "claude"
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		var ee *exec.Error
		if errors.As(err, &ee) {
			return "", fmt.Errorf("Claude Code is not installed (the `%s` command was not found)", bin)
		}
		return "", err
	}
	return string(out), nil
}

// MinVersion is the oldest Claude Code we support: the trusted-domain hook
// needs `onFailure: "block"` (2.1.295). TestedVersion is the newest tested.
const (
	MinVersion    = "2.1.295"
	TestedVersion = "2.1.296"
)

// CheckVersion parses `claude --version` output. It returns an error below
// MinVersion and a warning above TestedVersion.
func CheckVersion(out string) (version, warning string, err error) {
	var a, b, c int
	if _, err := fmt.Sscanf(out, "%d.%d.%d", &a, &b, &c); err != nil {
		return "", "", fmt.Errorf("can't read the Claude Code version from %q", out)
	}
	version = fmt.Sprintf("%d.%d.%d", a, b, c)
	cmp := func(v string) int {
		var x, y, z int
		fmt.Sscanf(v, "%d.%d.%d", &x, &y, &z)
		for _, d := range [][2]int{{a, x}, {b, y}, {c, z}} {
			if d[0] != d[1] {
				if d[0] < d[1] {
					return -1
				}
				return 1
			}
		}
		return 0
	}
	if cmp(MinVersion) < 0 {
		return version, "", fmt.Errorf("Claude Code %s is too old; Designer needs %s or later (run `claude update`)", version, MinVersion)
	}
	if cmp(TestedVersion) > 0 {
		warning = fmt.Sprintf("Claude Code %s is newer than the version Designer was tested with (%s)", version, TestedVersion)
	}
	return version, warning, nil
}
