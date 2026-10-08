// Command i18n maintains Termablo's translations in locales/: it checks
// them, writes the prompts an LLM translates and reviews from, and
// merges the replies back. The runbook is docs/translating.md.
//
//	go run ./cmd/i18n status                       coverage per language
//	go run ./cmd/i18n check [-lang de]             errors and warnings; exit 1 on errors
//	go run ./cmd/i18n prompt -lang de [-only msg.] [-n 150] [-all]
//	                                               the translation prompt for what is missing or stale
//	go run ./cmd/i18n merge -lang de reply.maml    merge an LLM's reply (or - for stdin)
//	go run ./cmd/i18n review -lang de [-only msg.] the prompt for a second LLM to review a translation
//	go run ./cmd/i18n fmt                          rewrite every translation in source order
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/antonmedv/termablo/internal/i18n"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd, args := os.Args[1], os.Args[2:]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	dir := fs.String("dir", "locales", "the locales directory")
	lang := fs.String("lang", "", "language code")
	only := fs.String("only", "", "only keys starting with this prefix")
	n := fs.Int("n", 0, "at most this many entries (0 = all)")
	all := fs.Bool("all", false, "prompt: every key, not only missing and stale ones")
	_ = fs.Parse(args)
	var err error
	switch cmd {
	case "status":
		err = status(*dir)
	case "check":
		err = check(*dir, *lang)
	case "prompt", "review":
		err = prompt(*dir, cmd, need(*lang), *only, *n, *all)
	case "merge":
		if fs.NArg() != 1 {
			usage()
		}
		err = merge(*dir, need(*lang), fs.Arg(0))
	case "fmt":
		err = format(*dir)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "i18n:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: i18n status | check [-lang xx] | prompt -lang xx [-only prefix] [-n N] [-all] | merge -lang xx file | review -lang xx | fmt")
	os.Exit(2)
}

func need(lang string) string {
	if _, ok := i18n.LangOf(lang); !ok || lang == i18n.Source {
		fmt.Fprintf(os.Stderr, "i18n: -lang: want one of the translations, got %q\n", lang)
		os.Exit(2)
	}
	return lang
}

func status(dir string) error {
	r, err := i18n.CheckDir(dir)
	if err != nil {
		return err
	}
	printCoverage(r)
	return nil
}

func printCoverage(r *i18n.Report) {
	fmt.Printf("%-4s %-22s %6s %8s %6s %6s %6s\n", "lang", "", "done", "missing", "stale", "errors", "warns")
	for _, c := range r.Coverage {
		l, _ := i18n.LangOf(c.Lang)
		e, w := 0, 0
		for _, p := range r.Problems {
			if p.Lang == c.Lang {
				if p.Error {
					e++
				} else {
					w++
				}
			}
		}
		fmt.Printf("%-4s %-22s %5.0f%% %8d %6d %6d %6d\n", c.Lang, l.Name, 100*float64(c.Done)/float64(c.Total), c.Missing, c.Stale, e, w)
	}
}

func check(dir, lang string) error {
	r, err := i18n.CheckDir(dir)
	if err != nil {
		return err
	}
	errs := 0
	for _, p := range r.Problems {
		if lang != "" && p.Lang != lang {
			continue
		}
		fmt.Println(p)
		if p.Error {
			errs++
		}
	}
	fmt.Println()
	printCoverage(r)
	if errs > 0 {
		return fmt.Errorf("%d errors", errs)
	}
	return nil
}

// entry is one key as a prompt shows it.
type entry struct {
	Key, Status string
	Notes       []string // the comments not already shown above the entry before
	Max         int
	Source      string // the English, as MAML
	Current     string // the translation now, as MAML; "" when missing
}

// promptData fills locales/prompts/*.md.
type promptData struct {
	Lang     i18n.Lang
	Style    string
	Terms    []termLine
	Entries  []entry
	Examples []entry
}

type termLine struct{ En, Note, Forms string }

func prompt(dir, kind, lang, only string, n int, all bool) error {
	p, err := i18n.Open(dir)
	if err != nil {
		return err
	}
	l, _ := i18n.LangOf(lang)
	d := promptData{Lang: l}
	if g := p.Glossary; g != nil {
		d.Style = strings.TrimSpace(g.Style["all"] + "\n\n" + g.Style[lang])
		for _, t := range g.Terms {
			d.Terms = append(d.Terms, termLine{t.En, t.Note, strings.Join(t.Forms[lang], " / ")})
		}
	}
	cat := p.Cats[lang]
	for _, key := range p.Source.Keys() {
		if !strings.HasPrefix(key, only) {
			continue
		}
		st := p.Status(lang, key)
		e := entry{Key: key, Status: st, Notes: p.Notes[key].Comments, Max: p.Notes[key].Max, Source: leaf(p.Source, key)}
		// a namespace's comments are shown once, above its first entry
		if len(d.Entries) > 0 {
			prev := p.Notes[d.Entries[len(d.Entries)-1].Key].Comments
			k := 0
			for k < len(prev) && k < len(e.Notes) && prev[k] == e.Notes[k] {
				k++
			}
			e.Notes = e.Notes[k:]
		}
		if _, ok := cat.Msg(key); ok {
			e.Current = leaf(cat, key)
		}
		pending := st != "" || all
		if kind == "review" {
			pending = st == "" && e.Current != ""
		}
		switch {
		case pending && (n == 0 || len(d.Entries) < n):
			d.Entries = append(d.Entries, e)
		case kind == "prompt" && st == "" && e.Current != "" && len(d.Examples) < 60:
			d.Examples = append(d.Examples, e)
		}
	}
	if len(d.Entries) == 0 {
		fmt.Fprintf(os.Stderr, "i18n: nothing to %s for %s\n", kind, lang)
		return nil
	}
	src, err := os.ReadFile(filepath.Join(dir, "prompts", kind+".md"))
	if err != nil {
		return err
	}
	t, err := template.New(kind).Funcs(template.FuncMap{"join": strings.Join}).Parse(string(src))
	if err != nil {
		return err
	}
	return t.Execute(os.Stdout, d)
}

// leaf renders one catalog entry as MAML, the way it sits in a file.
func leaf(c *i18n.Catalog, key string) string {
	m, _ := c.Msg(key)
	out := string(i18n.Marshal([]string{"k"}, map[string]*i18n.Msg{"k": m}))
	out = strings.TrimPrefix(strings.TrimSuffix(out, "}\n"), "{\n  k: ")
	return strings.TrimRight(out, "\n")
}

// merge takes an LLM's reply, a MAML object of translated entries, and
// writes it into the language's catalog in source order, recording the
// English each entry was made from in the lock. Entries with errors are
// left out and reported.
func merge(dir, lang, file string) error {
	var src []byte
	var err error
	if file == "-" {
		src, err = io.ReadAll(os.Stdin)
	} else {
		src, err = os.ReadFile(file)
	}
	if err != nil {
		return err
	}
	src = unfence(src)
	reply, err := i18n.Parse(lang, src)
	if err != nil {
		return fmt.Errorf("the reply is not a MAML catalog: %w", err)
	}
	p, err := i18n.Open(dir)
	if err != nil {
		return err
	}
	// check the reply as if it were the whole translation
	trial := &i18n.Project{Dir: dir, Source: p.Source, Notes: p.Notes, Glossary: p.Glossary,
		Cats: map[string]*i18n.Catalog{lang: reply}, Locks: map[string]i18n.Lock{lang: {}}}
	bad := map[string]bool{}
	for _, pr := range trial.Check().Problems {
		if pr.Lang != lang {
			continue
		}
		fmt.Println(pr)
		if pr.Error {
			bad[pr.Key] = true
		}
	}
	msgs := map[string]*i18n.Msg{}
	cur := p.Cats[lang]
	for _, k := range cur.Keys() {
		msgs[k], _ = cur.Msg(k)
	}
	lock := p.Locks[lang]
	merged := 0
	for _, k := range reply.Keys() {
		if bad[k] {
			continue
		}
		srcMsg, ok := p.Source.Msg(k)
		if !ok {
			continue
		}
		msgs[k], _ = reply.Msg(k)
		lock[k] = i18n.Hash(srcMsg)
		merged++
	}
	if err := write(p, lang, msgs, lock); err != nil {
		return err
	}
	fmt.Printf("merged %d of %d entries into %s.maml; %d rejected\n", merged, len(reply.Keys()), lang, len(bad))
	return nil
}

// unfence strips a Markdown code fence an LLM may wrap its reply in.
func unfence(b []byte) []byte {
	s := strings.TrimSpace(string(b))
	if strings.HasPrefix(s, "```") {
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[i+1:]
		}
		s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	}
	return []byte(s)
}

func write(p *i18n.Project, lang string, msgs map[string]*i18n.Msg, lock i18n.Lock) error {
	var order []string
	for _, k := range p.Source.Keys() {
		if _, ok := msgs[k]; ok {
			order = append(order, k)
		}
	}
	l, _ := i18n.LangOf(lang)
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Termablo in %s. Translated from en.maml; see docs/translating.md.\n", l.Name)
	fmt.Fprintf(&b, "# Plural forms: %s. ", strings.Join(l.Plurals, ", "))
	if len(l.Genders) > 0 {
		fmt.Fprintf(&b, "Genders: %s.", strings.Join(l.Genders, ", "))
	}
	b.WriteString("\n")
	b.Write(i18n.Marshal(order, msgs))
	if err := os.WriteFile(filepath.Join(p.Dir, lang+".maml"), b.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(p.Dir, "lock"), 0o755); err != nil {
		return err
	}
	return lock.Write(p.LockPath(lang), order)
}

// format rewrites every translation in source order, dropping keys the
// source no longer has.
func format(dir string) error {
	p, err := i18n.Open(dir)
	if err != nil {
		return err
	}
	for _, l := range i18n.Langs {
		c, ok := p.Cats[l.Code]
		if !ok || len(c.Keys()) == 0 {
			continue
		}
		msgs := map[string]*i18n.Msg{}
		for _, k := range c.Keys() {
			if _, ok := p.Source.Msg(k); ok {
				msgs[k], _ = c.Msg(k)
			}
		}
		lock := i18n.Lock{}
		for k, h := range p.Locks[l.Code] {
			if _, ok := msgs[k]; ok {
				lock[k] = h
			}
		}
		if err := write(p, l.Code, msgs, lock); err != nil {
			return err
		}
	}
	return nil
}
