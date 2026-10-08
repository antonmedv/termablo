package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
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
	for _, k := range villagerLines {
		want("rumor."+k, "")
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
				g.talkName, g.talkText = "Voss", g.talkLines("talk.voss.intro")
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
