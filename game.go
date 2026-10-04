package main

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"sort"
	"strconv"
	"strings"
)

type Mode int

const (
	ModeTitle Mode = iota
	ModePlay
	ModeInv
	ModeChar
	ModeShop
	ModeHelp
	ModeMap
	ModeDead
	ModeTalk
	ModeChoice // the Last Wanderer is dead: choose how the story ends
	ModeEnd    // the story has ended; the run is over
)

const visThresh = 0.035

type LogMsg struct {
	Text string
	Col  RGB
	Turn int
	N    int
}

type Portal struct {
	Level string
	X, Y  int
}

type Shop struct {
	Name     string
	Kind     int // 0 smith, 1 alchemist
	Items    []*Item
	Services []*Item // gambles and the reroll: bought, never sold out
}

type Game struct {
	rng    *rand.Rand
	Seed   int64
	Levels map[string]*Level
	Lv     *Level
	P      *Player
	Log    []LogMsg
	logN   int // messages ever appended, for the bot trace
	Turn   int
	Mode   Mode
	time   float64

	fov    []uint32
	fovGen uint32
	vis    []bool
	light  []RGB
	lstamp []uint32
	dark   []Pos // where light drinkers stand, refreshed with the lights
	// The Last Wanderer between levels: once he has noticed the hero he
	// follows them out of any level and steps out followDelay turns later.
	stalker  *Monster
	stalkIn  int
	stalkAt  Pos      // where the hero arrived, where he will step out
	homeYet  bool     // he has reached Emberhold
	ahead    *Monster // gone ahead to Emberhold through his own portal
	aheadAt  int      // the turn he went ahead
	snuffed  []*Light // town lights he has put out; back when he dies
	Fallen   []string // townsfolk he has killed, by template ID, in order
	ForgeOut bool     // Hadrik is dead and the forge with him
	// Rift is the red portal he opens at half life; it leads to town.
	Rift      *Portal
	riftLight *Light
	lgen      uint32
	dist      []int32
	lightsBuf []*Light

	Effects     []*Effect
	Portal      *Portal
	portalLight *Light
	townPortalL *Light
	Target      *Monster
	hitter      *Monster // the last monster to strike the hero, for Stats.Death

	Quests  [3]int // 0 hunting, 1 slain, 2 rewarded; the third ends at 1
	Ending  int    // the choice made over the Last Wanderer's body
	Deepest int    // deepest Depth entered: what the shops and Voss's rewards roll at
	stocked int    // Deepest at the last restock
	shops   [2]*Shop
	shop    *Shop

	cur, pane, tab int
	helpOff        int // help screen scroll
	talkName       string
	talkLines      []string
	talkCol        RGB

	hoverX, hoverY int
	hoverOn        bool
	hoverLines     []hoverLine // this frame's hover info (nil = nothing hovered)
	beltHit        [3]hitBox   // this frame's belt rows: heal, mana, portal

	auto       bool
	autoNext   float64
	autoItems  int
	usedAltars map[string]bool

	Stats Stats
	Rules *Rules
}

func NewGame(seed int64) *Game { return NewGameWith(seed, DefaultRules()) }

// NewGameWith starts a game under a set of rules, which it shares with
// its player, levels and monsters.
func NewGameWith(seed int64, r *Rules) *Game {
	g := &Game{Seed: seed, rng: rand.New(rand.NewSource(seed)), Levels: map[string]*Level{}, usedAltars: map[string]bool{}, Stats: newStats(), Rules: r, Deepest: 1}
	g.P = NewPlayer(r)
	g.portalLight = NewLight(0, 0, &LightSpec{C(.35, .5, 1), 4.5, 1.1, .05, .3}, 1)
	g.townPortalL = NewLight(0, 0, &LightSpec{C(.35, .5, 1), 4.5, 1.1, .05, .3}, 2)
	g.riftLight = NewLight(0, 0, &LightSpec{C(1, .15, .1), 4.5, 1.1, .05, .3}, 3)
	sw := &Item{Kind: IKEquip, Base: baseByName("Short Sword"), Name: "Short Sword", ILvl: 1}
	sw.finishStats(g.rng)
	ar := &Item{Kind: IKEquip, Base: baseByName("Leather Armor"), Name: "Leather Armor", ILvl: 1}
	ar.finishStats(g.rng)
	g.P.Eq[EqWeapon] = sw
	g.P.Eq[EqArmor] = ar
	g.P.recalc()
	g.changeLevel("town", "", nil)
	g.msg(colOrange, "Hunt the Bone King in the crypt and the Drowned Oracle beneath Blackmarsh.")
	g.msg(colGray, "Press ? for help. Captain Voss by the tavern has work for you.")
	return g
}

func (g *Game) msg(col RGB, f string, a ...any) {
	s := fmt.Sprintf(f, a...)
	if n := len(g.Log); n > 0 && g.Log[n-1].Text == s {
		g.Log[n-1].N++
		g.Log[n-1].Turn = g.Turn
		return
	}
	g.Log = append(g.Log, LogMsg{s, col, g.Turn, 1})
	g.logN++
	if len(g.Log) > 200 {
		g.Log = g.Log[len(g.Log)-200:]
	}
}

// ------------------------------------------------------------ levels

func (g *Game) getLevel(id string) *Level {
	if l, ok := g.Levels[id]; ok {
		return l
	}
	seed := g.Seed*7919 + int64(len(id))*131
	for _, c := range id {
		seed = seed*31 + int64(c)
	}
	var l *Level
	num := func(prefix string) int { n, _ := strconv.Atoi(strings.TrimPrefix(id, prefix)); return n }
	switch {
	case id == "town":
		l = genTown(seed, g.Rules)
	case id == "fields":
		l = genFields(seed, g.Rules)
	case id == "marsh":
		l = genMarsh(seed, g.Rules)
	case strings.HasPrefix(id, "crypt"):
		n := num("crypt")
		s := DungeonSpec{ID: id, Name: fmt.Sprintf("Crypt of the Fallen %d", n), Depth: 1 + n, Style: 0, SpawnTable: "crypt", Rules: g.Rules}
		s.Up = "fields"
		if n > 1 {
			s.Up = fmt.Sprintf("crypt%d", n-1)
		}
		if n < 4 {
			s.Down = fmt.Sprintf("crypt%d", n+1)
		} else {
			s.Boss = "boneking"
			s.Name = "Throne of the Bone King"
		}
		l = genDungeon(s, seed)
	case strings.HasPrefix(id, "grotto"):
		n := num("grotto")
		s := DungeonSpec{ID: id, Name: fmt.Sprintf("Sunken Grotto %d", n), Depth: 6 + n, Style: 1, SpawnTable: "grotto", Rules: g.Rules}
		s.Up = "marsh"
		if n > 1 {
			s.Up = fmt.Sprintf("grotto%d", n-1)
		}
		if n < 3 {
			s.Down = fmt.Sprintf("grotto%d", n+1)
		} else {
			s.Down = "abyss1"
			s.Boss = "oracle"
			s.Name = "The Oracle's Pool"
		}
		l = genDungeon(s, seed)
	case strings.HasPrefix(id, "abyss"):
		n := num("abyss")
		s := DungeonSpec{ID: id, Name: fmt.Sprintf("The Burning Abyss %d", n), Depth: 9 + n, Style: 2, SpawnTable: "abyss", Rules: g.Rules}
		s.Up = "grotto3"
		if n > 1 {
			s.Up = fmt.Sprintf("abyss%d", n-1)
		}
		s.Down = fmt.Sprintf("abyss%d", n+1)
		if n == hearthFloor {
			s.Name, s.Boss = "The Hearth Below", "wanderer"
			s.SealedDown, s.Down = s.Down, ""
		}
		l = genDungeon(s, seed)
	default:
		l = genTown(seed, g.Rules)
	}
	g.Levels[id] = l
	return l
}

func (g *Game) changeLevel(id, from string, arrive *Pos) {
	if old := g.Lv; old != nil {
		g.followOut(old)
	}
	l := g.getLevel(id)
	g.Lv = l
	n := l.W * l.H
	g.fov = make([]uint32, n)
	g.fovGen = 0
	g.vis = make([]bool, n)
	g.light = make([]RGB, n)
	g.lstamp = make([]uint32, n)
	g.lgen = 0
	g.dist = make([]int32, n)
	for _, lt := range l.Lights {
		lt.ver = -1
	}
	p := g.P
	x, y := l.Start.X, l.Start.Y
	if arrive != nil {
		x, y = l.FreeNear(arrive.X, arrive.Y, -1, -1)
	} else if lk := l.LinkTo(from); lk != nil {
		if lk.AX >= 0 {
			x, y = l.FreeNear(lk.AX, lk.AY, -1, -1)
		} else {
			x, y = l.FreeNear(lk.X0, lk.Y0, -1, -1)
		}
	} else {
		x, y = l.FreeNear(x, y, -1, -1)
	}
	p.X, p.Y = x, y
	g.stalkAt = Pos{x, y}
	if m := g.ahead; m != nil && l.Kind == KTown {
		g.ahead = nil
		m.Gone, m.Awake, m.LostTurns = false, true, 0
		m.X, m.Y = l.FreeNear(l.PortalAt.X-2, l.PortalAt.Y, x, y)
		l.Monsters = append(l.Monsters, m)
		g.homeYet = true
		g.msg(colRed, "The Last Wanderer is already here.")
		g.tollWhileAway(g.Turn - g.aheadAt)
	}
	p.Torch.ver = -1
	g.Effects = nil
	g.Target = nil
	g.auto = false
	if !l.visited {
		l.visited = true
		if l.Lore != "" {
			g.msg(colLore, "%s", l.Lore)
		}
	} else {
		g.msg(colGray, "You enter %s.", l.Name)
	}
	if l.Kind != KTown && l.Depth > p.Lvl+2 {
		g.msg(colLore, "Something here is far beyond you.")
	}
	if l.Kind != KTown && l.Depth > g.Deepest {
		g.Deepest = l.Depth
	}
	if l.Kind == KTown && g.Deepest != g.stocked {
		g.restock()
	}
	g.computeVisibility()
}

// ------------------------------------------------------------ light & sight

// drinkRadius is how far the Last Wanderer's presence reaches: every
// light within it goes out but his own, the hero's torch included.
const drinkRadius = 6

// drinks says whether a cell lies within drinkRadius of a light drinker,
// measured the way light falls off: a cell is twice as tall as it is wide.
func drinks(at []Pos, x, y int) bool {
	for _, d := range at {
		fx, fy := float64(x-d.X)/Aspect, float64(y-d.Y)
		if fx*fx+fy*fy <= drinkRadius*drinkRadius {
			return true
		}
	}
	return false
}

func (g *Game) gatherLights() []*Light {
	l, p := g.Lv, g.P
	ls := g.lightsBuf[:0]
	g.dark = g.dark[:0]
	for _, m := range l.Monsters {
		if m.T.AI == AIWanderer && !m.Dead {
			g.dark = append(g.dark, Pos{m.X, m.Y})
		}
	}
	town := l.Kind == KTown
	add := func(lt *Light) {
		if !drinks(g.dark, lt.X, lt.Y) {
			ls = append(ls, lt)
		} else if town && !lt.Off && lt != p.Torch && slices.Contains(l.Lights, lt) {
			// in Emberhold what he puts out stays out while he lives
			lt.Off = true
			g.snuffed = append(g.snuffed, lt)
		}
	}
	near := func(x, y int, r float32) bool {
		return abs(x-p.X) < 80+int(r*2) && abs(y-p.Y) < 45+int(r)
	}
	for _, lt := range l.Lights {
		if near(lt.X, lt.Y, lt.Radius) {
			add(lt)
		}
	}
	p.Torch.X, p.Torch.Y = p.X, p.Y
	add(p.Torch)
	for _, m := range l.Monsters {
		if m.Light != nil && !m.Dead {
			m.Light.X, m.Light.Y = m.X, m.Y
			if m.T.AI == AIWanderer {
				ls = append(ls, m.Light)
			} else {
				add(m.Light)
			}
		}
	}
	for _, it := range l.Items {
		if it.Light != nil {
			add(it.Light)
		}
	}
	if g.Rift != nil && g.Rift.Level == l.ID {
		g.riftLight.X, g.riftLight.Y = g.Rift.X, g.Rift.Y
		add(g.riftLight)
	}
	if g.Portal != nil {
		if g.Portal.Level == l.ID {
			g.portalLight.X, g.portalLight.Y = g.Portal.X, g.Portal.Y
			add(g.portalLight)
		}
		if l.Kind == KTown {
			g.townPortalL.X, g.townPortalL.Y = l.PortalAt.X, l.PortalAt.Y
			add(g.townPortalL)
		}
	}
	for _, e := range g.Effects {
		if e.Light != nil {
			ls = append(ls, e.Light)
		}
	}
	g.lightsBuf = ls
	return ls
}

func (g *Game) composeLight(t float64, flick bool) {
	l := g.Lv
	amb := l.Ambient
	if g.townHunted() {
		amb = RGB{}
	}
	for i := range g.light {
		g.light[i] = amb
	}
	for _, lt := range g.gatherLights() {
		if lt.Off {
			continue
		}
		lt.ensure(l, g.lstamp, &g.lgen)
		f := lt.Intensity
		if flick {
			f *= lt.factor(t)
		}
		c := lt.Color.Scale(f)
		for _, lc := range lt.cells {
			px := &g.light[lc.i]
			px.R += c.R * lc.w
			px.G += c.G * lc.w
			px.B += c.B * lc.w
		}
	}
}

func (g *Game) computeVisibility() {
	l := g.Lv
	g.fovGen++
	castRays(l, g.P.X, g.P.Y, 34, g.fov, g.fovGen, nil)
	g.composeLight(0, false)
	for i := range g.vis {
		v := g.fov[i] == g.fovGen && g.light[i].Max() > visThresh
		g.vis[i] = v
		if v {
			l.Seen[i] = true
		}
	}
	// You can always feel what's directly around you.
	for _, d := range dirs8 {
		if x, y := g.P.X+d.X, g.P.Y+d.Y; l.In(x, y) {
			l.Seen[l.Idx(x, y)] = true
		}
	}
}

func (g *Game) canSee(x, y int) bool {
	l := g.Lv
	return l.In(x, y) && g.vis[l.Idx(x, y)]
}

func (g *Game) visibleHostiles() []*Monster {
	var r []*Monster
	for _, m := range g.Lv.Monsters {
		if !m.Dead && !m.Friendly && g.canSee(m.X, m.Y) {
			r = append(r, m)
		}
	}
	return r
}

func (g *Game) nearestHostile() *Monster {
	var best *Monster
	bd := 1 << 30
	for _, m := range g.visibleHostiles() {
		d := (m.X-g.P.X)*(m.X-g.P.X) + (m.Y-g.P.Y)*(m.Y-g.P.Y)*4
		if d < bd {
			best, bd = m, d
		}
	}
	return best
}

func (g *Game) validTarget() *Monster {
	if g.Target != nil && !g.Target.Dead && g.canSee(g.Target.X, g.Target.Y) {
		return g.Target
	}
	g.Target = g.nearestHostile()
	return g.Target
}

// cycleTarget steps through visible enemies from nearest to farthest
// (dir > 0, tab) or farthest to nearest (dir < 0, shift+tab).
func (g *Game) cycleTarget(dir int) {
	hs := g.visibleHostiles()
	if len(hs) == 0 {
		g.Target = nil
		return
	}
	p := g.P
	sort.SliceStable(hs, func(i, j int) bool {
		return cheb(hs[i].X, hs[i].Y, p.X, p.Y) < cheb(hs[j].X, hs[j].Y, p.X, p.Y)
	})
	idx := -1
	for i, m := range hs {
		if m == g.Target {
			idx = i
		}
	}
	if dir < 0 {
		if idx < 0 {
			idx = 0
		}
		g.Target = hs[(idx-1+len(hs))%len(hs)]
		return
	}
	g.Target = hs[(idx+1)%len(hs)]
}

// targetAt makes the visible enemy on map cell (mx,my) the target, as a
// mouse click does. It reports whether one was there.
func (g *Game) targetAt(mx, my int) bool {
	if !g.Lv.In(mx, my) || !g.canSee(mx, my) {
		return false
	}
	m := g.Lv.MonsterAt(mx, my)
	if m == nil || m.Dead || m.Friendly {
		return false
	}
	g.Target = m
	return true
}

// ------------------------------------------------------------ turns

// drain takes this turn's share of a potion pool: most of it lands in the
// first few turns.
func drain(pool *float64) float64 {
	d := drainStep(*pool)
	*pool -= d
	return d
}

// drainStep is how much of a potion pool the next turn delivers; the
// bot plans with it too (Bot.hp).
func drainStep(pool float64) float64 { return math.Min(pool, pool*0.35+1) }

func (g *Game) endTurn() {
	p := g.P
	g.Turn++
	g.Stats.Turns[g.Lv.ID]++
	if g.stalker != nil {
		if g.stalkIn--; g.stalkIn <= 0 {
			g.stalkerArrives()
		}
	}
	// No natural regeneration: life and mana come back from potions,
	// shrines, Mirela, and regeneration affixes on gear.
	hp := float64(p.S(StLifeRegen))*0.08 + drain(&p.HealPool)
	mp := float64(p.S(StManaRegen))*0.08 + drain(&p.ManaPool)
	p.HP = math.Min(float64(p.MaxHP()), p.HP+hp)
	p.MP = math.Min(float64(p.MaxMP()), p.MP+mp)
	g.computeDist()
	for _, m := range g.Lv.Monsters {
		if m.Dead {
			continue
		}
		m.Energy += m.Speed
		for m.Energy >= 100 && !m.Dead && g.Mode != ModeDead {
			m.Energy -= 100
			g.monsterTurn(m)
		}
	}
	g.cleanup()
	g.computeVisibility()
	if g.Target != nil && (g.Target.Dead || !g.canSee(g.Target.X, g.Target.Y)) {
		g.Target = nil
	}
}

func (g *Game) cleanup() { g.cleanupLevel(g.Lv) }

// cleanupLevel drops the dead and those who left from a level.
func (g *Game) cleanupLevel(l *Level) {
	alive := l.Monsters[:0]
	for _, m := range l.Monsters {
		if !m.Dead && !m.Gone {
			alive = append(alive, m)
		}
	}
	l.Monsters = alive
}

func (g *Game) computeDist() {
	l := g.Lv
	for i := range g.dist {
		g.dist[i] = -1
	}
	start := l.Idx(g.P.X, g.P.Y)
	g.dist[start] = 0
	q := []int{start}
	for len(q) > 0 {
		c := q[0]
		q = q[1:]
		if g.dist[c] > 45 {
			continue
		}
		cx, cy := c%l.W, c/l.W
		for _, d := range dirs8 {
			nx, ny := cx+d.X, cy+d.Y
			if !l.In(nx, ny) {
				continue
			}
			ni := ny*l.W + nx
			if g.dist[ni] >= 0 || !l.Walkable(nx, ny) {
				continue
			}
			g.dist[ni] = g.dist[c] + 1
			q = append(q, ni)
		}
	}
}

// ------------------------------------------------------------ player actions

func (g *Game) move(dx, dy int) {
	l, p := g.Lv, g.P
	nx, ny := p.X+dx, p.Y+dy
	if !l.In(nx, ny) {
		return
	}
	if m := l.MonsterAt(nx, ny); m != nil {
		if m.Friendly {
			g.talkTo(m)
			return
		}
		g.Target = m
		g.meleeAttack(m)
		g.endTurn()
		return
	}
	switch l.At(nx, ny) {
	case TDoor:
		l.Set(nx, ny, TDoorOpen)
		l.LightVer++
		g.endTurn()
		return
	case TChest:
		g.openChest(nx, ny)
		g.endTurn()
		return
	case TFountain:
		// Life only: mana comes from Mirela or potions.
		p.HP = float64(p.MaxHP())
		g.msg(colCyan, "The cold water closes your wounds.")
		g.endTurn()
		return
	case TAltar:
		g.useAltar(nx, ny)
		g.endTurn()
		return
	}
	if !l.Walkable(nx, ny) {
		if g.auto {
			g.auto = false
		}
		return
	}
	p.X, p.Y = nx, ny
	g.autoPickup()
	if lk := l.LinkAt(nx, ny); lk != nil {
		from := l.ID
		g.changeLevel(lk.To, from, nil)
		return
	}
	if g.Rift != nil && g.Rift.Level == l.ID && nx == g.Rift.X && ny == g.Rift.Y {
		g.Rift = nil
		g.msg(colRed, "You step through the red portal after him.")
		g.changeLevel("town", "", &Pos{l2(g).PortalAt.X, l2(g).PortalAt.Y + 1})
		return
	}
	if g.Portal != nil {
		if g.Portal.Level == l.ID && nx == g.Portal.X && ny == g.Portal.Y {
			g.msg(colBlue, "You step through the portal.")
			g.changeLevel("town", "", &Pos{l2(g).PortalAt.X, l2(g).PortalAt.Y + 1})
			return
		}
		if l.Kind == KTown && nx == l.PortalAt.X && ny == l.PortalAt.Y {
			dest := g.Portal
			g.Portal = nil
			g.msg(colBlue, "The portal collapses behind you.")
			g.changeLevel(dest.Level, "", &Pos{dest.X, dest.Y})
			return
		}
	}
	if items := l.ItemsAt(nx, ny); len(items) > 0 && !g.auto {
		if len(items) == 1 {
			g.msg(colGray, "You see %s here. (g to pick up)", items[0].It.DisplayName())
		} else {
			g.msg(colGray, "Several items lie here. (g to pick up)")
		}
	}
	g.endTurn()
}

// portalAt says whether a cell is a portal mouth: the open portal on its
// level, or its town end.
func (g *Game) portalAt(x, y int) bool {
	l := g.Lv
	if g.Rift != nil && g.Rift.Level == l.ID && x == g.Rift.X && y == g.Rift.Y {
		return true
	}
	if g.Portal == nil {
		return false
	}
	return (g.Portal.Level == l.ID && x == g.Portal.X && y == g.Portal.Y) || (l.Kind == KTown && x == l.PortalAt.X && y == l.PortalAt.Y)
}

// l2 returns the town level (generating it if needed).
func l2(g *Game) *Level { return g.getLevel("town") }

func (g *Game) wait() { g.endTurn() }

func (g *Game) autoPickup() {
	l, p := g.Lv, g.P
	keep := l.Items[:0]
	for _, fi := range l.Items {
		if fi.X != p.X || fi.Y != p.Y {
			keep = append(keep, fi)
			continue
		}
		it := fi.It
		switch it.Kind {
		case IKGold:
			p.Gold += it.Amount
			g.Stats.In[it.Src] += it.Amount
			g.msg(colGold, "You pick up %d gold.", it.Amount)
		case IKHealth, IKMana:
			n, name, col := &p.HPot, "Healing", C(1, .4, .4)
			if it.Kind == IKMana {
				n, name, col = &p.MPot, "Mana", C(.45, .6, 1)
			}
			if !g.canTake(it) {
				g.msg(colDim, "Your belt has no room for another %s Potion.", name)
				keep = append(keep, fi)
				continue
			}
			*n++
			g.msg(col, "You pick up a %s Potion.", name)
		case IKScroll:
			p.Scrolls++
			g.msg(C(.85, .8, .65), "You pick up a Scroll of Town Portal.")
		default:
			keep = append(keep, fi)
		}
	}
	l.Items = keep
}

// canTake says whether auto-pickup would take an item: a full belt leaves
// potions on the ground.
func (g *Game) canTake(it *Item) bool {
	p := g.P
	switch it.Kind {
	case IKHealth:
		return p.HPot < beltMax
	case IKMana:
		return p.MPot < beltMax
	}
	return it.Kind != IKEquip
}

func (g *Game) pickup() {
	l, p := g.Lv, g.P
	got, full := false, false
	keep := l.Items[:0]
	for _, fi := range l.Items {
		if fi.X == p.X && fi.Y == p.Y && fi.It.Kind == IKEquip {
			if len(p.Inv) >= invMax {
				if !full {
					g.msg(colRed, "Your pack is full.")
				}
				full = true
				keep = append(keep, fi)
				continue
			}
			p.Inv = append(p.Inv, fi.It)
			g.msg(fi.It.Color(), "You pick up %s.", fi.It.Name)
			got = true
			continue
		}
		keep = append(keep, fi)
	}
	l.Items = keep
	if got {
		g.endTurn()
	} else if !full {
		g.msg(colDim, "There is nothing here to pick up.")
	}
}

func (g *Game) dropItem(x, y int, it *Item) {
	l := g.Lv
	fx, fy := x, y
	for r := range 5 {
		found := false
		for range 12 {
			cx, cy := x+g.rng.Intn(2*r+1)-r, y+g.rng.Intn(2*r+1)-r
			if l.Walkable(cx, cy) && l.LinkAt(cx, cy) == nil && len(l.ItemsAt(cx, cy)) == 0 && l.At(cx, cy) != TDoor {
				fx, fy, found = cx, cy, true
				break
			}
		}
		if found {
			break
		}
	}
	fi := &FloorItem{X: fx, Y: fy, It: it}
	if it.Kind == IKEquip && it.Rarity >= RRare {
		spec := &LightSpec{C(1, .95, .45), 2.4, .7, .05, .3}
		if it.Rarity == RUnique {
			spec = &LightSpec{C(1, .7, .3), 3.4, 1, .05, .35}
		}
		fi.Light = NewLight(fx, fy, spec, g.rng.Float32()*10)
	}
	l.Items = append(l.Items, fi)
}

func (g *Game) meleeAttack(m *Monster) {
	p := g.P
	m.Awake = true
	chance := clampi(p.ToHit()+10-m.Level*2-m.Armor/5, 30, 97)
	if g.rng.Intn(100) >= chance {
		return
	}
	lo, hi := p.DmgRange()
	dmg := lo + g.rng.Intn(hi-lo+1)
	crit := g.rng.Intn(100) < p.Crit()
	if crit {
		dmg *= 2
	}
	dmg = int(float64(dmg) * (1 - float64(m.Armor)/(float64(m.Armor)+g.Rules.MonArmorK)))
	if dmg < 1 {
		dmg = 1
	}
	if ls := p.S(StLifeSteal); ls > 0 {
		p.HP = math.Min(float64(p.MaxHP()), p.HP+float64(dmg*ls)/100)
	}
	g.damageMonster(m, dmg, crit, colWhite)
}

func (g *Game) damageMonster(m *Monster, dmg int, crit bool, col RGB) {
	m.HP -= dmg
	m.Flash = g.time
	m.Awake = true
	g.Stats.DmgDealt += dmg
	s := strconv.Itoa(dmg)
	if crit {
		s += "!"
		col = colYellow
	}
	g.textFx(m.X, m.Y, s, col)
	if m.HP <= 0 {
		g.killMonster(m)
	}
}

func (g *Game) killMonster(m *Monster) {
	l, p := g.Lv, g.P
	m.Dead = true
	p.Kills++
	g.Stats.Kills[m.Rank]++
	i := l.Idx(m.X, m.Y)
	if l.At(m.X, m.Y) != TWater && l.At(m.X, m.Y) != TDeepWater {
		l.Decal[i] = DecalCorpse
		for range 3 {
			bx, by := m.X+g.rng.Intn(3)-1, m.Y+g.rng.Intn(3)-1
			if l.In(bx, by) && l.Walkable(bx, by) && l.Decal[l.Idx(bx, by)] == DecalNone {
				l.Decal[l.Idx(bx, by)] = DecalBlood
			}
		}
	}
	switch m.Rank {
	case RankBoss:
		g.msg(colGold, "%s has been destroyed!", m.Name)
		g.novaFx(m.X, m.Y, C(1, .8, .4), 6)
	case RankUnique, RankChampion:
		g.msg(m.Color(), "%s is slain!", m.Name)
	default:
		g.msg(C(.6, .55, .5), "The %s dies.", m.Name)
	}
	if m.HasMod(ModFire) {
		g.novaFx(m.X, m.Y, colOrange, 2)
		if cheb(m.X, m.Y, p.X, p.Y) <= 1 {
			g.hitter = nil
			g.hurtPlayer(m.Level*2+3, m.Name+"'s death flames", "burn")
		}
	}
	switch m.T.ID {
	case "boneking":
		g.Quests[0] = maxi(g.Quests[0], 1)
		g.msg(colOrange, "The crypts fall silent. Return to Captain Voss.")
	case "oracle":
		g.Quests[1] = maxi(g.Quests[1], 1)
		g.msg(colOrange, "The Oracle's song ends. Deeper still, something burns... Return to Voss.")
	case "wanderer":
		g.relight()
		g.Quests[2] = 1
		g.Mode = ModeChoice
		g.msg(m.T.Color, "The Last Wanderer: \"Keep walking.\"")
		g.msg(colLore, "The red light goes out, and the dark lets go of the fire.")
		g.unseal(l)
	}
	if m.T.ID == "fallen" || m.T.ID == "shaman" {
		// the Fallen are cowards: seeing kin die sends them running
		for _, o := range l.Monsters {
			if !o.Dead && (o.T.ID == "fallen") && cheb(o.X, o.Y, m.X, m.Y) <= 6 && g.rng.Intn(3) != 0 {
				o.Flee = 3 + g.rng.Intn(3)
			}
		}
	}
	g.gainXP(m.XP)
	g.dropLoot(m)
}

func (g *Game) gainXP(xp int) {
	p := g.P
	p.XP += xp
	for p.XP >= g.Rules.xpNext(p.Lvl) {
		p.XP -= g.Rules.xpNext(p.Lvl)
		p.Lvl++
		p.Points += 5
		p.recalc()
		p.HP, p.MP = float64(p.MaxHP()), float64(p.MaxMP())
		g.msg(colGold, "You have reached level %d! (c to spend 5 attribute points)", p.Lvl)
		g.novaFx(p.X, p.Y, colGold, 4)
	}
}

func (g *Game) dropLoot(m *Monster) {
	p, r := g.P, g.Rules
	mf := float64(p.S(StMF)) / 100
	nItems, bonus, minR := 0, mf, RNormal
	if g.rng.Intn(100) < int(r.DropChance) {
		nItems = 1
	}
	goldChance := int(r.GoldChance)
	switch m.Rank {
	case RankChampion:
		nItems, bonus, minR, goldChance = 1, mf+1, RMagic, 70
	case RankUnique:
		nItems, bonus, minR, goldChance = 2+g.rng.Intn(2), mf+2.5, RMagic, 100
	case RankBoss:
		nItems, bonus, minR, goldChance = 5, mf+4, RRare, 100
	}
	if m.Minion {
		nItems, goldChance = 0, 10
	}
	for i := range nItems {
		rar := RollRarity(g.rng, bonus)
		if rar < minR {
			rar = minR
		}
		if m.Rank == RankBoss && i == 0 {
			if m.T.ID == "wanderer" {
				g.dropItem(m.X, m.Y, uniqueItem(lastShroud, m.Level))
				continue
			}
			rar = RUnique
		}
		if rar == RNormal && m.Rank == RankNormal && g.rng.Intn(2) == 0 {
			continue // plain monsters mostly drop nothing worth a look
		}
		g.dropItem(m.X, m.Y, GenItem(g.rng, m.Level, rar, SlotNone, r))
	}
	if g.rng.Intn(100) < goldChance {
		base := int(float64(m.Level) * r.GoldPerLvl)
		amt := base + g.rng.Intn(2*base+6)
		if m.Rank >= RankChampion {
			amt *= 2
		}
		amt = amt * (100 + p.S(StGoldFind)) / 100
		g.dropItem(m.X, m.Y, &Item{Kind: IKGold, Amount: amt, Src: GoldDrop})
	}
	switch x := g.rng.Intn(100); {
	case x < 5:
		g.dropItem(m.X, m.Y, NewPotion(IKHealth))
	case x < 8:
		g.dropItem(m.X, m.Y, NewPotion(IKMana))
	case x < 10:
		g.dropItem(m.X, m.Y, NewPotion(IKScroll))
	}
}

func (g *Game) openChest(x, y int) {
	l := g.Lv
	l.Set(x, y, TChestOpen)
	g.msg(C(.9, .7, .35), "You open the chest.")
	lvl := maxi(1, l.Depth)
	n := 1 + g.rng.Intn(2)
	for range n {
		r := RollRarity(g.rng, 0.8+float64(g.P.S(StMF))/100)
		if r == RNormal {
			r = RMagic
		}
		g.dropItem(x, y, GenItem(g.rng, lvl, r, SlotNone, g.Rules))
	}
	g.dropItem(x, y, &Item{Kind: IKGold, Amount: lvl*10 + g.rng.Intn(lvl*15+10), Src: GoldChest})
	if g.rng.Intn(2) == 0 {
		g.dropItem(x, y, NewPotion(IKHealth))
	}
}

func (g *Game) useAltar(x, y int) {
	key := fmt.Sprintf("%s:%d:%d", g.Lv.ID, x, y)
	if g.usedAltars[key] {
		g.msg(colDim, "The altar is cold now.")
		return
	}
	g.usedAltars[key] = true
	p := g.P
	switch g.rng.Intn(3) {
	case 0:
		p.MP = float64(p.MaxMP())
		p.HP = float64(p.MaxHP())
		g.msg(colPurple, "Blood-light washes over you. You are restored.")
	case 1:
		xp := g.Rules.xpNext(p.Lvl) / 4
		g.msg(colPurple, "Forbidden knowledge floods your mind. (+%d XP)", xp)
		g.gainXP(xp)
	default:
		g.msg(colPurple, "The altar offers up a gift.")
		g.dropItem(x, y+1, GenItem(g.rng, g.Lv.Depth+1, RRare, SlotNone, g.Rules))
	}
	g.novaFx(x, y, colPurple, 3)
}

// hurtPlayer applies damage and logs "<By> <verb> you for N." (by doubles as the killer's name).
func (g *Game) hurtPlayer(dmg int, by, verb string) {
	p := g.P
	if g.Mode == ModeDead {
		return // the blow that killed is the one on record
	}
	if dmg < 1 {
		dmg = 1
	}
	if verb == "" {
		verb = "hits"
	}
	g.msg(C(.9, .45, .4), "%s %s you for %d.", titleWord(by), verb, dmg)
	p.HP -= float64(dmg)
	g.Stats.DmgTaken += dmg
	p.Flash = g.time
	g.auto = false
	g.textFx(p.X, p.Y, strconv.Itoa(dmg), colRed)
	if p.HP <= 0 {
		p.HP = 0
		p.KilledBy = by
		g.Mode = ModeDead
		g.msg(colRed, "You have been slain by %s.", by)
		g.recordDeath()
	}
}

// recordDeath notes how the hero stood when the blow landed.
func (g *Game) recordDeath() {
	d := &g.Stats.Death
	d.Rank, d.Adj, d.Awake, d.Potions, d.Scrolls = -1, 0, 0, g.P.HPot, g.P.Scrolls
	if m := g.hitter; m != nil {
		d.Rank = m.Rank
	}
	for _, m := range g.visibleHostiles() {
		if cheb(m.X, m.Y, g.P.X, g.P.Y) == 1 {
			d.Adj++
		}
		if m.Awake {
			d.Awake++
		}
	}
}

func (g *Game) drinkHealth() {
	p := g.P
	if p.HPot <= 0 {
		g.msg(colDim, "You have no healing potions.")
		return
	}
	if p.HP+p.HealPool >= float64(p.MaxHP()) {
		g.msg(colDim, "You are already at full life.")
		return
	}
	p.HPot--
	g.Stats.HPots++
	amt := p.HealAmt()
	p.HealPool += amt
	g.textFx(p.X, p.Y, "+"+strconv.Itoa(int(amt)), colGreen)
	g.msg(C(1, .45, .45), "You drink a healing potion.")
	g.endTurn()
}

func (g *Game) drinkMana() {
	p := g.P
	if p.MPot <= 0 {
		g.msg(colDim, "You have no mana potions.")
		return
	}
	if p.MP+p.ManaPool >= float64(p.MaxMP()) {
		g.msg(colDim, "Your mana is already full.")
		return
	}
	p.MPot--
	g.Stats.MPots++
	p.ManaPool += p.ManaAmt()
	g.msg(C(.45, .6, 1), "You drink a mana potion.")
	g.endTurn()
}

func (g *Game) readPortal() {
	l, p := g.Lv, g.P
	if l.Kind == KTown {
		g.msg(colDim, "You are already in town.")
		return
	}
	if p.Scrolls <= 0 {
		g.msg(colDim, "You have no Scrolls of Town Portal.")
		return
	}
	p.Scrolls--
	// The portal opens on the nearest free cell: in a crowd, the one
	// side that is still open, not the far side of whoever stands east.
	x, y := l.FreeNear(p.X, p.Y, p.X, p.Y)
	g.Portal = &Portal{l.ID, x, y}
	g.portalLight.ver = -1
	g.msg(colBlue, "A shimmering blue portal tears open.")
	g.novaFx(x, y, colBlue, 3)
	g.endTurn()
}

// ------------------------------------------------------------ spells

// lineCells returns the Bresenham line from a to b (excluding a).
func lineCells(x0, y0, x1, y1 int) []Pos {
	var pts []Pos
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	x, y := x0, y0
	for range 200 {
		if x == x1 && y == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x += sx
		}
		if e2 <= dx {
			err += dx
			y += sy
		}
		pts = append(pts, Pos{x, y})
	}
	return pts
}

// traceBolt follows a line until it hits a wall or a creature.
func (g *Game) traceBolt(x0, y0, x1, y1 int, maxLen int, fromPlayer bool) ([]Pos, *Monster, bool) {
	l := g.Lv
	pts := lineCells(x0, y0, x1, y1)
	// extend past target
	if len(pts) > 0 {
		dx, dy := x1-x0, y1-y0
		for k := 2; len(pts) < maxLen && k < 6; k++ {
			ext := lineCells(x0, y0, x0+dx*k, y0+dy*k)
			if len(ext) > len(pts) {
				pts = ext
			}
		}
	}
	var path []Pos
	for i, pt := range pts {
		if i >= maxLen || !l.In(pt.X, pt.Y) {
			break
		}
		path = append(path, pt)
		if l.Opaque(pt.X, pt.Y) || (!l.Walkable(pt.X, pt.Y) && tdefs[l.At(pt.X, pt.Y)].Solid) {
			return path, nil, false
		}
		if fromPlayer {
			if m := l.MonsterAt(pt.X, pt.Y); m != nil && !m.Friendly {
				return path, m, false
			}
		} else if pt.X == g.P.X && pt.Y == g.P.Y {
			return path, nil, true
		}
	}
	return path, nil, false
}

func (g *Game) castFirebolt() {
	p := g.P
	t := g.validTarget()
	if t == nil {
		g.msg(colDim, "No target in sight.")
		return
	}
	if cheb(p.X, p.Y, t.X, t.Y) > fireboltRange {
		g.msg(colDim, "The %s is out of range.", t.Name)
		return
	}
	if p.MP < float64(p.FireboltCost()) {
		g.msg(C(.45, .6, 1), "Not enough mana.")
		return
	}
	p.MP -= float64(p.FireboltCost())
	g.Stats.Bolts++
	path, hit, _ := g.traceBolt(p.X, p.Y, t.X, t.Y, fireboltRange, true)
	g.boltFx(path, C(1, .5, .15), '*', true)
	if hit != nil && g.rng.Intn(100) < hit.T.Dodge {
		hit.Awake = true // sidestepped, and now it knows
	} else if hit != nil {
		lo, hi := p.FireboltDmg()
		dmg := lo + g.rng.Intn(hi-lo+1)
		crit := g.rng.Intn(100) < p.Crit()/2
		if crit {
			dmg = dmg * 3 / 2
		}
		g.damageMonster(hit, dmg, crit, colOrange)
		if hit.Dead {
			g.Lv.Decal[g.Lv.Idx(hit.X, hit.Y)] = DecalScorch
		}
	}
	g.endTurn()
}

func (g *Game) castNova() {
	p, l := g.P, g.Lv
	if p.MP < float64(p.NovaCost()) {
		g.msg(C(.45, .6, 1), "Not enough mana.")
		return
	}
	p.MP -= float64(p.NovaCost())
	g.Stats.Novas++
	g.novaFx(p.X, p.Y, C(.5, .85, 1), 3.4)
	lo, hi := p.NovaDmg()
	for _, m := range l.Monsters {
		if m.Dead || m.Friendly {
			continue
		}
		dx, dy := float64(m.X-p.X)/Aspect, float64(m.Y-p.Y)
		if math.Sqrt(dx*dx+dy*dy) > 3.4 || g.fov[l.Idx(m.X, m.Y)] != g.fovGen {
			continue
		}
		fr := novaFreeze // champions shake it off sooner; uniques and bosses never freeze
		switch {
		case m.Rank >= RankUnique:
			fr = 0
		case m.Rank == RankChampion:
			fr--
		}
		if fr > 0 && m.FreezeCD == 0 {
			m.Frozen = maxi(m.Frozen, fr)
			m.FreezeCD = fr + freezeImmune
		}
		g.damageMonster(m, lo+g.rng.Intn(hi-lo+1), false, colCyan)
	}
	g.endTurn()
}

// ------------------------------------------------------------ monsters

func (g *Game) monsterTurn(m *Monster) {
	l, p := g.Lv, g.P
	if m.Friendly {
		if w := g.hunter(); w != nil && cheb(m.X, m.Y, w.X, w.Y) <= 8 {
			// panicked: they stumble away two turns in three
			if g.rng.Intn(3) != 0 {
				g.fleeFrom(m, w.X, w.Y)
			}
			return
		}
		if g.rng.Intn(8) == 0 {
			d := dirs8[g.rng.Intn(8)]
			nx, ny := m.X+d.X, m.Y+d.Y
			if cheb(nx, ny, m.HomeX, m.HomeY) <= 3 {
				g.stepMonster(m, nx, ny)
			}
		}
		return
	}
	if m.FreezeCD > 0 {
		m.FreezeCD--
	}
	if m.Frozen > 0 {
		m.Frozen--
		return
	}
	d := cheb(m.X, m.Y, p.X, p.Y)
	sees := g.fov[l.Idx(m.X, m.Y)] == g.fovGen && d <= int(g.Rules.WakeRange)
	if sees {
		if !m.Awake {
			m.Awake = true
			for _, o := range l.Monsters {
				if !o.Dead && !o.Friendly && cheb(o.X, o.Y, m.X, m.Y) <= 6 {
					o.Awake = true
				}
			}
			if m.Rank == RankBoss {
				g.bossTaunt(m)
			}
		}
		m.LostTurns = 0
	} else if m.Awake && m.T.AI == AIWanderer && l.Kind == KTown {
		m.LostTurns = 0 // loose in Emberhold he never stops hunting
	} else if m.Awake {
		m.LostTurns++
		if m.LostTurns > 30 {
			m.Awake = false
		}
	}
	if m.Flee > 0 && m.Awake {
		m.Flee--
		g.stepAway(m)
		return
	}
	if !m.Awake {
		if g.rng.Intn(7) == 0 {
			dd := dirs8[g.rng.Intn(8)]
			if cheb(m.X+dd.X, m.Y+dd.Y, m.HomeX, m.HomeY) <= 5 {
				g.stepMonster(m, m.X+dd.X, m.Y+dd.Y)
			}
		}
		return
	}
	if m.T.Erratic > 0 && g.rng.Intn(100) < m.T.Erratic {
		dd := dirs8[g.rng.Intn(8)]
		g.stepMonster(m, m.X+dd.X, m.Y+dd.Y)
		return
	}
	m.Timer++
	switch m.T.AI {
	case AIBoneKing:
		if sees && m.Timer%6 == 0 && g.countMinions() < 10 {
			g.msg(C(.75, 1, .6), "The Bone King raises the dead!")
			for range 2 {
				s := placeMonsterAvoid(l, "skel", m.X, m.Y, m.Level, RankNormal, p.X, p.Y)
				s.Awake, s.Minion = true, true
			}
			g.novaFx(m.X, m.Y, C(.5, 1, .45), 2.5)
			return
		}
	case AIWanderer:
		if float64(m.HP) <= g.Rules.WandererRift*float64(m.MaxHP) && l.SealedDown != "" && g.Rift == nil {
			g.openRift(m)
			return
		}
		if l.Kind == KTown {
			if prey := g.prey(m); prey != nil {
				g.huntStep(m, prey)
				return
			}
		}
		// he closes in, throwing fire now and then; he never backs away
		if sees && d > 1 && d <= m.T.Range && g.rng.Intn(100) < 35 {
			if _, _, hitsP := g.traceBolt(m.X, m.Y, p.X, p.Y, m.T.Range+2, false); hitsP {
				g.monsterShoot(m)
				return
			}
		}
	case AIOracle:
		if sees && m.Timer%9 == 0 {
			for range 30 {
				nx, ny := p.X+g.rng.Intn(13)-6, p.Y+g.rng.Intn(7)-3
				if l.Walkable(nx, ny) && l.MonsterAt(nx, ny) == nil && cheb(nx, ny, p.X, p.Y) >= 3 && g.fov[l.Idx(nx, ny)] == g.fovGen {
					g.novaFx(m.X, m.Y, colCyan, 2)
					m.X, m.Y = nx, ny
					g.msg(colCyan, "The Oracle dissolves into mist and reforms.")
					return
				}
			}
		}
		if sees && m.Timer%13 == 0 && g.countMinions() < 8 {
			g.msg(colCyan, "Cold lights gather around the Oracle.")
			for range 2 {
				w := placeMonsterAvoid(l, "wisp", m.X, m.Y, m.Level-1, RankNormal, p.X, p.Y)
				w.Awake, w.Minion = true, true
			}
			return
		}
	}
	if d == 1 {
		g.monsterMelee(m)
		return
	}
	ranged := m.T.AI == AIRanged || m.T.AI == AIOracle
	if ranged && sees && d <= m.T.Range && g.rng.Intn(100) < 70 {
		if _, _, hitsP := g.traceBolt(m.X, m.Y, p.X, p.Y, m.T.Range+2, false); hitsP {
			g.monsterShoot(m)
			return
		}
	}
	if ranged && d <= 2 && g.rng.Intn(2) == 0 {
		g.stepAway(m)
		return
	}
	g.stepToward(m)
}

func (g *Game) countMinions() int {
	n := 0
	for _, m := range g.Lv.Monsters {
		if m.Minion && !m.Dead {
			n++
		}
	}
	return n
}

func (g *Game) bossTaunt(m *Monster) {
	switch m.T.ID {
	case "boneking":
		g.msg(C(.8, 1, .6), "The Bone King: \"Another torch to snuff. Kneel, and join my court.\"")
	case "oracle":
		g.msg(colCyan, "The Drowned Oracle: \"I have seen your ending, little flame. It is wet and cold.\"")
	case "wanderer":
		g.msg(m.T.Color, "The Last Wanderer: \"Another stranger. They always send a stranger.\"")
	}
}

func (g *Game) stepMonster(m *Monster, nx, ny int) bool {
	l, p := g.Lv, g.P
	if !l.In(nx, ny) || (nx == p.X && ny == p.Y) || l.MonsterAt(nx, ny) != nil {
		return false
	}
	if l.At(nx, ny) == TDoor {
		if m.Friendly {
			return false
		}
		l.Set(nx, ny, TDoorOpen)
		l.LightVer++
		return true
	}
	if !l.Walkable(nx, ny) || l.LinkAt(nx, ny) != nil {
		return false
	}
	m.X, m.Y = nx, ny
	return true
}

func (g *Game) stepToward(m *Monster) {
	l, p := g.Lv, g.P
	cur := g.dist[l.Idx(m.X, m.Y)]
	if cur < 0 {
		// no path: greedy approach
		dx, dy := sign(p.X-m.X), sign(p.Y-m.Y)
		if !g.stepMonster(m, m.X+dx, m.Y+dy) {
			if !g.stepMonster(m, m.X+dx, m.Y) {
				g.stepMonster(m, m.X, m.Y+dy)
			}
		}
		return
	}
	best := []Pos{}
	bd := cur
	for _, d := range dirs8 {
		nx, ny := m.X+d.X, m.Y+d.Y
		if !l.In(nx, ny) {
			continue
		}
		v := g.dist[l.Idx(nx, ny)]
		if v < 0 || l.MonsterAt(nx, ny) != nil {
			continue
		}
		if v < bd {
			bd = v
			best = best[:0]
			best = append(best, Pos{nx, ny})
		} else if v == bd && v < cur {
			best = append(best, Pos{nx, ny})
		}
	}
	if len(best) == 0 {
		// crowded: sidestep to an equal-distance cell
		for _, d := range dirs8 {
			nx, ny := m.X+d.X, m.Y+d.Y
			if l.In(nx, ny) && g.dist[l.Idx(nx, ny)] == cur && g.rng.Intn(3) == 0 {
				g.stepMonster(m, nx, ny)
				return
			}
		}
		return
	}
	b := best[g.rng.Intn(len(best))]
	g.stepMonster(m, b.X, b.Y)
}

func (g *Game) stepAway(m *Monster) {
	l := g.Lv
	cur := g.dist[l.Idx(m.X, m.Y)]
	for _, d := range dirs8 {
		nx, ny := m.X+d.X, m.Y+d.Y
		if l.In(nx, ny) && g.dist[l.Idx(nx, ny)] > cur {
			if g.stepMonster(m, nx, ny) {
				return
			}
		}
	}
	g.stepToward(m)
}

func sign(v int) int {
	if v > 0 {
		return 1
	}
	if v < 0 {
		return -1
	}
	return 0
}

// Armor is one thing: it reduces the damage that lands, dexterity makes
// the blow miss, and nothing does both.
func (g *Game) monsterHitChance(m *Monster) int {
	p := g.P
	return clampi(m.ToHit-p.DEX()/4+10, 20, 95)
}

// monsterDamage rolls the monster's blow and takes off the armor's share,
// at most half.
func (g *Game) monsterDamage(m *Monster) int {
	p, r := g.P, g.Rules
	dmg := m.MinD + g.rng.Intn(m.MaxD-m.MinD+1)
	a := float64(p.ArmorVal())
	red := math.Min(r.ArmorCap, a/(a+r.ArmorK+r.ArmorPerLvl*float64(m.Level)))
	return maxi(1, int(float64(dmg)*(1-red)+0.5))
}

func (g *Game) monsterMelee(m *Monster) {
	p := g.P
	if g.rng.Intn(100) >= g.monsterHitChance(m) {
		return
	}
	dmg := g.monsterDamage(m)
	if m.HasMod(ModFire) {
		dmg += 1 + m.Level/2
	}
	if m.HasMod(ModVampiric) {
		m.HP = mini(m.MaxHP, m.HP+dmg/2+1)
	}
	name := "the " + m.Name
	if m.Rank >= RankUnique {
		name = m.Name
	}
	g.hitter = m
	g.hurtPlayer(dmg, name, m.T.Verb)
	if th := p.S(StThorns); th > 0 && !m.Dead {
		g.damageMonster(m, th, false, C(.8, .6, .4))
	}
}

func (g *Game) monsterShoot(m *Monster) {
	p := g.P
	path, _, hitsP := g.traceBolt(m.X, m.Y, p.X, p.Y, m.T.Range+2, false)
	g.boltFx(path, m.T.ProjColor, m.T.ProjGlyph, m.T.ProjLight)
	if !hitsP {
		return
	}
	if g.rng.Intn(100) >= g.monsterHitChance(m)+5 {
		return
	}
	name := "the " + m.Name
	if m.Rank >= RankUnique {
		name = m.Name
	}
	g.hitter = m
	g.hurtPlayer(g.monsterDamage(m)*4/5+1, name, m.T.Verb)
}

// ------------------------------------------------------------ auto-explore

const (
	autoStopNear  = 10 // any enemy this close stops auto-explore
	autoStopAware = 18 // ...as does one this close that has noticed you
)

func monsterRef(m *Monster) string {
	if m.Rank >= RankUnique {
		return m.Name
	}
	if strings.ContainsRune("AEIOU", rune(m.Name[0])) {
		return "an " + m.Name
	}
	return "a " + m.Name
}

func (g *Game) autoStep() bool {
	l, p := g.Lv, g.P
	// Distant, unaware enemies (common on the open fields) don't interrupt.
	for _, m := range g.visibleHostiles() {
		d := cheb(m.X, m.Y, p.X, p.Y)
		if d <= autoStopNear || (m.Awake && d <= autoStopAware) {
			g.msg(colOrange, "You spot %s.", monsterRef(m))
			return false
		}
	}
	nItems := 0
	for _, it := range l.Items {
		if g.canSee(it.X, it.Y) {
			nItems++
		}
	}
	if nItems > g.autoItems {
		g.autoItems = nItems
		g.msg(colGray, "You notice something on the ground.")
		return false
	}
	g.autoItems = nItems
	// Prefer walking to visible loot that auto-pickup would grab.
	W := l.W
	par := make([]int32, l.W*l.H)
	for i := range par {
		par[i] = -1
	}
	start := l.Idx(p.X, p.Y)
	par[start] = int32(start)
	q := []int{start}
	target := -1
	for len(q) > 0 && target < 0 {
		c := q[0]
		q = q[1:]
		cx, cy := c%W, c/W
		if c != start {
			for _, fi := range l.Items {
				if fi.X == cx && fi.Y == cy && g.canTake(fi.It) {
					target = c
				}
			}
			for _, d := range dirs8 {
				nx, ny := cx+d.X, cy+d.Y
				if l.In(nx, ny) && !l.Seen[l.Idx(nx, ny)] {
					target = c
					break
				}
			}
			if target >= 0 {
				break
			}
		}
		for _, d := range dirs8 {
			nx, ny := cx+d.X, cy+d.Y
			if !l.In(nx, ny) {
				continue
			}
			ni := ny*W + nx
			if par[ni] >= 0 || !l.Seen[ni] || !l.Walkable(nx, ny) || l.LinkAt(nx, ny) != nil {
				continue
			}
			if l.At(nx, ny) == TChest {
				continue
			}
			if g.portalAt(nx, ny) {
				continue
			}
			par[ni] = int32(c)
			q = append(q, ni)
		}
	}
	if target < 0 {
		g.msg(colGray, "Nothing left to explore here.")
		return false
	}
	c := target
	for int(par[c]) != start {
		c = int(par[c])
	}
	nx, ny := c%W, c/W
	if l.MonsterAt(nx, ny) != nil {
		return false
	}
	lvl := g.Lv
	g.move(nx-p.X, ny-p.Y)
	return g.Lv == lvl && g.Mode == ModePlay
}

// ------------------------------------------------------------ town

// restock rolls the shops' stock at the deepest depth the hero has seen:
// the shop follows the descent, not the level count, and only changes
// when the descent does, or when Hadrik is paid to.
func (g *Game) restock() {
	g.stocked = g.Deepest
	smith := &Shop{Name: "Hadrik's Forge", Kind: 0}
	for range 11 {
		var r Rarity
		switch x := g.rng.Intn(100); {
		case x < 30:
			r = RNormal
		case x < 85:
			r = RMagic
		default:
			r = RRare
		}
		slots := []Slot{SlotWeapon, SlotWeapon, SlotOffhand, SlotArmor, SlotHelm, SlotGloves, SlotBoots}
		smith.Items = append(smith.Items, GenItem(g.rng, g.Deepest+g.rng.Intn(3), r, slots[g.rng.Intn(len(slots))], g.Rules))
	}
	for _, b := range gambleBases {
		smith.Services = append(smith.Services, &Item{Kind: IKGamble, Base: b, Name: "Unidentified " + b.Name})
	}
	smith.Services = append(smith.Services, &Item{Kind: IKReroll, Name: "Fresh stock"})
	alch := &Shop{Name: "Mirela's Remedies", Kind: 1}
	alch.Items = append(alch.Items, NewPotion(IKHealth), NewPotion(IKMana), NewPotion(IKScroll))
	for range 6 {
		slots := []Slot{SlotRing, SlotAmulet, SlotRing, SlotHelm}
		r := RMagic
		if g.rng.Intn(6) == 0 {
			r = RRare
		}
		alch.Items = append(alch.Items, GenItem(g.rng, g.Deepest+g.rng.Intn(3), r, slots[g.rng.Intn(len(slots))], g.Rules))
	}
	g.shops = [2]*Shop{smith, alch}
}

// hearthFloor is the Abyss floor where the Last Wanderer waits.
const hearthFloor = 3

// questDepth is where each quest's boss sits: crypt4 and grotto3.
var questDepth = [2]int{5, 9}

var villagerLines = []string{
	"They say the braziers in the crypt light themselves at dusk. Nobody tends them.",
	"My brother went into Blackmarsh after the lights. He came back wet. He came back wrong.",
	"Keep your torch high. The dark down there isn't empty — it's hungry.",
	"Hadrik's forge hasn't gone out in forty years. He says the day it does, we run.",
	"Rare things glimmer in the dark. Gold-light means something old and named.",
	"Old Mirela's crystal was dug out of the grotto. It sings when the Oracle dreams.",
	"A portal scroll is cheaper than a funeral.",
	"There was a stranger before you. Bought the same sword. Went past the marsh and kept going.",
	"King Edran carried the Ember up in his bare hands. It cost him his face.",
	"My uncle joined the Ember Cult. Said the dark was owed. He went down to pay it.",
	"Mirela says the Oracle isn't hunting anyone. She's holding something down.",
}

func (g *Game) talkTo(m *Monster) {
	p := g.P
	if g.townHunted() {
		who := m.Name
		if m.T.ID == "villager" {
			who = "The villager"
		}
		g.msg(m.T.Color, "%s is running for their life.", who)
		return
	}
	switch m.T.ID {
	case "smith":
		g.shop = g.shops[0]
		g.Mode, g.cur, g.tab = ModeShop, 0, 0
		line := "Steel for the dark. Take a look."
		if g.Quests[1] > 0 {
			line = "Last one I armed for the deep never came back for repairs."
		}
		g.msg(C(1, .6, .3), "Hadrik: \"%s\"", line)
	case "alch":
		g.mirelaHeal()
		g.shop = g.shops[1]
		g.Mode, g.cur, g.tab = ModeShop, 0, 0
	case "captain":
		lines := []string{}
		var rewards []int
		if g.Quests[0] == 1 {
			g.Quests[0] = 2
			rewards = append(rewards, 0)
			lines = append(lines, "The Bone King is dust? Then the crypt bells may ring again.", "Take this — it was my father's. And the gold, of course.")
		}
		if g.Quests[1] == 1 {
			g.Quests[1] = 2
			rewards = append(rewards, 1)
			lines = append(lines, "The Oracle, silenced... I never thought I'd sleep without hearing her.", "Beneath her pool, the rock burns. The Abyss has no floor, they say. Be careful.", "I sent someone down there once. I stopped counting the days.")
		}
		if len(rewards) == 0 {
			switch {
			case g.Quests[0] == 0:
				lines = []string{
					"Wanderer. Good. We need a blade that doesn't shake.",
					"The Ember keeps the dark out. The dark wants it back.",
					"East of the gate, past the Ashen Fields, lies the Crypt of the Fallen.",
					"Four floors down, the Bone King sits his throne of ribs. Destroy him.",
					"And north, in Blackmarsh, the Drowned Oracle calls the lost into the water.",
					"Buy potions from Mirela. Keep a portal scroll. Come back alive.",
				}
			case g.Quests[1] == 0:
				lines = []string{"The crypt is quiet. Now: Blackmarsh, north of the fields.", "Follow the cold lights down into the grotto. End the Oracle."}
			case g.Quests[2] > 0:
				lines = []string{"The lanterns are lit again. Whoever he was, he was one of ours once.", "Keep walking, stranger. It is what we hire you for."}
			default:
				lines = []string{"You've done more than any of us dared.", "If you must go deeper... the Abyss waits below the Oracle's pool."}
			}
		}
		for _, q := range rewards {
			// The unique follows the quest's depth, not the hero's level.
			gold := int(g.Rules.QuestGoldPerLvl * float64(p.Lvl))
			p.Gold += gold
			g.Stats.In[GoldQuest] += gold
			it := GenItem(g.rng, questDepth[q]+2, RUnique, SlotNone, g.Rules)
			g.dropItem(p.X, p.Y, it)
			g.msg(colGold, "Voss gives you %d gold and %s.", gold, it.Name)
		}
		g.talkName, g.talkLines, g.talkCol, g.Mode = m.Name, lines, m.T.Color, ModeTalk
	default:
		g.talkName = m.Name
		g.talkLines = []string{villagerLines[g.rng.Intn(len(villagerLines))]}
		g.talkCol = m.T.Color
		g.Mode = ModeTalk
	}
}

func (g *Game) buy(it *Item) {
	p := g.P
	price := g.buyPrice(it)
	if p.Gold < price {
		g.msg(colRed, "You cannot afford that.")
		return
	}
	switch it.Kind {
	case IKHealth, IKMana:
		n := &p.HPot
		if it.Kind == IKMana {
			n = &p.MPot
		}
		if *n >= beltMax {
			g.msg(colRed, "Your belt is full.")
			return
		}
		*n++
	case IKScroll:
		p.Scrolls++
	case IKGamble:
		if len(p.Inv) >= invMax {
			g.msg(colRed, "Your pack is full.")
			return
		}
		r := g.Rules
		got := GenItem(g.rng, g.Deepest+g.rng.Intn(3), RollRarity(g.rng, r.GambleBonus), it.Slot(), r)
		p.Inv = append(p.Inv, got)
		p.Gold -= price
		g.Stats.Out[SinkGamble] += price
		g.msg(got.Color(), "Hadrik unwraps %s. %dg, no refunds.", got.Name, price)
		return
	case IKReroll:
		p.Gold -= price
		g.Stats.Out[SinkReroll] += price
		g.restock()
		g.shop = g.shops[0]
		g.msg(colGold, "Hadrik hauls out fresh stock for %dg.", price)
		return
	default:
		if len(p.Inv) >= invMax {
			g.msg(colRed, "Your pack is full.")
			return
		}
		p.Inv = append(p.Inv, it)
		if i := slices.Index(g.shop.Items, it); i >= 0 {
			g.shop.Items = slices.Delete(g.shop.Items, i, i+1)
		}
	}
	p.Gold -= price
	g.Stats.Out[sinkOf(it)] += price
	g.msg(colGold, "Bought %s for %dg.", it.DisplayName(), price)
}

// Mirela heals for free until the hero is past freeHealLvl.
const freeHealLvl = 3

// mirelaHeal restores life, then mana, as far as the hero can pay, at
// HealCost a point.
func (g *Game) mirelaHeal() {
	p, healCost := g.P, g.Rules.HealCost
	need := float64(p.MaxHP()) - p.HP + float64(p.MaxMP()) - p.MP
	if need < 1 {
		g.msg(colCyan, "Mirela looks you over. \"Hale as an ox. Buy something, dear.\"")
		return
	}
	if p.Lvl <= freeHealLvl {
		p.HP, p.MP = float64(p.MaxHP()), float64(p.MaxMP())
		g.msg(colCyan, "Mirela tends your wounds. \"No charge for the young. Now buy something, dear.\"")
		return
	}
	pts := math.Min(need, float64(p.Gold)/healCost)
	if pts < 1 {
		g.msg(colCyan, "Mirela shakes her head. \"Herbs cost coin, dear.\"")
		return
	}
	cost := int(math.Ceil(pts * healCost))
	p.Gold -= cost
	g.Stats.Out[SinkHeal] += cost
	hp := math.Min(pts, float64(p.MaxHP())-p.HP)
	p.HP += hp
	p.MP = math.Min(float64(p.MaxMP()), p.MP+pts-hp)
	if pts < need {
		g.msg(colCyan, "Mirela does what your %dg allows. \"Come back with more, dear.\"", cost)
		return
	}
	g.msg(colCyan, "Mirela tends your wounds for %dg. \"There. Now buy something, dear.\"", cost)
}

// buyPrice: remedies cost more as the hero grows, so a stack of potions
// stays a real expense; Hadrik's services cost more the deeper the hero
// has been.
func (g *Game) buyPrice(it *Item) int {
	lvl, deep, r := float64(g.P.Lvl-1), float64(g.Deepest), g.Rules
	switch it.Kind {
	case IKHealth, IKMana:
		return int(r.PotionPrice + r.PotionPricePerLvl*lvl)
	case IKScroll:
		return int(r.ScrollPrice + r.ScrollPricePerLvl*lvl)
	case IKGamble:
		return int(r.GamblePrice + r.GamblePerDepth*deep)
	case IKReroll:
		return int(r.RerollPrice + r.RerollPerDepth*deep)
	}
	return it.Value()
}

// sellPrice: merchants pay little for gear, next to nothing for plain gear.
func sellPrice(it *Item, r *Rules) int {
	if it.Rarity == RNormal {
		return maxi(1, int(float64(it.Value())/(r.SellDiv*5/3)))
	}
	return int(float64(it.Value()) / r.SellDiv)
}

func (g *Game) sell(idx int) {
	p := g.P
	if idx < 0 || idx >= len(p.Inv) {
		return
	}
	it := p.Inv[idx]
	price := sellPrice(it, g.Rules)
	p.Gold += price
	g.Stats.In[GoldSale] += price
	p.Inv = append(p.Inv[:idx], p.Inv[idx+1:]...)
	if g.shop != nil && g.shop.Kind == 0 {
		g.shop.Items = append(g.shop.Items, it)
	}
	g.msg(colGold, "Sold %s for %dg.", it.Name, price)
}

// spendPoint puts one attribute point into Str, Dex, Vit or Ene (0-3).
func (g *Game) spendPoint(attr int) {
	p := g.P
	if p.Points <= 0 {
		return
	}
	switch attr {
	case 0:
		p.Str++
	case 1:
		p.Dex++
	case 2:
		p.Vit++
		p.HP += 2
	case 3:
		p.Ene++
	default:
		return
	}
	p.Points--
	p.recalc()
}

// ------------------------------------------------------------ equipment

func (g *Game) equip(idx int) {
	p := g.P
	if idx < 0 || idx >= len(p.Inv) {
		return
	}
	it := p.Inv[idx]
	var slot int
	switch it.Slot() {
	case SlotWeapon:
		slot = EqWeapon
	case SlotOffhand:
		slot = EqOffhand
	case SlotHelm:
		slot = EqHelm
	case SlotArmor:
		slot = EqArmor
	case SlotGloves:
		slot = EqGloves
	case SlotBoots:
		slot = EqBoots
	case SlotAmulet:
		slot = EqAmulet
	case SlotRing:
		slot = EqRing1
		if p.Eq[EqRing1] != nil && p.Eq[EqRing2] == nil {
			slot = EqRing2
		}
	default:
		return
	}
	returning := 0
	if p.Eq[slot] != nil {
		returning++
	}
	if slot == EqWeapon && it.Base.TwoHanded && p.Eq[EqOffhand] != nil {
		returning++
	}
	if slot == EqOffhand && p.Eq[EqWeapon] != nil && p.Eq[EqWeapon].Base.TwoHanded {
		returning++
	}
	if len(p.Inv)-1+returning > invMax {
		g.msg(colRed, "Your pack is too full to swap that.")
		return
	}
	p.Inv = append(p.Inv[:idx], p.Inv[idx+1:]...)
	if old := p.Eq[slot]; old != nil {
		p.Inv = append(p.Inv, old)
	}
	p.Eq[slot] = it
	// two-handed conflicts
	if slot == EqWeapon && it.Base.TwoHanded && p.Eq[EqOffhand] != nil {
		p.Inv = append(p.Inv, p.Eq[EqOffhand])
		p.Eq[EqOffhand] = nil
	}
	if slot == EqOffhand && p.Eq[EqWeapon] != nil && p.Eq[EqWeapon].Base.TwoHanded {
		p.Inv = append(p.Inv, p.Eq[EqWeapon])
		p.Eq[EqWeapon] = nil
	}
	p.recalc()
	g.Stats.Equips++
	g.msg(it.Color(), "You equip %s.", it.Name)
}

func (g *Game) unequip(slot int) {
	p := g.P
	if p.Eq[slot] == nil {
		return
	}
	if len(p.Inv) >= invMax {
		g.msg(colRed, "Your pack is full.")
		return
	}
	p.Inv = append(p.Inv, p.Eq[slot])
	g.msg(colGray, "You remove %s.", p.Eq[slot].Name)
	p.Eq[slot] = nil
	p.recalc()
}

func (g *Game) dropInv(idx int) {
	p := g.P
	if idx < 0 || idx >= len(p.Inv) {
		return
	}
	it := p.Inv[idx]
	p.Inv = append(p.Inv[:idx], p.Inv[idx+1:]...)
	g.dropItem(p.X, p.Y, it)
	g.msg(colGray, "You drop %s.", it.Name)
}

// equippedFor returns what the item would replace (for comparisons).
func (g *Game) equippedFor(it *Item) *Item {
	p := g.P
	switch it.Slot() {
	case SlotWeapon:
		return p.Eq[EqWeapon]
	case SlotOffhand:
		return p.Eq[EqOffhand]
	case SlotHelm:
		return p.Eq[EqHelm]
	case SlotArmor:
		return p.Eq[EqArmor]
	case SlotGloves:
		return p.Eq[EqGloves]
	case SlotBoots:
		return p.Eq[EqBoots]
	case SlotAmulet:
		return p.Eq[EqAmulet]
	case SlotRing:
		if p.Eq[EqRing1] != nil && p.Eq[EqRing2] == nil {
			return nil
		}
		return p.Eq[EqRing1]
	}
	return nil
}

// followDelay is how many turns the Last Wanderer takes to step out after
// the hero leaves a level he has noticed them on.
const followDelay = 2

// followOut takes the Last Wanderer off a level the hero is leaving, if
// he has noticed them there. He arrives followDelay turns later wherever
// they are by then.
func (g *Game) followOut(l *Level) {
	for _, m := range l.Monsters {
		if m.T.AI == AIWanderer && !m.Dead && !m.Gone && m.Awake {
			m.Gone = true
			g.stalker, g.stalkIn = m, followDelay
		}
	}
	g.cleanupLevel(l)
}

// stalkerArrives steps the Last Wanderer out where the hero arrived. In
// town he comes through the town portal, which collapses behind him.
func (g *Game) stalkerArrives() {
	l, m := g.Lv, g.stalker
	g.stalker = nil
	at := g.stalkAt
	viaPortal := l.Kind == KTown && g.Portal != nil
	if viaPortal {
		at = l.PortalAt
	}
	m.X, m.Y = l.FreeNear(at.X, at.Y, g.P.X, g.P.Y)
	m.Gone, m.Awake, m.LostTurns = false, true, 0
	l.Monsters = append(l.Monsters, m)
	g.novaFx(m.X, m.Y, C(1, .15, .1), 3)
	if viaPortal {
		g.Portal = nil
		g.msg(colRed, "The Last Wanderer steps out of the portal. It collapses behind him.")
	} else {
		g.msg(colRed, "The Last Wanderer follows you.")
	}
	if l.Kind == KTown && !g.homeYet {
		g.homeYet = true
		g.msg(m.T.Color, "The Last Wanderer: \"You showed me the way home.\"")
	}
}

// openRift is the Last Wanderer at half life on his own floor: he reads
// a scroll of his own and goes to Emberhold ahead of the hero. His red
// portal stays open behind him for the hero to follow.
func (g *Game) openRift(m *Monster) {
	l := g.Lv
	x, y := l.FreeNear(m.X, m.Y, g.P.X, g.P.Y)
	g.Rift = &Portal{l.ID, x, y}
	g.riftLight.ver = -1
	g.novaFx(x, y, C(1, .15, .1), 3)
	g.msg(colRed, "The Last Wanderer reads a scroll. A red portal tears open.")
	g.msg(m.T.Color, "The Last Wanderer: \"I know the way home too.\"")
	m.Gone = true // off this level at cleanup
	g.ahead, g.aheadAt = m, g.Turn
}

// unseal opens the Hearth Below's way down where its chamber stands,
// wherever the Last Wanderer died.
func (g *Game) unseal(here *Level) {
	for _, l := range g.Levels {
		if l.SealedDown == "" {
			continue
		}
		x, y := l.SealedAt.X, l.SealedAt.Y
		l.Set(x, y, TStairsDown)
		l.Links = append(l.Links, Link{x, y, x, y, l.SealedDown, -1, -1})
		l.SealedDown = ""
		if l == here {
			g.msg(colGray, "Where he waited, a way down opens.")
		} else {
			g.msg(colGray, "Far below, in the Hearth, a way down opens.")
		}
	}
}

// townHuntTurns is how long the Last Wanderer takes to find and kill
// one townsperson while the hero is not in Emberhold to stop him.
const townHuntTurns = 25

// hunter returns the Last Wanderer when he is loose in the hero's town.
func (g *Game) hunter() *Monster {
	l := g.Lv
	if l.Kind != KTown {
		return nil
	}
	for _, m := range l.Monsters {
		if m.T.AI == AIWanderer && !m.Dead && !m.Gone {
			return m
		}
	}
	return nil
}

// townHunted says whether Emberhold is dark: he is in it and alive.
func (g *Game) townHunted() bool { return g.hunter() != nil }

// prey picks who the Last Wanderer goes for in town: the nearest living
// townsperson, unless the hero stands nearer. Nil means the hero.
func (g *Game) prey(w *Monster) *Monster {
	best, bd := (*Monster)(nil), cheb(w.X, w.Y, g.P.X, g.P.Y)
	for _, m := range g.Lv.Monsters {
		if m.Friendly && !m.Dead {
			if d := cheb(w.X, w.Y, m.X, m.Y); d <= bd {
				best, bd = m, d
			}
		}
	}
	return best
}

// huntStep moves the Last Wanderer one step toward a townsperson, or cuts
// them when he is beside them. Three cuts kill.
func (g *Game) huntStep(w, prey *Monster) {
	l := g.Lv
	if cheb(w.X, w.Y, prey.X, prey.Y) <= 1 {
		prey.HP -= prey.MaxHP/3 + 1
		prey.Flash = g.time
		g.textFx(prey.X, prey.Y, "cut", colRed)
		if prey.HP <= 0 {
			g.townspersonDies(prey)
		} else {
			g.msg(colRed, "The Last Wanderer cuts %s.", townName(prey))
		}
		return
	}
	d := bfsDist(l, prey.X, prey.Y)
	cur := d[l.Idx(w.X, w.Y)]
	for _, dd := range dirs8 {
		nx, ny := w.X+dd.X, w.Y+dd.Y
		if l.In(nx, ny) {
			if v := d[l.Idx(nx, ny)]; v >= 0 && (cur < 0 || v < cur) && g.stepMonster(w, nx, ny) {
				return
			}
		}
	}
	g.stepToward(w) // walled off from them: come for the hero instead
}

func townName(m *Monster) string {
	if m.T.ID == "villager" {
		return "a villager"
	}
	return m.Name
}

// townspersonDies is a townsperson killed by the Last Wanderer. With
// Hadrik the forge goes cold for good.
func (g *Game) townspersonDies(m *Monster) {
	l := g.Lv
	m.Dead = true
	l.Decal[l.Idx(m.X, m.Y)] = DecalCorpse
	g.msg(colRed, "The Last Wanderer cuts down %s.", townName(m))
	if len(g.Fallen) == 0 {
		g.msg(C(1, .32, .26), "The Last Wanderer: \"They gave me a sword and a captain's speech too.\"")
	}
	g.Fallen = append(g.Fallen, m.T.ID)
	if m.T.ID == "smith" {
		g.forgeOut(l)
	}
	for _, o := range l.Monsters {
		if o.Friendly && !o.Dead {
			return
		}
	}
	g.msg(colLore, "No one is left in Emberhold but you.")
}

// forgeOut puts out Hadrik's forge, the Ember's hearth, for good.
func (g *Game) forgeOut(l *Level) {
	g.ForgeOut = true
	f := l.Forge
	l.Set(f.X, f.Y, TColdBrazier)
	keep := l.Lights[:0]
	for _, lt := range l.Lights {
		if lt.X != f.X || lt.Y != f.Y {
			keep = append(keep, lt)
		}
	}
	l.Lights = keep
	g.msg(colLore, "Hadrik's forge goes cold.")
}

// relight brings back the town lights he put out, all but a dead forge.
func (g *Game) relight() {
	for _, lt := range g.snuffed {
		lt.Off = false
	}
	g.snuffed = nil
}

// tollWhileAway is what the Last Wanderer did in Emberhold while the hero
// was elsewhere: a townsperson for every townHuntTurns, nearest the portal
// first, and the lights around each of them put out.
func (g *Game) tollWhileAway(turns int) {
	l := g.Lv
	if turns >= townHuntTurns {
		g.msg(colLore, "Emberhold is dark. You are too late for some of them.")
	}
	for n := turns / townHuntTurns; n > 0; n-- {
		var next *Monster
		for _, m := range l.Monsters {
			if m.Friendly && !m.Dead && (next == nil || cheb(m.X, m.Y, l.PortalAt.X, l.PortalAt.Y) < cheb(next.X, next.Y, l.PortalAt.X, l.PortalAt.Y)) {
				next = m
			}
		}
		if next == nil {
			break
		}
		for _, lt := range l.Lights {
			if !lt.Off && drinks([]Pos{{next.X, next.Y}}, lt.X, lt.Y) {
				lt.Off = true
				g.snuffed = append(g.snuffed, lt)
			}
		}
		g.townspersonDies(next)
	}
	g.cleanup()
}

// fleeFrom steps a monster to the neighboring cell farthest from (x, y).
func (g *Game) fleeFrom(m *Monster, x, y int) {
	best, bd := Pos{}, cheb(m.X, m.Y, x, y)
	for _, d := range dirs8 {
		nx, ny := m.X+d.X, m.Y+d.Y
		if dd := cheb(nx, ny, x, y); dd > bd && g.Lv.Walkable(nx, ny) && g.Lv.MonsterAt(nx, ny) == nil {
			best, bd = Pos{nx, ny}, dd
		}
	}
	if bd > cheb(m.X, m.Y, x, y) {
		g.stepMonster(m, best.X, best.Y)
	}
}

// The endings, chosen over the Last Wanderer's body.
const (
	EndReturn = 1 // carry the Ember down; Emberhold goes dark for good
	EndHold   = 2 // stay below as the new dam
	EndWalk   = 3 // put it off: the run goes on into the Abyss
)

// chooseEnding settles the choice screen. Keep walking returns to play;
// the other two end the story and the run.
func (g *Game) chooseEnding(e int) {
	g.Ending = e
	if e == EndWalk {
		g.Mode = ModePlay
		g.msg(colLore, "You leave the choice where he left it, and keep walking.")
		return
	}
	g.Mode = ModeEnd
}

// survivors lists who is left alive in Emberhold, by name.
func (g *Game) survivors() []string {
	var out []string
	villagers := 0
	for _, m := range g.getLevel("town").Monsters {
		if !m.Friendly || m.Dead {
			continue
		}
		if m.T.ID == "villager" {
			villagers++
		} else {
			out = append(out, m.Name)
		}
	}
	switch villagers {
	case 0:
	case 1:
		out = append(out, "one villager")
	default:
		out = append(out, fmt.Sprintf("%d villagers", villagers))
	}
	return out
}

// endingText is the story's last page for an ending, given who is left.
func (g *Game) endingText(e int) (title string, lines []string) {
	alive := g.survivors()
	switch e {
	case EndReturn:
		title = "THE EMBER RETURNED"
		if g.ForgeOut {
			lines = append(lines, "The forge went cold with Hadrik. You carry down a handful of its ash and bury it where Edran dug. It is enough. The wound closes on nothing.")
		} else {
			lines = append(lines, "You carry the coal down past the Oracle's empty pool and set it where Edran found it. Above you, every lantern in Emberhold goes out at once.")
		}
		lines = append(lines, "The dark stops climbing. There is nothing left to climb toward.")
	default:
		title = "THE NEW DAM"
		lines = append(lines, "You stay. The Hearth is quiet while someone holds it.")
		switch {
		case len(alive) == 0:
			lines = append(lines, "Far above, Emberhold is empty. Its lanterns burn all night for no one.")
		case g.ForgeOut:
			lines = append(lines, "Far above, what is left of Emberhold lights candles where the forge used to burn, and never learns why the dark stopped pressing at the gate.")
		default:
			lines = append(lines, "Far above, Emberhold lights its lanterns for another night and never learns why the dark stopped pressing at the gate.")
		}
	}
	if len(alive) == 0 {
		lines = append(lines, "No one in Emberhold survived.")
	} else {
		lines = append(lines, "Survived: "+joinAnd(alive)+".")
	}
	return title, lines
}

func joinAnd(xs []string) string {
	if len(xs) < 2 {
		return strings.Join(xs, "")
	}
	return strings.Join(xs[:len(xs)-1], ", ") + " and " + xs[len(xs)-1]
}

// Ready makes the hero fit for a start past the town (-level): character
// level lvl, attribute points spent the way the build's bot spends them,
// the best of a dozen rares per slot at the area's depth, a full belt,
// two scrolls, and the quests on the way already done.
func (g *Game) Ready(lvl int, build string) error {
	pol := policyByName(build)
	if pol == nil {
		return fmt.Errorf("build %q: want fighter or caster", build)
	}
	if lvl < 1 || lvl > 99 {
		return fmt.Errorf("level %d: want 1 to 99", lvl)
	}
	p := g.P
	p.Lvl, p.XP, p.Points = lvl, 0, 5*(lvl-1)
	for i := 0; p.Points > 0; i++ {
		g.spendPoint(pol.points[i%len(pol.points)])
	}
	depth := maxi(g.Lv.Depth, maxi(1, lvl/2))
	for slot := range EqCount {
		if slot == EqOffhand && p.Eq[EqWeapon] != nil && p.Eq[EqWeapon].Base.TwoHanded {
			p.Eq[slot] = nil
			continue
		}
		var best *Item
		bestScore := 0
		for range 12 {
			it := GenItem(g.rng, depth, RRare, eqSlotFor[slot], g.Rules)
			if v, _ := pol.gearScore(it); best == nil || v > bestScore {
				best, bestScore = it, v
			}
		}
		p.Eq[slot] = best
	}
	p.HPot, p.MPot, p.Scrolls, p.Gold = beltMax, pol.mana, 2, 60*lvl
	if depth > questDepth[0] {
		g.Quests[0] = 2
	}
	if depth > questDepth[1] {
		g.Quests[1] = 2
	}
	g.Deepest = maxi(g.Deepest, depth)
	p.recalc()
	p.HP, p.MP = float64(p.MaxHP()), float64(p.MaxMP())
	return nil
}
