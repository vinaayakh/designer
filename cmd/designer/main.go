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
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"time"

	"github.com/vinaayakh/designer/internal/app"
	"github.com/vinaayakh/designer/internal/catalog"
	"github.com/vinaayakh/designer/internal/kb"
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
	if err := run(dir, *port, !*noOpen, *claude); err != nil {
		fmt.Fprintln(os.Stderr, "designer:", err)
		os.Exit(1)
	}
}

func run(dir string, port int, open bool, claudeBin string) error {
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
	fmt.Printf("Designer %s\nProject: %s\nOpen %s  (Ctrl+C to stop)\n", version, p.Dir, url)
	if open {
		go openBrowser(url)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
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
