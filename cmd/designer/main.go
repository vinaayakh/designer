// Command designer runs Designer for one Project folder and opens it in the
// browser:
//
//	designer [project-folder]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"github.com/vinaayakh/designer/internal/app"
	"github.com/vinaayakh/designer/internal/catalog"
	"github.com/vinaayakh/designer/internal/kb"
	"github.com/vinaayakh/designer/internal/obs"
	"github.com/vinaayakh/designer/internal/project"
	"github.com/vinaayakh/designer/internal/server"
	kbfs "github.com/vinaayakh/designer/kb"
	"github.com/vinaayakh/designer/web"
)

var version = "dev"

func main() {
	port := flag.Int("port", 4717, "preferred port on 127.0.0.1 (a free one is used if it's taken)")
	noOpen := flag.Bool("no-open", false, "don't open the browser")
	claude := flag.String("claude", "claude", "the Claude Code command")
	showVersion := flag.Bool("version", false, "print the version and exit")
	logLevel := flag.String("log-level", "info", "log level: debug | info | warn | error")
	verbose := flag.Bool("verbose", false, "verbose logs (same as -log-level debug): every request, tool call, timer and resource sample")
	logDir := flag.String("log-dir", "", "where to write designer.log (default: <project>/.designer/logs)")
	slowMs := flag.Int("slow-ms", 500, "log requests, tool calls and tasks slower than this as warnings")
	sample := flag.Duration("sample", 15*time.Second, "how often to sample CPU and memory of Designer and Claude Code (0 = off)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: designer [flags] [project-folder]\n\nOpens (or creates) a Designer Project in the folder (default: the current folder).\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if *showVersion {
		fmt.Println("designer", version)
		return
	}
	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	lvl, err := obs.ParseLevel(*logLevel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "designer: -log-level must be debug, info, warn or error")
		os.Exit(2)
	}
	if *verbose {
		lvl = slog.LevelDebug
	}
	lo := logOpts{level: lvl, dir: *logDir, slow: time.Duration(*slowMs) * time.Millisecond, sample: *sample}
	if err := run(dir, *port, !*noOpen, *claude, lo); err != nil {
		slog.Error("designer stopped with an error", "err", err)
		fmt.Fprintln(os.Stderr, "designer:", err)
		os.Exit(1)
	}
}

type logOpts struct {
	level  slog.Level
	dir    string
	slow   time.Duration
	sample time.Duration
}

func run(dir string, port int, open bool, claudeBin string, lo logOpts) error {
	boot := time.Now()
	cat, err := catalog.Load()
	if err != nil {
		return err
	}
	k, err := kb.Load(kbfs.Records, "records")
	if err != nil {
		return err
	}
	p, err := project.Open(dir)
	if err != nil {
		return err
	}
	if lo.dir == "" {
		lo.dir = filepath.Join(p.StateDir(), "logs")
	}
	closeLog, err := obs.Setup(obs.Options{Dir: lo.dir, Level: lo.level, Slow: lo.slow})
	if err != nil {
		return fmt.Errorf("logs: %w", err)
	}
	defer closeLog()
	slog.Info("designer starting", "version", version, "project", p.Dir, "log_dir", lo.dir, "log_level", lo.level.String(), "slow_ms", lo.slow.Milliseconds(),
		"sample", lo.sample.String(), "go", runtime.Version(), "os", runtime.GOOS+"/"+runtime.GOARCH, "cpus", runtime.NumCPU(),
		"kb_records", len(k.Records), "kb_version", k.Version)
	a, err := app.New(p, cat, k)
	if err != nil {
		return err
	}
	a.Bin = claudeBin

	ln, err := server.Listen(port)
	if err != nil {
		return err
	}
	addr := ln.Addr().String()
	a.Base = "http://" + addr
	s := &server.Server{A: a, Web: web.Dist(), Addr: addr}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}

	url := "http://" + addr + "/"
	fmt.Printf("Designer %s\nProject: %s\nOpen %s  (Ctrl+C to stop)\nLogs: %s\n", version, p.Dir, url, filepath.Join(lo.dir, "designer.log"))
	slog.Info("designer listening", "url", url, "startup_ms", time.Since(boot).Milliseconds())
	if open {
		go openBrowser(url)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	obs.Sample(ctx, lo.sample, a.ChildPID, obs.Limits{CPUPct: 90, SelfMB: 1024, ChildMB: 2048})
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	fmt.Println("\nStopping…")
	tok := obs.M.TokenTotals()
	slog.Info("designer stopping", "uptime_s", int(time.Since(boot).Seconds()), "turns", tok.Turns, "tokens_in", tok.Input, "tokens_out", tok.Output,
		"cache_read", tok.CacheRead, "cache_write", tok.CacheCreation, "cost_usd", tok.CostUSD)
	a.Stop("Designer is shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return srv.Shutdown(sctx)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
