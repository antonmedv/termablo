package balance

import (
	"math"
	"math/rand"
	"sort"
)

// Two results on the same seeds compare pairwise: each (policy, seed)
// run in A against the same in B. That removes the seed's own luck from
// the difference, so far fewer seeds tell a real change from noise.

// A Paired is one per-run measure compared across two results.
type Paired struct {
	Name         string
	N            int     // pairs
	MeanA, MeanB float64 // over the pairs
	Diff         float64 // mean of B − A
	SE           float64 // standard error of the mean difference
	Lo, Hi       float64 // 95% interval of the difference
	Up, Down     int     // pairs where B is above, below A
}

// Sure says whether the interval excludes zero.
func (p Paired) Sure() bool { return p.N >= 2 && (p.Lo > 0 || p.Hi < 0) }

// runMeasures are the per-run numbers worth pairing, by name.
var runMeasures = []struct {
	name string
	f    func(Run) float64
}{
	{"deepest", func(r Run) float64 { return float64(r.DeepestDepth) }},
	{"clvl", func(r Run) float64 { return float64(r.Lvl) }},
	{"kills", func(r Run) float64 { return float64(r.Kills) }},
	{"turns", func(r Run) float64 { return float64(r.Turns) }},
	{"king", func(r Run) float64 { return b2f(r.King) }},
	{"oracle", func(r Run) float64 { return b2f(r.Oracle) }},
	{"dead", func(r Run) float64 { return b2f(r.Dead) }},
	{"goldIn", func(r Run) float64 { return float64(sum(r.In)) }},
	{"goldHeld", func(r Run) float64 { return float64(r.Gold) }},
	{"surplus/lvl", Surplus},
	{"potions/lvl", func(r Run) float64 { return float64(r.Potions) / math.Max(1, float64(r.Levels)) }},
	{"equips/lvl", func(r Run) float64 { return float64(r.Equips) / math.Max(1, float64(r.Levels)) }},
	{"trips", func(r Run) float64 { return float64(r.Trips) }},
	{"bolts/kill", func(r Run) float64 { return float64(r.Bolts) / math.Max(1, float64(r.Kills)) }},
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// snapMeasures are the per-arrival numbers worth pairing.
var snapMeasures = []struct {
	name string
	f    func(pol string, s Snap) float64
}{
	{"kills", func(pol string, s Snap) float64 {
		if pol == "caster" {
			return s.Bolts
		}
		return s.Swings
	}},
	{"live", func(_ string, s Snap) float64 { return s.Live }},
	{"turnAt", func(_ string, s Snap) float64 { return float64(s.Turn) }},
	{"gear", func(_ string, s Snap) float64 { return float64(s.Gear) }},
}

type runKey struct {
	pol  string
	seed int64
}

// ComparePolicy pairs the runs of one policy in a and b and returns the
// paired measures: the run-level ones, then each checkpoint's.
func ComparePolicy(pol string, a, b *Result) []Paired {
	ra, rb := map[runKey]Run{}, map[runKey]Run{}
	for _, r := range a.Runs {
		ra[runKey{r.Policy, r.Seed}] = r
	}
	for _, r := range b.Runs {
		rb[runKey{r.Policy, r.Seed}] = r
	}
	var keys []runKey
	for k := range ra {
		if _, ok := rb[k]; ok && k.pol == pol {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].seed < keys[j].seed })
	var out []Paired
	for _, m := range runMeasures {
		var xs, ys []float64
		for _, k := range keys {
			xs, ys = append(xs, m.f(ra[k])), append(ys, m.f(rb[k]))
		}
		out = append(out, pair(m.name, xs, ys))
	}
	for _, cp := range Checkpoints {
		for _, m := range snapMeasures {
			var xs, ys []float64
			for _, k := range keys {
				sa, oka := snapAt(ra[k], cp)
				sb, okb := snapAt(rb[k], cp)
				if oka && okb {
					xs, ys = append(xs, m.f(pol, sa)), append(ys, m.f(pol, sb))
				}
			}
			out = append(out, pair(m.name+"."+cp, xs, ys))
		}
	}
	return out
}

func snapAt(r Run, level string) (Snap, bool) {
	for _, s := range r.Snaps {
		if s.Level == level && s.Sampled {
			return s, true
		}
	}
	return Snap{}, false
}

// pair computes the paired statistics of ys − xs.
func pair(name string, xs, ys []float64) Paired {
	p := Paired{Name: name, N: len(xs)}
	if p.N == 0 {
		p.Lo, p.Hi = math.NaN(), math.NaN()
		return p
	}
	var sx, sy, sd, sd2 float64
	for i := range xs {
		d := ys[i] - xs[i]
		sx, sy, sd, sd2 = sx+xs[i], sy+ys[i], sd+d, sd2+d*d
		switch {
		case d > 0:
			p.Up++
		case d < 0:
			p.Down++
		}
	}
	n := float64(p.N)
	p.MeanA, p.MeanB, p.Diff = sx/n, sy/n, sd/n
	if p.N >= 2 {
		v := (sd2 - sd*sd/n) / (n - 1)
		p.SE = math.Sqrt(math.Max(0, v) / n)
	}
	t := tCrit(p.N - 1)
	p.Lo, p.Hi = p.Diff-t*p.SE, p.Diff+t*p.SE
	return p
}

// tCrit is the two-sided 95% t critical value, coarse.
func tCrit(df int) float64 {
	switch {
	case df <= 0:
		return math.Inf(1)
	case df < 5:
		return 2.78
	case df < 10:
		return 2.26
	case df < 20:
		return 2.09
	case df < 40:
		return 2.02
	}
	return 1.96
}

// ScoreDiff bootstraps the score of b minus a over the shared seeds,
// resampling the same seeds in both, and returns the mean difference
// and its 95% interval.
func ScoreDiff(a, b *Result, n int, seed int64) (diff, lo, hi float64) {
	rng := rand.New(rand.NewSource(seed))
	shared := sharedSeeds(a, b)
	if len(shared) == 0 || a.Score == nil || b.Score == nil {
		return math.NaN(), math.NaN(), math.NaN()
	}
	var ds []float64
	for range n {
		pick := make([]int64, len(shared))
		for i := range pick {
			pick[i] = shared[rng.Intn(len(shared))]
		}
		sa, _ := Score(Metrics(Resample(a.Runs, pick), a.Refs, a.Gamble), Goals)
		sb, _ := Score(Metrics(Resample(b.Runs, pick), b.Refs, b.Gamble), Goals)
		ds = append(ds, sb-sa)
	}
	sa, _ := Score(Metrics(Resample(a.Runs, shared), a.Refs, a.Gamble), Goals)
	sb, _ := Score(Metrics(Resample(b.Runs, shared), b.Refs, b.Gamble), Goals)
	return sb - sa, Pct(ds, .025), Pct(ds, .975)
}

func sharedSeeds(a, b *Result) []int64 {
	in := map[int64]bool{}
	for _, s := range SeedsOf(b.Runs) {
		in[s] = true
	}
	var out []int64
	for _, s := range SeedsOf(a.Runs) {
		if in[s] {
			out = append(out, s)
		}
	}
	return out
}

// SeedsFor is how many seeds put the 95% half-width of a mean at most
// want, given the per-seed standard deviation sd of one policy's runs:
// (1.96·sd/want)², at least 1. For a rate, sd = sqrt(p(1−p)).
func SeedsFor(sd, want float64) int {
	if want <= 0 || sd == 0 {
		return 1
	}
	z := 1.96 * sd / want
	return int(math.Ceil(z * z))
}

// SD is the sample standard deviation.
func SD(xs []float64) float64 {
	n := float64(len(xs))
	if n < 2 {
		return 0
	}
	var s, s2 float64
	for _, x := range xs {
		s, s2 = s+x, s2+x*x
	}
	return math.Sqrt(math.Max(0, (s2-s*s/n)/(n-1)))
}

// Measure evaluates a run-level measure by name for every run of a
// policy, for SeedsFor.
func Measure(name, pol string, runs []Run) []float64 {
	var out []float64
	for _, m := range runMeasures {
		if m.name != name {
			continue
		}
		for _, r := range runs {
			if r.Policy == pol {
				out = append(out, m.f(r))
			}
		}
	}
	return out
}
