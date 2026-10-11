// Package obs is Designer's observability: structured logs (slog, JSON lines
// in the Project's .designer/logs, plus readable text on stderr), timers for
// requests, Assistant turns, tool calls and heavy work, CPU and memory
// samples of Designer and Claude Code, and in-memory metrics served at
// /api/metrics.
package obs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Options configure logging. Zero values are sensible defaults.
type Options struct {
	Dir        string        // log folder; "" = stderr only
	Level      slog.Level    // file and stderr level (debug with --verbose)
	Slow       time.Duration // a request or task slower than this is logged as a warning
	MaxSize    int64         // rotate the log file at this size
	Keep       int           // rotated files to keep
	ConsoleOff bool          // no stderr logging (tests)
}

var (
	level    = new(slog.LevelVar)
	slowWarn = 500 * time.Millisecond
)

// Slow is the "slow" threshold for requests and tasks.
func Slow() time.Duration { return slowWarn }

// Setup installs the default logger. The returned func flushes and closes the log file.
func Setup(o Options) (func(), error) {
	level.Set(o.Level)
	if o.Slow > 0 {
		slowWarn = o.Slow
	}
	var handlers []slog.Handler
	closeFn := func() {}
	if o.Dir != "" {
		if o.MaxSize == 0 {
			o.MaxSize = 20 << 20
		}
		if o.Keep == 0 {
			o.Keep = 5
		}
		rw, err := openRotating(filepath.Join(o.Dir, "designer.log"), o.MaxSize, o.Keep)
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, slog.NewJSONHandler(rw, &slog.HandlerOptions{Level: level}))
		closeFn = func() { _ = rw.Close() }
	}
	if !o.ConsoleOff {
		handlers = append(handlers, slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level, ReplaceAttr: shortTime}))
	}
	slog.SetDefault(slog.New(fanout(handlers)))
	return closeFn, nil
}

// Verbose reports whether debug logging is on.
func Verbose() bool { return level.Level() <= slog.LevelDebug }

func shortTime(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey && len(groups) == 0 {
		return slog.String("time", a.Value.Time().Format("15:04:05.000"))
	}
	return a
}

// ParseLevel accepts debug | info | warn | error.
func ParseLevel(s string) (slog.Level, error) {
	var l slog.Level
	err := l.UnmarshalText([]byte(strings.ToUpper(s)))
	return l, err
}

// ---- fan-out handler

type fanout []slog.Handler

func (f fanout) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f fanout) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range f {
		if h.Enabled(ctx, r.Level) {
			_ = h.Handle(ctx, r.Clone())
		}
	}
	return nil
}

func (f fanout) WithAttrs(as []slog.Attr) slog.Handler {
	out := make(fanout, len(f))
	for i, h := range f {
		out[i] = h.WithAttrs(as)
	}
	return out
}

func (f fanout) WithGroup(name string) slog.Handler {
	out := make(fanout, len(f))
	for i, h := range f {
		out[i] = h.WithGroup(name)
	}
	return out
}

// ---- size-rotated log file: designer.log, designer.log.1 … designer.log.N

type rotating struct {
	mu   sync.Mutex
	path string
	max  int64
	keep int
	f    *os.File
	size int64
}

func openRotating(path string, max int64, keep int) (*rotating, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	r := &rotating{path: path, max: max, keep: keep}
	return r, r.open()
}

func (r *rotating) open() error {
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	st, _ := f.Stat()
	r.f, r.size = f, 0
	if st != nil {
		r.size = st.Size()
	}
	return nil
}

func (r *rotating) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.size+int64(len(p)) > r.max {
		r.rotate()
	}
	n, err := r.f.Write(p)
	r.size += int64(n)
	return n, err
}

func (r *rotating) rotate() {
	_ = r.f.Close()
	for i := r.keep - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", r.path, i), fmt.Sprintf("%s.%d", r.path, i+1))
	}
	_ = os.Rename(r.path, r.path+".1")
	_ = os.Remove(fmt.Sprintf("%s.%d", r.path, r.keep+1))
	if err := r.open(); err != nil {
		r.f, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	}
}

func (r *rotating) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.f.Close()
}

var _ io.WriteCloser = (*rotating)(nil)

// ---- timers

// Timer measures one piece of work. End logs it: debug normally, warn when
// slower than the threshold (Slow() unless given), error when err != nil.
type Timer struct {
	name  string
	start time.Time
	slow  time.Duration
	attrs []any
}

// Start begins timing work called name ("layout", "kb.search", "export.docx"…).
func Start(name string, attrs ...any) *Timer {
	return &Timer{name: name, start: time.Now(), slow: slowWarn, attrs: attrs}
}

// SlowAfter overrides the slow threshold.
func (t *Timer) SlowAfter(d time.Duration) *Timer { t.slow = d; return t }

// End logs the work and records it in the metrics; it returns the duration.
func (t *Timer) End(err error, attrs ...any) time.Duration {
	d := time.Since(t.start)
	M.observe("task", t.name, d, err != nil)
	all := append(append([]any{"task", t.name, "ms", ms(d)}, t.attrs...), attrs...)
	switch {
	case err != nil:
		slog.Error("task failed", append(all, "err", err)...)
	case d > t.slow:
		slog.Warn("slow task", append(all, "slow_ms", ms(t.slow))...)
	default:
		slog.Debug("task", all...)
	}
	return d
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// sortedKeys returns a map's keys in order.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
