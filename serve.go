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
	maxSessions   int           // 0 = no limit
	idle          time.Duration // 0 = never
	connectEvery  time.Duration // per address; 0 = no limit
}

type modelKey struct{}

// serve runs an SSH server where every connection plays its own game.
func serve(o serveOpts) error {
	// Middlewares run last to first: log, rate limit, cap, then play.
	mw := []wish.Middleware{
		bm.MiddlewareWithColorProfile(func(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
			m := newModel(o.seed, o.level)
			m.frame, m.idle = time.Second/sshFPS, o.idle
			sess.Context().SetValue(modelKey{}, m)
			return m, []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseAllMotion(), tea.WithFPS(sshFPS)}
		}, termenv.TrueColor),
		idleNotice(o.idle),
		activeterm.Middleware(),
	}
	if o.maxSessions > 0 {
		mw = append(mw, sessionLimit(o.maxSessions))
	}
	if o.connectEvery > 0 {
		// Behind a proxy every player shares one address: raise or disable.
		mw = append(mw, ratelimiter.Middleware(ratelimiter.NewRateLimiter(rate.Every(o.connectEvery), 3, 1000)))
	}
	mw = append(mw, logging.Middleware())
	srv, err := wish.NewServer(
		wish.WithAddress(o.addr),
		wish.WithHostKeyPath(o.hostKey),
		wish.WithMiddleware(mw...),
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

// idleNotice tells a player who was dropped for idling why.
func idleNotice(idle time.Duration) wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(sess ssh.Session) {
			next(sess)
			if m, ok := sess.Context().Value(modelKey{}).(*model); ok && m.idledOut {
				wish.Printf(sess, "Disconnected after %v without input.\n", idle)
			}
		}
	}
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
