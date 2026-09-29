package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// A scripted player that fights, loots, and heads down the crypt. Its runs
// over many seeds form a balance report; see TestBotBalance.

// botRoute is the way down: where the bot goes once a level is explored.
var botRoute = map[string]string{
	"town": "fields", "fields": "crypt1",
	"crypt1": "crypt2", "crypt2": "crypt3", "crypt3": "crypt4",
}

var botDepth = map[string]int{"town": 0, "fields": 1, "crypt1": 2, "crypt2": 3, "crypt3": 4, "crypt4": 5}

// botLevelBudget is how many turns the bot explores a level before it
// heads for the way down regardless.
const botLevelBudget = 900

type bot struct {
	g       *Game
	dist    map[string][]int // BFS maps by level and goal
	onLevel int              // turns spent on the current level
	level   *Level
	homing  bool // out of potions: portal to town, restock, come back
	shopped bool
	trips   int
}

// wantsTown: a sensible player goes home when the potions run out.
func (b *bot) wantsTown() bool {
	p := b.g.P
	return p.HPot == 0 && (p.Scrolls > 0 || b.g.Portal != nil)
}

// errand runs the town trip. It reports whether it used the turn.
func (b *bot) errand() bool {
	g := b.g
	p, l := g.P, g.Lv
	if l.Kind != KTown {
		if b.shopped { // back from town
			b.homing, b.shopped = false, false
			return false
		}
		if g.Portal == nil || g.Portal.Level != l.ID {
			if p.Scrolls == 0 {
				b.homing = false
				return false
			}
			g.readPortal()
			b.shopped = false
			b.trips++
			return true
		}
		return b.walkTo(g.Portal.X, g.Portal.Y)
	}
	if !b.shopped {
		alch := g.shops[1]
		for _, m := range l.Monsters {
			if m.T.ID != "alch" {
				continue
			}
			if cheb(m.X, m.Y, p.X, p.Y) == 1 {
				g.move(m.X-p.X, m.Y-p.Y) // heals, opens the shop
				hp, mp, sc := alch.Items[0], alch.Items[1], alch.Items[2]
				reserve := g.buyPrice(sc)
				for p.Gold >= g.buyPrice(hp)+reserve && p.HPot < beltMax {
					g.buy(hp)
				}
				if p.Scrolls == 0 && p.Gold >= reserve {
					g.buy(sc)
				}
				for p.Gold >= g.buyPrice(mp)+reserve && p.MPot < 3 {
					g.buy(mp)
				}
				g.Mode = ModePlay
				b.shopped = true
				return true
			}
			return b.walkTo(m.X, m.Y)
		}
	}
	if g.Portal == nil {
		b.homing = false // portal gone; walk back like anyone else
		return false
	}
	b.walkTo(l.PortalAt.X, l.PortalAt.Y)
	return true
}

// walkTo takes one step toward (x,y) along a BFS map.
func (b *bot) walkTo(x, y int) bool {
	l := b.g.Lv
	key := fmt.Sprintf("%s:%d:%d", l.ID, x, y)
	d, ok := b.dist[key]
	if !ok {
		d = bfsDist(l, x, y)
		b.dist[key] = d
	}
	return b.stepDown(d)
}

func (b *bot) turn() {
	g := b.g
	p := g.P
	if g.Lv != b.level {
		b.level, b.onLevel = g.Lv, 0
	}
	b.onLevel++
	if p.HP+p.HealPool < float64(p.MaxHP())*0.5 && p.HPot > 0 {
		g.drinkHealth()
		return
	}
	// out of potions and hurting: escape through a portal, even mid-fight
	if p.HPot == 0 && p.HP+p.HealPool < float64(p.MaxHP())*0.45 && g.Lv.Kind != KTown && b.wantsTown() {
		b.homing = true
		if b.errand() {
			return
		}
	}
	adj := 0
	for _, m := range g.visibleHostiles() {
		if cheb(m.X, m.Y, p.X, p.Y) <= 1 {
			adj++
		}
	}
	if adj >= 3 && p.MP >= costNova {
		g.castNova()
		return
	}
	if t := g.nearestHostile(); t != nil && cheb(t.X, t.Y, p.X, p.Y) <= autoStopAware {
		if cheb(t.X, t.Y, p.X, p.Y) == 1 {
			g.move(t.X-p.X, t.Y-p.Y)
			return
		}
		if p.MP >= costFirebolt && cheb(t.X, t.Y, p.X, p.Y) <= fireboltRange {
			g.Target = t
			g.castFirebolt()
			return
		}
		ox, oy, ot := p.X, p.Y, g.Turn
		g.move(sign(t.X-p.X), sign(t.Y-p.Y))
		if ox == p.X && oy == p.Y && ot == g.Turn {
			g.wait()
		}
		return
	}
	for _, fi := range g.Lv.ItemsAt(p.X, p.Y) {
		if fi.It.Kind == IKEquip {
			g.pickup()
			return
		}
	}
	if !b.homing && b.wantsTown() {
		b.homing = true
	}
	if b.homing && b.errand() {
		// on the way
	} else if b.onLevel > botLevelBudget && b.descend() {
		// done here
	} else if !g.autoStep() && !b.descend() {
		d := dirs8[g.rng.Intn(8)]
		g.move(d.X, d.Y)
	}
	for i := len(p.Inv) - 1; i >= 0; i-- {
		if g.upgradeHint(p.Inv[i]) != "" {
			g.equip(i)
		}
	}
	for p.Points > 0 { // a caster: spells scale with Energy alone
		p.Vit++
		p.Ene++
		p.Points -= 2
	}
}

// descend steps toward the next level on the route.
func (b *bot) descend() bool {
	l := b.g.Lv
	lk := l.LinkTo(botRoute[l.ID])
	if lk == nil {
		return false
	}
	return b.walkTo(lk.X0, lk.Y0)
}

// stepDown moves to the neighbor closest to the goal of a BFS map.
func (b *bot) stepDown(d []int) bool {
	g := b.g
	l, p := g.Lv, g.P
	best, bd := -1, d[l.Idx(p.X, p.Y)]
	if bd < 0 {
		bd = 1 << 30
	}
	for k, dd := range dirs8 {
		nx, ny := p.X+dd.X, p.Y+dd.Y
		if l.In(nx, ny) && d[l.Idx(nx, ny)] >= 0 && d[l.Idx(nx, ny)] < bd {
			if m := l.MonsterAt(nx, ny); m != nil && m.Friendly {
				continue
			}
			if l.LinkAt(nx, ny) != nil && d[l.Idx(nx, ny)] != 0 {
				continue // only take stairs we are headed for
			}
			best, bd = k, d[l.Idx(nx, ny)]
		}
	}
	if best < 0 {
		return false
	}
	ot := g.Turn
	g.move(dirs8[best].X, dirs8[best].Y)
	return g.Turn != ot || g.Lv != l
}

type botResult struct {
	seed    int64
	dead    bool
	by      string
	turns   int
	lvl     int
	kills   int
	trips   int
	deepest string
}

func runBot(seed int64, maxTurns int) botResult {
	g := NewGame(seed)
	g.Mode = ModePlay
	g.changeLevel("fields", "town", nil)
	b := &bot{g: g, dist: map[string][]int{}}
	r := botResult{seed: seed, deepest: "fields"}
	for ; r.turns < maxTurns && g.Mode != ModeDead; r.turns++ {
		b.turn()
		if g.Mode == ModeTalk || g.Mode == ModeShop {
			g.Mode = ModePlay
		}
		if botDepth[g.Lv.ID] > botDepth[r.deepest] {
			r.deepest = g.Lv.ID
		}
	}
	r.dead, r.by, r.lvl, r.kills, r.trips = g.Mode == ModeDead, g.P.KilledBy, g.P.Lvl, g.P.Kills, b.trips
	return r
}

// TestBotBalance plays the bot over many seeds and reports how it fared.
// It fails only on a gross balance break; the report (go test -v -run
// BotBalance) is the point. BOTSEEDS=n runs more seeds.
func TestBotBalance(t *testing.T) {
	seeds := 12
	if testing.Short() {
		seeds = 3
	}
	if v := os.Getenv("BOTSEEDS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			t.Fatalf("BOTSEEDS=%q: want a positive number", v)
		}
		seeds = n
	}
	var rs []botResult
	for s := 1; s <= seeds; s++ {
		rs = append(rs, runBot(int64(s), 4000))
	}

	alive, reached := 0, map[string]int{}
	causes := map[string]int{}
	var lvls []int
	for _, r := range rs {
		status := "alive"
		if r.dead {
			status = "killed by " + r.by
			causes[r.by]++
		} else {
			alive++
		}
		reached[r.deepest]++
		lvls = append(lvls, r.lvl)
		t.Logf("seed %2d  %-8s clvl %2d  kills %3d  town trips %d  turns %4d  %s", r.seed, r.deepest, r.lvl, r.kills, r.trips, r.turns, status)
	}
	sort.Ints(lvls)
	var deep []string
	for _, id := range []string{"fields", "crypt1", "crypt2", "crypt3", "crypt4"} {
		deep = append(deep, fmt.Sprintf("%s %d", id, reached[id]))
	}
	var cs []string
	for c, n := range causes {
		cs = append(cs, fmt.Sprintf("%s ×%d", c, n))
	}
	sort.Strings(cs)
	t.Logf("survived %d/%d · median clvl %d · deepest: %s", alive, len(rs), lvls[len(lvls)/2], strings.Join(deep, ", "))
	if len(cs) > 0 {
		t.Logf("deaths: %s", strings.Join(cs, ", "))
	}

	// Loose guards against gross balance breaks; the bot is no expert.
	deeper, turns := 0, []int{}
	for _, r := range rs {
		if botDepth[r.deepest] >= botDepth["crypt1"] {
			deeper++
		}
		turns = append(turns, r.turns)
	}
	sort.Ints(turns)
	if med := lvls[len(lvls)/2]; med < 3 {
		t.Errorf("median character level %d, want at least 3", med)
	}
	if deeper*2 < len(rs) {
		t.Errorf("only %d/%d runs reached the crypt", deeper, len(rs))
	}
	if med := turns[len(turns)/2]; med < 1500 {
		t.Errorf("median run lasted %d turns, want at least 1500", med)
	}
}
