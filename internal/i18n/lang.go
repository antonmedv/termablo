// Package i18n is Termablo's localization: message catalogs in MAML
// (locales/*.maml), English as the source and the fallback, with
// interpolation, CLDR plural forms, gender agreement for nouns, and the
// text helpers a terminal needs for wide (CJK) and right-to-left (Arabic)
// scripts.
package i18n

import "strings"

// Lang describes one supported language.
type Lang struct {
	Code   string // file name and -lang value: locales/<code>.maml
	Name   string // English name
	Native string // the language's name for itself, for the language picker
	RTL    bool   // written right to left
	// Plurals are the CLDR plural categories this language uses for
	// integers, in the order translators should write them.
	Plurals []string
	// Genders are the gender (and number) classes nouns may carry;
	// adjectives and articles agree with them. Plural nouns ("Boots")
	// take pl, or mp/fp where plural adjectives keep the gender; mv/fv
	// mark nouns that elide an article (l'arme) and fall back to m/f.
	// English uses one class of its own, "an", for nouns that take "an".
	Genders []string
}

// Source is the language every key is written in first and every lookup
// falls back to.
const Source = "en"

// Langs is every supported language, the source first. Adding one is a
// row here, a plural rule in PluralCategory and a locales/<code>.maml.
var Langs = []Lang{
	{Code: "en", Name: "English", Native: "English", Plurals: []string{"one", "other"}, Genders: []string{"an"}},
	{Code: "de", Name: "German", Native: "Deutsch", Plurals: []string{"one", "other"}, Genders: []string{"m", "f", "n", "pl"}},
	{Code: "fr", Name: "French", Native: "Français", Plurals: []string{"one", "other"}, Genders: []string{"m", "f", "mp", "fp", "mv", "fv"}},
	{Code: "it", Name: "Italian", Native: "Italiano", Plurals: []string{"one", "other"}, Genders: []string{"m", "f", "mp", "fp", "mv", "fv"}},
	{Code: "ru", Name: "Russian", Native: "Русский", Plurals: []string{"one", "few", "many"}, Genders: []string{"m", "f", "n", "pl"}},
	{Code: "ar", Name: "Arabic", Native: "العربية", RTL: true, Plurals: []string{"zero", "one", "two", "few", "many", "other"}, Genders: []string{"m", "f"}},
	{Code: "zh", Name: "Chinese (Simplified)", Native: "简体中文", Plurals: []string{"other"}},
}

// aliases map other spellings people type to a code.
var aliases = map[string]string{"ch": "zh", "cn": "zh", "zh-cn": "zh", "zh-hans": "zh", "zh_cn": "zh"}

// LangOf returns the language for a code, or false.
func LangOf(code string) (Lang, bool) {
	for _, l := range Langs {
		if l.Code == code {
			return l, true
		}
	}
	return Lang{}, false
}

// Match finds the supported language for a user-supplied tag: a code
// ("de"), an alias ("ch"), or a POSIX locale ("ru_RU.UTF-8"). It returns
// "" when nothing matches.
func Match(tag string) string {
	t := strings.ToLower(strings.TrimSpace(tag))
	if i := strings.IndexAny(t, ".@"); i >= 0 {
		t = t[:i]
	}
	if c, ok := aliases[t]; ok {
		return c
	}
	if _, ok := LangOf(t); ok {
		return t
	}
	if i := strings.IndexAny(t, "_-"); i > 0 {
		return Match(t[:i])
	}
	return ""
}

// FromEnv picks a language from POSIX locale variables, as getenv
// returns them, in their order of precedence; "" when none matches.
func FromEnv(getenv func(string) string) string {
	for _, k := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		v := getenv(k)
		if v == "" || v == "C" || v == "POSIX" || strings.HasPrefix(v, "C.") {
			continue
		}
		// LANGUAGE is a colon-separated preference list
		for _, part := range strings.Split(v, ":") {
			if c := Match(part); c != "" {
				return c
			}
		}
	}
	return ""
}

// PluralCategory is the CLDR plural category of an integer count.
func PluralCategory(lang string, n int) string {
	if n < 0 {
		n = -n
	}
	switch lang {
	case "zh":
		return "other"
	case "fr":
		if n <= 1 {
			return "one"
		}
		return "other"
	case "ru":
		switch m10, m100 := n%10, n%100; {
		case m10 == 1 && m100 != 11:
			return "one"
		case m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14):
			return "few"
		default:
			return "many"
		}
	case "ar":
		switch m100 := n % 100; {
		case n == 0:
			return "zero"
		case n == 1:
			return "one"
		case n == 2:
			return "two"
		case m100 >= 3 && m100 <= 10:
			return "few"
		case m100 >= 11:
			return "many"
		default:
			return "other"
		}
	default: // en, de, it
		if n == 1 {
			return "one"
		}
		return "other"
	}
}
