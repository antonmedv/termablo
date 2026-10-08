package i18n

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/maml-dev/go-maml"
)

// Msg is one catalog entry: a text, a noun (a text with a gender), or a
// set of variants chosen at lookup by the plural category of the count
// {n} or by the gender of a noun argument. Variants nest.
type Msg struct {
	Text   string
	Gender string
	Vars   map[string]*Msg
	Order  []string // variant keys as written
}

// IsVariants reports whether the entry chooses among variants.
func (m *Msg) IsVariants() bool { return m.Vars != nil }

// Texts is every text the entry can produce, variants flattened.
func (m *Msg) Texts() []string {
	if m.Vars == nil {
		return []string{m.Text}
	}
	var out []string
	for _, k := range m.Order {
		out = append(out, m.Vars[k].Texts()...)
	}
	return out
}

// Noun is a word that other words agree with: an item base, a monster.
// It prints as its text.
type Noun struct {
	Text   string
	Gender string
}

func (n Noun) String() string { return n.Text }

// Catalog is one language's messages, falling back to the source.
type Catalog struct {
	Lang     Lang
	msgs     map[string]*Msg
	keys     []string
	fallback *Catalog
}

// leafKeys are the object keys that make an object an entry rather than
// a namespace: noun fields, plural categories and gender classes.
var leafKeys = map[string]bool{
	"text": true, "gender": true,
	"zero": true, "one": true, "two": true, "few": true, "many": true, "other": true,
	"m": true, "f": true, "n": true, "pl": true, "mp": true, "fp": true, "mv": true, "fv": true, "an": true,
}

// Parse reads a catalog from MAML source. Nested objects are
// namespaces, joined into dotted keys: {msg: {died: "..."}} is
// "msg.died".
func Parse(lang string, src []byte) (*Catalog, error) {
	l, ok := LangOf(lang)
	if !ok {
		return nil, fmt.Errorf("unknown language %q", lang)
	}
	v, err := maml.Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", lang, err)
	}
	if !v.IsObject() {
		return nil, fmt.Errorf("%s: top level must be an object", lang)
	}
	c := &Catalog{Lang: l, msgs: map[string]*Msg{}}
	if err := c.walk("", v.AsObject()); err != nil {
		return nil, fmt.Errorf("%s: %w", lang, err)
	}
	return c, nil
}

func isLeaf(o *maml.OrderedMap) bool {
	if o.Len() == 0 {
		return false
	}
	for _, k := range o.Keys() {
		if !leafKeys[k] {
			return false
		}
	}
	return true
}

func (c *Catalog) walk(prefix string, o *maml.OrderedMap) error {
	for _, e := range o.Entries() {
		key := e.Key
		if prefix != "" {
			key = prefix + "." + e.Key
		}
		if e.Value.IsObject() && !isLeaf(e.Value.AsObject()) {
			if err := c.walk(key, e.Value.AsObject()); err != nil {
				return err
			}
			continue
		}
		m, err := toMsg(key, e.Value)
		if err != nil {
			return err
		}
		if _, dup := c.msgs[key]; dup {
			return fmt.Errorf("%s: duplicate key", key)
		}
		c.msgs[key] = m
		c.keys = append(c.keys, key)
	}
	return nil
}

func toMsg(key string, v maml.Value) (*Msg, error) {
	if v.IsString() {
		return &Msg{Text: v.AsString()}, nil
	}
	if !v.IsObject() {
		return nil, fmt.Errorf("%s: want a string or an object, got %s", key, v.String())
	}
	o := v.AsObject()
	if t, ok := o.Get("text"); ok {
		g, hasG := o.Get("gender")
		if !t.IsString() || (hasG && !g.IsString()) || o.Len() != map[bool]int{false: 1, true: 2}[hasG] {
			return nil, fmt.Errorf("%s: a noun is {text, gender}", key)
		}
		m := &Msg{Text: t.AsString()}
		if hasG {
			m.Gender = g.AsString()
		}
		return m, nil
	}
	m := &Msg{Vars: map[string]*Msg{}}
	for _, e := range o.Entries() {
		if e.Key == "gender" {
			return nil, fmt.Errorf("%s: gender without text", key)
		}
		sub, err := toMsg(key+"."+e.Key, e.Value)
		if err != nil {
			return nil, err
		}
		m.Vars[e.Key] = sub
		m.Order = append(m.Order, e.Key)
	}
	return m, nil
}

// Keys is every key the catalog defines, in file order.
func (c *Catalog) Keys() []string { return c.keys }

// Msg is the catalog's own entry for a key, without fallback.
func (c *Catalog) Msg(key string) (*Msg, bool) {
	m, ok := c.msgs[key]
	return m, ok
}

func (c *Catalog) lookup(key string) (*Msg, *Catalog) {
	for cc := c; cc != nil; cc = cc.fallback {
		if m, ok := cc.msgs[key]; ok {
			return m, cc
		}
	}
	return nil, nil
}

// Has reports whether the key resolves, in this catalog or the source.
func (c *Catalog) Has(key string) bool {
	m, _ := c.lookup(key)
	return m != nil
}

// T is the message for key with its placeholders filled. Arguments come
// in name, value pairs: T("msg.gold", "n", 12) fills {n}. A Noun value
// also chooses gender variants; the int named n chooses plural ones. A
// key missing everywhere comes back as itself.
func (c *Catalog) T(key string, args ...any) string {
	m, owner := c.lookup(key)
	if m == nil {
		return key
	}
	return Format(owner.Lang.Code, owner.pick(m, args), args...)
}

// Noun is the entry for key as a noun to agree with.
func (c *Catalog) Noun(key string, args ...any) Noun {
	m, owner := c.lookup(key)
	if m == nil {
		return Noun{Text: key}
	}
	if m.Vars != nil {
		return Noun{Text: Format(owner.Lang.Code, owner.pick(m, args), args...)}
	}
	return Noun{Text: Format(owner.Lang.Code, m.Text, args...), Gender: m.Gender}
}

// pick walks variants down to a text.
func (c *Catalog) pick(m *Msg, args []any) string {
	for m.Vars != nil {
		m = m.Vars[c.choose(m, args)]
	}
	return m.Text
}

// choose is the variant key for the arguments: the gender of the first
// noun that has a variant, else the plural category of n, else "other",
// else the first variant.
func (c *Catalog) choose(m *Msg, args []any) string {
	for i := 1; i < len(args); i += 2 {
		if n, ok := args[i].(Noun); ok && n.Gender != "" {
			for _, g := range []string{n.Gender, n.Gender[:1]} {
				if _, ok := m.Vars[g]; ok {
					return g
				}
			}
		}
	}
	if n, ok := intArg(args, "n"); ok {
		if k := PluralCategory(c.Lang.Code, n); m.Vars[k] != nil {
			return k
		}
	}
	if _, ok := m.Vars["other"]; ok {
		return "other"
	}
	return m.Order[0]
}

func intArg(args []any, name string) (int, bool) {
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == name {
			switch v := args[i+1].(type) {
			case int:
				return v, true
			case int64:
				return int(v), true
			}
		}
	}
	return 0, false
}

// Format fills {name} placeholders from name, value pairs. {{ is a
// literal brace. Unknown placeholders stay as written.
func Format(lang, s string, args ...any) string {
	if !strings.ContainsRune(s, '{') {
		return s
	}
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '{')
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		s = s[i:]
		if strings.HasPrefix(s, "{{") {
			b.WriteByte('{')
			s = s[2:]
			continue
		}
		j := strings.IndexByte(s, '}')
		if j < 0 {
			b.WriteString(s)
			return b.String()
		}
		name := s[1:j]
		if v, ok := arg(args, name); ok {
			b.WriteString(str(v))
		} else {
			b.WriteString(s[:j+1])
		}
		s = s[j+1:]
	}
}

func arg(args []any, name string) (any, bool) {
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == name {
			return args[i+1], true
		}
	}
	return nil, false
}

func str(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case fmt.Stringer:
		return v.String()
	}
	return fmt.Sprint(v)
}

// Placeholders is the set of {names} in a text, sorted.
func Placeholders(s string) []string {
	var out []string
	for {
		i := strings.IndexByte(s, '{')
		if i < 0 {
			break
		}
		if strings.HasPrefix(s[i:], "{{") {
			s = s[i+2:]
			continue
		}
		j := strings.IndexByte(s[i:], '}')
		if j < 0 {
			break
		}
		if name := s[i+1 : i+j]; !slices.Contains(out, name) {
			out = append(out, name)
		}
		s = s[i+j+1:]
	}
	slices.Sort(out)
	return out
}

// Bundle is every loaded catalog.
type Bundle struct {
	cats map[string]*Catalog
}

// Load reads dir/<code>.maml for every language in Langs. The source
// catalog must exist; a missing translation is an empty catalog that
// falls back to it.
func Load(fsys fs.FS, dir string) (*Bundle, error) {
	b := &Bundle{cats: map[string]*Catalog{}}
	for _, l := range Langs {
		src, err := fs.ReadFile(fsys, path.Join(dir, l.Code+".maml"))
		if err != nil {
			if l.Code == Source {
				return nil, err
			}
			src = []byte("{}")
		}
		c, err := Parse(l.Code, src)
		if err != nil {
			return nil, err
		}
		b.cats[l.Code] = c
	}
	for code, c := range b.cats {
		if code != Source {
			c.fallback = b.cats[Source]
		}
	}
	return b, nil
}

// Get is the catalog for a language code, or the source for an unknown one.
func (b *Bundle) Get(code string) *Catalog {
	if c, ok := b.cats[code]; ok {
		return c
	}
	return b.cats[Source]
}
