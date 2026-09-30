package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// Reference heroes: fixtures, not bots. For each
// checkpoint, build and gear tier, a hero built in code fights the depth's
// monsters through the sampler (duel_test.go). go test -v -run RefHeroes
// prints the table; make report does too. No asserts until the bands
// are calibrated: rows outside them are marked in the last column.

// A refCheckpoint is a depth on the route with the bot's median arrival
// clvl and gearScore there, the monsters typical of it, and how
// many rolls per slot par picks the best of.
type refCheckpoint struct {
	name       string
	depth, lvl int
	par        int    // rolls per slot for the par tier; 0 is the start kit
	gear       [2]int // the bot's P50 gearScore on arrival, fighter and caster
	normals    [2]string
	champion   string
	boss       string
}

// par doubles every two depths: 3 at crypt2, 6 at crypt4, 12 at grotto1,
// 24 at grotto3, 48 at abyss2 put par's gearScore within ±15% of the
// bot's P50 at every checkpoint (calibrated 2026-09-30 against the
// `7e30c19` baseline; abyss2 caster rests on the one run at `1dfefa6`).
var refCheckpoints = []refCheckpoint{
	{"fields", 1, 1, 0, [2]int{31, 9}, [2]string{"fallen", "zombie"}, "zombie", ""},
	{"crypt2", 3, 6, 3, [2]int{227, 200}, [2]string{"skel", "zombie"}, "skel", ""},
	{"crypt4", 5, 9, 6, [2]int{373, 368}, [2]string{"skel", "ghoul"}, "ghoul", "boneking"},
	{"grotto1", 7, 13, 12, [2]int{525, 581}, [2]string{"spider", "drowned"}, "golem", ""},
	{"grotto3", 9, 16, 24, [2]int{707, 687}, [2]string{"spider", "golem"}, "spider", "oracle"},
	{"abyss2", 11, 20, 48, [2]int{871, 960}, [2]string{"imp", "hellspawn"}, "hellspawn", ""},
}

// wrongWay is how far past its depth the par hero is sent.
const wrongWay = 3

// Gear tiers: par is the best of the checkpoint's par rolls per slot at
// drop rarities (RollRarity with bonus 1) by the policy's gearScore, the
// median kit of parKits such kits so one lucky weapon does not move the
// fixture; lucky is the best of luckyRolls Rares, each Unique one time in
// uniqueOdds.
const (
	parKits    = 15
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
	p := NewPlayer(g.Rules)
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
// best by the policy's gearScore. A two-handed weapon leaves the off-hand
// empty.
func gearTier(rng *rand.Rand, r *Rules, pol *botPolicy, ilvl, n int, rarity func() Rarity) [EqCount]*Item {
	var eq [EqCount]*Item
	for slot := range EqCount {
		if slot == EqOffhand && eq[EqWeapon] != nil && eq[EqWeapon].Base.TwoHanded {
			continue
		}
		its := make([]*Item, n)
		for i := range its {
			its[i] = GenItem(rng, ilvl, rarity(), eqSlotFor[slot], r)
		}
		sort.SliceStable(its, func(i, j int) bool {
			a, _ := pol.gearScore(its[i])
			b, _ := pol.gearScore(its[j])
			return a < b
		})
		eq[slot] = its[n-1]
	}
	return eq
}

// parKit is the par tier: the start kit where the checkpoint says so,
// otherwise the median by gearTotal of parKits kits, each the best of the
// checkpoint's par rolls per slot at drop rarities.
func parKit(rng *rand.Rand, r *Rules, pol *botPolicy, cp refCheckpoint) [EqCount]*Item {
	if cp.par == 0 {
		return startKit(rng)
	}
	kits := make([][EqCount]*Item, parKits)
	for i := range kits {
		kits[i] = gearTier(rng, r, pol, cp.depth, cp.par, func() Rarity { return RollRarity(rng, 1.0) })
	}
	sort.SliceStable(kits, func(i, j int) bool { return gearTotal(pol, kits[i]) < gearTotal(pol, kits[j]) })
	return kits[parKits/2]
}

// gearTotal is the policy's gearScore of everything worn, the bot's
// snapshot measure.
func gearTotal(pol *botPolicy, eq [EqCount]*Item) int {
	total := 0
	for _, it := range eq {
		if it != nil {
			v, _ := pol.gearScore(it)
			total += v
		}
	}
	return total
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

// refMonsters builds a checkpoint's monsters at depth: two normals, an
// Extra Strong champion, and the boss where there is one.
func refMonsters(rng *rand.Rand, r *Rules, cp refCheckpoint, depth int) []*Monster {
	ms := []*Monster{
		NewMonster(rng, mtemps[cp.normals[0]], depth, RankNormal, r),
		NewMonster(rng, mtemps[cp.normals[1]], depth, RankNormal, r),
		strongChampion(rng, r, mtemps[cp.champion], depth),
	}
	if cp.boss != "" {
		ms = append(ms, NewMonster(rng, mtemps[cp.boss], depth, RankBoss, r))
	}
	return ms
}

// strongChampion rolls a champion until its one mod is Extra Strong, so
// the champion column compares across checkpoints.
func strongChampion(rng *rand.Rand, r *Rules, t *MTemplate, depth int) *Monster {
	for {
		if m := NewMonster(rng, t, depth, RankChampion, r); m.HasMod(ModStrong) {
			return m
		}
	}
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

// refFlags marks a row outside the bands: "kill" for swings or bolts
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
	t.Logf("reference heroes, at least %d rolls and %d kills or deaths per measure · gear: the policy's gearScore of everything worn, bot: the bot's P50 on arrival that par is calibrated to · per monster: swings to kill, bolts to kill, player turns survived from full life with no potions, damage per monster attack with misses · ranged monsters fight in melee here · dps/hp: mean hit (melee for fighter, bolt for caster) over the first normal's life · ! = outside a band", n, minTrials)
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
		r := ar.g.Rules
		var rows []refRow
		for _, pol := range botPolicies {
			start, par, lucky := refHero(ar.g, pol, cp.lvl), refHero(ar.g, pol, cp.lvl), refHero(ar.g, pol, cp.lvl)
			wear(start, startKit(rng))
			wear(par, parKit(rng, r, pol, cp))
			wear(lucky, gearTier(rng, r, pol, cp.depth, luckyRolls, func() Rarity {
				if rng.Intn(uniqueOdds) == 0 {
					return RUnique
				}
				return RRare
			}))
			rows = append(rows, refRow{"start", pol, start, cp.depth}, refRow{"par", pol, par, cp.depth}, refRow{"lucky", pol, lucky, cp.depth})
		}
		for _, r := range rows {
			if r.tier == "par" {
				rows = append(rows, refRow{"wrong", r.pol, r.p, cp.depth + wrongWay})
			}
		}
		ms := refMonsters(rng, r, cp, cp.depth)
		far := refMonsters(rng, r, cp, cp.depth+wrongWay)
		var labels []string
		hdr := fmt.Sprintf("%-14s %4s %5s %-8s %4s %-6s %4s %4s", "hero", "HP", "armor", "melee", "crit", "bolt", "gear", "bot")
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
			bot := ""
			if r.tier == "par" {
				want := cp.gear[0]
				if r.pol.caster {
					want = cp.gear[1]
				}
				bot = strconv.Itoa(want)
			}
			line := fmt.Sprintf("%-14s %4d %5d %-8s %3d%% %-6s %4d %4s", r.pol.name+" "+r.tier, p.MaxHP(), p.ArmorVal(), fmt.Sprintf("%d-%d", lo, hi), p.Crit(), fmt.Sprintf("%d-%d", blo, bhi), gearTotal(r.pol, p.Eq), bot)
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
