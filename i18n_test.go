package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/antonmedv/termablo/internal/i18n"
)

// TestCatalogCoversData holds en.maml to the game's tables: every row an
// English line, the line the row's own English.
func TestCatalogCoversData(t *testing.T) {
	en := locales.Get(i18n.Source)
	want := func(key, text string) {
		t.Helper()
		m, ok := en.Msg(key)
		switch {
		case !ok:
			t.Errorf("en.maml has no %s (%q)", key, text)
		case text != "" && m.Text != text:
			t.Errorf("en.maml %s is %q, the table says %q", key, m.Text, text)
		}
	}
	for _, b := range bases {
		want("base."+slug(b.Name), b.Name)
	}
	for _, b := range gambleBases {
		want("gamble."+slug(b.Name), b.Name)
	}
	for _, d := range affixDefs {
		ns := "suffix."
		if d.Prefix {
			ns = "prefix."
		}
		for _, n := range d.Names {
			want(ns+slug(n), n)
		}
	}
	for _, n := range rareFirst {
		want("rare.first."+slug(n), n)
	}
	for _, ns := range rareSecond {
		for _, n := range ns {
			want("rare.second."+slug(n), n)
		}
	}
	for _, u := range append(uniques, lastShroud) {
		want("unique."+slug(u.Name)+".name", u.Name)
		want("unique."+slug(u.Name)+".flavor", u.Flavor)
	}
	for _, mt := range mtList {
		want("monster."+mt.ID, mt.Name)
		if mt.Verb != "" {
			want("hit."+slug(mt.Verb), "")
		}
	}
	want("hit.hits", "")
	want("hit.burn", "")
	for _, n := range uniqueFirst {
		want("unique_monster.first."+slug(n), n)
	}
	for _, n := range uniqueLast {
		want("unique_monster.last."+slug(n), n)
	}
	for _, n := range modNames {
		want("mod."+slug(n), n)
	}
	for _, td := range tdefs {
		if td.Name != "" {
			want("tile."+slug(td.Name), td.Name)
		}
	}
	for tl := range tileNotes {
		want("tile_note."+slug(tdefs[tl].Name), "")
	}
	for s := range StCount {
		want("stat."+statKeys[s], "")
	}
	for _, n := range eqNames {
		want("slot."+slug(n), n)
	}
	for _, k := range rumors {
		want("rumor."+k, "")
	}
	for _, k := range topics {
		want("topic."+k, "")
	}
	for _, k := range helpKeys {
		want("help.key."+k, "")
		want("help.does."+k, "")
	}
	for _, k := range helpTips {
		want("help.tip."+k, "")
	}
}

// TestCodeKeysExist finds every whole catalog key written in the Go
// source and looks it up in en.maml.
func TestCodeKeysExist(t *testing.T) {
	en := locales.Get(i18n.Source)
	files, _ := filepath.Glob("*.go")
	re := regexp.MustCompile(`"((?:msg|ui|talk|item|ref|shop|area|region|lore|rarity|monster|unique_monster)\.[a-z0-9_.]*[a-z0-9_])"`)
	n := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			n++
			if !en.Has(m[1]) {
				t.Errorf("%s: %s is not in en.maml", f, m[1])
			}
		}
	}
	if n < 100 {
		t.Errorf("found only %d keys in the source; is the pattern stale?", n)
	}
}

// TestEnglishNames: built from parts in English, every item is named as
// the tables named it.
func TestEnglishNames(t *testing.T) {
	en := locales.Get(i18n.Source)
	r := DefaultRules()
	for seed := range int64(300) {
		rng := rand.New(rand.NewSource(seed))
		for _, rar := range []Rarity{RNormal, RMagic, RRare, RUnique} {
			it := GenItem(rng, 1+int(seed%16), rar, SlotNone, r)
			if got := iname(en, it); got != it.DisplayName() {
				t.Fatalf("seed %d: %q named %q in English", seed, it.DisplayName(), got)
			}
		}
		m := NewMonster(rng, mtemps["fallen"], 3, RankUnique, r)
		if got := monsterNoun(en, m).Text; got != m.Name {
			t.Fatalf("unique monster %q named %q", m.Name, got)
		}
	}
	for _, k := range []ItemKind{IKHealth, IKMana, IKScroll} {
		it := NewPotion(k)
		if got := iname(en, it); got != it.DisplayName() {
			t.Errorf("%q named %q", it.DisplayName(), got)
		}
	}
}

// TestLocalesValid runs the translation checks: no errors in any
// shipped catalog.
func TestLocalesValid(t *testing.T) {
	rep, err := i18n.CheckDir("locales")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range rep.Problems {
		if p.Error {
			t.Errorf("%s", p)
		}
	}
}

// TestDrawEveryLanguage draws every screen in every language: nothing
// panics and no key shows through where a line is missing.
func TestDrawEveryLanguage(t *testing.T) {
	key := regexp.MustCompile(`\b(msg|ui|help|talk|item|ref|stat|tile|area|lore)\.[a-z_]+`)
	for _, l := range i18n.Langs {
		g := newTestGame(t)
		g.SetLang(l.Code)
		rng := rand.New(rand.NewSource(1))
		for _, rar := range []Rarity{RMagic, RRare, RUnique} {
			g.P.Inv = append(g.P.Inv, GenItem(rng, 9, rar, SlotNone, g.Rules))
		}
		g.restock()
		for _, sz := range [][2]int{{80, 24}, {140, 44}} {
			s := NewScreen(sz[0], sz[1])
			for _, mode := range []Mode{ModeTitle, ModePlay, ModeInv, ModeChar, ModeShop, ModeHelp, ModeMap, ModeTalk, ModeDead} {
				g.Mode = mode
				g.shop = g.shops[0]
				g.talk = &Talk{Who: "voss", Name: "Voss", Barter: true}
				g.hear("", "talk.voss.greet")
				g.ask("ember")
				g.Draw(s)
				if m := key.FindString(screenText(s)); m != "" {
					t.Errorf("%s %v %dx%d: key %s on screen", l.Code, mode, sz[0], sz[1], m)
				}
			}
		}
	}
}

func screenText(s *Screen) string {
	var b strings.Builder
	for i, c := range s.C {
		if c.Ch != wideTail {
			b.WriteRune(c.Ch)
		}
		if (i+1)%s.W == 0 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// TestBeltLabelsFit: every language's belt labels fit the cells the
// panel leaves them (beltRow), a two-digit scroll count included.
func TestBeltLabelsFit(t *testing.T) {
	bw := panelW - 4
	rows := []struct {
		key      string
		w, slots int
	}{{"ui.panel.belt_portal", bw - 14, 1}, {"ui.panel.belt_heal", bw, beltMax}, {"ui.panel.belt_mana", bw, beltMax}}
	for _, l := range i18n.Langs {
		loc := locales.Get(l.Code)
		for _, r := range rows {
			if s := loc.T(r.key, "n", 10); i18n.Width(s) > r.w-1-r.slots*4 {
				t.Errorf("%s %s: %q is %d cells, %d fit", l.Code, r.key, s, i18n.Width(s), r.w-1-r.slots*4)
			}
		}
	}
}

// TestAreaNames: every level, named from its catalog keys in English,
// is called what the generator called it.
func TestAreaNames(t *testing.T) {
	g := newTestGame(t)
	en := locales.Get(i18n.Source)
	for _, id := range []string{"town", "fields", "marsh", "crypt1", "crypt3", "crypt4", "grotto1", "grotto3", "abyss1", "abyss3", "abyss4"} {
		l := g.getLevel(id)
		if got := areaName(en, l); got != l.Name {
			t.Errorf("%s is %q, named %q", id, l.Name, got)
		}
	}
}

// TestSetLangKeepsLog: switching language writes every line of the log
// again, none lost.
func TestSetLangKeepsLog(t *testing.T) {
	g := NewGame(1)
	g.changeLevel("grotto1", "", nil) // the too-deep warning, before any turn
	n := len(g.Log)
	g.SetLang("de")
	if len(g.Log) != n {
		t.Fatalf("%d lines became %d", n, len(g.Log))
	}
	if want := g.L.T("msg.far_beyond"); g.Log[n-1].Text != want {
		t.Errorf("last line %q, want %q", g.Log[n-1].Text, want)
	}
	if want := capFirst(g.L.T("lore.grotto", "area", areaName(g.L, g.Lv))); g.Log[n-2].Text != want {
		t.Errorf("lore line %q, want %q", g.Log[n-2].Text, want)
	}
}

// TestTitleLanguages: on the title screen the arrows walk the row of
// languages and wrap, enter begins, and a click picks a language and a
// second click on it begins.
func TestTitleLanguages(t *testing.T) {
	m := newModel(1, "")
	g := m.g
	m.key("right")
	if g.L.Lang.Code != i18n.Langs[1].Code || g.Mode != ModeTitle {
		t.Fatalf("right: %s, mode %v", g.L.Lang.Code, g.Mode)
	}
	m.key("left")
	m.key("left")
	if g.L.Lang.Code != i18n.Langs[len(i18n.Langs)-1].Code {
		t.Fatalf("left from the first should wrap to the last, got %s", g.L.Lang.Code)
	}
	s := NewScreen(80, 24)
	g.Draw(s)
	ru := slices.IndexFunc(i18n.Langs, func(l i18n.Lang) bool { return l.Code == "ru" })
	b := g.langHit[ru]
	g.Click(b.X0, b.Y, s.W, s.H)
	if g.L.Lang.Code != "ru" || g.Mode != ModeTitle {
		t.Fatalf("click on Русский: %s, mode %v", g.L.Lang.Code, g.Mode)
	}
	g.Click(b.X1, b.Y, s.W, s.H)
	if g.Mode != ModePlay {
		t.Fatalf("a second click should begin, mode %v", g.Mode)
	}
	m2 := newModel(1, "")
	for _, k := range []string{"x", "q", "up", "esc"} {
		m2.key(k)
	}
	if m2.g.Mode != ModeTitle {
		t.Fatalf("only enter and space should begin, mode %v", m2.g.Mode)
	}
	m2.key("enter")
	if m2.g.Mode != ModePlay {
		t.Errorf("enter should begin")
	}
}
