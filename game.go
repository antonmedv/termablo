package main

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/antonmedv/termablo/internal/i18n"
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
	ModeQuests
)

const visThresh = 0.035

type LogMsg struct {
	Text string
	Col  RGB
	Turn int
	N    int
	// the catalog line it was written from, to write it again in
	// another language; "" for a line logged as is
	Key  string
	Args []any
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
	stalker   *Monster
	stalkIn   int
	stalkAt   Pos      // where the hero arrived, where he will step out
	homeYet   bool     // he has reached Emberhold
	byPortal  bool     // the hero came to this level through the town portal
	snuffed   []*Light // town lights he has put out; back when he dies
	Fallen    []string // townsfolk he has killed, by template ID, in order
	ForgeOut  bool     // Hadrik is dead and the forge with him
	lgen      uint32
	dist      []int32
	lightsBuf []*Light

	Effects     []*Effect
	Portal      *Portal
	portalLight *Light
	townPortalL *Light
	Target      *Monster
	hitter      *Monster // the last monster to strike the hero, for Stats.Death

	Quests  map[string]QuestState // by quest ID: how far the hero has come with each
	Slain   map[string]bool       // the bosses dead, by template ID
	Deepest int                   // deepest Depth entered: what the shops roll at
	stocked int                   // Deepest at the last restock
	shops   [2]*Shop
	shop    *Shop

	cur, pane, tab int
	helpOff        int             // help screen scroll
	talk           *Talk           // the conversation open, or the one a shop returns to
	Known          map[string]bool // dialog topics the hero has heard of
	heard          map[string]bool // dialog lines said to the hero, "<who> <key>"

	hoverX, hoverY int
	hoverOn        bool
	hoverLines     []hoverLine // this frame's hover info (nil = nothing hovered)
	beltHit        [3]hitBox   // this frame's belt rows: heal, mana, portal
	questsHit      hitBox      // this frame's quests header in the panel
	questsRows     int         // and the quest rows under it
	journalHit     []hitBox    // this frame's rows of the journal's list
	langHit        []hitBox    // the title screen's languages, in i18n.Langs order

	auto       bool
	autoNext   float64
	autoItems  int
	usedAltars map[string]bool

	Stats Stats
	Rules *Rules
	L     *i18n.Catalog // the player's language
	// the killing blow's source as the player read it, for the death screen
	killer string
}

func NewGame(seed int64) *Game { return NewGameWith(seed, DefaultRules()) }

// NewGameWith starts a game under a set of rules, which it shares with
// its player, levels and monsters.
func NewGameWith(seed int64, r *Rules) *Game {
	g := &Game{Seed: seed, rng: rand.New(rand.NewSource(seed)), Levels: map[string]*Level{}, usedAltars: map[string]bool{}, Known: map[string]bool{"rumors": true}, heard: map[string]bool{}, Quests: map[string]QuestState{}, Slain: map[string]bool{}, Stats: newStats(), Rules: r, Deepest: 1, L: locales.Get(i18n.Source)}
	g.P = NewPlayer(r)
	g.portalLight = NewLight(0, 0, &LightSpec{C(.35, .5, 1), 4.5, 1.1, .05, .3}, 1)
	g.townPortalL = NewLight(0, 0, &LightSpec{C(.35, .5, 1), 4.5, 1.1, .05, .3}, 2)
	sw := &Item{Kind: IKEquip, Base: baseByName("Short Sword"), Name: "Short Sword", ILvl: 1}
	sw.finishStats(g.rng)
	ar := &Item{Kind: IKEquip, Base: baseByName("Leather Armor"), Name: "Leather Armor", ILvl: 1}
	ar.finishStats(g.rng)
	g.P.Eq[EqWeapon] = sw
	g.P.Eq[EqArmor] = ar
	g.P.recalc()
	g.changeLevel("town", "", nil)
	g.say(colGray, "msg.welcome_help")
	return g
}

// msg logs a line as written; say logs one from the catalog.
func (g *Game) msg(col RGB, s string) {
	if n := len(g.Log); n > 0 && g.Log[n-1].Text == s {
		g.Log[n-1].N++
		g.Log[n-1].Turn = g.Turn
		return
	}
	g.Log = append(g.Log, LogMsg{Text: s, Col: col, Turn: g.Turn, N: 1})
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
		s := DungeonSpec{ID: id, Name: fmt.Sprintf("Crypt of the Fallen %d", n), NameKey: "crypt", NameN: n, Depth: 1 + n, Style: 0, SpawnTable: "crypt", Rules: g.Rules}
		s.Up = "fields"
		if n > 1 {
			s.Up = fmt.Sprintf("crypt%d", n-1)
		}
		if n < 4 {
			s.Down = fmt.Sprintf("crypt%d", n+1)
		} else {
			s.Boss = "boneking"
			s.Name, s.NameKey = "Throne of the Bone King", "throne"
		}
		l = genDungeon(s, seed)
	case strings.HasPrefix(id, "barrow"):
		n := num("barrow")
		s := DungeonSpec{ID: id, Name: fmt.Sprintf("Gravewardens' Barrow %d", n), NameKey: "barrow", NameN: n, Depth: 5 + n, Style: 3, SpawnTable: "barrow", Rules: g.Rules}
		s.Up = "fields"
		if n > 1 {
			s.Up = fmt.Sprintf("barrow%d", n-1)
		}
		if n < 2 {
			s.Down = fmt.Sprintf("barrow%d", n+1)
		} else {
			s.Boss = "buried"
			s.Name, s.NameKey = "The Buried Watch", "buried_watch"
		}
		l = genDungeon(s, seed)
	case strings.HasPrefix(id, "grotto"):
		n := num("grotto")
		s := DungeonSpec{ID: id, Name: fmt.Sprintf("Sunken Grotto %d", n), NameKey: "grotto", NameN: n, Depth: 6 + n, Style: 1, SpawnTable: "grotto", Rules: g.Rules}
		s.Up = "marsh"
		if n > 1 {
			s.Up = fmt.Sprintf("grotto%d", n-1)
		}
		if n < 3 {
			s.Down = fmt.Sprintf("grotto%d", n+1)
		} else {
			s.Down = "abyss1"
			s.Boss = "oracle"
			s.Name, s.NameKey = "The Oracle's Pool", "oracle_pool"
		}
		l = genDungeon(s, seed)
	case strings.HasPrefix(id, "abyss"):
		n := num("abyss")
		s := DungeonSpec{ID: id, Name: fmt.Sprintf("The Burning Abyss %d", n), NameKey: "abyss", NameN: n, Depth: 9 + n, Style: 2, SpawnTable: "abyss", Rules: g.Rules}
		s.Up = "grotto3"
		if n > 1 {
			s.Up = fmt.Sprintf("abyss%d", n-1)
		}
		s.Down = fmt.Sprintf("abyss%d", n+1)
		if n == hearthFloor {
			s.Name, s.NameKey, s.Boss = "The Hearth Below", "hearth", "wanderer"
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
	g.byPortal = false
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
	p.Torch.ver = -1
	g.Effects = nil
	g.Target = nil
	g.auto = false
	if !l.visited {
		l.visited = true
		g.say(colLore, "lore."+l.LoreKey, "area", areaArg{l})
		g.see("area." + l.LoreKey)
	} else {
		g.say(colGray, "msg.enter", "area", areaArg{l})
	}
	if l.Kind != KTown && l.Depth > p.Lvl+2 {
		g.say(colLore, "msg.far_beyond")
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
	add := func(lt *Light) {
		if !drinks(g.dark, lt.X, lt.Y) {
			ls = append(ls, lt)
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
		g.say(colCyan, "msg.fountain")
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
	if g.Portal != nil {
		if g.Portal.Level == l.ID && nx == g.Portal.X && ny == g.Portal.Y {
			g.say(colBlue, "msg.portal_step")
			g.changeLevel("town", "", &Pos{l2(g).PortalAt.X, l2(g).PortalAt.Y + 1})
			g.byPortal = true
			return
		}
		if l.Kind == KTown && nx == l.PortalAt.X && ny == l.PortalAt.Y {
			dest := g.Portal
			g.Portal = nil
			g.say(colBlue, "msg.portal_collapse")
			g.changeLevel(dest.Level, "", &Pos{dest.X, dest.Y})
			return
		}
	}
	if items := l.ItemsAt(nx, ny); len(items) > 0 && !g.auto {
		if len(items) == 1 {
			g.say(colGray, "msg.see_item", "item", itemNoun(g.L, items[0].It))
		} else {
			g.say(colGray, "msg.see_items")
		}
	}
	g.endTurn()
}

// portalAt says whether a cell is a portal mouth: the open portal on its
// level, or its town end.
func (g *Game) portalAt(x, y int) bool {
	l := g.Lv
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
			g.say(colGold, "msg.pickup_gold", "n", it.Amount)
		case IKHealth, IKMana:
			n, col := &p.HPot, C(1, .4, .4)
			if it.Kind == IKMana {
				n, col = &p.MPot, C(.45, .6, 1)
			}
			if !g.canTake(it) {
				g.say(colDim, "msg.belt_no_room", "item", itemNoun(g.L, it))
				keep = append(keep, fi)
				continue
			}
			*n++
			g.say(col, "msg.pickup_potion", "item", itemNoun(g.L, it))
		case IKScroll:
			p.Scrolls++
			g.say(C(.85, .8, .65), "msg.pickup_scroll", "item", itemNoun(g.L, it))
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
					g.say(colRed, "msg.pack_full")
				}
				full = true
				keep = append(keep, fi)
				continue
			}
			p.Inv = append(p.Inv, fi.It)
			g.say(fi.It.Color(), "msg.pickup_item", "item", itemNoun(g.L, fi.It))
			g.gain(fi.It)
			got = true
			continue
		}
		keep = append(keep, fi)
	}
	l.Items = keep
	if got {
		g.endTurn()
	} else if !full {
		g.say(colDim, "msg.nothing_to_pick_up")
	}
}

// gain notes an item come into the pack, however it came: a unique may
// teach a topic.
func (g *Game) gain(it *Item) {
	if it.Rarity == RUnique {
		g.see("item." + it.Name)
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
		g.say(colGold, "msg.boss_destroyed", "who", theRef(g.L, m))
		g.novaFx(m.X, m.Y, C(1, .8, .4), 6)
	case RankUnique, RankChampion:
		g.say(m.Color(), "msg.elite_slain", "who", theRef(g.L, m))
	default:
		g.say(C(.6, .55, .5), "msg.monster_dies", "who", theRef(g.L, m))
	}
	if m.HasMod(ModFire) {
		g.novaFx(m.X, m.Y, colOrange, 2)
		if cheb(m.X, m.Y, p.X, p.Y) <= 1 {
			g.hitter = nil
			g.hurt(m.Level*2+3, m.Name+"'s death flames", g.L.Noun("ref.death_flames", "name", monsterNoun(g.L, m)), "burn")
		}
	}
	switch m.T.ID {
	case "boneking":
		g.say(colOrange, "msg.boneking_dead")
	case "buried":
		g.say(colOrange, "msg.buried_dead")
	case "oracle":
		g.say(colOrange, "msg.oracle_dead")
	case "wanderer":
		g.relight()
		g.say(m.T.Color, "msg.wanderer_last_words")
		g.say(colLore, "msg.wanderer_dead")
		g.unseal(l)
	}
	if m.Rank == RankBoss {
		g.slay(m.T.ID)
	}
	g.see("kill." + m.T.ID)
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
		g.say(colGold, "msg.level_up", "n", p.Lvl)
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
	g.say(C(.9, .7, .35), "msg.open_chest")
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
		g.say(colDim, "msg.altar_cold")
		return
	}
	g.usedAltars[key] = true
	p := g.P
	switch g.rng.Intn(3) {
	case 0:
		p.MP = float64(p.MaxMP())
		p.HP = float64(p.MaxHP())
		g.say(colPurple, "msg.altar_restore")
	case 1:
		xp := g.Rules.xpNext(p.Lvl) / 4
		g.say(colPurple, "msg.altar_xp", "n", xp)
		g.gainXP(xp)
	default:
		g.say(colPurple, "msg.altar_gift")
		g.dropItem(x, y+1, GenItem(g.rng, g.Lv.Depth+1, RRare, SlotNone, g.Rules))
	}
	g.novaFx(x, y, colPurple, 3)
	g.see("altar")
}

// hurtPlayer applies damage from a source named in English.
func (g *Game) hurtPlayer(dmg int, by, verb string) { g.hurt(dmg, by, i18n.Noun{Text: by}, verb) }

// hurt applies damage and logs "<who> <verb> you for N." from the
// hit.<verb> line. by is the English name the killer goes on record as,
// who the same as the player reads it.
func (g *Game) hurt(dmg int, by string, who i18n.Noun, verb string) {
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
	g.say(C(.9, .45, .4), "hit."+slug(verb), "who", who, "n", dmg)
	p.HP -= float64(dmg)
	g.Stats.DmgTaken += dmg
	p.Flash = g.time
	g.auto = false
	g.textFx(p.X, p.Y, strconv.Itoa(dmg), colRed)
	if p.HP <= 0 {
		p.HP = 0
		p.KilledBy, g.killer = by, who.Text
		g.Mode = ModeDead
		g.say(colRed, "msg.slain", "who", who)
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
		g.say(colDim, "msg.no_healing_potions")
		return
	}
	if p.HP+p.HealPool >= float64(p.MaxHP()) {
		g.say(colDim, "msg.full_life")
		return
	}
	p.HPot--
	g.Stats.HPots++
	amt := p.HealAmt()
	p.HealPool += amt
	g.textFx(p.X, p.Y, "+"+strconv.Itoa(int(amt)), colGreen)
	g.say(C(1, .45, .45), "msg.drink_healing")
	g.endTurn()
}

func (g *Game) drinkMana() {
	p := g.P
	if p.MPot <= 0 {
		g.say(colDim, "msg.no_mana_potions")
		return
	}
	if p.MP+p.ManaPool >= float64(p.MaxMP()) {
		g.say(colDim, "msg.full_mana")
		return
	}
	p.MPot--
	g.Stats.MPots++
	p.ManaPool += p.ManaAmt()
	g.say(C(.45, .6, 1), "msg.drink_mana")
	g.endTurn()
}

func (g *Game) readPortal() {
	l, p := g.Lv, g.P
	if l.Kind == KTown {
		g.say(colDim, "msg.already_in_town")
		return
	}
	if p.Scrolls <= 0 {
		g.say(colDim, "msg.no_scrolls")
		return
	}
	p.Scrolls--
	// The portal opens on the nearest free cell: in a crowd, the one
	// side that is still open, not the far side of whoever stands east.
	x, y := l.FreeNear(p.X, p.Y, p.X, p.Y)
	g.Portal = &Portal{l.ID, x, y}
	g.portalLight.ver = -1
	g.say(colBlue, "msg.portal_open")
	g.see("portal")
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
		g.say(colDim, "msg.no_target")
		return
	}
	if cheb(p.X, p.Y, t.X, t.Y) > fireboltRange {
		g.say(colDim, "msg.out_of_range", "who", theRef(g.L, t))
		return
	}
	if p.MP < float64(p.FireboltCost()) {
		g.say(C(.45, .6, 1), "msg.no_mana")
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
		g.say(C(.45, .6, 1), "msg.no_mana")
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
	if m.T.AI == AIWanderer && l.Kind == KTown {
		g.snuffAround(m)
	}
	if m.Friendly {
		if w := g.hunter(); w != nil && cheb(m.X, m.Y, w.X, w.Y) <= 8 {
			// panicked: they stumble away two turns in three
			if g.rng.Intn(3) != 0 {
				g.fleeFrom(m, w.X, w.Y)
			}
			return
		}
		if cheb(m.X, m.Y, m.HomeX, m.HomeY) > 3 {
			// back home after running from him
			g.stepMonster(m, m.X+sign(m.HomeX-m.X), m.Y+sign(m.HomeY-m.Y))
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
			g.say(C(.75, 1, .6), "msg.boneking_raises")
			for range 2 {
				s := placeMonsterAvoid(l, "skel", m.X, m.Y, m.Level, RankNormal, p.X, p.Y)
				s.Awake, s.Minion = true, true
			}
			g.novaFx(m.X, m.Y, C(.5, 1, .45), 2.5)
			return
		}
	case AICaptain:
		// hurt, he calls the watch: every sleeper with a way to the hero
		// stands to and comes
		if !m.Rallied && m.HP*2 < m.MaxHP {
			m.Rallied = true
			for _, o := range l.Monsters {
				if !o.Dead && !o.Friendly && g.dist[l.Idx(o.X, o.Y)] >= 0 {
					o.Awake, o.LostTurns = true, 0
				}
			}
			g.say(m.T.Color, "msg.buried_rallies")
			g.novaFx(m.X, m.Y, m.T.Color, 2.5)
			return
		}
	case AIWanderer:
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
					g.say(colCyan, "msg.oracle_reforms")
					return
				}
			}
		}
		if sees && m.Timer%13 == 0 && g.countMinions() < 8 {
			g.say(colCyan, "msg.oracle_lights")
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
		g.say(C(.8, 1, .6), "msg.boneking_greets")
	case "buried":
		g.say(m.T.Color, "msg.buried_greets")
	case "oracle":
		g.say(colCyan, "msg.oracle_greets")
	case "wanderer":
		g.say(m.T.Color, "msg.wanderer_greets")
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
	g.hitter = m
	g.hurt(dmg, englishRef(m), theRef(g.L, m), m.T.Verb)
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
	g.hitter = m
	g.hurt(g.monsterDamage(m)*4/5+1, englishRef(m), theRef(g.L, m), m.T.Verb)
}

// englishRef is how the monster goes on record as a killer.
func englishRef(m *Monster) string {
	if named(m) {
		return m.Name
	}
	return "the " + m.Name
}

// ------------------------------------------------------------ auto-explore

const (
	autoStopNear  = 10 // any enemy this close stops auto-explore
	autoStopAware = 18 // ...as does one this close that has noticed you
)

func (g *Game) autoStep() bool {
	l, p := g.Lv, g.P
	// Distant, unaware enemies (common on the open fields) don't interrupt.
	for _, m := range g.visibleHostiles() {
		d := cheb(m.X, m.Y, p.X, p.Y)
		if d <= autoStopNear || (m.Awake && d <= autoStopAware) {
			g.say(colOrange, "msg.spot", "who", aRef(g.L, m))
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
		g.say(colGray, "msg.notice_item")
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
		g.say(colGray, "msg.explored")
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

func (g *Game) talkTo(m *Monster) {
	if g.townHunted() {
		who := monsterNoun(g.L, m)
		if m.T.ID == "villager" {
			who = i18n.Noun{Text: g.L.T("ref.the", "name", who), Gender: who.Gender}
		}
		g.say(m.T.Color, "msg.running", "who", who)
		return
	}
	// how the hero looks as they walk in, before Mirela patches them up
	t := &Talk{Name: monsterNoun(g.L, m).Text, Col: m.T.Color, looks: g.looksNow()}
	var greet []string
	t.Who = speaker(m.T.ID)
	switch m.T.ID {
	case "smith":
		g.shop = g.shops[0]
		t.Barter = true
	case "alch":
		g.mirelaHeal()
		g.shop = g.shops[1]
		t.Barter = true
	case "villager":
		greet = []string{g.rumor("")}
	}
	// meeting someone with a name is learning it
	if slices.Contains(topics, t.Who) {
		g.Known[t.Who] = true
	}
	// A reward is its own greeting; what the giver has to say about the
	// quest after waits for the next visit.
	if paid := g.payQuests(m); len(paid) > 0 {
		greet = paid
		g.briefed(t.Who)
	}
	g.talk, g.Mode = t, ModeTalk
	g.describe()
	if k := g.says(t.Who, "greet"); len(greet) == 0 && k != "" {
		greet = []string{k}
	}
	for _, k := range greet {
		g.hear("", k)
	}
	t.Cur = t.firstTopic(g)
}

func (g *Game) buy(it *Item) {
	p := g.P
	price := g.buyPrice(it)
	if p.Gold < price {
		g.say(colRed, "msg.cannot_afford")
		return
	}
	switch it.Kind {
	case IKHealth, IKMana:
		n := &p.HPot
		if it.Kind == IKMana {
			n = &p.MPot
		}
		if *n >= beltMax {
			g.say(colRed, "msg.belt_full")
			return
		}
		*n++
	case IKScroll:
		p.Scrolls++
	case IKGamble:
		if len(p.Inv) >= invMax {
			g.say(colRed, "msg.pack_full")
			return
		}
		r := g.Rules
		got := GenItem(g.rng, g.Deepest+g.rng.Intn(3), RollRarity(g.rng, r.GambleBonus), it.Slot(), r)
		p.Inv = append(p.Inv, got)
		p.Gold -= price
		g.Stats.Out[SinkGamble] += price
		g.say(got.Color(), "msg.gamble", "item", itemNoun(g.L, got), "n", price)
		g.gain(got)
		return
	case IKReroll:
		p.Gold -= price
		g.Stats.Out[SinkReroll] += price
		g.restock()
		g.shop = g.shops[0]
		g.say(colGold, "msg.reroll", "n", price)
		return
	default:
		if len(p.Inv) >= invMax {
			g.say(colRed, "msg.pack_full")
			return
		}
		p.Inv = append(p.Inv, it)
		if i := slices.Index(g.shop.Items, it); i >= 0 {
			g.shop.Items = slices.Delete(g.shop.Items, i, i+1)
		}
	}
	p.Gold -= price
	g.Stats.Out[sinkOf(it)] += price
	g.say(colGold, "msg.bought", "item", itemNoun(g.L, it), "n", price)
	g.gain(it)
}

// Mirela heals for free until the hero is past freeHealLvl.
const freeHealLvl = 3

// mirelaHeal restores life, then mana, as far as the hero can pay, at
// HealCost a point.
func (g *Game) mirelaHeal() {
	p, healCost := g.P, g.Rules.HealCost
	need := float64(p.MaxHP()) - p.HP + float64(p.MaxMP()) - p.MP
	if need < 1 {
		g.say(colCyan, "msg.mirela_healthy")
		return
	}
	if p.Lvl <= freeHealLvl {
		p.HP, p.MP = float64(p.MaxHP()), float64(p.MaxMP())
		g.say(colCyan, "msg.mirela_free")
		return
	}
	pts := math.Min(need, float64(p.Gold)/healCost)
	if pts < 1 {
		g.say(colCyan, "msg.mirela_broke")
		return
	}
	cost := int(math.Ceil(pts * healCost))
	p.Gold -= cost
	g.Stats.Out[SinkHeal] += cost
	hp := math.Min(pts, float64(p.MaxHP())-p.HP)
	p.HP += hp
	p.MP = math.Min(float64(p.MaxMP()), p.MP+pts-hp)
	if pts < need {
		g.say(colCyan, "msg.mirela_partial", "n", cost)
		return
	}
	g.say(colCyan, "msg.mirela_paid", "n", cost)
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
	g.say(colGold, "msg.sold", "item", itemNoun(g.L, it), "n", price)
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
		g.say(colRed, "msg.pack_too_full")
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
	g.say(it.Color(), "msg.equip", "item", itemNoun(g.L, it))
}

func (g *Game) unequip(slot int) {
	p := g.P
	if p.Eq[slot] == nil {
		return
	}
	if len(p.Inv) >= invMax {
		g.say(colRed, "msg.pack_full")
		return
	}
	p.Inv = append(p.Inv, p.Eq[slot])
	g.say(colGray, "msg.unequip", "item", itemNoun(g.L, p.Eq[slot]))
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
	g.say(colGray, "msg.drop", "item", itemNoun(g.L, it))
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
	viaPortal := l.Kind == KTown && g.byPortal && g.Portal != nil
	if viaPortal {
		at = l.PortalAt
	}
	m.X, m.Y = l.FreeNear(at.X, at.Y, g.P.X, g.P.Y)
	m.Gone, m.Awake, m.LostTurns = false, true, 0
	l.Monsters = append(l.Monsters, m)
	g.novaFx(m.X, m.Y, C(1, .15, .1), 3)
	if viaPortal {
		g.Portal = nil
		g.say(colRed, "msg.wanderer_portal")
	} else {
		g.say(colRed, "msg.wanderer_follows")
	}
	if l.Kind == KTown && !g.homeYet {
		g.homeYet = true
		g.say(m.T.Color, "msg.wanderer_home")
		g.takeQuest(questByID("wanderer"))
	}
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
			g.say(colGray, "msg.unseal_here")
		} else {
			g.say(colGray, "msg.unseal_far")
		}
	}
}

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
		g.textFx(prey.X, prey.Y, g.L.T("ui.fx.cut"), colRed)
		if prey.HP <= 0 {
			g.townspersonDies(prey)
		} else {
			g.say(colRed, "msg.wanderer_cuts", "who", townRef(g.L, prey))
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

// townRef is a townsperson as the object of the Wanderer's blade:
// "a villager", "Mirela the Alchemist".
func townRef(loc *i18n.Catalog, m *Monster) i18n.Noun {
	n := monsterNoun(loc, m)
	if m.T.ID == "villager" {
		return i18n.Noun{Text: loc.T("ref.a", "name", n), Gender: n.Gender}
	}
	return n
}

// townspersonDies is a townsperson killed by the Last Wanderer. With
// Hadrik the forge goes cold for good.
func (g *Game) townspersonDies(m *Monster) {
	l := g.Lv
	m.Dead = true
	l.Decal[l.Idx(m.X, m.Y)] = DecalCorpse
	g.say(colRed, "msg.wanderer_kills", "who", townRef(g.L, m))
	if len(g.Fallen) == 0 {
		g.say(C(1, .32, .26), "msg.wanderer_taunt")
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
	g.say(colLore, "msg.town_empty")
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
	g.say(colLore, "msg.forge_cold")
}

// snuffAround puts out the town's lights within his reach. In Emberhold
// what he puts out stays out while he lives.
func (g *Game) snuffAround(m *Monster) {
	at := []Pos{{m.X, m.Y}}
	for _, lt := range g.Lv.Lights {
		if !lt.Off && drinks(at, lt.X, lt.Y) {
			lt.Off = true
			g.snuffed = append(g.snuffed, lt)
		}
	}
}

// snuffedAt says whether a town light he put out stands at (x, y).
func (g *Game) snuffedAt(x, y int) bool {
	for _, lt := range g.snuffed {
		if lt.X == x && lt.Y == y {
			return true
		}
	}
	return false
}

// relight brings back the town lights he put out, all but a dead forge.
func (g *Game) relight() {
	for _, lt := range g.snuffed {
		lt.Off = false
	}
	g.snuffed = nil
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
	g.readyQuests(depth)
	g.Deepest = maxi(g.Deepest, depth)
	if g.Lv.Kind == KTown {
		g.restock()
	}
	p.recalc()
	p.HP, p.MP = float64(p.MaxHP()), float64(p.MaxMP())
	return nil
}
