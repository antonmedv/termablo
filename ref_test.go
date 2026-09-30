package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"testing"
)

// Reference heroes (BALANCE.md §2.1): fixtures, not bots. For each
// checkpoint, build and gear tier, a hero built in code fights the depth's
// monsters through the sampler (duel_test.go). go test -v -run RefHeroes
// prints the table; make report does too. No asserts until the §1 bands
// are calibrated: rows outside them are marked in the last column.

// A refCheckpoint is a depth on the route with the bot's median arrival
// clvl there (§6) and the monsters typical of it.
type refCheckpoint struct {
	name       string
	depth, lvl int
	normals    [2]string
	champion   string
	boss       string
}

var refCheckpoints = []refCheckpoint{
	{"fields", 1, 1, [2]string{"fallen", "zombie"}, "zombie", ""},
	{"crypt2", 3, 6, [2]string{"zombie", "skel"}, "skel", ""},
	{"crypt4", 5, 9, [2]string{"zombie", "ghoul"}, "ghoul", "boneking"},
	{"grotto1", 7, 13, [2]string{"spider", "drowned"}, "golem", ""},
	{"grotto3", 9, 16, [2]string{"spider", "golem"}, "spider", "oracle"},
	{"abyss2", 11, 20, [2]string{"imp", "hellspawn"}, "hellspawn", ""},
}

// wrongWay is how far past its depth the par hero is sent.
const wrongWay = 3

// Gear tiers: par is the median of parRolls Magic items per slot by the
// policy's gearScore, lucky the best of luckyRolls Rares, each Unique one
// time in uniqueOdds.
const (
	parRolls   = 200
	luckyRolls = 30
	uniqueOdds = 6
)

func refRolls() int {
	if testing.Short() {
		return 100
	}
	return 1000
}

// refHero builds a policy's hero at lvl, points spent in its pattern
// through spendPoint, so fixtures and bot agree.
func refHero(g *Game, pol *botPolicy, lvl int) *Player {
	p := NewPlayer()
	p.Lvl, p.Points = lvl, 5*(lvl-1)
	old := g.P
	g.P = p
	for i := 0; p.Points > 0; i++ {
		g.spendPoint(pol.points[i%len(pol.points)])
	}
	g.P = old
	return p
}

// startKit is what NewGame hands out.
func startKit(rng *rand.Rand) [EqCount]*Item {
	var eq [EqCount]*Item
	for _, s := range []struct {
		slot int
		name string
	}{{EqWeapon, "Short Sword"}, {EqArmor, "Leather Armor"}} {
		it := &Item{Kind: IKEquip, Base: baseByName(s.name), Name: s.name, ILvl: 1}
		it.finishStats(rng)
		eq[s.slot] = it
	}
	return eq
}

// gearTier rolls n items of ilvl per slot, weapon first, and keeps the
// median (or the best) by the policy's gearScore. A two-handed weapon
// leaves the off-hand empty.
func gearTier(rng *rand.Rand, pol *botPolicy, ilvl, n int, rarity func() Rarity, best bool) [EqCount]*Item {
	var eq [EqCount]*Item
	for slot := range EqCount {
		if slot == EqOffhand && eq[EqWeapon] != nil && eq[EqWeapon].Base.TwoHanded {
			continue
		}
		its := make([]*Item, n)
		for i := range its {
			its[i] = GenItem(rng, ilvl, rarity(), eqSlotFor[slot])
		}
		sort.SliceStable(its, func(i, j int) bool {
			a, _ := pol.gearScore(its[i])
			b, _ := pol.gearScore(its[j])
			return a < b
		})
		if best {
			eq[slot] = its[n-1]
		} else {
			eq[slot] = its[n/2]
		}
	}
	return eq
}

func wear(p *Player, eq [EqCount]*Item) {
	p.Eq = eq
	p.recalc()
	p.HP, p.MP = float64(p.MaxHP()), float64(p.MaxMP())
}

// A refRow is one hero against one set of monsters.
type refRow struct {
	tier  string
	pol   *botPolicy
	p     *Player
	depth int
}

// refMonsters builds a checkpoint's monsters at depth: two normals, a
// champion, and the boss where there is one.
func refMonsters(rng *rand.Rand, cp refCheckpoint, depth int) []*Monster {
	ms := []*Monster{
		NewMonster(rng, mtemps[cp.normals[0]], depth, RankNormal),
		NewMonster(rng, mtemps[cp.normals[1]], depth, RankNormal),
		NewMonster(rng, mtemps[cp.champion], depth, RankChampion),
	}
	if cp.boss != "" {
		ms = append(ms, NewMonster(rng, mtemps[cp.boss], depth, RankBoss))
	}
	return ms
}

// monsterLabel names a monster for the checkpoint line; short is the
// column head.
func monsterLabel(m *Monster) (label, short string) {
	label, short = fmt.Sprintf("%s %dhp", m.Name, m.MaxHP), m.T.ID
	switch m.Rank {
	case RankChampion:
		label, short = fmt.Sprintf("%s champion (%s) %dhp", m.Name, m.Describe(), m.MaxHP), m.T.ID+" champion"
	case RankBoss:
		short = "boss"
	}
	if m.T.AI == AIRanged || m.T.AI == AIOracle {
		label += " ranged"
	}
	return label, short
}

// kills is the row's way of killing: swings for a fighter, bolts for a caster.
func kills(pol *botPolicy, d duel) float64 {
	if pol.caster {
		return d.Bolts
	}
	return d.Swings
}

// refFlags marks a row outside the §1 bands: "kill" for swings or bolts
// to kill, "live" for turns survived. Par rows are held to the encounter
// bands; wrong way to ~3× kills and ~⅓ turns of par; lucky to at most 3×
// par either way; start to at most 2× par kills.
func refFlags(r refRow, ds, par []duel, boss bool) string {
	in := func(v, lo, hi float64) bool { return v >= lo && v <= hi }
	kill, live := true, true
	switch r.tier {
	case "par":
		for i, d := range ds[:2] {
			kill = kill && in(kills(r.pol, d), 3, 5)
			live = live && in(ds[i].Turns, 8, 12)
		}
		kill = kill && in(kills(r.pol, ds[2]), 8, 12)
		live = live && in(ds[2].Turns, 4, 6)
		if boss {
			kill = kill && in(kills(r.pol, ds[3]), 30, 50)
		}
	case "wrong":
		for i := range 2 {
			kill = kill && in(kills(r.pol, ds[i])/kills(r.pol, par[i]), 2, 4)
			live = live && in(ds[i].Turns/par[i].Turns, .2, .5)
		}
	case "lucky":
		for i := range 2 {
			kill = kill && kills(r.pol, ds[i])*3 >= kills(r.pol, par[i])
			live = live && ds[i].Turns <= 3*par[i].Turns
		}
	case "start":
		for i := range 2 {
			kill = kill && kills(r.pol, ds[i]) <= 2*kills(r.pol, par[i])
		}
	}
	var out []string
	if !kill {
		out = append(out, "!kill")
	}
	if !live {
		out = append(out, "!live")
	}
	return strings.Join(out, " ")
}

// TestRefHeroes prints the reference table, one checkpoint per goroutine.
func TestRefHeroes(t *testing.T) {
	n := refRolls()
	t.Logf("reference heroes, at least %d rolls and %d kills or deaths per measure · per monster: swings to kill, bolts to kill, player turns survived from full life with no potions, damage per monster attack with misses · ranged monsters fight in melee here · dps/hp: mean hit (melee for fighter, bolt for caster) over the first normal's life · ! = outside a §1 band", n, minTrials)
	blocks := make([][]string, len(refCheckpoints))
	var wg sync.WaitGroup
	for i, cp := range refCheckpoints {
		ar := newArena(t)
		wg.Add(1)
		go func() {
			defer wg.Done()
			blocks[i] = refBlock(ar, cp, rand.New(rand.NewSource(int64(i)+1)), n)
		}()
	}
	wg.Wait()
	for _, b := range blocks {
		for _, line := range b {
			t.Log(line)
		}
	}
}

// refBlock builds a checkpoint's heroes and monsters, samples every row
// and returns the lines to print.
func refBlock(ar *arena, cp refCheckpoint, rng *rand.Rand, n int) []string {
	var out []string
	logf := func(f string, a ...any) { out = append(out, fmt.Sprintf(f, a...)) }
	{
		var rows []refRow
		for _, pol := range botPolicies {
			start, par, lucky := refHero(ar.g, pol, cp.lvl), refHero(ar.g, pol, cp.lvl), refHero(ar.g, pol, cp.lvl)
			wear(start, startKit(rng))
			wear(par, gearTier(rng, pol, cp.depth, parRolls, func() Rarity { return RMagic }, false))
			wear(lucky, gearTier(rng, pol, cp.depth, luckyRolls, func() Rarity {
				if rng.Intn(uniqueOdds) == 0 {
					return RUnique
				}
				return RRare
			}, true))
			rows = append(rows, refRow{"start", pol, start, cp.depth}, refRow{"par", pol, par, cp.depth}, refRow{"lucky", pol, lucky, cp.depth})
		}
		for _, r := range rows {
			if r.tier == "par" {
				rows = append(rows, refRow{"wrong", r.pol, r.p, cp.depth + wrongWay})
			}
		}
		ms := refMonsters(rng, cp, cp.depth)
		far := refMonsters(rng, cp, cp.depth+wrongWay)
		var labels []string
		hdr := fmt.Sprintf("%-14s %4s %5s %-8s %4s %-6s", "hero", "HP", "armor", "melee", "crit", "bolt")
		for _, m := range ms {
			label, short := monsterLabel(m)
			labels = append(labels, label)
			hdr += fmt.Sprintf(" | %-22s", short)
		}
		logf("%s  depth %d  clvl %d  ·  %s  ·  wrong way: depth %d, same monsters", cp.name, cp.depth, cp.lvl, strings.Join(labels, "  ·  "), cp.depth+wrongWay)
		logf("%s | dps/hp | !", hdr)
		duels := make([][]duel, len(rows))
		par := map[bool][]duel{}
		for i, r := range rows {
			targets := ms
			if r.tier == "wrong" {
				targets = far
			}
			for _, m := range targets {
				duels[i] = append(duels[i], ar.sample(r.p, m, n))
			}
			if r.tier == "par" {
				par[r.pol.caster] = duels[i]
			}
		}
		for i, r := range rows {
			ds, p := duels[i], r.p
			lo, hi := p.DmgRange()
			blo, bhi := p.FireboltDmg()
			line := fmt.Sprintf("%-14s %4d %5d %-8s %3d%% %-6s", r.pol.name+" "+r.tier, p.MaxHP(), p.ArmorVal(), fmt.Sprintf("%d-%d", lo, hi), p.Crit(), fmt.Sprintf("%d-%d", blo, bhi))
			for _, d := range ds {
				line += fmt.Sprintf(" | %5.1f %5.1f %5.1f %4.1f", d.Swings, d.Bolts, d.Turns, d.DPT)
			}
			hit := float64(lo+hi) / 2 * (1 + float64(p.Crit())/100)
			if r.pol.caster {
				hit = float64(blo+bhi) / 2 * (1 + float64(p.Crit())/400)
			}
			logf("%s | %6.2f | %s", line, hit/float64(ms[0].MaxHP), refFlags(r, ds, par[r.pol.caster], cp.boss != ""))
		}
	}
	return out
}
