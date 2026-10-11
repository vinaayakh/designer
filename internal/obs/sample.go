package obs

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"time"
)

// ProcSample is one CPU and memory reading of a process.
type ProcSample struct {
	Name   string    `json:"name"`
	PID    int       `json:"pid"`
	CPUPct float64   `json:"cpuPct"` // of one core over the last interval
	RSSMB  float64   `json:"rssMb"`
	RSS    uint64    `json:"-"`
	At     time.Time `json:"at"`
}

// RuntimeSample is the Go runtime's view of Designer itself.
type RuntimeSample struct {
	HeapMB     float64 `json:"heapMb"`
	SysMB      float64 `json:"sysMb"`
	Goroutines int     `json:"goroutines"`
	NumGC      uint32  `json:"numGc"`
	GCPauseMs  float64 `json:"gcPauseTotalMs"`
}

// Limits above which a sample is logged as a warning.
type Limits struct {
	CPUPct  float64 // sustained over 3 samples
	SelfMB  float64
	ChildMB float64
}

// Sample starts reading CPU and memory every interval until ctx ends: of
// Designer, and of Claude Code while it runs (child returns its PID, or 0).
func Sample(ctx context.Context, interval time.Duration, child func() int, lim Limits) {
	if interval <= 0 {
		return
	}
	type prev struct {
		pid int
		cpu time.Duration
		at  time.Time
		hot int
	}
	last := map[string]*prev{}
	read := func(name string, pid int, limitMB float64) {
		if pid == 0 {
			delete(last, name)
			return
		}
		cpu, rss, err := procStat(pid)
		now := time.Now()
		if err != nil {
			slog.Debug("resource sample failed", "process", name, "pid", pid, "err", err)
			return
		}
		s := ProcSample{Name: name, PID: pid, RSS: rss, RSSMB: mb(rss), At: now}
		p := last[name]
		if p != nil && p.pid == pid {
			if wall := now.Sub(p.at); wall > 0 {
				s.CPUPct = round(float64(cpu-p.cpu) / float64(wall) * 100)
			}
		} else {
			p = &prev{}
		}
		*p = prev{pid: pid, cpu: cpu, at: now, hot: p.hot}
		if lim.CPUPct > 0 && s.CPUPct >= lim.CPUPct {
			p.hot++
		} else {
			p.hot = 0
		}
		last[name] = p
		M.setProc(s)
		attrs := []any{"process", name, "pid", pid, "cpu_pct", s.CPUPct, "rss_mb", s.RSSMB}
		switch {
		case p.hot == 3:
			slog.Warn("high CPU", append(attrs, "for_s", int(3*interval.Seconds()))...)
		case limitMB > 0 && s.RSSMB > limitMB:
			slog.Warn("high memory", append(attrs, "limit_mb", limitMB)...)
		default:
			slog.Debug("resources", attrs...)
		}
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			read("designer", os.Getpid(), lim.SelfMB)
			read("claude", child(), lim.ChildMB)
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			rs := RuntimeSample{HeapMB: mb(ms.HeapAlloc), SysMB: mb(ms.Sys), Goroutines: runtime.NumGoroutine(), NumGC: ms.NumGC, GCPauseMs: float64(ms.PauseTotalNs) / 1e6}
			M.setRuntime(rs)
			slog.Debug("go runtime", "heap_mb", rs.HeapMB, "sys_mb", rs.SysMB, "goroutines", rs.Goroutines, "gc", rs.NumGC, "gc_pause_total_ms", round(rs.GCPauseMs))
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

func mb(b uint64) float64 { return round(float64(b) / (1 << 20)) }
