package obs

import (
	"slices"
	"sync"
	"time"
)

// M holds this run's metrics (served at /api/metrics).
var M = newMetrics()

const window = 256 // recent samples kept per timer, for percentiles

type stat struct {
	Count  int
	Errors int
	Slow   int
	Total  time.Duration
	Max    time.Duration
	recent []float64 // ms, ring buffer
	next   int
}

// Tokens are summed over Assistant turns.
type Tokens struct {
	Turns         int     `json:"turns"`
	Input         int64   `json:"input"`
	Output        int64   `json:"output"`
	Thinking      int64   `json:"thinking"`
	CacheRead     int64   `json:"cacheRead"`
	CacheCreation int64   `json:"cacheCreation"`
	CostUSD       float64 `json:"costUsd"`
}

func (t *Tokens) add(o Tokens) {
	t.Turns += o.Turns
	t.Input += o.Input
	t.Output += o.Output
	t.Thinking += o.Thinking
	t.CacheRead += o.CacheRead
	t.CacheCreation += o.CacheCreation
	t.CostUSD += o.CostUSD
}

type Metrics struct {
	mu      sync.Mutex
	started time.Time
	stats   map[string]map[string]*stat // kind (http, task, tool, turn, client) → name → stat
	tokens  map[string]*Tokens          // by turn kind, plus "all"
	procs   map[string]ProcSample       // latest sample per process
	peaks   map[string]ProcSample       // peak memory per process
	runtime RuntimeSample
}

func newMetrics() *Metrics {
	return &Metrics{started: time.Now(), stats: map[string]map[string]*stat{}, tokens: map[string]*Tokens{},
		procs: map[string]ProcSample{}, peaks: map[string]ProcSample{}}
}

func (m *Metrics) observe(kind, name string, d time.Duration, failed bool) {
	m.ObserveSlow(kind, name, d, failed, false)
}

// ObserveSlow records one timing.
func (m *Metrics) ObserveSlow(kind, name string, d time.Duration, failed, slow bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g := m.stats[kind]
	if g == nil {
		g = map[string]*stat{}
		m.stats[kind] = g
	}
	s := g[name]
	if s == nil {
		s = &stat{recent: make([]float64, 0, window)}
		g[name] = s
	}
	s.Count++
	s.Total += d
	s.Max = max(s.Max, d)
	if failed {
		s.Errors++
	}
	if slow {
		s.Slow++
	}
	v := ms(d)
	if len(s.recent) < window {
		s.recent = append(s.recent, v)
	} else {
		s.recent[s.next] = v
		s.next = (s.next + 1) % window
	}
}

// AddTokens records one Assistant turn's token use.
func (m *Metrics) AddTokens(kind string, t Tokens) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range []string{kind, "all"} {
		if m.tokens[k] == nil {
			m.tokens[k] = &Tokens{}
		}
		m.tokens[k].add(t)
	}
}

// TokenTotals returns the totals over all turns.
func (m *Metrics) TokenTotals() Tokens {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t := m.tokens["all"]; t != nil {
		return *t
	}
	return Tokens{}
}

func (m *Metrics) setProc(s ProcSample) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.procs[s.Name] = s
	if s.RSS >= m.peaks[s.Name].RSS {
		m.peaks[s.Name] = s
	}
}

func (m *Metrics) setRuntime(r RuntimeSample) {
	m.mu.Lock()
	m.runtime = r
	m.mu.Unlock()
}

// StatView is one timer's summary.
type StatView struct {
	Count  int     `json:"count"`
	Errors int     `json:"errors"`
	Slow   int     `json:"slow"`
	AvgMs  float64 `json:"avgMs"`
	P50Ms  float64 `json:"p50Ms"`
	P95Ms  float64 `json:"p95Ms"`
	MaxMs  float64 `json:"maxMs"`
}

// Snapshot summarises everything for /api/metrics.
func (m *Metrics) Snapshot() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	timers := map[string]map[string]StatView{}
	for kind, g := range m.stats {
		timers[kind] = map[string]StatView{}
		for _, name := range sortedKeys(g) {
			s := g[name]
			r := slices.Clone(s.recent)
			slices.Sort(r)
			pct := func(p float64) float64 {
				if len(r) == 0 {
					return 0
				}
				return r[min(len(r)-1, int(p*float64(len(r))))]
			}
			timers[kind][name] = StatView{Count: s.Count, Errors: s.Errors, Slow: s.Slow, AvgMs: round(ms(s.Total) / float64(s.Count)), P50Ms: pct(0.5), P95Ms: pct(0.95), MaxMs: ms(s.Max)}
		}
	}
	tokens := map[string]Tokens{}
	for k, t := range m.tokens {
		tokens[k] = *t
	}
	return map[string]any{
		"uptimeSec": int(time.Since(m.started).Seconds()),
		"slowMs":    ms(slowWarn),
		"timers":    timers,
		"tokens":    tokens,
		"processes": m.procs,
		"peaks":     m.peaks,
		"runtime":   m.runtime,
	}
}

func round(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
