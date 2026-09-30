package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// The scripted player (bot.go) as a balance instrument: both policies over
// many seeds, one report. See BALANCE.md §2.3.

type botResult struct {
	policy  string
	seed    int64
	turns   int
	dead    bool
	by      string // the killer
	where   string // level at the end
	stuck   bool
	king    bool // Bone King slain
	oracle  bool // Drowned Oracle slain
	lvl     int
	kills   int
	deepest string
	in      [GoldSrcCount]int // gold by source
	out     [SinkCount]int    // gold by sink
	potions int               // healing potions drunk
	trips   int
	equips  int
	levels  int // levels left behind, as "cleared"
}

// A botRun is a result with the hero's level-arrival snapshots.
type botRun struct {
	botResult
	snaps []botSnap
}

func (r botResult) status() string {
	switch {
	case r.dead:
		return "killed by " + r.by + " in " + r.where
	case r.stuck:
		return "stuck in " + r.where
	}
	return "alive in " + r.where
}

// runBot plays one policy on one seed until death, a stuck run or the turn
// limit.
func runBot(seed int64, pol *botPolicy, maxTurns int) botRun {
	g := NewGame(seed)
	g.Mode = ModePlay
	b := NewBot(g, pol)
	// Trading spends no game turn; the second bound keeps a run finite anyway.
	calls := 0
	for ; g.Turn < maxTurns && calls < 2*maxTurns && g.Mode != ModeDead && b.State() != BotStuck; calls++ {
		b.turn()
	}
	p := g.P
	return botRun{botResult{policy: pol.name, seed: seed, turns: g.Turn, dead: g.Mode == ModeDead, by: p.KilledBy, where: g.Lv.ID,
		stuck: b.State() == BotStuck || calls >= 2*maxTurns, king: g.Quests[0] > 0, oracle: g.Quests[1] > 0,
		lvl: p.Lvl, kills: p.Kills, deepest: b.Stats.Deepest,
		in: g.Stats.In, out: g.Stats.Out, potions: g.Stats.HPots, trips: b.Stats.Trips,
		equips: g.Stats.Equips, levels: maxi(1, len(b.Snaps)-1)}, b.Snaps}
}

// runBots plays every policy over seeds 1..n, runs in parallel.
func runBots(n, maxTurns int) []botRun {
	var rs []botRun
	for _, pol := range botPolicies {
		for s := 1; s <= n; s++ {
			rs = append(rs, botRun{botResult: botResult{policy: pol.name, seed: int64(s)}})
		}
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for i := range rs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			rs[i] = runBot(rs[i].seed, policyByName(rs[i].policy), maxTurns)
			<-sem
		}()
	}
	wg.Wait()
	return rs
}

func median(xs []int) int {
	sort.Ints(xs)
	return xs[len(xs)/2]
}

// pct is the q-th percentile of xs by nearest rank.
func pct(xs []float64, q float64) float64 {
	sort.Float64s(xs)
	return xs[int(math.Round(q*float64(len(xs)-1)))]
}

// botCheckpoints are the levels the report counts arrivals at and
// tabulates the hero on. crypt2 is there for the reference table's sake.
var botCheckpoints = []string{"fields", "crypt1", "crypt2", "crypt4", "marsh", "grotto1", "grotto3", "abyss1", "abyss2"}

// botTypical is the normal monster a checkpoint's arrivals are measured
// against, by area.
var botTypical = map[string]string{"fields": "fallen", "crypt": "zombie", "marsh": "drowned", "grotto": "spider", "abyss": "hellspawn"}

// botRolls is how many kills and deaths the sampler averages per snapshot.
const botRolls = 100

// botReport logs one policy's runs and their summary.
func botReport(t *testing.T, name string, rs []botRun) {
	var lvls, kills, turns, gold []int
	alive, stuck, king, oracle := 0, 0, 0, 0
	reached := map[string]int{}
	killers, areas := map[string]int{}, map[string]int{}
	t.Logf("%s: gold is +%s -%s", name, strings.Join(goldSrcNames[:], "/"), strings.Join(goldSinkNames[:SinkGear+1], "/"))
	for _, r := range rs {
		t.Logf("%-7s seed %2d  %-8s clvl %2d  kills %3d  gold +%s -%s  potions %2d  trips %2d  equips %2d (%.1f/lvl)  turns %5d  %s",
			r.policy, r.seed, r.deepest, r.lvl, r.kills, slashed(r.in[:]), slashed(r.out[:SinkGear+1]), r.potions, r.trips,
			r.equips, float64(r.equips)/float64(r.levels), r.turns, r.status())
		lvls, kills, turns, gold = append(lvls, r.lvl), append(kills, r.kills), append(turns, r.turns), append(gold, sum(r.in[:]))
		switch {
		case r.dead:
			killers[r.by]++
			areas[r.where]++
		case r.stuck:
			stuck++
		default:
			alive++
		}
		if r.king {
			king++
		}
		if r.oracle {
			oracle++
		}
		for _, cp := range botCheckpoints {
			if botDepth(r.deepest) >= botDepth(cp) {
				reached[cp]++
			}
		}
	}
	var deep []string
	for _, cp := range botCheckpoints {
		deep = append(deep, fmt.Sprintf("%s %d", cp, reached[cp]))
	}
	t.Logf("%s: %d runs · alive %d · stuck %d · Bone King %d · Oracle %d · median clvl %d, kills %d, turns %d, gold earned %d · reached: %s",
		name, len(rs), alive, stuck, king, oracle, median(lvls), median(kills), median(turns), median(gold), strings.Join(deep, ", "))
	if len(killers) > 0 {
		t.Logf("%s deaths: %s · in: %s", name, counts(killers), counts(areas))
	}
	botTable(t, name, rs)
}

// botTable prints the hero at each checkpoint: medians over the runs that
// arrived, and P10/P50/P90 across them of the sampler against the area's
// typical normal at that depth.
func botTable(t *testing.T, name string, rs []botRun) {
	ar := newArena(t)
	rng := rand.New(rand.NewSource(1))
	t.Logf("%-7s %-8s %4s %4s %4s %5s %7s %7s %4s %5s %5s   vs normal      swings P10/P50/P90  bolts P10/P50/P90  turns to die P10/P50/P90",
		name, "arrival", "runs", "clvl", "HP", "armor", "melee", "bolt", "crit", "gear", "gold")
	for _, cp := range botCheckpoints {
		var lvl, hp, armor, lo, hi, blo, bhi, crit, gear, gold []int
		var swings, bolts, turns []float64
		area, _ := splitID(cp)
		m := NewMonster(rng, mtemps[botTypical[area]], botDepth(cp), RankNormal)
		for _, r := range rs {
			for i := range r.snaps {
				s := &r.snaps[i]
				if s.Level != cp {
					continue
				}
				lvl, hp, armor, crit, gear, gold = append(lvl, s.Lvl), append(hp, s.HP), append(armor, s.Armor), append(crit, s.Crit), append(gear, s.Gear), append(gold, s.Gold)
				lo, hi, blo, bhi = append(lo, s.MinD), append(hi, s.MaxD), append(blo, s.BoltLo), append(bhi, s.BoltHi)
				d := ar.sample(&s.P, m, botRolls)
				swings, bolts, turns = append(swings, d.Swings), append(bolts, d.Bolts), append(turns, d.Turns)
			}
		}
		if len(lvl) == 0 {
			t.Logf("%-7s %-8s %4d", name, cp, 0)
			continue
		}
		t.Logf("%-7s %-8s %4d %4d %4d %5d %3d-%-3d %3d-%-3d %3d%% %5d %5d   %-13s %s  %s  %s", name, cp, len(lvl),
			median(lvl), median(hp), median(armor), median(lo), median(hi), median(blo), median(bhi), median(crit), median(gear), median(gold),
			m.Name, spread(swings), spread(bolts), spread(turns))
	}
}

// spread formats P10/P50/P90.
func spread(xs []float64) string {
	return fmt.Sprintf("%.1f/%.1f/%.1f", pct(xs, .1), pct(xs, .5), pct(xs, .9))
}

// slashed joins counters as 12/0/340/0.
func slashed(xs []int) string {
	var s []string
	for _, x := range xs {
		s = append(s, strconv.Itoa(x))
	}
	return strings.Join(s, "/")
}

// counts formats a tally, most common first.
func counts(m map[string]int) string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return m[ks[i]] > m[ks[j]] || m[ks[i]] == m[ks[j]] && ks[i] < ks[j] })
	var s []string
	for _, k := range ks {
		s = append(s, fmt.Sprintf("%s ×%d", k, m[k]))
	}
	return strings.Join(s, ", ")
}

// TestBotBalance plays both policies over many seeds and reports how they
// fared. It fails only on a gross balance break; the report (make report)
// is the point. BOTSEEDS=n runs more seeds.
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
	rs := runBots(seeds, 20000)
	for _, pol := range botPolicies {
		var prs []botRun
		for _, r := range rs {
			if r.policy == pol.name {
				prs = append(prs, r)
			}
		}
		botReport(t, pol.name, prs)
	}

	// Loose guards against gross balance breaks; the bot is no expert.
	var lvls, turns []int
	deeper := 0
	for _, r := range rs {
		lvls, turns = append(lvls, r.lvl), append(turns, r.turns)
		if botDepth(r.deepest) >= botDepth("crypt1") {
			deeper++
		}
	}
	if med := median(lvls); med < 3 {
		t.Errorf("median character level %d, want at least 3", med)
	}
	if deeper*2 < len(rs) {
		t.Errorf("only %d/%d runs reached the crypt", deeper, len(rs))
	}
	if med := median(turns); med < 1500 {
		t.Errorf("median run lasted %d turns, want at least 1500", med)
	}
}

// A run is a function of the seed and the policy: the bot rolls its own
// dice and never touches the game's.
func TestBotDeterministic(t *testing.T) {
	for _, pol := range botPolicies {
		a, b := runBot(3, pol, 3000), runBot(3, pol, 3000)
		as, bs := arrivals(a.snaps), arrivals(b.snaps)
		if a.botResult != b.botResult || as != bs {
			t.Errorf("%s: two runs of seed 3 differ:\n%+v %s\n%+v %s", pol.name, a.botResult, as, b.botResult, bs)
		}
	}
}

// arrivals lists snapshots as level@turn.
func arrivals(snaps []botSnap) string {
	var s []string
	for _, sn := range snaps {
		s = append(s, fmt.Sprintf("%s@%d", sn.Level, sn.Turn))
	}
	return strings.Join(s, " ")
}

// TestBotSmoke plays the bot headless through the model, drawing as it
// goes, and checks the game invariants every turn.
func TestBotSmoke(t *testing.T) {
	for _, pol := range botPolicies {
		for seed := int64(1); seed <= 3; seed++ {
			m := newTestModel(NewGame(seed))
			m.drive(pol)
			g := m.g
			for i := range 2000 {
				if g.Mode == ModeDead {
					break
				}
				g.time += 0.05
				m.bot.turn()
				if err := checkGame(g); err != nil {
					t.Fatalf("%s seed %d turn %d: %v", pol.name, seed, g.Turn, err)
				}
				if i%25 == 0 {
					_ = m.View()
				}
			}
			for _, md := range []Mode{ModeInv, ModeChar, ModeHelp, ModeMap, ModeDead} {
				g.Mode = md
				_ = m.View()
			}
			t.Logf("%s seed %d: %s at turn %d, clvl %d", pol.name, seed, m.bot.State(), g.Turn, g.P.Lvl)
		}
	}
}

// The bot's keys: space pauses and resumes, enter steps, + and - change
// speed, any other key pauses and is handed to the player; n after death
// restarts with the bot.
func TestBotKeys(t *testing.T) {
	m := newTestModel(NewGame(2))
	m.start = time.Now().Add(-time.Second)
	m.drive(policyByName("caster"))
	g := m.g
	key := func(k string) {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		switch k {
		case " ":
			msg = tea.KeyMsg{Type: tea.KeySpace}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		}
		m.Update(msg)
	}
	if !m.botRun || m.botTPS != 5 {
		t.Fatalf("bot not running at 5 tps: run %v tps %d", m.botRun, m.botTPS)
	}
	turn := g.Turn
	m.Update(tickMsg(time.Now()))
	if g.Turn == turn {
		t.Fatal("tick did not play a turn")
	}
	key(" ")
	if m.botRun {
		t.Fatal("space did not pause")
	}
	turn = g.Turn
	m.Update(tickMsg(time.Now()))
	if g.Turn != turn {
		t.Fatal("paused bot played a turn")
	}
	key("enter")
	if g.Turn != turn+1 || m.botRun {
		t.Fatalf("enter: turn %d -> %d, run %v", turn, g.Turn, m.botRun)
	}
	key("+")
	key("+")
	key("-")
	if m.botTPS != 10 {
		t.Fatalf("speed %d after + + -", m.botTPS)
	}
	for _, c := range []struct{ tps, dir, want int }{{7, 1, 10}, {7, -1, 5}, {200, 1, 200}, {200, -1, 60}, {1, -1, 1}} {
		if got := botSpeed(c.tps, c.dir); got != c.want {
			t.Errorf("botSpeed(%d, %d) = %d, want %d", c.tps, c.dir, got, c.want)
		}
	}
	key(" ")
	key("i")
	if m.botRun || g.Mode != ModeInv {
		t.Fatalf("i: run %v mode %v; want paused and the inventory", m.botRun, g.Mode)
	}
	key("esc")
	key(" ")
	if !m.botRun {
		t.Fatal("space did not resume")
	}
	g.hurtPlayer(1e6, "a test", "smites")
	bot := m.bot
	key("n")
	if m.g == g || m.bot == bot || m.bot == nil || !m.botRun || m.g.Mode != ModePlay {
		t.Fatal("n did not restart with the bot")
	}
	m.g.Mode = ModePlay
	_ = m.View()
}
