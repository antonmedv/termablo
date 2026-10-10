// Package balance is the measuring side of a balance pass: the rows one
// evaluation of the game produces (Run, Snap, Ref), the metrics
// computed from them, the goals they are held to and the score. It knows
// nothing about the game: the test that plays the bots (eval_test.go)
// fills the rows, cmd/balance compares and logs results. One place for
// the arithmetic, so the test and the tool cannot disagree.
package balance

import (
	"fmt"
	"math"
	"math/rand"
	"slices"
	"sort"
	"strings"
)

// A Run is one bot policy played on one seed to its end.
type Run struct {
	Policy       string
	Seed         int64
	Turns        int
	Dead         bool
	By           string // the killer
	Where        string // level at the end
	WhereDepth   int
	Stuck        bool
	King, Oracle bool // bosses slain
	Lvl, Kills   int
	Deepest      string
	DeepestDepth int
	Gold         int   // in hand at the end
	In           []int // gold by source, GoldSrcNames order
	Out          []int // gold by sink, GoldSinkNames order
	Potions      int   // healing potions drunk
	MPots        int   // mana potions drunk
	Bolts, Novas int   // spells cast
	Trips        int   // arrivals in town
	Equips       int
	Levels       int // levels left behind
	Fail         string
	ByRank       string // the killer's rank: normal, champion, unique, boss, none
	Adj, Awake   int    // hostiles adjacent and awake in view at death
	PotionsLeft  int    // healing potions in the belt at death
	ScrollsLeft  int    // portal scrolls at death
	Snaps        []Snap
}

// Ranks a killer can have.
var Ranks = []string{"normal", "champion", "unique", "boss", "none"}

// A Snap is the hero on first arrival at a level. Kill measures come
// from the sampler against the area's typical normal at that depth and
// are only taken at checkpoints (Sampled).
type Snap struct {
	Level                     string
	Depth, Turn, Lvl          int
	HP, Armor, Gear, Gold     int
	MinD, MaxD, BoltLo, BoltH int
	Sampled                   bool
	Swings, Bolts, Live       float64 // swings to kill, bolts to kill, turns survived
	Pack, Speed               float64 // the typical normal's mean pack size and speed (1 = one attack a turn)
}

// A Ref is a reference-hero measure at a checkpoint: the fixtures, not
// the bot. LuckyPar is par kills over lucky kills (how much the best gear
// helps), WrongKill and WrongLive are the par hero three depths too deep
// over itself at par depth.
type Ref struct {
	Policy, Checkpoint             string
	LuckyPar, WrongKill, WrongLive float64
}

// Names for the gold counters, in Run.In and Run.Out order.
var (
	GoldSrcNames  = []string{"drop", "chest", "sell", "quest"}
	GoldSinkNames = []string{"potion", "scroll", "heal", "gear", "reroll", "gamble"}
)

// Checkpoints the metrics tabulate arrivals at, and the reference
// checkpoints (a subset).
var (
	Checkpoints    = []string{"crypt2", "crypt4", "grotto1", "grotto3"}
	RefCheckpoints = []string{"crypt4", "grotto1"}
	Areas          = []string{"fields", "crypt", "barrow", "marsh", "grotto", "abyss"}
	Policies       = []string{"fighter", "caster"}
)

// CheckpointMeasures are the per-arrival numbers a checkpoint carries;
// each becomes a metric policy.measure.checkpoint.
var CheckpointMeasures = []string{"kills", "live", "packLive", "fight", "killsSpread", "liveSpread", "turnAt", "gear", "clvl"}

// Route depths, as the bot counts them.
const (
	AbyssDepth   = 10 // abyss1
	GrottoDepth  = 7  // grotto1, where the gold band prices a gamble
	MinArrivals  = 3  // fewer arrivals at a checkpoint leave its measures unmeasured
	MaxStuck     = 2  // more stuck runs is a hard fail
	StartingGold = 60
)

// A Result is one evaluation: where it ran, what it ran, what it found.
type Result struct {
	Name     string             `json:"name"`
	Time     string             `json:"time"`
	Rev      string             `json:"rev"`
	Dirty    bool               `json:"dirty"`
	Hashes   map[string]string  `json:"hashes"` // code, bot: content hashes
	First    int                `json:"first"`
	Seeds    int                `json:"seeds"`
	Turns    int                `json:"turns"`
	Rules    map[string]float64 `json:"rules"`
	Changed  []string           `json:"changed"` // knobs that differ from the defaults
	Warnings []string           `json:"warnings,omitempty"`
	Gamble   float64            `json:"gamble"` // a gamble's price at GrottoDepth under Rules
	Refs     []Ref              `json:"refs"`
	Runs     []Run              `json:"runs"`

	Metrics map[string]float64 `json:"metrics"` // NaN left out
	Score   *float64           `json:"score"`   // nil on a hard fail
	ScoreCI [2]float64         `json:"scoreCI"` // bootstrap over seeds, 95%
	Fail    string             `json:"fail,omitempty"`
	Misses  []Miss             `json:"misses"`
	Goals   []Goal             `json:"goals"`
	Seconds float64            `json:"seconds"`
}

// ------------------------------------------------------------ goals

// A Goal is one balance target: a metric (with * for each policy, @ for
// each checkpoint, # for each reference checkpoint), the range that
// scores 0, and the weight on the distance outside it. Distance is the
// relative overshoot, (v − hi)/hi or (lo − v)/lo; lo 0 means no floor; a
// metric that could not be measured counts as a full miss (1). Weight 0
// is reported, never scored.
type Goal struct {
	Metric string  `json:"metric"`
	Lo     float64 `json:"lo"`
	Hi     float64 `json:"hi"`
	W      float64 `json:"w"`
	Why    string  `json:"why"`
}

// Goals are the targets a balance pass optimizes, lower score better.
// Edit here; the eval writes the table it used into every result.
var Goals = []Goal{
	{"*.king", 0.70, 0.85, 3, "most runs of either build slay the Bone King"},
	{"*.oracle", 0.30, 0.45, 3, "a good third reach and slay the Oracle"},
	{"abyssDeath", 11, 13, 2, "runs that reach the abyss die on its second or third floor; until any does, the median death depth stands in"},
	{"gap", 0, 0.15, 2, "fighter and caster boss rates within 15 points"},
	{"*.kills.@", 3, 5, 1, "swings (fighter) or bolts (caster) to kill the area's typical normal on arrival"},
	{"*.live.@", 24, 36, 0, "turns survived from full life against one typical normal, no potions (reported; packLive is scored)"},
	{"*.packLive.@", 8, 12, 1, "turns survived against the area's typical pack: live over mean pack size times speed"},
	{"*.fight.@", 1, 2, 0.5, "packLive over the turns it takes to clear the pack (kills times pack size): above 1 the hero outlasts a typical pack without potions"},
	{"*.killsSpread.@", 0, 3, 0.5, "P90/P10 of kills across arrivals: luck spreads power at most 3x"},
	{"*.liveSpread.@", 0, 3, 0.5, "P90/P10 of turns survived across arrivals"},
	{"*.luckyPar.#", 0, 3, 1, "the best-of-30-rares hero kills at most 3x faster than par"},
	{"*.wrongKill.#", 2, 4, 1, "three depths too deep, par kills 2–4x slower"},
	{"*.wrongLive.#", 0.2, 0.5, 1, "three depths too deep, par lives a fifth to a half as long"},
	{"*.gold", 0.8, 1.5, 1, "gold left after potions, scrolls and healing per level cleared, in gambles at grotto depth: gold has a use and does not pile up"},
	{"*.equips", 0.5, 1.0, 0, "upgrades worn per level cleared (the bot equips any gain: taste, not rules)"},
}

// Expand spells out the placeholders in the goals.
func Expand(goals []Goal) []Goal {
	var out []Goal
	for _, g := range goals {
		pols := []string{""}
		if strings.Contains(g.Metric, "*") {
			pols = Policies
		}
		cps := []string{""}
		switch {
		case strings.Contains(g.Metric, "@"):
			cps = Checkpoints
		case strings.Contains(g.Metric, "#"):
			cps = RefCheckpoints
		}
		for _, p := range pols {
			for _, cp := range cps {
				n := strings.NewReplacer("*", p, "@", cp, "#", cp).Replace(g.Metric)
				out = append(out, Goal{n, g.Lo, g.Hi, g.W, g.Why})
			}
		}
	}
	return out
}

// Distance is how far v lies outside the goal, relative.
func (g Goal) Distance(v float64) float64 {
	switch {
	case math.IsNaN(v):
		return 1
	case v > g.Hi:
		return (v - g.Hi) / g.Hi
	case g.Lo > 0 && v < g.Lo:
		return (g.Lo - v) / g.Lo
	}
	return 0
}

// unmeasured is the distance of a checkpoint metric nobody arrived to
// measure: a full miss with no arrivals, shrinking with each arrival
// short of MinArrivals, so the score does not jump when the third run
// gets there. Any other unmeasured metric is a full miss.
func unmeasured(m map[string]float64, metric string) float64 {
	parts := strings.Split(metric, ".")
	if len(parts) != 3 || !slices.Contains(CheckpointMeasures, parts[1]) {
		return 1
	}
	arrivals, ok := m[parts[0]+".arrivals."+parts[2]]
	if !ok || math.IsNaN(arrivals) {
		return 1
	}
	return 1 - math.Min(arrivals, MinArrivals)/MinArrivals
}

// A Miss is a goal's contribution to the score.
type Miss struct {
	Metric string  `json:"metric"`
	Value  float64 `json:"value"` // NaN is written as -1
	Lo     float64 `json:"lo"`
	Hi     float64 `json:"hi"`
	W      float64 `json:"w"`
	D      float64 `json:"d"` // w × distance
}

func (m Miss) String() string {
	v := fmt.Sprintf("%.2f", m.Value)
	if m.Value < 0 {
		v = "n/a"
	}
	return fmt.Sprintf("%-24s %6s  (%g–%g)  +%.2f", m.Metric, v, m.Lo, m.Hi, m.D)
}

// Score is the weighted sum of goal distances, lower is better, with the
// misses that make it up, largest first. Unweighted goals are listed
// with D 0 when missed, so the report shows them.
func Score(m map[string]float64, goals []Goal) (float64, []Miss) {
	total := 0.0
	var ms []Miss
	for _, g := range Expand(goals) {
		v, ok := m[g.Metric]
		if !ok {
			v = math.NaN()
		}
		dist := g.Distance(v)
		if math.IsNaN(v) {
			dist = unmeasured(m, g.Metric)
		}
		if dist > 0 {
			d := dist * g.W
			total += d
			if math.IsNaN(v) {
				v = -1
			}
			ms = append(ms, Miss{g.Metric, v, g.Lo, g.Hi, g.W, d})
		}
	}
	sort.SliceStable(ms, func(i, j int) bool { return ms[i].D > ms[j].D })
	return total, ms
}

// ------------------------------------------------------------ metrics

// Metrics computes every named number from the rows. gamble is the price
// of a gamble at grotto depth, the unit of the gold band. A metric that
// cannot be measured is NaN.
func Metrics(runs []Run, refs []Ref, gamble float64) map[string]float64 {
	m := map[string]float64{}
	var abyssDeaths, allDeaths []float64
	stuck := 0
	for _, pol := range Policies {
		var prs []Run
		for _, r := range runs {
			if r.Policy == pol {
				prs = append(prs, r)
			}
		}
		p := pol
		n := float64(len(prs))
		if len(prs) == 0 {
			continue
		}
		var deep, lvls, kills, turns, gold, goldIn, goldHeld, equips, potions, trips, deaths, adj, left, scrolls, bolts []float64
		king, oracle, pstuck, alive, arrivals := 0, 0, 0, 0, 0
		byArea, byRank := map[string]int{}, map[string]int{}
		in := make([]float64, len(GoldSrcNames))
		out := make([]float64, len(GoldSinkNames))
		for _, r := range prs {
			if r.King {
				king++
			}
			if r.Oracle {
				oracle++
			}
			if r.Stuck {
				pstuck++
				stuck++
			}
			if !r.Dead && !r.Stuck {
				alive++
			}
			deep = append(deep, float64(r.DeepestDepth))
			lvls, kills, turns = append(lvls, float64(r.Lvl)), append(kills, float64(r.Kills)), append(turns, float64(r.Turns))
			gold = append(gold, Surplus(r)/gamble)
			goldIn, goldHeld = append(goldIn, float64(sum(r.In))), append(goldHeld, float64(r.Gold))
			levels := math.Max(1, float64(r.Levels))
			equips, potions = append(equips, float64(r.Equips)/levels), append(potions, float64(r.Potions)/levels)
			trips = append(trips, float64(r.Trips))
			bolts = append(bolts, float64(r.Bolts)/math.Max(1, float64(r.Kills)))
			if r.DeepestDepth >= AbyssDepth {
				arrivals++
				if r.Dead {
					deaths = append(deaths, float64(r.WhereDepth))
				}
			}
			if r.Dead {
				byArea[area(r.Where)]++
				byRank[r.ByRank]++
				allDeaths = append(allDeaths, float64(r.WhereDepth))
				adj, left, scrolls = append(adj, float64(r.Adj)), append(left, float64(r.PotionsLeft)), append(scrolls, float64(r.ScrollsLeft))
			}
			for i, v := range r.In {
				in[i] += float64(v) / n
			}
			for i, v := range r.Out {
				out[i] += float64(v) / n
			}
		}
		m[p+".king"], m[p+".oracle"] = float64(king)/n, float64(oracle)/n
		m[p+".deepest"], m[p+".clvl"], m[p+".kills"], m[p+".turns"] = Pct(deep, .5), Pct(lvls, .5), Pct(kills, .5), Pct(turns, .5)
		m[p+".gold"], m[p+".goldIn"], m[p+".goldHeld"] = Pct(gold, .5), Pct(goldIn, .5), Pct(goldHeld, .5)
		m[p+".equips"], m[p+".potions"], m[p+".trips"] = Pct(equips, .5), Pct(potions, .5), Pct(trips, .5)
		m[p+".boltsPerKill"] = Pct(bolts, .5)
		m[p+".stuck"], m[p+".alive"], m[p+".abyssArrivals"] = float64(pstuck), float64(alive), float64(arrivals)
		m[p+".abyssDeath"] = math.NaN()
		if len(deaths) > 0 {
			m[p+".abyssDeath"] = Pct(deaths, .5)
		}
		abyssDeaths = append(abyssDeaths, deaths...)
		for _, a := range Areas {
			m[p+".deaths."+a] = float64(byArea[a])
		}
		for _, k := range Ranks {
			m[p+".deathsBy."+k] = float64(byRank[k])
		}
		m[p+".deathAdj"], m[p+".deathPotions"], m[p+".deathScrolls"] = Pct(adj, .5), Pct(left, .5), Pct(scrolls, .5)
		for i, s := range GoldSrcNames {
			m[p+".in."+s] = in[i]
		}
		for i, s := range GoldSinkNames {
			m[p+".out."+s] = out[i]
		}
		for _, cp := range Checkpoints {
			var ks, live, packLive, fight, turn, gear, clvl []float64
			for _, r := range prs {
				for _, s := range r.Snaps {
					if s.Level != cp || !s.Sampled {
						continue
					}
					k := s.Swings
					if pol == "caster" {
						k = s.Bolts
					}
					ks, live = append(ks, k), append(live, s.Live)
					turn, gear, clvl = append(turn, float64(s.Turn)), append(gear, float64(s.Gear)), append(clvl, float64(s.Lvl))
					if s.Pack > 0 && s.Speed > 0 {
						pl := s.Live / (s.Pack * s.Speed)
						packLive, fight = append(packLive, pl), append(fight, pl/(k*s.Pack))
					}
				}
			}
			m[p+".arrivals."+cp] = float64(len(ks))
			for _, k := range CheckpointMeasures {
				m[p+"."+k+"."+cp] = math.NaN()
			}
			if len(ks) >= MinArrivals {
				m[p+".kills."+cp], m[p+".live."+cp] = Pct(ks, .5), Pct(live, .5)
				m[p+".killsSpread."+cp], m[p+".liveSpread."+cp] = Pct(ks, .9)/Pct(ks, .1), Pct(live, .9)/Pct(live, .1)
				m[p+".turnAt."+cp], m[p+".gear."+cp], m[p+".clvl."+cp] = Pct(turn, .5), Pct(gear, .5), Pct(clvl, .5)
			}
			if len(packLive) >= MinArrivals {
				m[p+".packLive."+cp], m[p+".fight."+cp] = Pct(packLive, .5), Pct(fight, .5)
			}
		}
		for _, cp := range RefCheckpoints {
			m[p+".luckyPar."+cp], m[p+".wrongKill."+cp], m[p+".wrongLive."+cp] = math.NaN(), math.NaN(), math.NaN()
		}
	}
	for _, r := range refs {
		m[r.Policy+".luckyPar."+r.Checkpoint] = r.LuckyPar
		m[r.Policy+".wrongKill."+r.Checkpoint] = r.WrongKill
		m[r.Policy+".wrongLive."+r.Checkpoint] = r.WrongLive
	}
	m["king"], m["oracle"] = (m["fighter.king"]+m["caster.king"])/2, (m["fighter.oracle"]+m["caster.oracle"])/2
	m["gapKing"], m["gapOracle"] = math.Abs(m["fighter.king"]-m["caster.king"]), math.Abs(m["fighter.oracle"]-m["caster.oracle"])
	m["gap"] = math.Max(m["gapKing"], m["gapOracle"])
	switch {
	case len(abyssDeaths) > 0:
		m["abyssDeath"] = Pct(abyssDeaths, .5)
	case len(allDeaths) > 0:
		m["abyssDeath"] = Pct(allDeaths, .5)
	default:
		m["abyssDeath"] = math.NaN()
	}
	m["stuck"] = float64(stuck)
	return m
}

// Surplus is a run's gold left after consumables, per level cleared.
func Surplus(r Run) float64 {
	consumables := 0
	for i, s := range GoldSinkNames {
		if s == "potion" || s == "scroll" || s == "heal" {
			consumables += r.Out[i]
		}
	}
	return float64(sum(r.In)-consumables) / math.Max(1, float64(r.Levels))
}

// Reconciled says whether the wallet matched the counters at the end.
func Reconciled(r Run) bool { return r.Gold == StartingGold+sum(r.In)-sum(r.Out) }

// HardFail names what makes a result unscorable: a panic, a wallet that
// does not reconcile, too many stuck runs.
func HardFail(runs []Run) string {
	var fails []string
	stuck := 0
	for _, r := range runs {
		switch {
		case r.Fail != "":
			fails = append(fails, fmt.Sprintf("%s seed %d: %s", r.Policy, r.Seed, r.Fail))
		case !Reconciled(r):
			fails = append(fails, fmt.Sprintf("%s seed %d: gold %d, counters say %d", r.Policy, r.Seed, r.Gold, StartingGold+sum(r.In)-sum(r.Out)))
		}
		if r.Stuck {
			stuck++
		}
	}
	if stuck > MaxStuck {
		fails = append(fails, fmt.Sprintf("%d stuck runs", stuck))
	}
	return strings.Join(fails, "; ")
}

// Evaluate fills a result's metrics, score, misses, goals and the
// bootstrap interval from its rows.
func Evaluate(res *Result) {
	res.Goals = Goals
	res.Metrics = Metrics(res.Runs, res.Refs, res.Gamble)
	res.Fail = HardFail(res.Runs)
	res.Score, res.Misses = nil, nil
	if res.Fail != "" {
		return
	}
	s, ms := Score(res.Metrics, Goals)
	res.Score, res.Misses = &s, ms
	res.ScoreCI = BootstrapCI(res, 200, 1)
}

// BootstrapCI resamples seeds with replacement n times and returns the
// 2.5th and 97.5th percentile of the score.
func BootstrapCI(res *Result, n int, seed int64) [2]float64 {
	rng := rand.New(rand.NewSource(seed))
	seeds := SeedsOf(res.Runs)
	var scores []float64
	for range n {
		pick := make([]int64, len(seeds))
		for i := range pick {
			pick[i] = seeds[rng.Intn(len(seeds))]
		}
		s, _ := Score(Metrics(Resample(res.Runs, pick), res.Refs, res.Gamble), Goals)
		scores = append(scores, s)
	}
	return [2]float64{Pct(scores, .025), Pct(scores, .975)}
}

// SeedsOf lists the distinct seeds in the rows, ascending.
func SeedsOf(runs []Run) []int64 {
	set := map[int64]bool{}
	for _, r := range runs {
		set[r.Seed] = true
	}
	var out []int64
	for s := range set {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Resample keeps the runs of the picked seeds, once per pick.
func Resample(runs []Run, pick []int64) []Run {
	by := map[int64][]Run{}
	for _, r := range runs {
		by[r.Seed] = append(by[r.Seed], r)
	}
	var out []Run
	for _, s := range pick {
		out = append(out, by[s]...)
	}
	return out
}

// StripNaN drops unmeasured metrics, so the map can be written as JSON.
func StripNaN(m map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			out[k] = v
		}
	}
	return out
}

// Pct is the q-th percentile of xs by nearest rank; NaN of nothing.
func Pct(xs []float64, q float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	ys := append([]float64(nil), xs...)
	sort.Float64s(ys)
	return ys[int(math.Round(q*float64(len(ys)-1)))]
}

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}

// area is the region of a level ID: "crypt3" is "crypt".
func area(id string) string {
	i := len(id)
	for i > 0 && id[i-1] >= '0' && id[i-1] <= '9' {
		i--
	}
	return id[:i]
}
