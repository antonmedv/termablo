package balance

import (
	"math"
	"testing"
)

func TestExpandAndScore(t *testing.T) {
	goals := Expand(Goals)
	seen := map[string]bool{}
	for _, g := range goals {
		if seen[g.Metric] {
			t.Errorf("%s expanded twice", g.Metric)
		}
		seen[g.Metric] = true
		if g.Lo > g.Hi || g.W < 0 {
			t.Errorf("%s: band %v–%v weight %v", g.Metric, g.Lo, g.Hi, g.W)
		}
	}
	for _, want := range []string{"fighter.king", "caster.kills.grotto3", "fighter.luckyPar.crypt4", "abyssDeath", "gap"} {
		if !seen[want] {
			t.Errorf("%s not expanded", want)
		}
	}
	g := Goal{Metric: "x", Lo: 2, Hi: 4, W: 2}
	for v, want := range map[float64]float64{3: 0, 5: 0.25, 1: 0.5, math.NaN(): 1} {
		if d := g.Distance(v); d != want {
			t.Errorf("distance(%v) = %v, want %v", v, d, want)
		}
	}
	m := map[string]float64{}
	for _, g := range goals {
		m[g.Metric] = (g.Lo + g.Hi) / 2 // everything in band
	}
	if s, ms := Score(m, Goals); s != 0 || len(ms) != 0 {
		t.Errorf("all in band scores %v with %d misses", s, len(ms))
	}
	m["fighter.king"] = 0.35 // half the floor: distance 0.5 × weight 3
	s, ms := Score(m, Goals)
	if s != 1.5 || len(ms) != 1 || ms[0].Metric != "fighter.king" {
		t.Errorf("one miss scores %v: %v", s, ms)
	}
	delete(m, "caster.oracle") // unmeasured: a full miss
	if s, _ := Score(m, Goals); s != 4.5 {
		t.Errorf("missing metric scores %v, want 4.5", s)
	}
	m["caster.oracle"] = 0.4
	delete(m, "fighter.kills.grotto3") // unmeasured at a checkpoint: shrinks with arrivals
	m["fighter.arrivals.grotto3"] = 2
	if s, _ := Score(m, Goals); math.Abs(s-1.5-1.0/3) > 1e-9 {
		t.Errorf("two arrivals score %v, want 1.833", s)
	}
}

func run(pol string, seed int64, deepest int, king bool) Run {
	return Run{Policy: pol, Seed: seed, Dead: true, Where: "crypt1", WhereDepth: 2, DeepestDepth: deepest, King: king, Lvl: 5, Levels: 2,
		Gold: 60 + 300 - 100, In: []int{300, 0, 0, 0}, Out: []int{100, 0, 0, 0, 0, 0},
		Snaps: []Snap{{Level: "crypt2", Depth: 3, Sampled: true, Swings: 4, Bolts: 2, Live: 30, Pack: 3, Speed: 1}}}
}

func TestMetrics(t *testing.T) {
	var runs []Run
	for s := int64(1); s <= 4; s++ {
		runs = append(runs, run("fighter", s, 3, s%2 == 0), run("caster", s, 4, true))
	}
	m := Metrics(runs, []Ref{{"fighter", "crypt4", 2, 3, .3}}, 100)
	checks := map[string]float64{"fighter.king": .5, "caster.king": 1, "gap": .5, "fighter.deepest": 3, "caster.clvl": 5,
		"fighter.gold": 1, "fighter.kills.crypt2": 4, "caster.kills.crypt2": 2, "fighter.live.crypt2": 30, "fighter.arrivals.crypt2": 4,
		"fighter.luckyPar.crypt4": 2, "abyssDeath": 2, "fighter.deaths.crypt": 4, "stuck": 0,
		"fighter.packLive.crypt2": 10, "fighter.fight.crypt2": 10.0 / 12, "caster.fight.crypt2": 10.0 / 6}
	for k, want := range checks {
		if got := m[k]; got != want {
			t.Errorf("%s = %v, want %v", k, got, want)
		}
	}
	if !math.IsNaN(m["caster.luckyPar.crypt4"]) || !math.IsNaN(m["fighter.kills.grotto1"]) {
		t.Errorf("unmeasured metrics are not NaN: %v %v", m["caster.luckyPar.crypt4"], m["fighter.kills.grotto1"])
	}
	if !Reconciled(runs[0]) {
		t.Error("wallet does not reconcile")
	}
	runs[0].Gold++
	if HardFail(runs) == "" {
		t.Error("a wallet off by one is not a hard fail")
	}
}

func TestPaired(t *testing.T) {
	p := pair("x", []float64{1, 2, 3, 4, 5, 6}, []float64{2, 3, 4, 5, 6, 7})
	if p.N != 6 || p.Diff != 1 || p.SE != 0 || !p.Sure() || p.Up != 6 {
		t.Errorf("constant shift: %+v", p)
	}
	p = pair("y", []float64{1, 2, 3, 4}, []float64{4, 1, 2, 3})
	if p.Sure() || p.Diff != 0 || p.Up != 1 || p.Down != 3 {
		t.Errorf("shuffle: %+v", p)
	}
	if n := SeedsFor(0.5, 0.1); n != 97 {
		t.Errorf("SeedsFor(.5, .1) = %d, want 97", n)
	}
	if sd := SD([]float64{2, 4, 4, 4, 5, 5, 7, 9}); math.Abs(sd-2.138) > 0.001 {
		t.Errorf("SD = %v", sd)
	}
}
