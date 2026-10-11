package obs

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRotation(t *testing.T) {
	dir := t.TempDir()
	r, err := openRotating(filepath.Join(dir, "designer.log"), 100, 2)
	if err != nil {
		t.Fatal(err)
	}
	line := []byte(strings.Repeat("x", 40) + "\n")
	for range 10 {
		if _, err := r.Write(line); err != nil {
			t.Fatal(err)
		}
	}
	r.Close()
	for _, name := range []string{"designer.log", "designer.log.1", "designer.log.2"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s missing: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "designer.log.3")); err == nil {
		t.Error("kept more files than asked")
	}
}

func TestMiddleware(t *testing.T) {
	M = newMetrics()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/versions/{n}", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	mux.HandleFunc("POST /api/fail", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":"boom"}`))
	})
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			t.Error("the event stream lost http.Flusher")
		}
	})
	h := Middleware(mux)
	for _, req := range []*http.Request{httptest.NewRequest("GET", "/api/versions/3", nil), httptest.NewRequest("POST", "/api/fail", nil), httptest.NewRequest("GET", "/api/events", nil)} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Header().Get("X-Request-Id") == "" {
			t.Error("no request id")
		}
	}
	snap := M.Snapshot()["timers"].(map[string]map[string]StatView)["http"]
	if snap["GET /api/versions/{n}"].Count != 1 {
		t.Errorf("route pattern not used: %v", snap)
	}
	if snap["POST /api/fail"].Errors != 1 {
		t.Errorf("5xx not counted as an error: %v", snap)
	}
}

func TestPercentiles(t *testing.T) {
	m := newMetrics()
	for i := 1; i <= 100; i++ {
		m.observe("task", "x", time.Duration(i)*time.Millisecond, false)
	}
	v := m.Snapshot()["timers"].(map[string]map[string]StatView)["task"]["x"]
	if v.P50Ms != 51 || v.P95Ms != 96 || v.MaxMs != 100 || v.Count != 100 {
		t.Errorf("got %+v", v)
	}
}

func TestProcStatSelf(t *testing.T) {
	cpu, rss, err := procStat(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if rss < 1<<20 {
		t.Errorf("rss %d is implausibly small", rss)
	}
	if cpu < 0 {
		t.Errorf("cpu %v", cpu)
	}
}
