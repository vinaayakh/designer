package app

import (
	"log/slog"
	"time"

	"github.com/vinaayakh/designer/internal/agent"
	"github.com/vinaayakh/designer/internal/obs"
)

// slowTurn: an Assistant turn taking longer than this is logged as a warning.
const slowTurn = 3 * time.Minute

func turnKind(t *Turn) string {
	if t == nil {
		return ""
	}
	return t.Kind
}

// turnUsage reads the tokens of one turn from the result event's usage (Claude
// Code's per-model totals run over the whole session, resumed history too).
// Cost comes as a session total, so this turn's is the change since the last
// one the session saved (or the whole total in a fresh session).
func turnUsage(ev agent.Event, prevCost float64) obs.Tokens {
	u, _ := ev["usage"].(map[string]any)
	n := func(m map[string]any, k string) int64 { v, _ := m[k].(float64); return int64(v) }
	t := obs.Tokens{Turns: 1, Input: n(u, "input_tokens"), Output: n(u, "output_tokens"), CacheRead: n(u, "cache_read_input_tokens"), CacheCreation: n(u, "cache_creation_input_tokens")}
	if det, ok := u["output_tokens_details"].(map[string]any); ok {
		t.Thinking = n(det, "thinking_tokens")
	}
	if c, ok := ev["total_cost_usd"].(float64); ok {
		t.CostUSD = c - prevCost
		if t.CostUSD < 0 {
			t.CostUSD = c
		}
	}
	return t
}

// logTurnLocked logs a finished turn: timings, tokens, cost. Called before the
// session's cost total is updated.
func (a *App) logTurnLocked(t *Turn, ev agent.Event) {
	d := turnUsage(ev, a.sess.CostUSD)
	kind := turnKind(t)
	if kind == "" {
		kind = "unknown"
	}
	obs.M.AddTokens(kind, d)

	f := func(k string) float64 { v, _ := ev[k].(float64); return v }
	isErr, _ := ev["is_error"].(bool)
	attrs := []any{"kind", kind, "subtype", ev.Subtype(), "is_error", isErr,
		"ms", f("duration_ms"), "ttft_ms", f("ttft_ms"), "api_calls", int(f("num_turns")),
		"tokens_in", d.Input, "tokens_out", d.Output, "tokens_thinking", d.Thinking, "cache_read", d.CacheRead, "cache_write", d.CacheCreation,
		"cost_usd", round4(d.CostUSD), "session_cost_usd", round4(f("total_cost_usd"))}
	var wall time.Duration
	if t != nil {
		wall = time.Since(t.Started)
		attrs = append(attrs, "model", t.model, "wall_ms", float64(wall.Milliseconds()), "first_text_ms", float64(t.firstToken.Milliseconds()),
			"tool_calls", t.tools, "message_chars", t.msgChars, "submitted", t.Submitted, "interrupted", t.Interrupted)
	}
	slow := wall > slowTurn
	obs.M.ObserveSlow("turn", kind, wall, isErr, slow)
	switch {
	case isErr:
		r, _ := ev["result"].(string)
		slog.Error("assistant turn errored", append(attrs, "result", truncate(r, 300))...)
	case slow:
		slog.Warn("slow assistant turn", append(attrs, "slow_ms", float64(slowTurn.Milliseconds()))...)
	default:
		slog.Info("assistant turn", attrs...)
	}
}

func round4(v float64) float64 { return float64(int64(v*10000+0.5)) / 10000 }

// ChildPID is Claude Code's process ID while it runs (for resource samples).
func (a *App) ChildPID() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.proc == nil {
		return 0
	}
	return a.proc.PID()
}
