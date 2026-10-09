package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/antonmedv/termablo/internal/i18n"
)

type tickMsg time.Time

type model struct {
	g     *Game
	scr   *Screen
	start time.Time
	seed  int64

	frame    time.Duration // tick interval, 1/20s when zero
	idle     time.Duration // quit after this long without input, never when zero
	lastSeen time.Time
	idledOut bool

	// -bot: a scripted player drives, at botTPS turns per second.
	bot     *Bot
	policy  *botPolicy // the bot comes back with this after death
	botRun  bool       // driving, not paused
	botTPS  int
	botNext float64 // game time of the next bot turn
}

// botSpeeds are the turns per second + and - step through.
var botSpeeds = []int{1, 2, 5, 10, 20, 60}

// period is the tick interval.
func (m *model) period() time.Duration {
	if m.frame == 0 {
		return time.Second / 20
	}
	return m.frame
}

func (m *model) tick() tea.Cmd {
	return tea.Tick(m.period(), func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *model) Init() tea.Cmd { return m.tick() }

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
	case tea.MouseMsg:
		m.lastSeen = time.Now()
		g.SetHover(msg.X, msg.Y)
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && !g.auto && m.bot == nil {
			g.Click(msg.X, msg.Y, m.scr.W, m.scr.H)
		}
		if g.Mode == ModeTalk && msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				g.talk.scroll(-2)
			case tea.MouseButtonWheelDown:
				g.talk.scroll(2)
			}
		}
	case tickMsg:
		if m.idle > 0 && time.Since(m.lastSeen) > m.idle {
			m.idledOut = true
			return m, tea.Quit
		}
		g.time = time.Since(m.start).Seconds()
		if g.auto && g.Mode == ModePlay && g.time >= g.autoNext {
			g.autoNext = g.time + 0.055
			if !g.autoStep() {
				g.auto = false
			}
		}
		m.botTick()
		return m, m.tick()
	case tea.KeyMsg:
		m.lastSeen = time.Now()
		g.time = time.Since(m.start).Seconds()
		k := msg.String()
		if k == "ctrl+c" {
			return m, tea.Quit
		}
		if g.auto {
			g.auto = false
			return m, nil
		}
		if m.bot != nil && m.botKey(k) {
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
		switch k {
		case "right", "l", "tab":
			g.SetLang(nextLang(g.L.Lang.Code, 1))
		case "left", "h", "shift+tab":
			g.SetLang(nextLang(g.L.Lang.Code, -1))
		case "enter", " ":
			g.Mode = ModePlay
		}
	case ModeDead:
		switch k {
		case "n":
			m.restart()
		case "Q", "esc":
			return true
		}
	case ModeTalk:
		g.talkKey(k)
	case ModeHelp, ModeMap:
		switch k {
		case "esc", "?", "m", "q", "enter", " ":
			g.Mode = ModePlay
		case "up", "k":
			g.helpOff--
		case "down", "j":
			g.helpOff++
		case "pgup":
			g.helpOff -= 10
		case "pgdown":
			g.helpOff += 10
		}
	case ModeChar:
		switch k {
		case "1", "2", "3", "4":
			g.spendPoint(int(k[0] - '1'))
		case "esc", "c", "q":
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

// restart begins a new game on a fresh seed; the bot, if any, comes along.
func (m *model) restart() {
	m.seed = time.Now().UnixNano()
	ng := NewGame(m.seed)
	ng.SetLang(m.g.L.Lang.Code)
	ng.Mode = ModePlay
	ng.time = m.g.time
	m.g = ng
	if m.policy != nil {
		m.drive(m.policy)
	}
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
		g.cycleTarget(1)
	case "shift+tab":
		g.cycleTarget(-1)
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
		g.Mode, g.helpOff = ModeHelp, 0
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
		if g.talk != nil {
			g.Mode = ModeTalk // back to the conversation Barter came from
		}
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

// ------------------------------------------------------------ bot mode

// drive hands the game to a scripted player, running at the speed set
// before, or 5 turns a second.
func (m *model) drive(pol *botPolicy) {
	m.policy = pol
	m.bot = NewBot(m.g, pol)
	m.g.Mode = ModePlay
	if m.botTPS <= 0 {
		m.botTPS = 5
	}
	m.botRun, m.botNext = true, m.g.time
}

// botTick plays the bot's turns that have come due.
func (m *model) botTick() {
	g := m.g
	if m.bot == nil || !m.botRun || g.Mode != ModePlay {
		return
	}
	if m.botNext < g.time-1 { // no catching up after a pause
		m.botNext = g.time
	}
	perTick := int(math.Ceil(float64(m.botTPS) * m.period().Seconds()))
	for n := 0; g.time >= m.botNext && n < perTick && g.Mode == ModePlay; n++ {
		m.bot.turn()
		m.botNext += 1 / float64(m.botTPS)
	}
}

// botKey handles the bot's keys: space pauses or resumes, enter steps one
// turn, + and - change speed. Any other key pauses the bot and then acts
// as usual, so a fight can be played by hand until space gives it back.
func (m *model) botKey(k string) bool {
	if m.g.Mode != ModePlay {
		return false
	}
	switch k {
	case " ":
		m.botRun = !m.botRun
		m.botNext = m.g.time
	case "enter":
		m.botRun = false
		m.bot.turn()
	case "+", "=":
		m.botTPS = botSpeed(m.botTPS, 1)
	case "-", "_":
		m.botTPS = botSpeed(m.botTPS, -1)
	default:
		m.botRun = false
		return false
	}
	return true
}

// botSpeed steps through botSpeeds from tps: the next one up or down,
// from wherever -tps put it.
func botSpeed(tps, dir int) int {
	if dir > 0 {
		for _, s := range botSpeeds {
			if s > tps {
				return s
			}
		}
		return tps
	}
	for i := len(botSpeeds) - 1; i >= 0; i-- {
		if botSpeeds[i] < tps {
			return botSpeeds[i]
		}
	}
	return botSpeeds[0]
}

// drawBot puts the bot's status on the top line of the map, in the style
// of the hover line: policy, state, why, turn, speed.
func (m *model) drawBot() {
	s, g, b := m.scr, m.g, m.bot
	if s.W < 80 || s.H < 24 || (g.Mode != ModePlay && g.Mode != ModeDead) {
		return // the other modes draw boxes up to the top row
	}
	speed := fmt.Sprintf("%d tps", m.botTPS)
	if !m.botRun {
		speed = "paused"
	}
	lines := []hoverLine{{S: "bot " + b.Policy(), C: colBot}, {S: b.State().String(), C: colWhite}, {S: b.Why(), C: colGray},
		{S: fmt.Sprintf("turn %d", g.Turn), C: colGray}, {S: speed, C: colGold}}
	w := s.W - panelW - 2
	used := 0
	for _, ln := range lines {
		used += len([]rune(ln.S)) + 3
	}
	if why := []rune(b.Why()); used > w && len(why) > 8 { // the reason gives way first
		cut := maxi(8, len(why)-(used-w)-1)
		lines[2].S = string(why[:cut]) + "…"
		used -= len(why) - cut - 1
	}
	if hint := "space run/pause · enter step · +/- speed"; used+len(hint)+3 <= w {
		lines = append(lines, hoverLine{S: hint, C: colDim})
	}
	drawStatus(s, 0, s.W-panelW, lines)
}

func (m *model) View() string {
	m.g.Draw(m.scr)
	if m.bot != nil {
		m.drawBot()
	}
	return m.scr.String()
}

func newModel(seed int64, level string) *model {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	g := NewGame(seed)
	if level != "" {
		g.changeLevel(level, "", nil)
	}
	now := time.Now()
	return &model{g: g, scr: NewScreen(120, 40), start: now, seed: seed, lastSeen: now}
}

func main() {
	seed := flag.Int64("seed", 0, "world seed (0 = random)")
	area := flag.String("area", "", "start in this area instead of town (e.g. crypt1, grotto2, abyss3)")
	lvl := flag.Int("level", 0, "start at this character level, geared and stocked for the area")
	build := flag.String("build", "fighter", "with -level: spend points like the fighter or the caster")
	bot := flag.String("bot", "", "watch a scripted player: fighter or caster (ignored with -ssh)")
	tps := flag.Int("tps", 5, "bot: turns per second to start at; + and - change it")
	addr := flag.String("ssh", "", "serve the game over SSH on this address (e.g. :2222)")
	hostKey := flag.String("hostkey", ".ssh/termablo_ed25519", "SSH host key, created if missing")
	maxSessions := flag.Int("max-sessions", 50, "SSH: most games at once (0 = no limit)")
	idle := flag.Duration("idle", 15*time.Minute, "SSH: disconnect after this long without input")
	connectEvery := flag.Duration("connect-every", 10*time.Second, "SSH: one new game per address this often, 3 at once (0 = no limit)")
	lang := flag.String("lang", "", "language: "+langCodes()+" (default from $LANG; with -ssh, for players whose client sends none)")
	flag.Parse()
	code := ""
	if *lang != "" {
		if code = i18n.Match(*lang); code == "" {
			fmt.Fprintf(os.Stderr, "-lang %q: want one of %s\n", *lang, langCodes())
			os.Exit(2)
		}
	}
	if *addr != "" {
		// -lang is the default for players whose client asks for none
		err := serve(serveOpts{addr: *addr, hostKey: *hostKey, seed: *seed, level: *area, lang: code, maxSessions: *maxSessions, idle: *idle, connectEvery: *connectEvery})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if code == "" {
		code = i18n.FromEnv(os.Getenv)
	}
	m := newModel(*seed, *area)
	m.g.SetLang(code)
	if *lvl > 0 {
		if err := m.g.Ready(*lvl, *build); err != nil {
			fmt.Fprintf(os.Stderr, "-level: %v\n", err)
			os.Exit(2)
		}
	}
	if *bot != "" {
		pol := policyByName(*bot)
		if pol == nil {
			fmt.Fprintf(os.Stderr, "-bot %q: want fighter or caster\n", *bot)
			os.Exit(2)
		}
		if *tps < 1 {
			fmt.Fprintf(os.Stderr, "-tps %d: want at least 1\n", *tps)
			os.Exit(2)
		}
		m.botTPS = *tps
		m.drive(pol)
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
