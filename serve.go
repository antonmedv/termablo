package main

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/muesli/termenv"
)

// serve runs an SSH server where every connection plays its own game:
// ssh -p 2222 localhost. A fixed seed gives everyone the same world.
func serve(addr, hostKey string, seed int64, level string) error {
	srv, err := wish.NewServer(
		wish.WithAddress(addr),
		wish.WithHostKeyPath(hostKey),
		wish.WithMiddleware(
			bm.MiddlewareWithColorProfile(func(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
				return newModel(seed, level), []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseAllMotion()}
			}, termenv.TrueColor),
			activeterm.Middleware(),
			logging.Middleware(),
		),
	)
	if err != nil {
		return err
	}
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
	log.Printf("termablo: ssh -p %s localhost", port(addr))
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

func port(addr string) string {
	if _, p, err := net.SplitHostPort(addr); err == nil {
		return p
	}
	return addr
}
