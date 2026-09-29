package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/charmbracelet/wish/ratelimiter"
	"github.com/muesli/termenv"
	"golang.org/x/time/rate"
)

// Over SSH every frame crosses the network: the flickering light repaints
// most lines, so remote games run at half the local frame rate.
const sshFPS = 10

type serveOpts struct {
	addr, hostKey string
	seed          int64 // 0 gives every player a fresh world
	level         string
	maxSessions   int
	idle          time.Duration
}

// serve runs an SSH server where every connection plays its own game.
func serve(o serveOpts) error {
	srv, err := wish.NewServer(
		wish.WithAddress(o.addr),
		wish.WithHostKeyPath(o.hostKey),
		wish.WithMiddleware(
			bm.MiddlewareWithColorProfile(func(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
				m := newModel(o.seed, o.level)
				m.frame, m.idle = time.Second/sshFPS, o.idle
				return m, []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseAllMotion(), tea.WithFPS(sshFPS)}
			}, termenv.TrueColor),
			activeterm.Middleware(),
			sessionLimit(o.maxSessions),
			// Each address may open a game every 10s, 3 in a burst.
			ratelimiter.Middleware(ratelimiter.NewRateLimiter(rate.Every(10*time.Second), 3, 1000)),
			logging.Middleware(),
		),
	)
	if err != nil {
		return err
	}
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
	log.Printf("termablo: listening for ssh on %s", o.addr)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-done
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// sessionLimit turns players away once n games are running.
func sessionLimit(n int) wish.Middleware {
	var live atomic.Int64
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			defer live.Add(-1)
			if live.Add(1) > int64(n) {
				wish.Fatalln(sess, "termablo is full, try again later")
				return
			}
			next(sess)
		}
	}
}
