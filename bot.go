package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
)

// A scripted player, for balance runs (TestBotBalance) and for watching
// (-bot fighter|caster). It sees what a player sees (visibleHostiles, Seen,
// canSee), acts through the functions a key press reaches, and rolls its
// own dice, so a run is a function of the seed and the policy alone.

type BotState int

const (
	BotExplore BotState = iota
	BotFight
	BotRetreat
	BotLoot
	BotErrand
	BotDescend
	BotStuck
)

func (s BotState) String() string {
	return [...]string{"explore", "fight", "retreat", "loot", "errand", "descend", "stuck"}[s]
}

// botStuckAfter is how many turns without a new cell seen, a kill or a
// level change mark a run stuck.
const botStuckAfter = 300

// A botPolicy is a build: where level-up points go, how it fights, and what
// it wants from gear. An affix is worth statWeight × mul per point: the
// neutral power of the stat times how much this build cares. A fighter
// rates a point of Strength at 1.5× its neutral worth, a caster at 0.3×.
type botPolicy struct {
	name   string
	caster bool
	points [5]int           // attribute (0 Str, 1 Dex, 2 Vit, 3 Ene) for each point of a level-up
	mana   int              // mana potions to carry
	dmg    float64          // per point of average weapon damage
	armor  float64          // per point of armor
	mul    [StCount]float64 // on statWeight, per affix
}

var botPolicies = []*botPolicy{
	{name: "fighter", points: [5]int{0, 2, 0, 2, 0}, mana: 1, dmg: 6, armor: 1, mul: [StCount]float64{
		StStr: 1.5, StDex: 1.5, StVit: 1, StEne: .15, StLife: 1, StMana: .3, StDmgPct: 1.2, StFlatDmg: 1.5,
		StArmor: 1, StCrit: 1.5, StLifeSteal: 1.25, StLight: 1, StSpellPct: .1, StLifeRegen: 1, StManaRegen: .25,
		StMF: 1, StThorns: 1, StToHit: 1.2, StAllAttr: 1, StGoldFind: 1,
	}},
	{name: "caster", caster: true, points: [5]int{3, 2, 3, 2, 3}, mana: beltMax, dmg: .5, armor: 1, mul: [StCount]float64{
		StStr: .3, StDex: 1, StVit: 1, StEne: 2, StLife: 1, StMana: 1.6, StDmgPct: .2, StFlatDmg: .125,
		StArmor: 1, StCrit: .75, StLifeSteal: .25, StLight: 1, StSpellPct: 1.5, StLifeRegen: 1, StManaRegen: 2,
		StMF: 1, StThorns: 1, StToHit: .4, StAllAttr: 8.0 / 6, StGoldFind: 1,
	}},
}

func policyByName(name string) *botPolicy {
	for _, p := range botPolicies {
		if p.name == name {
			return p
		}
	}
	return nil
}

// gearScore rates an item for the build and says why: the two terms that
// matter most.
func (pol *botPolicy) gearScore(it *Item) (int, string) {
	type term struct {
		v float64
		s string
	}
	var ts []term
	if it.Base.Slot == SlotWeapon {
		ts = append(ts, term{pol.dmg * float64(it.MinD+it.MaxD) / 2, fmt.Sprintf("%d-%d damage", it.MinD, it.MaxD)})
	}
	if it.Armor > 0 {
		ts = append(ts, term{pol.armor * float64(it.Armor), fmt.Sprintf("%d armor", it.Armor)})
	}
	for _, a := range it.Aff {
		ts = append(ts, term{pol.mul[a.S] * a.power(), fmt.Sprintf(statFmt[a.S], a.V)})
	}
	sort.SliceStable(ts, func(i, j int) bool { return ts[i].v > ts[j].v })
	total := 0.0
	var why []string
	for i, t := range ts {
		total += t.v
		if i < 2 {
			why = append(why, t.s)
		}
	}
	return int(total), strings.Join(why, ", ")
}

type Bot struct {
	g   *Game
	pol *botPolicy
	rng *rand.Rand

	state BotState
	why   string
	n     int // turns taken

	level    *Level
	explored bool // auto-explore found nothing left here
	seen     int  // cells seen here at the last check
	kills    int  // player kills at the last check
	gold     int  // gold and pack size at the last check
	inv      int
	progress int              // turn of the last new cell, kill or level change
	ignore   map[*Monster]int // enemies with no path to them, until this turn
	prey     *Monster         // the enemy being chased: kept in sight, whatever the range
	used     map[cellKey]bool // altars tried, remembered across trips
	skip     map[Pos]bool     // spots with no known way there, until more is seen
	goal     Pos              // the loot being walked to
	front    Pos              // the frontier cell being walked to, or -1
	par      []int32          // search scratch

	errand  bool   // a town trip is under way
	urgent  bool   // ...because of need, not convenience
	shopped bool   // town business done; heading back
	forge   bool   // traded at Hadrik's this trip
	remedy  bool   // traded at Mirela's this trip
	reason  string // why the trip
	read    int    // turn of the last scroll read, so a fresh portal is used, not replaced
	stall   int    // turns spent waiting for a townsperson's doorway
	spent   int    // attribute points spent, for the policy's pattern

	visited map[string]bool // levels snapshotted
	Snaps   []botSnap
	Stats   BotStats
}

// A cellKey names a cell on a level.
type cellKey struct {
	level string
	p     Pos
}

// BotStats is what a run reports beyond g.Stats.
type BotStats struct {
	Trips   int // arrivals in town
	Deepest string
}

// A botSnap is the hero on first arrival at a level, for the
// power-versus-depth table.
type botSnap struct {
	Level                                                        string
	Turn, Lvl, HP, Armor, MinD, MaxD, BoltLo, BoltHi, Crit, Gold int
	Gear                                                         int    // gearScore of everything worn
	P                                                            Player // value copy: Eq copies item pointers, and items never change
}

func NewBot(g *Game, pol *botPolicy) *Bot {
	return &Bot{g: g, pol: pol, rng: rand.New(rand.NewSource(g.Seed)), why: "ready", visited: map[string]bool{}, used: map[cellKey]bool{}, front: Pos{-1, -1}, Stats: BotStats{Deepest: g.Lv.ID}}
}

func (b *Bot) State() BotState { return b.state }
func (b *Bot) Why() string     { return b.why }
func (b *Bot) Policy() string  { return b.pol.name }

func (b *Bot) say(f string, a ...any) { b.g.msg(colBot, "bot: "+f, a...) }

// enter moves to a state, logging the change and its reason.
func (b *Bot) enter(st BotState, why string) {
	b.why = why
	if st != b.state {
		b.state = st
		b.say("%s: %s", st, why)
	}
}

// leave is the esc key: out of a shop or a conversation.
func (b *Bot) leave() {
	if g := b.g; g.Mode == ModeShop || g.Mode == ModeTalk {
		g.Mode = ModePlay
	}
}

// turn plays one decision. It usually spends a game turn; trading and
// equipping are free, as they are for anyone.
func (b *Bot) turn() {
	g := b.g
	if g.Mode == ModeDead {
		return
	}
	b.n++
	b.leave()
	b.track()
	b.spend()
	b.wear()
	if b.n-b.progress > botStuckAfter {
		b.enter(BotStuck, fmt.Sprintf("no progress in %d turns", botStuckAfter))
		b.wander()
	} else {
		b.act()
	}
	b.leave()
	if botDepth(g.Lv.ID) > botDepth(b.Stats.Deepest) {
		b.Stats.Deepest = g.Lv.ID
	}
}

// track notices level changes and progress.
func (b *Bot) track() {
	g, l := b.g, b.g.Lv
	if l != b.level {
		if b.level != nil && l.Kind == KTown {
			b.Stats.Trips++
		}
		if b.level != nil && b.level.Kind == KTown { // the trip is over
			b.errand, b.shopped, b.forge, b.remedy = false, false, false, false
		}
		b.level, b.explored, b.seen, b.progress = l, false, -1, b.n
		if l.Kind != KTown && !b.visited[l.ID] {
			b.visited[l.ID] = true
			b.snapshot()
		}
		b.ignore = map[*Monster]int{}
		b.prey = nil
		b.skip = map[Pos]bool{}
		b.front = Pos{-1, -1}
		if len(b.par) != l.W*l.H {
			b.par = make([]int32, l.W*l.H)
		}
	}
	l.Spent[l.Idx(g.P.X, g.P.Y)] = true // for frontier, as autoStep does for itself
	seen := 0
	for _, s := range l.Seen {
		if s {
			seen++
		}
	}
	if seen != b.seen {
		if b.seen >= 0 {
			b.explored = false
			b.skip = map[Pos]bool{}
		}
		b.seen, b.progress = seen, b.n
	}
	if g.P.Kills != b.kills { // a kill can open a way that was blocked
		b.kills, b.progress = g.P.Kills, b.n
		b.explored = false
	}
	if g.P.Gold != b.gold || len(g.P.Inv) != b.inv { // a sweep for loot is progress too
		b.gold, b.inv, b.progress = g.P.Gold, len(g.P.Inv), b.n
	}
}

// snapshot records the hero as it arrives on a level.
func (b *Bot) snapshot() {
	g, p := b.g, b.g.P
	s := botSnap{Level: g.Lv.ID, Turn: g.Turn, Lvl: p.Lvl, HP: p.MaxHP(), Armor: p.ArmorVal(), Crit: p.Crit(), Gold: p.Gold, P: *p}
	s.MinD, s.MaxD = p.DmgRange()
	s.BoltLo, s.BoltHi = p.FireboltDmg()
	for _, it := range p.Eq {
		if it != nil {
			v, _ := b.pol.gearScore(it)
			s.Gear += v
		}
	}
	b.Snaps = append(b.Snaps, s)
}

// spend puts level-up points where the build wants them.
func (b *Bot) spend() {
	p := b.g.P
	if p.Points == 0 {
		return
	}
	var n [4]int
	for p.Points > 0 {
		attr := b.pol.points[b.spent%len(b.pol.points)]
		b.g.spendPoint(attr)
		n[attr]++
		b.spent++
	}
	var s []string
	for i, name := range []string{"Str", "Dex", "Vit", "Ene"} {
		if n[i] > 0 {
			s = append(s, fmt.Sprintf("+%d %s", n[i], name))
		}
	}
	b.say("level %d: %s", p.Lvl, strings.Join(s, " "))
}

// ------------------------------------------------------------ gear

// replaced is what an item would push out: the item, the score lost and
// a name for it. A two-handed weapon also costs the off-hand; a ring
// replaces the weaker ring.
func (b *Bot) replaced(it *Item) (*Item, int, string) {
	p := b.g.P
	score := func(cur *Item) int {
		if cur == nil {
			return 0
		}
		s, _ := b.pol.gearScore(cur)
		return s
	}
	var cur *Item
	switch it.Slot() {
	case SlotRing:
		cur = p.Eq[EqRing1]
		if r2 := p.Eq[EqRing2]; cur != nil && (r2 == nil || score(r2) < score(cur)) {
			cur = r2
		}
	case SlotWeapon:
		cur = p.Eq[EqWeapon]
		if off := p.Eq[EqOffhand]; it.Base.TwoHanded && off != nil {
			return cur, score(cur) + score(off), "both hands"
		}
	case SlotOffhand:
		cur = p.Eq[EqOffhand]
		if w := p.Eq[EqWeapon]; w != nil && w.Base.TwoHanded {
			return w, score(w), w.Name
		}
	default:
		cur = b.g.equippedFor(it)
	}
	if cur == nil {
		return nil, 0, "nothing"
	}
	return cur, score(cur), cur.Name
}

// delta is how much better an item is than what it would replace.
func (b *Bot) delta(it *Item) int {
	if it.Kind != IKEquip {
		return 0
	}
	s, _ := b.pol.gearScore(it)
	_, cur, _ := b.replaced(it)
	return s - cur
}

// wear equips every upgrade in the pack, best first. Free, like the
// inventory screen.
func (b *Bot) wear() {
	g, p := b.g, b.g.P
	for range invMax {
		best, bd := -1, 0
		for i, it := range p.Inv {
			if d := b.delta(it); d > bd {
				best, bd = i, d
			}
		}
		if best < 0 {
			return
		}
		it := p.Inv[best]
		s, why := b.pol.gearScore(it)
		cur, cs, name := b.replaced(it)
		b.say("equip %s [%d: %s] over %s [%d]", it.Name, s, why, name, cs)
		if cur != nil && cur == p.Eq[EqRing2] {
			g.unequip(EqRing2) // equip fills the empty ring slot
			if p.Eq[EqRing2] != nil {
				return // no room to take it off; equip would swap the better ring out
			}
		}
		g.equip(best)
		if best < len(p.Inv) && p.Inv[best] == it {
			return // the pack is too full to swap
		}
	}
}

// wants says whether a floor item is worth carrying: an upgrade, or magic
// and up to sell.
func (b *Bot) wants(it *Item) bool {
	n := len(b.g.P.Inv)
	if it.Kind != IKEquip || n >= invMax {
		return false
	}
	return b.delta(it) > 0 || (it.Rarity >= RMagic && n < invMax-1)
}

// ------------------------------------------------------------ the turn

// hp is the life to plan with: what is there plus what the next turn's
// drain of drunk potions brings back. The rest of the pool is too slow
// to count against a pack: with 17 life and 90 in the pool, one more
// round of bites ends the run.
func (b *Bot) hp() float64 {
	p := b.g.P
	next := math.Min(p.HealPool, p.HealPool*0.35+1)
	return (p.HP + next) / float64(p.MaxHP())
}

// threats are the visible enemies a player deals with now: the ones that
// stop auto-explore, less those with no way to reach them, plus the one
// being chased as long as it stays in sight. Awake is fair to read: the
// hover line says "unaware of you".
func (b *Bot) threats() []*Monster {
	g, p := b.g, b.g.P
	if b.prey != nil && (b.prey.Dead || !g.canSee(b.prey.X, b.prey.Y)) {
		b.prey = nil
	}
	var r []*Monster
	for _, m := range g.visibleHostiles() {
		if until, ok := b.ignore[m]; ok && b.n < until {
			continue
		}
		d := cheb(m.X, m.Y, p.X, p.Y)
		if d <= autoStopNear || (m.Awake && d <= autoStopAware) || m == b.prey {
			r = append(r, m)
		}
	}
	sort.SliceStable(r, func(i, j int) bool {
		return cheb(r[i].X, r[i].Y, p.X, p.Y) < cheb(r[j].X, r[j].Y, p.X, p.Y)
	})
	return r
}

func (b *Bot) act() {
	p, l := b.g.P, b.g.Lv
	hp := b.hp()
	ts := b.threats()
	if l.Kind != KTown {
		if why := b.losing(hp, ts); why != "" && b.escape(why) {
			return
		}
	}
	// A potion when the next turn's life is low and the pool has room for
	// one: the game refuses a drink past full life without spending a
	// turn, and the bot must not ask again and again.
	if hp < .5 && p.HPot > 0 && p.HP+p.HealPool < float64(p.MaxHP()) {
		b.drink()
		return
	}
	if len(ts) > 0 {
		b.fight(ts)
		return
	}
	if !b.errand {
		if want, urgent, why := b.wantsTown(); want {
			b.errand, b.urgent, b.reason = true, urgent, why
		} else if l.Kind == KTown && !b.shopped {
			b.errand, b.urgent, b.reason = true, false, "shopping"
		}
	}
	if b.errand && b.runErrand() {
		return
	}
	if b.loot() || b.explore() || b.descend() {
		return
	}
	if l.Kind == KTown {
		b.enter(BotDescend, "looking for the gate")
	} else {
		b.enter(BotExplore, "nothing to do here")
	}
	b.wander()
}

// losing says why a fight is not worth staying in: a portal takes two
// turns (read, step) under the pack's blows, so the call comes while
// there is life to spend on it, but only once the belt is nearly empty;
// while potions last, a potion beats two turns of exposure.
func (b *Bot) losing(hp float64, ts []*Monster) string {
	p := b.g.P
	if len(ts) == 0 {
		return ""
	}
	pct := int(hp * 100)
	switch {
	case p.HPot == 0 && hp < .5:
		return fmt.Sprintf("%d%% life, belt empty", pct)
	case p.HPot == 1 && hp < .4:
		return fmt.Sprintf("%d%% life, one potion left", pct)
	}
	return ""
}

func (b *Bot) drink() {
	b.say("potion at %d%% life, %d left", int(b.hp()*100), b.g.P.HPot-1)
	b.g.drinkHealth()
}

// escape leaves for town: through the open portal when it is close or
// just opened, else through a fresh one. It reports whether it acted.
func (b *Bot) escape(why string) bool {
	g, p, l := b.g, b.g.P, b.g.Lv
	if l.Kind == KTown {
		return false
	}
	if g.Portal != nil && g.Portal.Level == l.ID {
		fresh := b.n-b.read < 20
		if fresh || p.Scrolls == 0 || cheb(g.Portal.X, g.Portal.Y, p.X, p.Y) <= 8 {
			b.enter(BotRetreat, why+", to the portal")
			b.errand, b.urgent, b.reason = true, true, why
			return b.walkTo(g.Portal.X, g.Portal.Y, true)
		}
	}
	if p.Scrolls == 0 {
		return false
	}
	b.enter(BotRetreat, why+", portal out")
	b.errand, b.urgent, b.reason = true, true, why
	b.read = b.n
	g.readPortal()
	return true
}

// target picks m the way tab does.
func (b *Bot) target(m *Monster) {
	g := b.g
	for range len(g.visibleHostiles()) {
		if g.Target == m {
			return
		}
		g.cycleTarget(1)
	}
}

func (b *Bot) fight(ts []*Monster) {
	g, p := b.g, b.g.P
	var adj []*Monster
	for _, m := range ts {
		if cheb(m.X, m.Y, p.X, p.Y) == 1 {
			adj = append(adj, m)
		}
	}
	cost := float64(p.FireboltCost())
	if len(adj) >= 2 {
		if b.pol.caster && p.MP >= float64(p.NovaCost()) {
			b.enter(BotFight, fmt.Sprintf("nova, %d adjacent", len(adj)))
			g.castNova()
			return
		}
		if b.open(p.X, p.Y) > 3 && b.retreat(len(adj)) {
			return
		}
	}
	if len(adj) > 0 {
		t := adj[0]
		for _, m := range adj {
			if m == g.Target {
				t = m
			}
		}
		if t != g.Target {
			for _, m := range adj {
				if m.HP < t.HP {
					t = m
				}
			}
		}
		b.prey = t
		verb := "melee "
		if b.pol.caster {
			switch {
			case p.MP >= cost:
				b.enter(BotFight, "firebolt "+t.Name+" point-blank")
				b.target(t)
				g.castFirebolt()
				return
			case p.MPot > 0:
				b.enter(BotFight, "mana potion, "+t.Name+" adjacent")
				g.drinkMana()
				return
			case b.escape("mana dry"):
				return
			}
			verb = "dry and no scroll, melee "
		}
		b.enter(BotFight, verb+t.Name)
		g.move(t.X-p.X, t.Y-p.Y)
		return
	}
	t := ts[0]
	d := cheb(t.X, t.Y, p.X, p.Y)
	if d <= fireboltRange {
		if _, hit, _ := g.traceBolt(p.X, p.Y, t.X, t.Y, fireboltRange, true); hit != nil {
			if p.MP >= cost {
				b.enter(BotFight, "firebolt "+t.Name)
				b.target(t)
				g.castFirebolt()
				return
			}
			if p.MPot > 0 && (b.pol.caster || d >= 3) {
				b.enter(BotFight, "mana potion, "+t.Name+" at range")
				g.drinkMana()
				return
			}
		}
	}
	if b.pol.caster && p.MP < cost && p.MPot == 0 && b.escape("mana dry") {
		return
	}
	b.enter(BotFight, "closing on "+t.Name)
	b.search(t.X, t.Y, 30) // a longer way round is not worth the chase
	if i := g.Lv.Idx(t.X, t.Y); b.par[i] >= 0 && b.stepTo(i) {
		b.prey = t
		return
	}
	b.ignore[t] = b.n + 40
	b.prey = nil
	b.enter(BotFight, "no way to reach "+t.Name)
	g.wait()
}

// open counts the known walkable cells around (x,y): a corridor has two.
func (b *Bot) open(x, y int) int {
	l := b.g.Lv
	n := 0
	for _, d := range dirs8 {
		nx, ny := x+d.X, y+d.Y
		if l.In(nx, ny) && l.Seen[l.Idx(nx, ny)] && l.Walkable(nx, ny) {
			n++
		}
	}
	return n
}

// retreat backs into the nearest corridor cell within a few steps, so the
// pack has to come one at a time.
// retreat backs into the nearest tighter spot within a few steps: a
// corridor cell, or in a cave a nook with three open sides, so fewer of
// the pack can reach at once. A cell is worth it when it has fewer open
// sides than here.
func (b *Bot) retreat(n int) bool {
	l, p := b.g.Lv, b.g.P
	here := b.open(p.X, p.Y)
	order := b.search(-1, -1, 5)
	best, bo := -1, here
	for _, i := range order[1:] {
		if o := b.open(i%l.W, i/l.W); o < bo || (o == bo && best < 0 && o <= 2) {
			best, bo = i, o
		}
		if bo <= 2 {
			break // a corridor: nothing tighter is worth the walk
		}
	}
	if best < 0 || bo > 3 {
		return false
	}
	b.enter(BotRetreat, fmt.Sprintf("%d adjacent, backing into a spot with %d open sides", n, bo))
	return b.stepTo(best)
}

// ------------------------------------------------------------ town

// wantsTown says whether to go shopping, whether it is need or
// convenience, and why.
func (b *Bot) wantsTown() (want, urgent bool, why string) {
	g, p := b.g, b.g.P
	if g.Lv.Kind == KTown {
		return false, false, ""
	}
	broke := p.Gold < g.buyPrice(NewPotion(IKHealth)) && len(p.Inv) == 0
	switch {
	case p.HPot == 0 && !broke:
		return true, true, "belt empty"
	case b.pol.caster && p.MP+p.ManaPool < float64(p.FireboltCost()) && p.MPot == 0 && !broke:
		return true, true, "mana dry"
	case g.Quests[0] == 1 || g.Quests[1] == 1:
		return true, false, "bounty to collect"
	case len(p.Inv) >= invMax-2:
		return true, false, "pack full"
	}
	return false, false, ""
}

// runErrand runs the town trip: get there, heal, collect, sell, buy, come
// back. It reports whether it used the turn.
func (b *Bot) runErrand() bool {
	g, p, l := b.g, b.g.P, b.g.Lv
	if l.Kind != KTown {
		if g.Portal != nil && g.Portal.Level == l.ID {
			b.enter(BotErrand, b.reason+", to the portal")
			if b.walkTo(g.Portal.X, g.Portal.Y, true) {
				return true
			}
		}
		if p.Scrolls >= 2 || (b.urgent && p.Scrolls == 1) {
			b.enter(BotErrand, b.reason+", portal out")
			b.read = b.n
			g.readPortal()
			return true
		}
		home := botHome(l.ID)
		b.enter(BotErrand, b.reason+", walking home via "+home)
		if x, y, ok := b.stairs(home); ok && b.walkTo(x, y, true) {
			return true
		}
		return false
	}
	if p.HP < float64(p.MaxHP()) && b.fountain() {
		return true
	}
	if g.Quests[0] == 1 || g.Quests[1] == 1 {
		if acted, _ := b.visit("captain", func() { b.say("collect the bounty") }); acted {
			return true
		}
	}
	if b.loot() { // Voss leaves his gift on the ground
		return true
	}
	if !b.forge {
		if acted, found := b.visit("smith", b.tradeForge); acted || (found && b.linger()) {
			return true
		}
		b.forge = true // nobody there, or no way through: give up on him this trip
	}
	if !b.remedy {
		if acted, found := b.visit("alch", b.tradeRemedies); acted || (found && b.linger()) {
			return true
		}
		b.remedy = true
	}
	b.shopped = true
	if g.Portal != nil && b.next(g.Portal.Level) != botHome(g.Portal.Level) {
		b.enter(BotErrand, "back through the portal")
		return b.walkTo(l.PortalAt.X, l.PortalAt.Y, true)
	}
	b.errand = false // walk down; track ends the trip at the gate
	return false
}

// visit walks to a townsperson and, once alongside, walks into them and
// runs trade. It reports whether it acted, and whether anyone by that
// name is here at all.
func (b *Bot) visit(id string, trade func()) (acted, found bool) {
	g, p := b.g, b.g.P
	for _, m := range g.Lv.Monsters {
		if m.T.ID != id {
			continue
		}
		b.stall = 0
		if cheb(m.X, m.Y, p.X, p.Y) == 1 {
			b.enter(BotErrand, "trading with "+m.Name)
			g.move(m.X-p.X, m.Y-p.Y)
			trade()
			b.leave()
			return true, true
		}
		b.enter(BotErrand, "to "+m.Name)
		return b.walkTo(m.X, m.Y, true), true
	}
	return false, false
}

// linger waits a few turns for a doorway to clear. It reports whether it
// is still worth waiting.
func (b *Bot) linger() bool {
	if b.stall >= 6 {
		b.stall = 0
		return false
	}
	b.stall++
	b.enter(BotErrand, "waiting for the way to clear")
	b.g.wait()
	return true
}

// fountain walks into the nearest known fountain: free life, no mana.
func (b *Bot) fountain() bool {
	l, p := b.g.Lv, b.g.P
	bx, by, bd := 0, 0, 1<<30
	for i, t := range l.T {
		x, y := i%l.W, i/l.W
		if t == TFountain && l.Seen[i] && !b.skip[Pos{x, y}] {
			if d := cheb(x, y, p.X, p.Y); d < bd {
				bx, by, bd = x, y, d
			}
		}
	}
	if bd == 1<<30 {
		return false
	}
	b.enter(BotErrand, "to the fountain")
	if b.walkTo(bx, by, true) {
		return true
	}
	b.skip[Pos{bx, by}] = true
	return false
}

// reserve is the gold to keep for potions and scrolls before buying gear.
func (b *Bot) reserve() int {
	g, p := b.g, b.g.P
	r := g.buyPrice(NewPotion(IKHealth))*(beltMax-p.HPot) + g.buyPrice(NewPotion(IKScroll))*maxi(0, 2-p.Scrolls)
	return r + g.buyPrice(NewPotion(IKMana))*maxi(0, b.pol.mana-p.MPot)
}

// tradeForge sells, buys upgrades, and spends what is left on Hadrik's
// services: fresh stock when nothing there beats what is worn, and one
// gamble on the weakest slot when there is gold to burn.
func (b *Bot) tradeForge() {
	g, p := b.g, b.g.P
	b.sellJunk()
	reserve := b.reserve()
	b.buyGear(reserve)
	if it := b.service(IKReroll, SlotNone); it != nil && !b.upgradeInStock() {
		if price := g.buyPrice(it); p.Gold-reserve >= price {
			b.say("reroll for %dg: nothing in stock beats what I wear, %dg after the reserve", price, p.Gold-reserve)
			g.buy(it)
			b.buyGear(reserve)
		}
	}
	if it := b.weakestGamble(); it != nil {
		if price := g.buyPrice(it); p.Gold-reserve > 2*price {
			_, cs, name := b.replaced(it)
			b.say("gamble on %s for %dg: wearing %s [%d], %dg after the reserve", it.Name, price, name, cs, p.Gold-reserve)
			g.buy(it)
			b.wear()
		}
	}
	b.sellJunk()
	b.forge = true
}

// service finds a shop service by kind, and slot for a gamble.
func (b *Bot) service(kind ItemKind, slot Slot) *Item {
	for _, it := range b.g.shop.Services {
		if it.Kind == kind && (slot == SlotNone || it.Slot() == slot) {
			return it
		}
	}
	return nil
}

// upgradeInStock says whether anything for sale beats what is worn.
func (b *Bot) upgradeInStock() bool {
	for _, it := range b.g.shop.Items {
		if b.delta(it) > 0 {
			return true
		}
	}
	return false
}

// weakestGamble is the gamble on the slot whose worn item scores least.
func (b *Bot) weakestGamble() *Item {
	var best *Item
	bs := 1 << 30
	for _, it := range b.g.shop.Services {
		if it.Kind != IKGamble {
			continue
		}
		if _, cs, _ := b.replaced(it); cs < bs {
			best, bs = it, cs
		}
	}
	return best
}

func (b *Bot) tradeRemedies() {
	g, p := b.g, b.g.P
	var hp, mp, sc *Item
	for _, it := range g.shop.Items {
		switch it.Kind {
		case IKHealth:
			hp = it
		case IKMana:
			mp = it
		case IKScroll:
			sc = it
		}
	}
	buy := func(it *Item, n int) {
		for k := 0; k < n && it != nil && p.Gold >= g.buyPrice(it); k++ {
			g.buy(it)
		}
	}
	if p.Scrolls == 0 {
		buy(sc, 1) // the reserve comes first
	}
	buy(hp, beltMax-p.HPot)
	buy(sc, 2-p.Scrolls)
	buy(mp, b.pol.mana-p.MPot)
	b.say("restocked: %d healing, %d mana, %d scrolls, %dg left", p.HPot, p.MPot, p.Scrolls, p.Gold)
	b.sellJunk()
	b.buyGear(0)
	b.sellJunk()
	b.remedy = true
}

// sellJunk sells everything that is not an upgrade.
func (b *Bot) sellJunk() {
	g, p := b.g, b.g.P
	for i := len(p.Inv) - 1; i >= 0; i-- {
		it := p.Inv[i]
		if d := b.delta(it); d <= 0 {
			s, _ := b.pol.gearScore(it)
			_, cs, name := b.replaced(it)
			b.say("sell %s for %dg [%d vs %s %d]", it.Name, sellPrice(it, g.Rules), s, name, cs)
			g.sell(i)
		}
	}
}

// buyGear buys the best affordable upgrade, then the next, keeping the
// reserve.
func (b *Bot) buyGear(reserve int) {
	g, p := b.g, b.g.P
	for range 4 {
		var best *Item
		bd := 0
		for _, it := range g.shop.Items {
			if d := b.delta(it); d > bd && g.buyPrice(it) <= p.Gold-reserve {
				best, bd = it, d
			}
		}
		if best == nil {
			return
		}
		s, why := b.pol.gearScore(best)
		_, cs, name := b.replaced(best)
		b.say("buy %s for %dg [%d: %s] over %s [%d]", best.Name, g.buyPrice(best), s, why, name, cs)
		g.buy(best)
		b.wear()
	}
}

// ------------------------------------------------------------ the way down

// splitID takes "crypt3" apart into "crypt" and 3.
func splitID(id string) (string, int) {
	i := len(id)
	for i > 0 && id[i-1] >= '0' && id[i-1] <= '9' {
		i--
	}
	n := 0
	for _, c := range id[i:] {
		n = n*10 + int(c-'0')
	}
	return id[:i], n
}

// botDepth orders levels along the route, for "deepest reached".
func botDepth(id string) int {
	switch prefix, n := splitID(id); prefix {
	case "fields":
		return 1
	case "crypt":
		return 1 + n
	case "marsh":
		return 6
	case "grotto":
		return 6 + n
	case "abyss":
		return 9 + n
	}
	return 0
}

// botHome is the way back up.
func botHome(id string) string {
	switch id {
	case "fields":
		return "town"
	case "crypt1", "marsh":
		return "fields"
	case "grotto1":
		return "marsh"
	case "abyss1":
		return "grotto3"
	}
	prefix, n := splitID(id)
	return fmt.Sprintf("%s%d", prefix, n-1)
}

// next is where the route leads from a finished level: town, fields, the
// crypt to the Bone King, Voss for the bounty, the marsh, the grotto, the
// abyss.
func (b *Bot) next(id string) string {
	prefix, n := splitID(id)
	switch {
	case id == "town":
		return "fields"
	case id == "fields":
		if b.g.Quests[0] == 0 {
			return "crypt1"
		}
		return "marsh"
	case prefix == "crypt" && b.g.Quests[0] > 0: // the crypt is done: back up and on to the marsh
		return botHome(id)
	case id == "crypt4":
		return ""
	case id == "marsh":
		return "grotto1"
	case id == "grotto3":
		return "abyss1"
	}
	return fmt.Sprintf("%s%d", prefix, n+1)
}

// stairs finds a seen cell of the link to a level.
func (b *Bot) stairs(to string) (int, int, bool) {
	l := b.g.Lv
	lk := l.LinkTo(to)
	if lk == nil {
		return 0, 0, false
	}
	for y := lk.Y0; y <= lk.Y1; y++ {
		for x := lk.X0; x <= lk.X1; x++ {
			if l.In(x, y) && l.Seen[l.Idx(x, y)] {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// lootAt names what is worth having on a remembered cell: an item worth
// carrying, a chest, an altar not yet tried. Loot seen once is
// remembered, like the map.
func (b *Bot) lootAt(x, y int) string {
	l := b.g.Lv
	if !l.In(x, y) || !l.Seen[l.Idx(x, y)] || b.used[cellKey{l.ID, Pos{x, y}}] || b.skip[Pos{x, y}] {
		return ""
	}
	if t := l.At(x, y); t == TChest || t == TAltar {
		return tdefs[t].Name
	}
	for _, fi := range l.ItemsAt(x, y) {
		if b.wants(fi.It) {
			return fi.It.Name
		}
	}
	return ""
}

// loot walks to the nearest thing worth having and keeps going there
// until it is had or found out of reach.
func (b *Bot) loot() bool {
	g, l, p := b.g, b.g.Lv, b.g.P
	for _, fi := range l.ItemsAt(p.X, p.Y) {
		if b.wants(fi.It) {
			b.enter(BotLoot, "pick up "+fi.It.Name)
			g.pickup()
			return true
		}
	}
	what := b.lootAt(b.goal.X, b.goal.Y)
	if what == "" {
		bd := 1 << 30
		consider := func(x, y int) {
			if d := cheb(x, y, p.X, p.Y); d < bd {
				if name := b.lootAt(x, y); name != "" {
					b.goal, bd, what = Pos{x, y}, d, name
				}
			}
		}
		for _, fi := range l.Items {
			consider(fi.X, fi.Y)
		}
		for i, t := range l.T {
			if t == TChest || t == TAltar {
				consider(i%l.W, i/l.W)
			}
		}
		if what == "" {
			return false
		}
	}
	b.enter(BotLoot, "to "+what)
	if l.At(b.goal.X, b.goal.Y) == TAltar && cheb(b.goal.X, b.goal.Y, p.X, p.Y) <= 1 {
		b.used[cellKey{l.ID, b.goal}] = true // one bump is all an altar gets
	}
	if b.walkTo(b.goal.X, b.goal.Y, false) {
		return true
	}
	b.skip[b.goal] = true
	return false
}

// explore lets auto-explore drive. When enemies it cannot reach keep
// auto-explore from moving, it walks to the frontier itself.
func (b *Bot) explore() bool {
	g, l := b.g, b.g.Lv
	if l.Kind == KTown || b.explored {
		return false
	}
	b.enter(BotExplore, "auto-explore")
	// When an enemy it gave up on would stop auto-explore, the bot walks
	// to the frontier itself, and keeps that goal until it gets there:
	// auto-explore aims elsewhere, and handing the turn back and forth at
	// the edge of the enemy's reach would step there and back for good.
	if b.blocked() || b.front.X >= 0 {
		if b.front.X < 0 || !b.frontier(b.front.X, b.front.Y) {
			b.front = Pos{-1, -1}
			for _, i := range b.search(-1, -1, 1<<30)[1:] {
				if b.frontier(i%l.W, i/l.W) {
					b.front = Pos{i % l.W, i / l.W}
					break
				}
			}
		}
		if b.front.X < 0 {
			b.explored = true
			return false
		}
		b.enter(BotExplore, "to the frontier, past what auto-explore will not")
		if b.walkTo(b.front.X, b.front.Y, false) {
			return true
		}
		l.Spent[l.Idx(b.front.X, b.front.Y)] = true // no way there
		b.front = Pos{-1, -1}
		return false
	}
	g.autoItems = 1 << 30 // never stop for loot: pickups are automatic
	if g.autoStep() {
		return true
	}
	if g.Lv != l || g.Mode == ModeDead {
		return true
	}
	b.explored = true
	return false
}

// blocked says whether an enemy the bot gave up on would stop auto-explore.
func (b *Bot) blocked() bool {
	g, p := b.g, b.g.P
	for _, m := range g.visibleHostiles() {
		d := cheb(m.X, m.Y, p.X, p.Y)
		if until, ok := b.ignore[m]; ok && b.n < until && (d <= autoStopNear || (m.Awake && d <= autoStopAware)) {
			return true
		}
	}
	return false
}

// descend takes the stairs on the route, restocking first when the belt is
// low and gold allows.
func (b *Bot) descend() bool {
	g, l, p := b.g, b.g.Lv, b.g.P
	to := b.next(l.ID)
	if to == "" {
		return false
	}
	x, y, ok := b.stairs(to)
	if !ok {
		return false
	}
	// Top up before going deeper, when a spare scroll makes it cheap or
	// town is close.
	if l.Kind != KTown && p.HPot < 3 && p.Gold >= 3*g.buyPrice(NewPotion(IKHealth)) && (p.Scrolls >= 2 || botDepth(l.ID) <= 2) {
		b.errand, b.urgent, b.reason = true, false, "restock before going deeper"
		return b.runErrand()
	}
	b.enter(BotDescend, "stairs to "+to)
	if b.walkTo(x, y, true) {
		b.progress = b.n // a long walk across a cleared level is not a stall
		return true
	}
	return false
}

// wander takes a random step, the bot's own dice, away from stairs and
// portals.
func (b *Bot) wander() {
	g, p := b.g, b.g.P
	for _, k := range b.rng.Perm(8) {
		d := dirs8[k]
		x, y := p.X+d.X, p.Y+d.Y
		if b.pass(x, y) {
			g.move(d.X, d.Y)
			return
		}
	}
	g.wait()
}

// ------------------------------------------------------------ walking

// frontier says whether a cell borders the unknown.
func (b *Bot) frontier(x, y int) bool {
	l := b.g.Lv
	if !l.In(x, y) || l.Spent[l.Idx(x, y)] {
		return false // stood there already: what is unseen from it stays unseen
	}
	for _, d := range dirs8 {
		if nx, ny := x+d.X, y+d.Y; l.In(nx, ny) && !l.Seen[l.Idx(nx, ny)] {
			return true
		}
	}
	return false
}

// pass says whether a step may cross a cell: known, walkable, not a
// link or portal, nobody visible on it.
func (b *Bot) pass(x, y int) bool {
	g, l := b.g, b.g.Lv
	if !l.In(x, y) || !l.Seen[l.Idx(x, y)] || !l.Walkable(x, y) || l.LinkAt(x, y) != nil || g.portalAt(x, y) {
		return false
	}
	return l.MonsterAt(x, y) == nil || !g.canSee(x, y)
}

// search floods out from the player through passable cells up to depth
// steps, recording parents in b.par, and returns the cells in the order
// found. The goal cell, if any, is entered but not crossed, so a chest,
// a monster or the stairs can be a goal.
func (b *Bot) search(gx, gy, depth int) []int {
	l, p := b.g.Lv, b.g.P
	W := l.W
	for i := range b.par {
		b.par[i] = -1
	}
	start := l.Idx(p.X, p.Y)
	goal := -1
	if l.In(gx, gy) {
		goal = l.Idx(gx, gy)
	}
	if goal >= 0 && !l.Seen[goal] {
		goal = -1
	}
	b.par[start] = int32(start)
	order := []int{start}
	for d, k := 0, 0; d < depth && k < len(order); d++ {
		end := len(order)
		for ; k < end; k++ {
			c := order[k]
			if c == goal {
				continue
			}
			cx, cy := c%W, c/W
			for _, dd := range dirs8 {
				nx, ny := cx+dd.X, cy+dd.Y
				if !l.In(nx, ny) {
					continue
				}
				ni := ny*W + nx
				if b.par[ni] >= 0 || (ni != goal && !b.pass(nx, ny)) {
					continue
				}
				b.par[ni] = int32(c)
				order = append(order, ni)
				if ni == goal {
					return order
				}
			}
		}
	}
	return order
}

// stepTo takes the first step toward a cell found by search. It reports
// whether the game moved on.
func (b *Bot) stepTo(i int) bool {
	g, l, p := b.g, b.g.Lv, b.g.P
	start := l.Idx(p.X, p.Y)
	for int(b.par[i]) != start {
		i = int(b.par[i])
	}
	turn, lv := g.Turn, g.Lv
	g.move(i%l.W-p.X, i/l.W-p.Y)
	return g.Turn != turn || g.Lv != lv || g.Mode != ModePlay
}

// walkTo steps toward a cell along known ground. With no known way there
// and near set, it heads for the edge of the known map nearest to it,
// which is what anyone does with a half-explored map. It reports whether
// it moved.
func (b *Bot) walkTo(x, y int, near bool) bool {
	l, p := b.g.Lv, b.g.P
	if x == p.X && y == p.Y {
		return false
	}
	order := b.search(x, y, 1<<30)
	goal := l.Idx(x, y)
	if b.par[goal] >= 0 {
		return b.stepTo(goal)
	}
	if !near {
		return false
	}
	best, bd, edge := -1, cheb(x, y, p.X, p.Y), false
	for _, i := range order[1:] {
		cx, cy := i%l.W, i/l.W
		f, d := b.frontier(cx, cy), cheb(x, y, cx, cy)
		if (f && !edge) || (f == edge && d < bd) {
			best, bd, edge = i, d, f
		}
	}
	if best < 0 {
		return false
	}
	return b.stepTo(best)
}
