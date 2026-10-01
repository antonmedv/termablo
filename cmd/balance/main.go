// Command balance is the bookkeeping side of a balance pass. It reads
// the JSON results that make eval writes (internal/balance.Result) and
// never touches the game:
//
//	go run ./cmd/balance show   RUN.json            metrics, misses and score of one result
//	go run ./cmd/balance compare BASE.json RUN.json  paired per-seed differences, score change
//	go run ./cmd/balance seeds  RUN.json            how many seeds the key measures need
//	go run ./cmd/balance log    RUN.json -change .. -why .. -keep|-revert [-base BASE.json]
//	go run ./cmd/balance ls                          the experiment ledger
//
// The ledger is balance/ledger.jsonl, one entry per evaluation the pass
// decided on; balance/best.json is the best-known rules by score, and
// balance/LEDGER.md the ledger as a table. Every entry records the code
// and bot revisions, the rules, seeds, score, key metrics, what changed
// and why, the decision, and how it compared with the base and the best.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/antonmedv/termablo/internal/balance"
)

const (
	ledgerPath = "balance/ledger.jsonl"
	ledgerMD   = "balance/LEDGER.md"
	bestPath   = "balance/best.json"
	bestRun    = "balance/best-run.json"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "show":
		err = show(args)
	case "compare":
		err = compare(args)
	case "seeds":
		err = seeds(args)
	case "log":
		err = logEntry(args)
	case "ls":
		err = ls()
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: balance show|compare|seeds|log|ls ...  (see the package comment)")
	os.Exit(2)
}

func load(path string) (*balance.Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var res balance.Result
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	// Metrics come back without the unmeasured ones; rebuild so NaN is
	// NaN again and the score matches the goals in force now.
	balance.Evaluate(&res)
	return &res, nil
}

func scoreStr(res *balance.Result) string {
	if res.Score == nil {
		return "FAIL"
	}
	return fmt.Sprintf("%.2f", *res.Score)
}

// ------------------------------------------------------------ show

func show(args []string) error {
	if len(args) != 1 {
		usage()
	}
	res, err := load(args[0])
	if err != nil {
		return err
	}
	fmt.Println(header(res))
	if res.Fail != "" {
		fmt.Println("hard fail:", res.Fail)
	}
	fmt.Printf("score %s (95%% %.2f–%.2f over seeds)\n", scoreStr(res), res.ScoreCI[0], res.ScoreCI[1])
	fmt.Println("\nmisses, largest first:")
	for _, m := range res.Misses {
		fmt.Println(" ", m)
	}
	fmt.Println("\nmetrics:")
	for _, k := range sortedKeys(res.Metrics) {
		fmt.Printf("  %-28s %s\n", k, num(res.Metrics[k]))
	}
	return nil
}

func header(res *balance.Result) string {
	dirty := ""
	if res.Dirty {
		dirty = "+dirty"
	}
	changed := "defaults"
	if len(res.Changed) > 0 {
		changed = strings.Join(res.Changed, ", ")
	}
	return fmt.Sprintf("%s · %s · rev %s%s · code %s · bot %s · seeds %d–%d · %s", res.Name, res.Time, res.Rev, dirty,
		res.Hashes["code"], res.Hashes["bot"], res.First, res.First+res.Seeds-1, changed)
}

func num(v float64) string {
	switch {
	case math.IsNaN(v):
		return "n/a"
	case v == math.Trunc(v) && math.Abs(v) < 1e6:
		return fmt.Sprintf("%d", int(v))
	}
	return fmt.Sprintf("%.2f", v)
}

func sortedKeys(m map[string]float64) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// ------------------------------------------------------------ compare

func compare(args []string) error {
	if len(args) != 2 {
		usage()
	}
	a, err := load(args[0])
	if err != nil {
		return err
	}
	b, err := load(args[1])
	if err != nil {
		return err
	}
	fmt.Println("A:", header(a))
	fmt.Println("B:", header(b))
	fmt.Println()
	diff, lo, hi := balance.ScoreDiff(a, b, 400, 1)
	verdict := "noise"
	switch {
	case math.IsNaN(diff):
		verdict = "unscorable"
	case hi < 0:
		verdict = "B better"
	case lo > 0:
		verdict = "B worse"
	}
	fmt.Printf("score A %s → B %s · B−A %+.2f (95%% %+.2f…%+.2f, paired over shared seeds) · %s\n", scoreStr(a), scoreStr(b), diff, lo, hi, verdict)
	fmt.Println("\ngoals (A → B, + means B misses by more):")
	seen := map[string]bool{}
	type row struct {
		m    string
		a, b float64
		d    float64
	}
	var rows []row
	for _, ms := range [][]balance.Miss{a.Misses, b.Misses} {
		for _, m := range ms {
			if seen[m.Metric] {
				continue
			}
			seen[m.Metric] = true
			rows = append(rows, row{m.Metric, a.Metrics[m.Metric], b.Metrics[m.Metric], missOf(b, m.Metric) - missOf(a, m.Metric)})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return math.Abs(rows[i].d) > math.Abs(rows[j].d) })
	for _, r := range rows {
		fmt.Printf("  %-26s %7s → %-7s %+.2f\n", r.m, num(r.a), num(r.b), r.d)
	}
	for _, pol := range balance.Policies {
		fmt.Printf("\n%s, paired by seed (B − A, 95%% interval; * = sure):\n", pol)
		fmt.Printf("  %-18s %5s %9s %9s %9s %18s %7s\n", "measure", "n", "A", "B", "diff", "interval", "up/down")
		for _, p := range balance.ComparePolicy(pol, a, b) {
			if p.N == 0 {
				continue
			}
			sure := ""
			if p.Sure() {
				sure = " *"
			}
			fmt.Printf("  %-18s %5d %9s %9s %+9.2f %8.2f…%-8.2f %3d/%-3d%s\n", p.Name, p.N, num2(p.MeanA), num2(p.MeanB), p.Diff, p.Lo, p.Hi, p.Up, p.Down, sure)
		}
	}
	return nil
}

func num2(v float64) string { return fmt.Sprintf("%.2f", v) }

// missOf is a metric's weighted distance in a result, 0 if in band.
func missOf(res *balance.Result, metric string) float64 {
	for _, m := range res.Misses {
		if m.Metric == metric {
			return m.D
		}
	}
	return 0
}

// ------------------------------------------------------------ seeds

// seeds says what the seed count buys: the spread of the key per-run
// measures and how many seeds put a 95% interval on their mean inside a
// useful width.
func seeds(args []string) error {
	if len(args) != 1 {
		usage()
	}
	res, err := load(args[0])
	if err != nil {
		return err
	}
	fmt.Println(header(res))
	fmt.Printf("score %s, 95%% %.2f–%.2f: half-width %.2f at %d seeds", scoreStr(res), res.ScoreCI[0], res.ScoreCI[1], (res.ScoreCI[1]-res.ScoreCI[0])/2, res.Seeds)
	if res.Score != nil {
		hw := (res.ScoreCI[1] - res.ScoreCI[0]) / 2
		for _, want := range []float64{2, 1, 0.5} {
			n := int(math.Ceil(float64(res.Seeds) * (hw / want) * (hw / want)))
			fmt.Printf(" · ±%g needs ~%d", want, n)
		}
	}
	fmt.Println()
	wants := []struct {
		name string
		want float64
	}{{"king", .10}, {"oracle", .10}, {"deepest", .5}, {"clvl", .5}, {"turns", 500}, {"surplus/lvl", 100}}
	fmt.Printf("\n%-8s %-12s %9s %9s %9s %10s %8s\n", "policy", "measure", "mean", "sd", "±95% now", "want ±", "seeds")
	for _, pol := range balance.Policies {
		for _, w := range wants {
			xs := balance.Measure(w.name, pol, res.Runs)
			if len(xs) == 0 {
				continue
			}
			mean := 0.0
			for _, x := range xs {
				mean += x
			}
			mean /= float64(len(xs))
			sd := balance.SD(xs)
			fmt.Printf("%-8s %-12s %9.2f %9.2f %9.2f %10g %8d\n", pol, w.name, mean, sd, 1.96*sd/math.Sqrt(float64(len(xs))), w.want, balance.SeedsFor(sd, w.want))
		}
	}
	fmt.Println("\nA paired comparison on the same seeds needs fewer: see compare's intervals.")
	return nil
}

// ------------------------------------------------------------ ledger

// An Entry is one decided evaluation.
type Entry struct {
	ID       int                `json:"id"`
	Time     string             `json:"time"`
	Name     string             `json:"name"`
	Run      string             `json:"run"` // the result file
	Rev      string             `json:"rev"`
	Dirty    bool               `json:"dirty"`
	Code     string             `json:"code"` // content hashes
	Bot      string             `json:"bot"`
	First    int                `json:"first"`
	Seeds    int                `json:"seeds"`
	Turns    int                `json:"turns"`
	Changed  []string           `json:"changed"` // knobs off the defaults
	Rules    map[string]float64 `json:"rules"`
	Score    *float64           `json:"score"`
	ScoreCI  [2]float64         `json:"scoreCI"`
	Fail     string             `json:"fail,omitempty"`
	Key      map[string]float64 `json:"key"` // the metrics worth a column
	Change   string             `json:"change"`
	Why      string             `json:"why"`
	Decision string             `json:"decision"` // keep, revert, baseline, note
	Base     string             `json:"base,omitempty"`
	VsBase   string             `json:"vsBase,omitempty"` // score change and its interval
	Best     string             `json:"best"`             // best-known score after this entry
	BestRun  string             `json:"bestRun"`
}

var keyMetrics = []string{"fighter.king", "caster.king", "fighter.oracle", "caster.oracle", "abyssDeath", "gap",
	"fighter.deepest", "caster.deepest", "fighter.clvl", "caster.clvl", "fighter.gold", "caster.gold", "stuck"}

func logEntry(args []string) error {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	change := fs.String("change", "", "what was changed (knobs, code, bot)")
	why := fs.String("why", "", "the reasoning: what the previous result showed, what this should do")
	keep := fs.Bool("keep", false, "the change stays")
	revert := fs.Bool("revert", false, "the change is undone: a regression or noise")
	baseline := fs.Bool("baseline", false, "the starting point of a pass")
	base := fs.String("base", "", "the result this one is compared with (default: the best known)")
	if len(args) < 1 {
		usage()
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *change == "" || *why == "" {
		return fmt.Errorf("log: -change and -why are required")
	}
	decision := ""
	switch {
	case *keep:
		decision = "keep"
	case *revert:
		decision = "revert"
	case *baseline:
		decision = "baseline"
	default:
		return fmt.Errorf("log: one of -keep, -revert, -baseline")
	}
	res, err := load(args[0])
	if err != nil {
		return err
	}
	entries, err := readLedger()
	if err != nil {
		return err
	}
	e := Entry{ID: len(entries) + 1, Time: time.Now().Format(time.RFC3339), Name: res.Name, Run: args[0], Rev: res.Rev, Dirty: res.Dirty,
		Code: res.Hashes["code"], Bot: res.Hashes["bot"], First: res.First, Seeds: res.Seeds, Turns: res.Turns, Changed: res.Changed, Rules: res.Rules,
		Score: res.Score, ScoreCI: res.ScoreCI, Fail: res.Fail, Key: map[string]float64{}, Change: *change, Why: *why, Decision: decision}
	for _, k := range keyMetrics {
		if v, ok := res.Metrics[k]; ok && !math.IsNaN(v) {
			e.Key[k] = v
		}
	}
	// compare with the base
	basePath := *base
	if basePath == "" && len(entries) > 0 {
		basePath = entries[len(entries)-1].BestRun
	}
	if basePath != "" && basePath != args[0] {
		if b, err := load(basePath); err == nil {
			d, lo, hi := balance.ScoreDiff(b, res, 400, 1)
			e.Base = basePath
			e.VsBase = fmt.Sprintf("%+.2f (%+.2f…%+.2f)", d, lo, hi)
		}
	}
	// the best-known result
	bestScore, bestRunPath := math.Inf(1), ""
	if len(entries) > 0 {
		last := entries[len(entries)-1]
		bestRunPath = last.BestRun
		if b, err := load(bestRunPath); err == nil && b.Score != nil {
			bestScore = *b.Score
		}
	}
	if decision != "revert" && res.Score != nil && *res.Score < bestScore {
		bestScore, bestRunPath = *res.Score, args[0]
		if err := writeJSON(bestPath, res.Rules); err != nil {
			return err
		}
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		if err := os.WriteFile(bestRun, data, 0o644); err != nil {
			return err
		}
	}
	e.BestRun = bestRunPath
	e.Best = "none"
	if !math.IsInf(bestScore, 1) {
		e.Best = fmt.Sprintf("%.2f", bestScore)
	}
	entries = append(entries, e)
	if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(ledgerPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	line, _ := json.Marshal(e)
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := writeMarkdown(entries); err != nil {
		return err
	}
	fmt.Printf("#%d %s · score %s · best %s (%s)\n", e.ID, decision, scoreStr(res), e.Best, filepath.Base(e.BestRun))
	if e.VsBase != "" {
		fmt.Printf("vs %s: %s\n", e.Base, e.VsBase)
	}
	return nil
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func readLedger() ([]Entry, error) {
	f, err := os.Open(ledgerPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		if len(strings.TrimSpace(sc.Text())) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("%s: %w", ledgerPath, err)
		}
		out = append(out, e)
	}
	return out, sc.Err()
}

func ls() error {
	entries, err := readLedger()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("no entries; make eval, then balance log")
		return nil
	}
	fmt.Printf("%3s %-8s %-9s %-11s %5s %6s %-16s %6s %6s %6s %6s %-6s  %s\n", "#", "decision", "rev", "bot", "seeds", "score", "vs base", "F.king", "C.king", "F.orc", "C.orc", "best", "change")
	for _, e := range entries {
		s := "FAIL"
		if e.Score != nil {
			s = fmt.Sprintf("%.2f", *e.Score)
		}
		rev := e.Rev
		if e.Dirty {
			rev += "+"
		}
		fmt.Printf("%3d %-8s %-9s %-11s %5d %6s %-16s %6s %6s %6s %6s %-6s  %s\n", e.ID, e.Decision, rev, e.Bot, e.Seeds, s, e.VsBase,
			pctOf(e.Key["fighter.king"]), pctOf(e.Key["caster.king"]), pctOf(e.Key["fighter.oracle"]), pctOf(e.Key["caster.oracle"]), e.Best, e.Change)
	}
	return nil
}

func pctOf(v float64) string { return fmt.Sprintf("%.0f%%", v*100) }

// writeMarkdown renders the ledger as a table for people.
func writeMarkdown(entries []Entry) error {
	var b strings.Builder
	b.WriteString("# Balance ledger\n\nOne row per decided evaluation. `make eval` writes the run, `balance log` records the decision. Score is the weighted distance outside the goals (internal/balance), lower is better; best is the best-known score after the row.\n\n")
	b.WriteString("| # | time | decision | rev | bot | seeds | score | vs base | F/C king | F/C oracle | abyss | gap | best | change | why |\n|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, e := range entries {
		s := "FAIL"
		if e.Score != nil {
			s = fmt.Sprintf("%.2f (%.1f–%.1f)", *e.Score, e.ScoreCI[0], e.ScoreCI[1])
		}
		rev := e.Rev
		if e.Dirty {
			rev += "+"
		}
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %s | %d–%d | %s | %s | %s/%s | %s/%s | %s | %s | %s | %s | %s |\n", e.ID, e.Time[:16], e.Decision, rev, e.Bot,
			e.First, e.First+e.Seeds-1, s, e.VsBase, pctOf(e.Key["fighter.king"]), pctOf(e.Key["caster.king"]), pctOf(e.Key["fighter.oracle"]), pctOf(e.Key["caster.oracle"]),
			num(e.Key["abyssDeath"]), num(e.Key["gap"]), e.Best, md(e.Change), md(e.Why))
	}
	b.WriteString("\n## Rules per entry\n\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "- #%d `%s`: %s\n", e.ID, filepath.Base(e.Run), orDefaults(e.Changed))
	}
	return os.WriteFile(ledgerMD, []byte(b.String()), 0o644)
}

func md(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "|", "\\|"), "\n", " ") }

func orDefaults(changed []string) string {
	if len(changed) == 0 {
		return "defaults"
	}
	return strings.Join(changed, ", ")
}
