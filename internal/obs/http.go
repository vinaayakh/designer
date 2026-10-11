package obs

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// streaming routes stay open; they're logged when they close, never as slow.
var streaming = map[string]bool{"/api/events": true}

var idSegment = regexp.MustCompile(`/[0-9]+(/|$)`)

// Middleware times every request. It logs at debug, warns on slow requests
// and 4xx, and reports 5xx as errors with the start of the response body.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := reqID()
		rec := &recorder{ResponseWriter: w, status: 200}
		w.Header().Set("X-Request-Id", id)
		start := time.Now()
		next.ServeHTTP(rec, r)
		d := time.Since(start)

		route := r.Pattern
		if route == "" {
			route = r.Method + " " + idSegment.ReplaceAllString(r.URL.Path, "/{n}$1")
		}
		stream := streaming[r.URL.Path] || (r.URL.Path == "/mcp" && r.Method == http.MethodGet)
		slow := !stream && d > slowWarn
		M.ObserveSlow("http", route, d, rec.status >= 500, slow)
		attrs := []any{"req", id, "method", r.Method, "path", r.URL.Path, "route", route, "status", rec.status, "ms", ms(d), "bytes", rec.bytes}
		if r.ContentLength > 0 {
			attrs = append(attrs, "in_bytes", r.ContentLength)
		}
		if c := r.Header.Get("X-Client"); c != "" {
			attrs = append(attrs, "client", c)
		}
		switch {
		case rec.status >= 500:
			slog.Error("request failed", append(attrs, "body", rec.errBody())...)
		case rec.status >= 400:
			slog.Warn("request rejected", append(attrs, "body", rec.errBody())...)
		case slow:
			slog.Warn("slow request", append(attrs, "slow_ms", ms(slowWarn))...)
		case stream:
			slog.Debug("stream closed", attrs...)
		default:
			slog.Debug("request", attrs...)
		}
	})
}

func reqID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// recorder captures the status, size and the start of an error body. It keeps
// Flush (the event stream) and Hijack working.
type recorder struct {
	http.ResponseWriter
	status int
	bytes  int
	wrote  bool
	head   []byte
}

func (r *recorder) WriteHeader(code int) {
	if !r.wrote {
		r.status, r.wrote = code, true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(p []byte) (int, error) {
	r.wrote = true
	if r.status >= 400 && len(r.head) < 400 {
		r.head = append(r.head, p[:min(len(p), 400-len(r.head))]...)
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytes += n
	return n, err
}

func (r *recorder) errBody() string { return strings.TrimSpace(string(r.head)) }

func (r *recorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *recorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("hijacking not supported")
}

// Unwrap lets http.ResponseController reach the real writer.
func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
