package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"
)

// The Hearth trial: no bot run reaches abyss3, so the final fight is
// measured on its own. Each trial is a par hero one level past the abyss2
// checkpoint, belt full and two scrolls, started on the Hearth Below with
// both quests done, played by the bot until the Last Wanderer or the hero
// dies. Two variants: the floor as generated, and the duel, the floor
// cleared of everything but him. BOTHEARTH=n runs n seeds per policy,
// from BOTHEARTHFIRST (default 1); BOTRULES lays knobs over the defaults.
// Tuned 2026-10-03 to WandererHp 3: on seeds 1-96 the duel is won by the
// fighter every time on about 2.6 potions and by the caster about 7 in
// 10 on the whole belt.
//
//	BOTHEARTH=48 go test -count=1 -v -run HearthTrial .

type hearthResult struct {
	policy        string
	duel          bool
	seed          int64
	won, dead     bool
	stuck         bool
	where         string // where the run ended
	turns         int
	pots, fallen  int
	followed      bool // he reached Emberhold
	bossHP, maxHP int
	log           []string
}

const hearthTurnCap = 3000

func hearthTrial(r *Rules, pol *botPolicy, seed int64, duel bool) (res hearthResult) {
	res = hearthResult{policy: pol.name, seed: seed, duel: duel}
	defer func() {
		if e := recover(); e != nil {
			res.stuck, res.where = true, fmt.Sprint("panic: ", e)
		}
	}()
	g := NewGameWith(seed, r)
	g.Mode = ModePlay
	cp := refCheckpointByName("abyss2")
	rng := rand.New(rand.NewSource(seed))
	hero := refHero(g, pol, cp.lvl+1)
	wear(hero, parKit(rng, r, pol, cp))
	hero.HPot, hero.MPot, hero.Scrolls, hero.Gold = beltMax, pol.mana, 2, 400
	g.P = hero
	g.Quests = [3]int{2, 2, 0}
	g.Deepest = 11
	g.changeLevel("abyss3", "", nil)
	var boss *Monster
	for _, m := range g.Lv.Monsters {
		if m.T.ID == "wanderer" {
			boss = m
		} else if duel {
			m.Dead = true
		}
	}
	g.cleanup()
	b := NewBot(g, pol)
	for calls := 0; g.Turn < hearthTurnCap && calls < 2*hearthTurnCap && g.Mode != ModeDead && g.Quests[2] == 0 && b.State() != BotStuck; calls++ {
		b.turn()
	}
	res.won, res.dead = g.Quests[2] > 0, g.Mode == ModeDead
	res.stuck = !res.won && !res.dead && g.Turn < hearthTurnCap
	res.where, res.turns = g.Lv.ID, g.Turn
	res.pots, res.fallen, res.followed = g.Stats.HPots, len(g.Fallen), g.homeYet
	res.bossHP, res.maxHP = boss.HP, boss.MaxHP
	for _, m := range g.Log[max(0, len(g.Log)-40):] {
		res.log = append(res.log, fmt.Sprintf("%5d %s", m.Turn, m.Text))
	}
	return res
}

func TestHearthTrial(t *testing.T) {
	n, _ := strconv.Atoi(os.Getenv("BOTHEARTH"))
	first, _ := strconv.Atoi(os.Getenv("BOTHEARTHFIRST"))
	first = max(first, 1)
	if n == 0 {
		t.Skip("set BOTHEARTH to run the Hearth trial")
	}
	r, _, err := rulesFromFile(os.Getenv("BOTRULES"))
	if err != nil {
		t.Fatal(err)
	}
	var rs []hearthResult
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, duel := range []bool{false, true} {
		for _, pol := range botPolicies {
			for s := first; s < first+n; s++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					res := hearthTrial(r, pol, int64(s), duel)
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
			var won, dead, stuck, followed, town, fallen, pots, turns int
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
				default:
					stuck++
				}
				if x.followed {
					followed++
				}
				if x.where == "town" {
					town++
				}
				fallen += x.fallen
				pots += x.pots
			}
			variant := "floor"
			if duel {
				variant = "duel"
			}
			t.Logf("%-5s %-8s won %d/%d · dead %d · stuck or timed out %d · he reached town %d · ended in town %d · townsfolk lost %.1f per run · potions %.1f per run · turns to win %.0f",
				variant, pol.name, won, n, dead, stuck, followed, town, float64(fallen)/float64(n), float64(pots)/float64(n), float64(turns)/float64(max(1, won)))
		}
	}
	if v := os.Getenv("BOTHEARTHLOG"); v != "" {
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
