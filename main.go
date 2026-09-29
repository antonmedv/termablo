package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type tickMsg time.Time

type model struct {
	g     *Game
	scr   *Screen
	start time.Time
	seed  int64
}

func tick() tea.Cmd {
	return tea.Tick(time.Second/20, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *model) Init() tea.Cmd { return tick() }

var moveKeys = map[string]Pos{
	"up": {0, -1}, "k": {0, -1}, "8": {0, -1},
	"down": {0, 1}, "j": {0, 1}, "2": {0, 1},
	"left": {-1, 0}, "h": {-1, 0}, "4": {-1, 0},
	"right": {1, 0}, "l": {1, 0}, "6": {1, 0},
	"y": {-1, -1}, "7": {-1, -1}, "home": {-1, -1},
	"u": {1, -1}, "9": {1, -1}, "pgup": {1, -1},
	"b": {-1, 1}, "1": {-1, 1}, "end": {-1, 1},
	"n": {1, 1}, "3": {1, 1}, "pgdown": {1, 1},
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	g := m.g
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.scr.Resize(msg.Width, msg.Height)
	case tickMsg:
		g.time = time.Since(m.start).Seconds()
		if g.auto && g.Mode == ModePlay && g.time >= g.autoNext {
			g.autoNext = g.time + 0.055
			if !g.autoStep() {
				g.auto = false
			}
		}
		return m, tick()
	case tea.KeyMsg:
		g.time = time.Since(m.start).Seconds()
		k := msg.String()
		if k == "ctrl+c" {
			return m, tea.Quit
		}
		if g.auto {
			g.auto = false
			return m, nil
		}
		if quit := m.key(k); quit {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *model) key(k string) bool {
	g := m.g
	switch g.Mode {
	case ModeTitle:
		g.Mode = ModePlay
	case ModeDead:
		switch k {
		case "n":
			m.seed = time.Now().UnixNano()
			ng := NewGame(m.seed)
			ng.Mode = ModePlay
			ng.time = g.time
			m.g = ng
		case "Q", "esc":
			return true
		}
	case ModeTalk:
		g.Mode = ModePlay
	case ModeHelp, ModeMap:
		if k == "esc" || k == "?" || k == "m" || k == "q" || k == "enter" || k == " " {
			g.Mode = ModePlay
		}
	case ModeChar:
		p := g.P
		if p.Points > 0 {
			switch k {
			case "1":
				p.Str++
				p.Points--
			case "2":
				p.Dex++
				p.Points--
			case "3":
				p.Vit++
				p.Points--
				p.HP += 2
			case "4":
				p.Ene++
				p.Points--
			}
			p.recalc()
		}
		if k == "esc" || k == "c" || k == "q" {
			g.Mode = ModePlay
		}
	case ModeInv:
		m.invKey(k)
	case ModeShop:
		m.shopKey(k)
	case ModePlay:
		return m.playKey(k)
	}
	return false
}

func (m *model) playKey(k string) bool {
	g := m.g
	if d, ok := moveKeys[k]; ok {
		g.move(d.X, d.Y)
		return false
	}
	switch k {
	case ".", "5", "s":
		g.wait()
	case "g", ",":
		g.pickup()
	case "f":
		g.castFirebolt()
	case "r":
		g.castNova()
	case "q":
		g.drinkHealth()
	case "w":
		g.drinkMana()
	case "t":
		g.readPortal()
	case "tab":
		g.cycleTarget()
	case "o":
		g.auto = true
		g.autoItems = 1 << 30
		g.autoNext = g.time
		if !g.autoStep() {
			g.auto = false
		}
	case "i":
		g.Mode, g.cur, g.pane = ModeInv, 0, 1
	case "c":
		g.Mode = ModeChar
	case "m":
		g.Mode = ModeMap
	case "?":
		g.Mode = ModeHelp
	case "Q":
		return true
	}
	return false
}

func (m *model) invKey(k string) {
	g := m.g
	p := g.P
	n := EqCount
	if g.pane == 1 {
		n = len(p.Inv)
	}
	switch k {
	case "esc", "i", "q":
		g.Mode = ModePlay
	case "tab", "left", "right", "h", "l":
		g.pane = 1 - g.pane
		g.cur = 0
	case "up", "k":
		if g.cur > 0 {
			g.cur--
		}
	case "down", "j":
		if g.cur < n-1 {
			g.cur++
		}
	case "enter", "e", " ":
		if g.pane == 1 {
			g.equip(g.cur)
		} else {
			g.unequip(g.cur)
		}
	case "d":
		if g.pane == 1 {
			g.dropInv(g.cur)
		}
	}
	if g.pane == 1 {
		g.cur = clampi(g.cur, 0, maxi(0, len(p.Inv)-1))
	}
}

func (m *model) shopKey(k string) {
	g := m.g
	items := g.shopList()
	switch k {
	case "esc", "q":
		g.Mode = ModePlay
	case "tab", "left", "right", "h", "l":
		g.tab = 1 - g.tab
		g.cur = 0
	case "up", "k":
		if g.cur > 0 {
			g.cur--
		}
	case "down", "j":
		if g.cur < len(items)-1 {
			g.cur++
		}
	case "enter", " ", "b", "s":
		if g.cur < len(items) {
			if g.tab == 0 {
				g.buy(items[g.cur])
			} else {
				g.sell(g.cur)
			}
		}
	}
	g.cur = clampi(g.cur, 0, maxi(0, len(g.shopList())-1))
}

func (m *model) View() string {
	m.g.Draw(m.scr)
	return m.scr.String()
}

func main() {
	seed := flag.Int64("seed", 0, "world seed (0 = random)")
	flag.Parse()
	if *seed == 0 {
		*seed = time.Now().UnixNano()
	}
	m := &model{g: NewGame(*seed), scr: NewScreen(120, 40), start: time.Now(), seed: *seed}
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
