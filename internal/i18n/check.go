package i18n

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/maml-dev/go-maml"
	"github.com/maml-dev/go-maml/ast"
)

// Note is what en.maml says about a key besides its text: the comments
// above it (and above the namespaces holding it) and a width limit.
type Note struct {
	Comments []string       // outermost first
	Max      int            // terminal cells; 0 = no limit
	Sample   map[string]int // cells a placeholder is measured at, when not 2
}

// maxRe reads "# max: N", optionally with the width of placeholders
// that are shorter than the two cells assumed: "# max: 8 n=1".
var maxRe = regexp.MustCompile(`^\s*max:\s*(\d+)((?:\s+[a-z_]+=\d+)*)\s*$`)

// Notes reads the comments of a MAML source catalog, keyed like its
// entries. A "# max: N" comment limits the key it sits on and every key
// inside it.
func Notes(src []byte) (map[string]Note, error) {
	doc, err := ast.Parse(string(src))
	if err != nil {
		return nil, err
	}
	out := map[string]Note{}
	obj, ok := doc.Value.(*ast.ObjectNode)
	if !ok {
		return nil, fmt.Errorf("top level must be an object")
	}
	var walk func(prefix string, o *ast.ObjectNode, inherited Note)
	walk = func(prefix string, o *ast.ObjectNode, inherited Note) {
		for _, p := range o.Properties {
			key := p.Key.KeyValue()
			if prefix != "" {
				key = prefix + "." + key
			}
			n := Note{Comments: slices.Clone(inherited.Comments), Max: inherited.Max, Sample: inherited.Sample}
			for _, c := range p.LeadingComments {
				if m := maxRe.FindStringSubmatch(c.Value); m != nil {
					n.Max, _ = strconv.Atoi(m[1])
					n.Sample = map[string]int{}
					for _, kv := range strings.Fields(m[2]) {
						k, v, _ := strings.Cut(kv, "=")
						n.Sample[k], _ = strconv.Atoi(v)
					}
					continue
				}
				n.Comments = append(n.Comments, strings.TrimSpace(c.Value))
			}
			if p.TrailingComment != nil {
				n.Comments = append(n.Comments, strings.TrimSpace(p.TrailingComment.Value))
			}
			if sub, ok := p.Value.(*ast.ObjectNode); ok && !isLeafNode(sub) {
				walk(key, sub, n)
				continue
			}
			out[key] = n
		}
	}
	walk("", obj, Note{})
	return out, nil
}

func isLeafNode(o *ast.ObjectNode) bool {
	if len(o.Properties) == 0 {
		return false
	}
	for _, p := range o.Properties {
		if !leafKeys[p.Key.KeyValue()] {
			return false
		}
	}
	return true
}

// NounKeys match the keys that are nouns other words agree with; in a
// language with genders each should carry one.
var NounKeys = []*regexp.Regexp{
	regexp.MustCompile(`^base\.`),
	regexp.MustCompile(`^gamble\.`),
	regexp.MustCompile(`^rare\.second\.`),
	regexp.MustCompile(`^monster\.`),
	regexp.MustCompile(`^unique_monster\.first\.`),
	regexp.MustCompile(`^unique\.[a-z0-9_]+\.name$`),
	regexp.MustCompile(`^ref\.death_flames$`),
	regexp.MustCompile(`^item\.(healing_potion|mana_potion|town_portal_scroll|fresh_stock)$`),
}

// IsNounKey reports whether key names a noun.
func IsNounKey(key string) bool {
	for _, re := range NounKeys {
		if re.MatchString(key) {
			return true
		}
	}
	return false
}

// Term is a glossary entry: an English term, what it means, and its
// translation per language. A translation lists the forms that count as
// using it, the first the dictionary form; ["-"] means the term is kept
// in English.
type Term struct {
	En    string
	Note  string
	Forms map[string][]string
}

// Glossary is locales/glossary.maml: the shared terms and each
// language's style notes.
type Glossary struct {
	Style map[string]string // "all" and language codes
	Terms []Term
}

// LoadGlossary reads a glossary file.
func LoadGlossary(path string) (*Glossary, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	v, err := maml.Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	g := &Glossary{Style: map[string]string{}}
	if st, ok := v.Get("style"); ok && st.IsObject() {
		for _, e := range st.AsObject().Entries() {
			g.Style[e.Key] = e.Value.AsString()
		}
	}
	terms, _ := v.Get("terms")
	if !terms.IsArray() {
		return nil, fmt.Errorf("%s: terms must be an array", path)
	}
	for i, tv := range terms.AsArray() {
		if !tv.IsObject() {
			return nil, fmt.Errorf("%s: term %d is not an object", path, i)
		}
		t := Term{Forms: map[string][]string{}}
		for _, e := range tv.AsObject().Entries() {
			switch {
			case e.Key == "en":
				t.En = e.Value.AsString()
			case e.Key == "note":
				t.Note = e.Value.AsString()
			case e.Value.IsString():
				t.Forms[e.Key] = []string{e.Value.AsString()}
			case e.Value.IsArray():
				for _, f := range e.Value.AsArray() {
					t.Forms[e.Key] = append(t.Forms[e.Key], f.AsString())
				}
			default:
				return nil, fmt.Errorf("%s: term %q: %s must be a string or an array", path, t.En, e.Key)
			}
		}
		if t.En == "" {
			return nil, fmt.Errorf("%s: term %d has no en", path, i)
		}
		g.Terms = append(g.Terms, t)
	}
	return g, nil
}

// Uses reports whether an English text uses the term: as a whole word,
// written as the glossary writes it, so the stat "Life" is not the
// "life" of "running for their life".
func (t Term) Uses(text string) bool {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(t.En) + `\b`)
	return re.MatchString(text)
}

// Matches reports whether a translation uses one of the term's forms.
func (t Term) Matches(lang, text string) bool {
	forms := t.Forms[lang]
	if len(forms) == 0 {
		return true
	}
	low := strings.ToLower(text)
	for _, f := range forms {
		if f == "-" && strings.Contains(low, strings.ToLower(t.En)) || f != "-" && strings.Contains(low, strings.ToLower(f)) {
			return true
		}
	}
	return false
}

// Lock is a translation's record of the English each key was
// translated from: key → Hash of the source entry.
type Lock map[string]string

// ReadLock reads a lock file, "key hash" a line; a missing file is empty.
func ReadLock(path string) (Lock, error) {
	l := Lock{}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if k, h, ok := strings.Cut(sc.Text(), " "); ok {
			l[k] = h
		}
	}
	return l, sc.Err()
}

// Write saves the lock with keys in the order given.
func (l Lock) Write(path string, order []string) error {
	var b strings.Builder
	for _, k := range order {
		if h, ok := l[k]; ok {
			b.WriteString(k + " " + h + "\n")
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// Problem is one finding of a check.
type Problem struct {
	Lang, Key, Msg string
	Error          bool // the catalog is wrong; else a warning
}

func (p Problem) String() string {
	kind := "warning"
	if p.Error {
		kind = "error"
	}
	return fmt.Sprintf("%s %s %s: %s", kind, p.Lang, p.Key, p.Msg)
}

// Coverage counts a translation's keys against the source.
type Coverage struct {
	Lang                        string
	Total, Done, Missing, Stale int
}

// Report is the result of checking a locales directory.
type Report struct {
	Problems []Problem
	Coverage []Coverage
}

// Project is a locales directory loaded for the tools.
type Project struct {
	Dir      string
	Source   *Catalog
	Notes    map[string]Note
	Glossary *Glossary // nil without glossary.maml
	Cats     map[string]*Catalog
	Locks    map[string]Lock
}

// Open loads every catalog, lock and the glossary in dir.
func Open(dir string) (*Project, error) {
	p := &Project{Dir: dir, Cats: map[string]*Catalog{}, Locks: map[string]Lock{}}
	src, err := os.ReadFile(filepath.Join(dir, Source+".maml"))
	if err != nil {
		return nil, err
	}
	if p.Source, err = Parse(Source, src); err != nil {
		return nil, err
	}
	if p.Notes, err = Notes(src); err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, "glossary.maml")); err == nil {
		if p.Glossary, err = LoadGlossary(filepath.Join(dir, "glossary.maml")); err != nil {
			return nil, err
		}
	}
	for _, l := range Langs {
		if l.Code == Source {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, l.Code+".maml"))
		if os.IsNotExist(err) {
			b = []byte("{}")
		} else if err != nil {
			return nil, err
		}
		c, err := Parse(l.Code, b)
		if err != nil {
			return nil, err
		}
		p.Cats[l.Code] = c
		if p.Locks[l.Code], err = ReadLock(p.LockPath(l.Code)); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// LockPath is where a language's lock lives.
func (p *Project) LockPath(lang string) string { return filepath.Join(p.Dir, "lock", lang+".lock") }

// Status says where a key stands in a translation: "missing", "stale"
// (the English changed since), or "" when it is current.
func (p *Project) Status(lang, key string) string {
	if _, ok := p.Cats[lang].Msg(key); !ok {
		return "missing"
	}
	src, _ := p.Source.Msg(key)
	if h, ok := p.Locks[lang][key]; ok && h != Hash(src) {
		return "stale"
	}
	return ""
}

// CheckDir checks every catalog in a locales directory.
func CheckDir(dir string) (*Report, error) {
	p, err := Open(dir)
	if err != nil {
		return nil, err
	}
	return p.Check(), nil
}

// Check checks the source and every translation.
func (p *Project) Check() *Report {
	r := &Report{}
	add := func(lang, key string, isErr bool, f string, a ...any) {
		r.Problems = append(r.Problems, Problem{lang, key, fmt.Sprintf(f, a...), isErr})
	}
	for _, key := range p.Source.Keys() {
		m, _ := p.Source.Msg(key)
		p.checkWidth(add, Source, key, m)
	}
	for _, l := range Langs {
		c, ok := p.Cats[l.Code]
		if !ok {
			continue
		}
		cov := Coverage{Lang: l.Code, Total: len(p.Source.Keys())}
		for _, key := range c.Keys() {
			if _, ok := p.Source.Msg(key); !ok {
				add(l.Code, key, true, "not in %s.maml", Source)
			}
		}
		for _, key := range p.Source.Keys() {
			switch p.Status(l.Code, key) {
			case "missing":
				cov.Missing++
				continue
			case "stale":
				cov.Stale++
				add(l.Code, key, false, "stale: the English changed since it was translated")
			}
			cov.Done++
			src, _ := p.Source.Msg(key)
			m, _ := c.Msg(key)
			p.checkEntry(add, l, key, src, m)
		}
		r.Coverage = append(r.Coverage, cov)
	}
	return r
}

type addFunc func(lang, key string, isErr bool, f string, a ...any)

func (p *Project) checkEntry(add addFunc, l Lang, key string, src, m *Msg) {
	want := map[string]bool{}
	for _, t := range src.Texts() {
		for _, ph := range Placeholders(t) {
			want[ph] = true
		}
	}
	got := map[string]bool{}
	for _, t := range m.Texts() {
		for _, ph := range Placeholders(t) {
			got[ph] = true
			if !want[ph] {
				add(l.Code, key, true, "unknown placeholder {%s}", ph)
			}
		}
		if strings.TrimSpace(t) == "" && strings.TrimSpace(src.Texts()[0]) != "" {
			add(l.Code, key, true, "empty text")
		}
	}
	for ph := range want {
		if !got[ph] {
			add(l.Code, key, true, "placeholder {%s} is missing", ph)
		}
	}
	p.checkVariants(add, l, key, m)
	if m.Vars == nil && m.Gender != "" && !slices.Contains(l.Genders, m.Gender) {
		add(l.Code, key, true, "gender %q is not one of %v", m.Gender, l.Genders)
	}
	if IsNounKey(key) && len(l.Genders) > 0 && l.Code != Source && m.Vars == nil && m.Gender == "" {
		add(l.Code, key, false, "a noun without a gender; agreeing words fall back to their first form")
	}
	p.checkWidth(add, l.Code, key, m)
	if p.Glossary != nil {
		for _, t := range p.Glossary.Terms {
			if t.Uses(strings.Join(src.Texts(), "\n")) && !t.Matches(l.Code, strings.Join(m.Texts(), "\n")) {
				add(l.Code, key, false, "glossary: %q should be %q", t.En, t.Forms[l.Code][0])
			}
		}
	}
}

func (p *Project) checkVariants(add addFunc, l Lang, key string, m *Msg) {
	if m.Vars == nil {
		return
	}
	plural := false
	for _, k := range m.Order {
		isPlural := slices.Contains(l.Plurals, k) || k == "other"
		isGender := slices.Contains(l.Genders, k)
		if !isPlural && !isGender {
			add(l.Code, key, true, "variant %q is neither a plural category %v nor a gender %v of %s", k, l.Plurals, l.Genders, l.Name)
		}
		plural = plural || isPlural && !isGender && k != "other"
		p.checkVariants(add, l, key, m.Vars[k])
	}
	if plural && m.Vars["other"] == nil {
		for _, c := range l.Plurals {
			if m.Vars[c] == nil {
				add(l.Code, key, false, "plural form %q is missing", c)
			}
		}
	}
}

var placeholder = regexp.MustCompile(`\{([a-z_]+)\}`)

func (p *Project) checkWidth(add addFunc, lang, key string, m *Msg) {
	n := p.Notes[key]
	if n.Max == 0 {
		return
	}
	for _, t := range m.Texts() {
		// a placeholder counts as two cells, a two-digit number, unless
		// the max comment says otherwise
		filled := placeholder.ReplaceAllStringFunc(t, func(ph string) string {
			w, ok := n.Sample[ph[1:len(ph)-1]]
			if !ok {
				w = 2
			}
			return strings.Repeat("0", w)
		})
		if w := Width(filled); w > n.Max {
			add(lang, key, lang == Source, "%d cells, at most %d fit: %q", w, n.Max, t)
		}
	}
}
