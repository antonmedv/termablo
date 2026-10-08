package i18n

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

const enSrc = `{
  msg: {
    gold: { one: "You pick up {n} gold piece.", other: "You pick up {n} gold." }
    dies: "The {name} dies."
    spot: { other: "a {name}", an: "an {name}" }
  }
  monster: {
    imp: { text: "Fire Imp", gender: "" }
    rat: "Plague Rat"
  }
  only_en: "English only"
}`

const deSrc = `{
  msg: {
    gold: { one: "Du hebst {n} Goldstück auf.", other: "Du hebst {n} Gold auf." }
    dies: { m: "Der {name} stirbt.", f: "Die {name} stirbt.", n: "Das {name} stirbt." }
  }
  monster: {
    imp: { text: "Feuerkobold", gender: "m" }
    rat: { text: "Pestratte", gender: "f" }
  }
  flames: { m: { text: "die Flammen des {name}", gender: "pl" }, f: { text: "die Flammen der {name}", gender: "pl" } }
}`

const ruSrc = `{
  msg: {
    gold: { one: "Вы подобрали {n} золотой.", few: "Вы подобрали {n} золотых.", many: "Вы подобрали {n} золотых." }
  }
}`

func bundle(t *testing.T) *Bundle {
	t.Helper()
	b, err := Load(fstest.MapFS{
		"l/en.maml": {Data: []byte(enSrc)},
		"l/de.maml": {Data: []byte(deSrc)},
		"l/ru.maml": {Data: []byte(ruSrc)},
	}, "l")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLookup(t *testing.T) {
	b := bundle(t)
	en, de := b.Get("en"), b.Get("de")
	cases := []struct{ got, want string }{
		{en.T("msg.gold", "n", 1), "You pick up 1 gold piece."},
		{en.T("msg.gold", "n", 7), "You pick up 7 gold."},
		{de.T("msg.gold", "n", 7), "Du hebst 7 Gold auf."},
		{de.T("msg.dies", "name", de.Noun("monster.rat")), "Die Pestratte stirbt."},
		{de.T("msg.dies", "name", de.Noun("monster.imp")), "Der Feuerkobold stirbt."},
		{en.T("msg.dies", "name", en.Noun("monster.rat")), "The Plague Rat dies."},
		{de.T("only_en"), "English only"},
		{de.T("no.such.key"), "no.such.key"},
		{b.Get("xx").T("msg.dies", "name", "Bat"), "The Bat dies."},
		{b.Get("it").T("msg.gold", "n", 2), "You pick up 2 gold."},
	}
	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("%d: got %q, want %q", i, c.got, c.want)
		}
	}
}

func TestVariantNounKeepsGender(t *testing.T) {
	de := bundle(t).Get("de")
	n := de.Noun("flames", "name", de.Noun("monster.rat"))
	if n.Text != "die Flammen der Pestratte" || n.Gender != "pl" {
		t.Errorf("got %+v", n)
	}
}

func TestEnglishArticle(t *testing.T) {
	en := bundle(t).Get("en")
	if got := en.T("msg.spot", "name", Noun{Text: "Imp", Gender: "an"}); got != "an Imp" {
		t.Errorf("got %q", got)
	}
	if got := en.T("msg.spot", "name", en.Noun("monster.rat")); got != "a Plague Rat" {
		t.Errorf("got %q", got)
	}
}

func TestPlurals(t *testing.T) {
	cases := map[string]map[int]string{
		"ru": {0: "many", 1: "one", 2: "few", 4: "few", 5: "many", 11: "many", 12: "many", 21: "one", 22: "few", 111: "many"},
		"ar": {0: "zero", 1: "one", 2: "two", 3: "few", 10: "few", 11: "many", 99: "many", 100: "other", 102: "other", 103: "few"},
		"fr": {0: "one", 1: "one", 2: "other"},
		"de": {0: "other", 1: "one", 2: "other"},
		"zh": {1: "other"},
	}
	for lang, m := range cases {
		for n, want := range m {
			if got := PluralCategory(lang, n); got != want {
				t.Errorf("%s %d: got %s, want %s", lang, n, got, want)
			}
		}
	}
	ru := bundle(t).Get("ru")
	if got := ru.T("msg.gold", "n", 22); got != "Вы подобрали 22 золотых." {
		t.Errorf("got %q", got)
	}
}

func TestPlaceholders(t *testing.T) {
	if got := Placeholders("{a} and {b}, {a} {{literal}"); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("got %v", got)
	}
	if got := Format("en", "{{x}} {x}", "x", 3); got != "{x}} 3" {
		t.Errorf("got %q", got)
	}
}

func TestMatch(t *testing.T) {
	for in, want := range map[string]string{"de": "de", "ru_RU.UTF-8": "ru", "ch": "zh", "zh_CN.UTF-8": "zh", "pt_BR": "", "C": "", "FR": "fr", "ar_EG": "ar"} {
		if got := Match(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
	env := map[string]string{"LANG": "it_IT.UTF-8", "LC_ALL": "C"}
	if got := FromEnv(func(k string) string { return env[k] }); got != "it" {
		t.Errorf("got %q", got)
	}
}

func TestRoundTrip(t *testing.T) {
	c, err := Parse("de", []byte(deSrc))
	if err != nil {
		t.Fatal(err)
	}
	msgs := map[string]*Msg{}
	for _, k := range c.Keys() {
		msgs[k], _ = c.Msg(k)
	}
	c2, err := Parse("de", Marshal(c.Keys(), msgs))
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range c.Keys() {
		a, _ := c.Msg(k)
		b, ok := c2.Msg(k)
		if !ok || Hash(a) != Hash(b) {
			t.Errorf("%s did not round-trip", k)
		}
	}
}

func TestSlug(t *testing.T) {
	for in, want := range map[string]string{"Executioner's Axe": "executioners_axe", "Will-o'-Wisp": "will_o_wisp", "of the Jackal": "of_the_jackal", "Short Sword": "short_sword"} {
		if got := Slug(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestWidthAndWrap(t *testing.T) {
	if Width("你好a") != 5 || Width("·≈♣") != 3 {
		t.Errorf("widths: %d %d", Width("你好a"), Width("·≈♣"))
	}
	if got := Truncate("你好世界", 5); got != "你好" {
		t.Errorf("truncate: %q", got)
	}
	for _, l := range Wrap("你好世界，这是一个很长的句子。", 6) {
		if Width(l) > 6 {
			t.Errorf("line %q too wide", l)
		}
	}
	got := Wrap("the quick brown fox", 9)
	if strings.Join(got, "|") != "the quick|brown fox" {
		t.Errorf("wrap: %q", got)
	}
	if got := Wrap("你", 1); len(got) != 1 {
		t.Errorf("narrow wrap: %q", got)
	}
}

func TestVisual(t *testing.T) {
	// سلام: seen (initial), lam (medial), alef (final, joins only back), meem (isolated)
	if got, want := Visual("سلام", true), string([]rune{0xFEE1, 0xFEFC, 0xFEB3}); got != want {
		t.Errorf("shape: got %U, want %U", []rune(got), []rune(want))
	}
	if got := Visual("abc 12", true); got != "12 abc" {
		t.Errorf("ltr in rtl: %q", got)
	}
	if got := Visual("plain text", false); got != "plain text" {
		t.Errorf("ltr untouched: %q", got)
	}
	// a lam-alef pair is measured as the one cell it is drawn as
	if w, d := Width("السلام"), len([]rune(Visual("السلام", true))); w != d {
		t.Errorf("width %d, drawn %d cells", w, d)
	}
	// a number keeps its order and sign inside Arabic
	got := []rune(Visual("ذهب +12%", true))
	if string(got[:4]) != "+12%" {
		t.Errorf("number in rtl: %q", string(got))
	}
}
