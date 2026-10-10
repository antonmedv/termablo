package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"
)

// The Kindling trial: few bot runs reach the Cinder Prior and none has
// killed him, so his fight is measured on its own. Each trial is a par
// hero as the bot arrives there (lvl 22, the grotto3 kit, belt full and
// two scrolls), started on the Kindling with the line up to the coal
// done, played by the bot until he or the hero dies. Two variants: the
// floor as generated, and the duel, the floor cleared of everything but
// him and his braziers. BOTKINDLING=n runs n seeds per policy, from
// BOTKINDLINGFIRST (default 1); BOTRULES lays knobs over the defaults.
//
//	BOTKINDLING=48 go test -count=1 -v -run KindlingTrial .
//	BOTKINDLING=24 BOTKINDLINGLOG=fighter:3:true  (one run's last lines)

type kindlingResult struct {
	policy        string
	duel          bool
	seed          int64
	won, dead     bool
	stuck         bool
	where         string
	turns         int
	pots          int
	burned        bool // he reached half life and burned
	bossHP, maxHP int
	log           []string
}

const (
	kindlingTurnCap = 3000
	kindlingLvl     = 22 // the bot's median level on arrival (ledger #84)
)

func kindlingTrial(r *Rules, pol *botPolicy, seed int64, duel bool) (res kindlingResult) {
	res = kindlingResult{policy: pol.name, seed: seed, duel: duel}
	defer func() {
		if e := recover(); e != nil {
			res.stuck, res.where = true, fmt.Sprint("panic: ", e)
		}
	}()
	g := NewGameWith(seed, r)
	g.Mode = ModePlay
	// BOTKINDLINGKIT dresses the hero from another checkpoint's par
	// rolls (abyss2 is the kit the Hearth trial fights in).
	kit := os.Getenv("BOTKINDLINGKIT")
	if kit == "" {
		kit = "grotto3"
	}
	cp := refCheckpointByName(kit)
	rng := rand.New(rand.NewSource(seed))
	hero := refHero(g, pol, kindlingLvl)
	wear(hero, parKit(rng, r, pol, cp))
	hero.HPot, hero.MPot, hero.Scrolls, hero.Gold = beltMax, pol.mana, 2, 400
	if v, err := strconv.Atoi(os.Getenv("BOTKINDLINGGOLD")); err == nil {
		hero.Gold = v // the bot's arrivals carry a few thousand; 400 is the Hearth trial's
	}
	g.P = hero
	// BOTKINDLINGAT puts the same hero on another floor to compare: there
	// the win is leaving it alive by the way down, and the line is
	// settled past the Prior so the bot heads down.
	at := os.Getenv("BOTKINDLINGAT")
	if at == "" {
		at = "sanctum2"
	}
	settle(g, "boneking", "barrow", "oracle", "coal")
	if at != "sanctum2" {
		settle(g, "prior")
	}
	g.Deepest = max(11, g.getLevel(at).Depth)
	g.changeLevel(at, "", nil)
	var boss *Monster
	for _, m := range g.Lv.Monsters {
		if m.T.ID == "prior" {
			boss = m
		} else if duel {
			m.Dead = true
		}
	}
	g.cleanup()
	b := NewBot(g, pol)
	won := func() bool {
		if boss != nil {
			return g.Slain["prior"]
		}
		return g.Lv.ID != at && g.Lv.Kind != KTown
	}
	seen, last := 0, ""
	for calls := 0; g.Turn < kindlingTurnCap && calls < 2*kindlingTurnCap && g.Mode != ModeDead && !won() && b.State() != BotStuck; calls++ {
		b.turn()
		// the log as the player read it, with the bot's state changes
		if now := b.State().String() + ": " + b.Why(); now != last {
			last = now
			res.log = append(res.log, fmt.Sprintf("%5d @%d,%d bot %s", g.Turn, g.P.X, g.P.Y, now))
		}
		for fresh := min(g.logN-seen, len(g.Log)); fresh > 0; fresh-- {
			m := g.Log[len(g.Log)-fresh]
			res.log = append(res.log, fmt.Sprintf("%5d hp %d/%d %s", m.Turn, int(g.P.HP), g.P.MaxHP(), m.Text))
		}
		seen = g.logN
		if len(res.log) > 600 {
			res.log = res.log[len(res.log)-300:]
		}
	}
	res.won, res.dead = won(), g.Mode == ModeDead
	res.stuck = !res.won && !res.dead && g.Turn < kindlingTurnCap
	res.where, res.turns = g.Lv.ID, g.Turn
	res.pots = g.Stats.HPots
	if boss != nil {
		res.burned, res.bossHP, res.maxHP = boss.Spent, boss.HP, boss.MaxHP
	}
	return res
}

func TestKindlingTrial(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("BOTKINDLING"))
	first, _ := strconv.Atoi(os.Getenv("BOTKINDLINGFIRST"))
	first = max(first, 1)
	if n == 0 {
		t.Skip("set BOTKINDLING to run the Kindling trial")
	}
	r, _, err := rulesFromFile(os.Getenv("BOTRULES"))
	if err != nil {
		t.Fatal(err)
	}
	var rs []kindlingResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, duel := range []bool{false, true} {
		for _, pol := range botPolicies {
			for s := first; s < first+n; s++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					res := kindlingTrial(r, pol, int64(s), duel)
					mu.Lock()
					rs = append(rs, res)
					mu.Unlock()
				}()
			}
		}
	}
	wg.Wait()
	for _, duel := range []bool{false, true} {
		for _, pol := range botPolicies {
			var won, dead, stuck, burned, town, pots, turns, hp int
			for _, x := range rs {
				if x.policy != pol.name || x.duel != duel {
					continue
				}
				switch {
				case x.won:
					won++
					turns += x.turns
				case x.dead:
					dead++
					if x.maxHP > 0 {
						hp += 100 * x.bossHP / x.maxHP
					}
				default:
					stuck++
				}
				if x.burned {
					burned++
				}
				if x.where == "town" {
					town++
				}
				pots += x.pots
			}
			variant := "floor"
			if duel {
				variant = "duel"
			}
			t.Logf("%-5s %-8s won %d/%d · dead %d · stuck or timed out %d · he burned %d · ended in town %d · his life left at a death %d%% · potions %.1f per run · turns to win %.0f",
				variant, pol.name, won, n, dead, stuck, burned, town, hp/max(1, dead), float64(pots)/float64(n), float64(turns)/float64(max(1, won)))
		}
	}
	if v := os.Getenv("BOTKINDLINGLOG"); v != "" {
		for _, x := range rs {
			if fmt.Sprintf("%s:%d:%v", x.policy, x.seed, x.duel) == v {
				for _, l := range x.log {
					t.Log(l)
				}
			}
		}
	}
	for _, x := range rs {
		if x.stuck {
			t.Logf("stuck: %s:%d:%v in %s at turn %d, boss %d/%d", x.policy, x.seed, x.duel, x.where, x.turns, x.bossHP, x.maxHP)
		}
	}
}
