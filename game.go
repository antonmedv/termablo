package main

import (
	"fmt"
	"math"
	"math/rand"
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
	Name  string
	Kind  int // 0 smith, 1 alchemist
	Items []*Item
}

type Game struct {
	rng    *rand.Rand
	Seed   int64
	Levels map[string]*Level
	Lv     *Level
	P      *Player
	Log    []LogMsg
	Turn   int
	Mode   Mode
	time   float64

	fov       []uint32
	fovGen    uint32
	vis       []bool
	light     []RGB
	lstamp    []uint32
	lgen      uint32
	dist      []int32
	lightsBuf []*Light

	Effects     []*Effect
	Portal      *Portal
	portalLight *Light
	townPortalL *Light
	Target      *Monster

	Quests [2]int // 0 hunting, 1 slain, 2 rewarded
	shops  [2]*Shop
	shop   *Shop

	cur, pane, tab int
	talkName       string
	talkLines      []string
	talkCol        RGB

	auto       bool
	autoNext   float64
	autoItems  int
	usedAltars map[string]bool
}

func NewGame(seed int64) *Game {
	g := &Game{Seed: seed, rng: rand.New(rand.NewSource(seed)), Levels: map[string]*Level{}, usedAltars: map[string]bool{}}
	g.P = NewPlayer()
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
		l = genTown(seed)
	case id == "fields":
		l = genFields(seed)
	case id == "marsh":
		l = genMarsh(seed)
	case strings.HasPrefix(id, "crypt"):
		n := num("crypt")
		s := DungeonSpec{ID: id, Name: fmt.Sprintf("Crypt of the Fallen %d", n), Depth: 1 + n, Style: 0, SpawnTable: "crypt"}
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
		s := DungeonSpec{ID: id, Name: fmt.Sprintf("Sunken Grotto %d", n), Depth: 5 + n, Style: 1, SpawnTable: "grotto"}
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
		s := DungeonSpec{ID: id, Name: fmt.Sprintf("The Burning Abyss %d", n), Depth: 8 + n, Style: 2, SpawnTable: "abyss"}
		s.Up = "grotto3"
		if n > 1 {
			s.Up = fmt.Sprintf("abyss%d", n-1)
		}
		s.Down = fmt.Sprintf("abyss%d", n+1)
		l = genDungeon(s, seed)
	default:
		l = genTown(seed)
	}
	g.Levels[id] = l
	return l
}

func (g *Game) changeLevel(id, from string, arrive *Pos) {
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
	p.Torch.ver = -1
	g.Effects = nil
	g.Target = nil
	g.auto = false
	if !l.visited {
		l.visited = true
		if l.Lore != "" {
			g.msg(C(.75, .65, .5), "%s", l.Lore)
		}
	} else {
		g.msg(colGray, "You enter %s.", l.Name)
	}
	if l.Kind == KTown {
		g.restock()
	}
	g.computeVisibility()
}

// ------------------------------------------------------------ light & sight

func (g *Game) gatherLights() []*Light {
	l, p := g.Lv, g.P
	ls := g.lightsBuf[:0]
	near := func(x, y int, r float32) bool {
		return abs(x-p.X) < 80+int(r*2) && abs(y-p.Y) < 45+int(r)
	}
	for _, lt := range l.Lights {
		if near(lt.X, lt.Y, lt.Radius) {
			ls = append(ls, lt)
		}
	}
	p.Torch.X, p.Torch.Y = p.X, p.Y
	ls = append(ls, p.Torch)
	for _, m := range l.Monsters {
		if m.Light != nil && !m.Dead {
			m.Light.X, m.Light.Y = m.X, m.Y
			ls = append(ls, m.Light)
		}
	}
	for _, it := range l.Items {
		if it.Light != nil {
			ls = append(ls, it.Light)
		}
	}
	if g.Portal != nil {
		if g.Portal.Level == l.ID {
			g.portalLight.X, g.portalLight.Y = g.Portal.X, g.Portal.Y
			ls = append(ls, g.portalLight)
		}
		if l.Kind == KTown {
			g.townPortalL.X, g.townPortalL.Y = l.PortalAt.X, l.PortalAt.Y
			ls = append(ls, g.townPortalL)
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

func (g *Game) cycleTarget() {
	hs := g.visibleHostiles()
	if len(hs) == 0 {
		g.Target = nil
		return
	}
	idx := -1
	for i, m := range hs {
		if m == g.Target {
			idx = i
		}
	}
	g.Target = hs[(idx+1)%len(hs)]
}

// ------------------------------------------------------------ turns

func (g *Game) endTurn() {
	p := g.P
	g.Turn++
	p.HP = math.Min(float64(p.MaxHP()), p.HP+0.02+float64(p.S(StLifeRegen))*0.06+float64(p.VIT())*0.0015)
	p.MP = math.Min(float64(p.MaxMP()), p.MP+0.08+float64(p.ENE())*0.005+float64(p.S(StManaRegen))*0.06)
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

func (g *Game) cleanup() {
	l := g.Lv
	alive := l.Monsters[:0]
	for _, m := range l.Monsters {
		if !m.Dead {
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
		p.HP, p.MP = float64(p.MaxHP()), float64(p.MaxMP())
		g.msg(colCyan, "The cold water restores you.")
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
			g.msg(colGold, "You pick up %d gold.", it.Amount)
		case IKHealth:
			p.HPot++
			g.msg(C(1, .4, .4), "You pick up a Healing Potion.")
		case IKMana:
			p.MPot++
			g.msg(C(.45, .6, 1), "You pick up a Mana Potion.")
		case IKScroll:
			p.Scrolls++
			g.msg(C(.85, .8, .65), "You pick up a Scroll of Town Portal.")
		default:
			keep = append(keep, fi)
		}
	}
	l.Items = keep
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
	for r := 0; r < 5; r++ {
		found := false
		for tries := 0; tries < 12; tries++ {
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
		g.textFx(m.X, m.Y, "miss", colGray)
		return
	}
	lo, hi := p.DmgRange()
	dmg := lo + g.rng.Intn(hi-lo+1)
	crit := g.rng.Intn(100) < p.Crit()
	if crit {
		dmg *= 2
	}
	dmg = int(float64(dmg) * (1 - float64(m.Armor)/float64(m.Armor+120)))
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
	i := l.Idx(m.X, m.Y)
	if l.At(m.X, m.Y) != TWater && l.At(m.X, m.Y) != TDeepWater {
		l.Decal[i] = DecalCorpse
		for k := 0; k < 3; k++ {
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
			g.hurtPlayer(m.Level*2+3, m.Name+"'s death flames")
		}
	}
	switch m.T.ID {
	case "boneking":
		g.Quests[0] = maxi(g.Quests[0], 1)
		g.msg(colOrange, "The crypts fall silent. Return to Captain Voss.")
	case "oracle":
		g.Quests[1] = maxi(g.Quests[1], 1)
		g.msg(colOrange, "The Oracle's song ends. Deeper still, something burns... Return to Voss.")
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
	for p.XP >= xpNext(p.Lvl) {
		p.XP -= xpNext(p.Lvl)
		p.Lvl++
		p.Points += 5
		p.recalc()
		p.HP, p.MP = float64(p.MaxHP()), float64(p.MaxMP())
		g.msg(colGold, "You have reached level %d! (c to spend 5 attribute points)", p.Lvl)
		g.novaFx(p.X, p.Y, colGold, 4)
	}
}

func (g *Game) dropLoot(m *Monster) {
	p := g.P
	mf := float64(p.S(StMF)) / 100
	nItems, bonus, minR := 0, mf, RNormal
	if g.rng.Intn(100) < 20 {
		nItems = 1
	}
	goldChance := 40
	switch m.Rank {
	case RankChampion:
		nItems, bonus, minR, goldChance = 1+g.rng.Intn(2), mf+1, RMagic, 90
	case RankUnique:
		nItems, bonus, minR, goldChance = 2+g.rng.Intn(2), mf+2.5, RMagic, 100
	case RankBoss:
		nItems, bonus, minR, goldChance = 5, mf+4, RRare, 100
	}
	if m.Minion {
		nItems, goldChance = 0, 10
	}
	for i := 0; i < nItems; i++ {
		r := RollRarity(g.rng, bonus)
		if r < minR {
			r = minR
		}
		if m.Rank == RankBoss && i == 0 {
			r = RUnique
		}
		g.dropItem(m.X, m.Y, GenItem(g.rng, m.Level, r, SlotNone))
	}
	if g.rng.Intn(100) < goldChance {
		amt := m.Level*3 + g.rng.Intn(m.Level*6+6)
		if m.Rank >= RankChampion {
			amt *= 3
		}
		amt = amt * (100 + p.S(StGoldFind)) / 100
		g.dropItem(m.X, m.Y, &Item{Kind: IKGold, Amount: amt})
	}
	r := g.rng.Intn(100)
	switch {
	case r < 12:
		g.dropItem(m.X, m.Y, NewPotion(IKHealth))
	case r < 18:
		g.dropItem(m.X, m.Y, NewPotion(IKMana))
	case r < 20:
		g.dropItem(m.X, m.Y, NewPotion(IKScroll))
	}
}

func (g *Game) openChest(x, y int) {
	l := g.Lv
	l.Set(x, y, TChestOpen)
	g.msg(C(.9, .7, .35), "You open the chest.")
	lvl := maxi(1, l.Depth)
	n := 1 + g.rng.Intn(2)
	for i := 0; i < n; i++ {
		r := RollRarity(g.rng, 0.8+float64(g.P.S(StMF))/100)
		if r == RNormal {
			r = RMagic
		}
		g.dropItem(x, y, GenItem(g.rng, lvl, r, SlotNone))
	}
	g.dropItem(x, y, &Item{Kind: IKGold, Amount: lvl*10 + g.rng.Intn(lvl*15+10)})
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
		xp := xpNext(p.Lvl) / 4
		g.msg(colPurple, "Forbidden knowledge floods your mind. (+%d XP)", xp)
		g.gainXP(xp)
	default:
		g.msg(colPurple, "The altar offers up a gift.")
		g.dropItem(x, y+1, GenItem(g.rng, g.Lv.Depth+1, RRare, SlotNone))
	}
	g.novaFx(x, y, colPurple, 3)
}

func (g *Game) hurtPlayer(dmg int, by string) {
	p := g.P
	if dmg < 1 {
		dmg = 1
	}
	p.HP -= float64(dmg)
	p.Flash = g.time
	g.auto = false
	g.textFx(p.X, p.Y, strconv.Itoa(dmg), colRed)
	if p.HP <= 0 {
		p.HP = 0
		p.KilledBy = by
		g.Mode = ModeDead
		g.msg(colRed, "You have been slain by %s.", by)
	}
}

func (g *Game) drinkHealth() {
	p := g.P
	if p.HPot <= 0 {
		g.msg(colDim, "You have no healing potions.")
		return
	}
	p.HPot--
	amt := float64(p.MaxHP())*0.5 + 12
	p.HP = math.Min(float64(p.MaxHP()), p.HP+amt)
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
	p.MPot--
	p.MP = math.Min(float64(p.MaxMP()), p.MP+float64(p.MaxMP())*0.6+8)
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
	x, y := l.FreeNear(p.X+1, p.Y, p.X, p.Y)
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
	for i := 0; i < 200; i++ {
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
	if p.MP < costFirebolt {
		g.msg(C(.45, .6, 1), "Not enough mana.")
		return
	}
	p.MP -= costFirebolt
	path, hit, _ := g.traceBolt(p.X, p.Y, t.X, t.Y, 22, true)
	g.boltFx(path, C(1, .5, .15), '*', true)
	if hit != nil {
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
	if p.MP < costNova {
		g.msg(C(.45, .6, 1), "Not enough mana.")
		return
	}
	p.MP -= costNova
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
		fr := 4
		if m.Rank >= RankUnique {
			fr = 2
		}
		m.Frozen = maxi(m.Frozen, fr)
		g.damageMonster(m, lo+g.rng.Intn(hi-lo+1), false, colCyan)
	}
	g.endTurn()
}

// ------------------------------------------------------------ monsters

func (g *Game) monsterTurn(m *Monster) {
	l, p := g.Lv, g.P
	if m.Friendly {
		if g.rng.Intn(8) == 0 {
			d := dirs8[g.rng.Intn(8)]
			nx, ny := m.X+d.X, m.Y+d.Y
			if cheb(nx, ny, m.HomeX, m.HomeY) <= 3 {
				g.stepMonster(m, nx, ny)
			}
		}
		return
	}
	if m.Frozen > 0 {
		m.Frozen--
		return
	}
	d := cheb(m.X, m.Y, p.X, p.Y)
	sees := g.fov[l.Idx(m.X, m.Y)] == g.fovGen && d <= 16
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
			for i := 0; i < 2; i++ {
				s := placeMonsterAvoid(l, "skel", m.X, m.Y, m.Level, RankNormal, p.X, p.Y)
				s.Awake, s.Minion = true, true
			}
			g.novaFx(m.X, m.Y, C(.5, 1, .45), 2.5)
			return
		}
	case AIOracle:
		if sees && m.Timer%9 == 0 {
			for tries := 0; tries < 30; tries++ {
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
			for i := 0; i < 2; i++ {
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

func (g *Game) monsterHitChance(m *Monster) int {
	p := g.P
	return clampi(m.ToHit-(p.ArmorVal()/4+p.DEX()/4)+10, 20, 95)
}

func (g *Game) monsterDamage(m *Monster) int {
	p := g.P
	dmg := m.MinD + g.rng.Intn(m.MaxD-m.MinD+1)
	a := float64(p.ArmorVal())
	red := a / (a + 50 + 10*float64(m.Level))
	return maxi(1, int(float64(dmg)*(1-red)+0.5))
}

func (g *Game) monsterMelee(m *Monster) {
	p := g.P
	if g.rng.Intn(100) >= g.monsterHitChance(m) {
		g.textFx(p.X, p.Y, "dodge", colGray)
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
	g.hurtPlayer(dmg, name)
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
		g.textFx(p.X, p.Y, "miss", colGray)
		return
	}
	name := "the " + m.Name
	if m.Rank >= RankUnique {
		name = m.Name
	}
	g.hurtPlayer(g.monsterDamage(m)*4/5+1, name)
}

// ------------------------------------------------------------ auto-explore

func (g *Game) autoStep() bool {
	l, p := g.Lv, g.P
	if len(g.visibleHostiles()) > 0 {
		g.msg(colOrange, "You spot an enemy.")
		return false
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
				if fi.X == cx && fi.Y == cy && fi.It.Kind != IKEquip {
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
			if g.Portal != nil && ((g.Portal.Level == l.ID && nx == g.Portal.X && ny == g.Portal.Y) || (l.Kind == KTown && nx == l.PortalAt.X && ny == l.PortalAt.Y)) {
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

func (g *Game) restock() {
	p := g.P
	smith := &Shop{Name: "Hadrik's Forge", Kind: 0}
	for i := 0; i < 11; i++ {
		r := RMagic
		switch x := g.rng.Intn(100); {
		case x < 30:
			r = RNormal
		case x < 85:
			r = RMagic
		default:
			r = RRare
		}
		slots := []Slot{SlotWeapon, SlotWeapon, SlotOffhand, SlotArmor, SlotHelm, SlotGloves, SlotBoots}
		smith.Items = append(smith.Items, GenItem(g.rng, p.Lvl+g.rng.Intn(3), r, slots[g.rng.Intn(len(slots))]))
	}
	alch := &Shop{Name: "Mirela's Remedies", Kind: 1}
	alch.Items = append(alch.Items, NewPotion(IKHealth), NewPotion(IKMana), NewPotion(IKScroll))
	for i := 0; i < 6; i++ {
		slots := []Slot{SlotRing, SlotAmulet, SlotRing, SlotHelm}
		r := RMagic
		if g.rng.Intn(6) == 0 {
			r = RRare
		}
		alch.Items = append(alch.Items, GenItem(g.rng, p.Lvl+g.rng.Intn(3), r, slots[g.rng.Intn(len(slots))]))
	}
	g.shops = [2]*Shop{smith, alch}
}

var villagerLines = []string{
	"They say the braziers in the crypt light themselves at dusk. Nobody tends them.",
	"My brother went into Blackmarsh after the lights. He came back wet. He came back wrong.",
	"Keep your torch high. The dark down there isn't empty — it's hungry.",
	"Hadrik's forge hasn't gone out in forty years. He says the day it does, we run.",
	"Rare things glimmer in the dark. Gold-light means something old and named.",
	"Old Mirela's crystal was dug out of the grotto. It sings when the Oracle dreams.",
	"A portal scroll is cheaper than a funeral.",
}

func (g *Game) talkTo(m *Monster) {
	p := g.P
	switch m.T.ID {
	case "smith":
		g.shop = g.shops[0]
		g.Mode, g.cur, g.tab = ModeShop, 0, 0
		g.msg(C(1, .6, .3), "Hadrik: \"Steel for the dark. Take a look.\"")
	case "alch":
		p.HP, p.MP = float64(p.MaxHP()), float64(p.MaxMP())
		g.shop = g.shops[1]
		g.Mode, g.cur, g.tab = ModeShop, 0, 0
		g.msg(colCyan, "Mirela tends your wounds. \"There. Now buy something, dear.\"")
	case "captain":
		lines := []string{}
		reward := false
		if g.Quests[0] == 1 {
			g.Quests[0] = 2
			reward = true
			lines = append(lines, "The Bone King is dust? Then the crypt bells may ring again.", "Take this — it was my father's. And the gold, of course.")
		}
		if g.Quests[1] == 1 {
			g.Quests[1] = 2
			reward = true
			lines = append(lines, "The Oracle, silenced... I never thought I'd sleep without hearing her.", "Beneath her pool, the rock burns. The Abyss has no floor, they say. Be careful.")
		}
		if !reward {
			switch {
			case g.Quests[0] == 0:
				lines = []string{
					"Wanderer. Good. We need a blade that doesn't shake.",
					"East of the gate, past the Ashen Fields, lies the Crypt of the Fallen.",
					"Four floors down, the Bone King sits his throne of ribs. Destroy him.",
					"And north, in Blackmarsh, the Drowned Oracle calls the lost into the water.",
					"Buy potions from Mirela. Keep a portal scroll. Come back alive.",
				}
			case g.Quests[1] == 0:
				lines = []string{"The crypt is quiet. Now: Blackmarsh, north of the fields.", "Follow the cold lights down into the grotto. End the Oracle."}
			default:
				lines = []string{"You've done more than any of us dared.", "If you must go deeper... the Abyss waits below the Oracle's pool."}
			}
		}
		if reward {
			p.Gold += 250 * p.Lvl
			it := GenItem(g.rng, p.Lvl+2, RUnique, SlotNone)
			g.dropItem(p.X, p.Y, it)
			g.msg(colGold, "Voss gives you %d gold and %s.", 250*p.Lvl, it.Name)
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
	price := it.Value()
	if p.Gold < price {
		g.msg(colRed, "You cannot afford that.")
		return
	}
	switch it.Kind {
	case IKHealth:
		p.HPot++
	case IKMana:
		p.MPot++
	case IKScroll:
		p.Scrolls++
	default:
		if len(p.Inv) >= invMax {
			g.msg(colRed, "Your pack is full.")
			return
		}
		p.Inv = append(p.Inv, it)
		items := g.shop.Items
		for i, x := range items {
			if x == it {
				g.shop.Items = append(items[:i:i], items[i+1:]...)
				break
			}
		}
	}
	p.Gold -= price
	g.msg(colGold, "Bought %s for %dg.", it.DisplayName(), price)
}

func (g *Game) sell(idx int) {
	p := g.P
	if idx < 0 || idx >= len(p.Inv) {
		return
	}
	it := p.Inv[idx]
	price := it.Value() / 4
	p.Gold += price
	p.Inv = append(p.Inv[:idx], p.Inv[idx+1:]...)
	if g.shop != nil && g.shop.Kind == 0 {
		g.shop.Items = append(g.shop.Items, it)
	}
	g.msg(colGold, "Sold %s for %dg.", it.Name, price)
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
