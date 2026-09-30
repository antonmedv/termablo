package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/antonmedv/termablo/internal/balance"
)

// One balance evaluation. TestBotBalance plays both policies over many
// seeds and prints the report (make report); the environment picks what
// it plays and where the numbers go:
//
//	BOTSEEDS=48      seeds to play (12; 3 under -short)
//	BOTFIRST=1       the first seed
//	BOTRULES=x.json  knobs laid over the defaults (rulesFromFile)
//	BOTOUT=run.json  write the rows, metrics and score (internal/balance)
//	BOTNAME=label    a name for the result
//
// make eval wraps it; cmd/balance compares and logs the results. The
// arithmetic (metrics, goals, score) lives in internal/balance so the
// tool reads exactly what the test wrote.

// botTurns is the turn limit of one run: long enough that a run ends by
// death, not by the clock.
const botTurns = 20000

// refEvalRolls is the sampler budget of the reduced reference table an
// evaluation runs; the printed table (TestRefHeroes) uses refRolls.
const refEvalRolls = 300

func TestBotBalance(t *testing.T) {
	started := time.Now()
	seeds, first := 12, 1
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
	if v := os.Getenv("BOTFIRST"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			t.Fatalf("BOTFIRST=%q: want a positive number", v)
		}
		first = n
	}
	r, warnings, err := rulesFromFile(os.Getenv("BOTRULES"))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range warnings {
		t.Logf("BOTRULES: %s", w)
	}
	if d := knobDiff(DefaultRules(), r); len(d) > 0 {
		t.Logf("rules: %s", strings.Join(d, ", "))
	}

	rs := runBotsWith(r, first, seeds, botTurns)
	sampleRuns(t, r, rs)
	for _, pol := range botPolicies {
		var prs []botRun
		for _, run := range rs {
			if run.policy == pol.name {
				prs = append(prs, run)
			}
		}
		botReport(t, pol.name, prs, r)
	}

	res := evalResult(t, r, rs, first, seeds)
	res.Seconds = time.Since(started).Seconds()
	if res.Fail != "" {
		t.Logf("hard fail: %s", res.Fail)
	} else {
		t.Logf("score %.2f (95%% %.2f–%.2f over seeds) in %.0f s · %d goals missed, largest first:", *res.Score, res.ScoreCI[0], res.ScoreCI[1], res.Seconds, len(res.Misses))
		for _, m := range res.Misses {
			if m.D > 0 {
				t.Logf("  %s", m)
			}
		}
	}
	if out := os.Getenv("BOTOUT"); out != "" {
		if err := writeResult(out, res); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", out)
	}

	// Loose guards against gross balance breaks; the bot is no expert.
	var lvls, turns []int
	deeper := 0
	for _, run := range rs {
		lvls, turns = append(lvls, run.lvl), append(turns, run.turns)
		if botDepth(run.deepest) >= botDepth("crypt1") {
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
	if res.Fail != "" && os.Getenv("BOTOUT") == "" {
		t.Errorf("hard fail: %s", res.Fail)
	}
}

// evalResult converts the runs into the balance package's rows, adds
// the reference measures and the provenance, and scores it all.
func evalResult(t testing.TB, r *Rules, rs []botRun, first, seeds int) *balance.Result {
	res := &balance.Result{
		Name:    os.Getenv("BOTNAME"),
		Time:    time.Now().Format(time.RFC3339),
		First:   first,
		Seeds:   seeds,
		Turns:   botTurns,
		Rules:   knobValues(r),
		Changed: knobDiff(DefaultRules(), r),
		Gamble:  r.GamblePrice + r.GamblePerDepth*balance.GrottoDepth,
		Hashes:  map[string]string{},
	}
	res.Rev, res.Dirty = gitRevision()
	res.Hashes["code"] = hashFiles(codeFiles())
	res.Hashes["bot"] = hashFiles([]string{"bot.go"})
	res.Hashes["rules"] = hashFiles([]string{"rules.go"})
	for _, run := range rs {
		res.Runs = append(res.Runs, evalRun(run, r))
	}
	res.Refs = refMeasures(t, r)
	balance.Evaluate(res)
	return res
}

// evalRun is a bot run as a balance row.
func evalRun(run botRun, r *Rules) balance.Run {
	b := run.botResult
	out := balance.Run{Policy: b.policy, Seed: b.seed, Turns: b.turns, Dead: b.dead, By: b.by, Where: b.where, WhereDepth: botDepth(b.where),
		Stuck: b.stuck, King: b.king, Oracle: b.oracle, Lvl: b.lvl, Kills: b.kills, Deepest: b.deepest, DeepestDepth: botDepth(b.deepest),
		Gold: b.gold, In: append([]int(nil), b.in[:]...), Out: append([]int(nil), b.out[:]...), Potions: b.potions, MPots: b.mpots, Bolts: b.bolts, Novas: b.novas, Trips: b.trips,
		Equips: b.equips, Levels: b.levels, Fail: b.fail, Adj: b.death.Adj, Awake: b.death.Awake, PotionsLeft: b.death.Potions}
	if b.dead {
		out.ByRank = "none"
		if b.death.Rank >= 0 {
			out.ByRank = rankNames[b.death.Rank]
		}
	}
	for _, s := range run.snaps {
		sn := balance.Snap{Level: s.Level, Depth: botDepth(s.Level), Turn: s.Turn, Lvl: s.Lvl, HP: s.HP, Armor: s.Armor, Gear: s.Gear, Gold: s.Gold,
			MinD: s.MinD, MaxD: s.MaxD, BoltLo: s.BoltLo, BoltH: s.BoltHi}
		if d, ok := run.duels[s.Level]; ok {
			sn.Sampled, sn.Swings, sn.Bolts, sn.Live = true, d.Swings, d.Bolts, d.Turns
			area, _ := splitID(s.Level)
			sn.Pack, sn.Speed = packOf(mtemps[botTypical[area]], r)
		}
		out.Snaps = append(out.Snaps, sn)
	}
	return out
}

// refMeasures samples the reduced reference table (par, lucky and wrong
// way at balance.RefCheckpoints) and returns the ratios the goals read.
func refMeasures(t testing.TB, r *Rules) []balance.Ref {
	results := make([]refResult, len(balance.RefCheckpoints))
	var wg sync.WaitGroup
	for i, name := range balance.RefCheckpoints {
		ar := newArena(t, r)
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = refRun(ar, refCheckpointByName(name), rand.New(rand.NewSource(int64(i)+1)), refEvalRolls, false)
		}()
	}
	wg.Wait()
	var refs []balance.Ref
	for _, pol := range botPolicies {
		for _, res := range results {
			par, lucky, wrong := res.row("par", pol), res.row("lucky", pol), res.row("wrong", pol)
			refs = append(refs, balance.Ref{Policy: pol.name, Checkpoint: res.cp.name,
				LuckyPar: kills(pol, par[0]) / kills(pol, lucky[0]), WrongKill: kills(pol, wrong[0]) / kills(pol, par[0]), WrongLive: wrong[0].Turns / par[0].Turns})
		}
	}
	return refs
}

// writeResult writes the result as JSON, making the directory.
func writeResult(path string, res *balance.Result) error {
	res.Metrics = balance.StripNaN(res.Metrics)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(res, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// gitRevision is the short commit and whether tracked files differ from it.
func gitRevision() (rev string, dirty bool) {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "", false
	}
	rev = strings.TrimSpace(string(out))
	st, err := exec.Command("git", "status", "--porcelain", "--untracked-files=no").Output()
	return rev, err == nil && len(strings.TrimSpace(string(st))) > 0
}

// codeFiles are the game's sources, not the tests, sorted.
func codeFiles() []string {
	all, _ := filepath.Glob("*.go")
	var out []string
	for _, f := range all {
		if !strings.HasSuffix(f, "_test.go") {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// hashFiles is a short content hash over the files, in order.
func hashFiles(files []string) string {
	h := sha256.New()
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return ""
		}
		fmt.Fprintf(h, "%s\x00", f)
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))[:10]
}

// TestKnobs prints the knob registry (make knobs): what a balance pass
// may turn, with today's value.
func TestKnobs(t *testing.T) {
	r := DefaultRules()
	t.Logf("%-18s %-8s %9s %9s %9s %7s  %s", "knob", "group", "value", "lo", "hi", "step", "what it does")
	for _, k := range knobs {
		whole := ""
		if k.Int {
			whole = " (whole number)"
		}
		t.Logf("%-18s %-8s %9g %9g %9g %7g  %s%s", k.Name, k.Group, *k.Ptr(r), k.Lo, k.Hi, k.Step, k.Desc, whole)
	}
}
