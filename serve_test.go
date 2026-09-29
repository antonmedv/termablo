package main

import (
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish/testsession"
	gossh "golang.org/x/crypto/ssh"
)

func TestIdleQuits(t *testing.T) {
	m := newModel(7, "")
	m.frame, m.idle = 10*time.Millisecond, 100*time.Millisecond
	pr, _ := io.Pipe()
	p := tea.NewProgram(m, tea.WithInput(pr), tea.WithOutput(io.Discard))
	done := make(chan error, 1)
	go func() { _, err := p.Run(); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		p.Kill()
		t.Fatal("idle game still running")
	}
}

func TestInputKeepsGameAlive(t *testing.T) {
	m := newModel(7, "")
	m.idle = time.Minute
	m.lastSeen = time.Now().Add(-2 * time.Minute)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'.'}})
	if _, cmd := m.Update(tickMsg(time.Now())); cmd == nil || isQuit(cmd) {
		t.Error("game quit right after a key press")
	}
	m.lastSeen = time.Now().Add(-2 * time.Minute)
	if _, cmd := m.Update(tickMsg(time.Now())); !isQuit(cmd) {
		t.Error("idle game did not quit")
	}
}

func isQuit(cmd tea.Cmd) bool {
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestSessionLimit(t *testing.T) {
	release := make(chan struct{})
	srv := &ssh.Server{Handler: sessionLimit(1)(func(s ssh.Session) {
		_, _ = io.WriteString(s, "playing\n")
		<-release
	})}
	addr := testsession.Listen(t, srv)
	cfg := &gossh.ClientConfig{HostKeyCallback: gossh.InsecureIgnoreHostKey()}
	first, err := testsession.NewClientSession(t, addr, cfg)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := first.StdoutPipe()
	if err := first.Shell(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	if n, _ := out.Read(buf); !strings.Contains(string(buf[:n]), "playing") {
		t.Fatalf("first player got %q", buf[:n])
	}
	second, err := testsession.NewClientSession(t, addr, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := second.CombinedOutput(""); !strings.Contains(string(b), "full") {
		t.Errorf("second player got %q", b)
	}
	close(release)
}

func TestIdleNotice(t *testing.T) {
	srv := &ssh.Server{Handler: idleNotice(time.Minute)(func(s ssh.Session) {
		s.Context().SetValue(modelKey{}, &model{idledOut: true})
	})}
	addr := testsession.Listen(t, srv)
	s, err := testsession.NewClientSession(t, addr, &gossh.ClientConfig{HostKeyCallback: gossh.InsecureIgnoreHostKey()})
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := s.CombinedOutput(""); !strings.Contains(string(b), "without input") {
		t.Errorf("idle player told %q", b)
	}
}
