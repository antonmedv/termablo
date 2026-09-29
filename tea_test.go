package main

import (
	"bytes"
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTeaRun(t *testing.T) {
	pr, pw := io.Pipe()
	var out bytes.Buffer
	m := &model{g: NewGame(7), scr: NewScreen(120, 40), start: time.Now(), seed: 7}
	p := tea.NewProgram(m, tea.WithInput(pr), tea.WithOutput(&out))
	go func() {
		time.Sleep(200 * time.Millisecond)
		p.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
		for _, k := range []string{"x", "o", "l", "l", "i", "\x1b", "c", "\x1b", "?", "\x1b", "m", "\x1b", "f", "r", "t"} {
			pw.Write([]byte(k))
			time.Sleep(150 * time.Millisecond)
		}
		time.Sleep(500 * time.Millisecond)
		p.Quit()
	}()
	if _, err := p.Run(); err != nil {
		t.Fatal(err)
	}
	t.Logf("output bytes: %d", out.Len())
}
