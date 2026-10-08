package main

import (
	"embed"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/antonmedv/termablo/internal/i18n"
)

// The game's text lives in locales/*.maml, English the source. The
// game logic keeps its English names (item, monster and tile tables,
// Stats, the bot); what the player reads goes through the session's
// catalog, g.L. Table rows are keyed by the slug of their English name,
// so en.maml must have a line for every row (TestCatalogCoversData).

//go:embed locales/*.maml
var localeFS embed.FS

var locales = func() *i18n.Bundle {
	b, err := i18n.Load(localeFS, "locales")
	if err != nil {
		panic(err)
	}
	return b
}()

// SetLang switches the game's language. Before the first turn the log
// is only the welcome, so it is written again in the new language.
func (g *Game) SetLang(code string) {
	g.L = locales.Get(code)
	if g.Turn == 0 {
		g.Log, g.logN = nil, 0
		g.welcome()
	}
}

// nextLang is the language after code in i18n.Langs, dir 1 or -1.
func nextLang(code string, dir int) string {
	n := len(i18n.Langs)
	for i, l := range i18n.Langs {
		if l.Code == code {
			return i18n.Langs[((i+dir)%n+n)%n].Code
		}
	}
	return i18n.Source
}

func langCodes() string {
	codes := make([]string, len(i18n.Langs))
	for i, l := range i18n.Langs {
		codes[i] = l.Code
	}
	return strings.Join(codes, ", ")
}

// say logs a message from the catalog, its first letter capitalized.
func (g *Game) say(col RGB, key string, args ...any) {
	g.msg(col, capFirst(g.L.T(key, args...)))
}

func capFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 || unicode.IsUpper(r) {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

func slug(s string) string { return i18n.Slug(s) }

// ------------------------------------------------------------ monsters

// monsterNoun is a monster's name in the player's language.
func monsterNoun(loc *i18n.Catalog, m *Monster) i18n.Noun {
	if m.UFirst != "" {
		first := loc.Noun("unique_monster.first." + slug(m.UFirst))
		last := loc.T("unique_monster.last."+slug(m.ULast), "noun", first)
		return i18n.Noun{Text: loc.T("unique_monster.name", "first", first, "last", last), Gender: first.Gender}
	}
	return loc.Noun("monster." + m.T.ID)
}

// named monsters (uniques and bosses) go without an article.
func named(m *Monster) bool { return m.Rank >= RankUnique }

// theRef is the monster as a sentence's subject: "the Plague Rat",
// "Bloodmaw the Hungry".
func theRef(loc *i18n.Catalog, m *Monster) i18n.Noun {
	n := monsterNoun(loc, m)
	if named(m) {
		return n
	}
	return i18n.Noun{Text: loc.T("ref.the", "name", n), Gender: n.Gender}
}

// aRef is the monster met for the first time: "a Plague Rat".
func aRef(loc *i18n.Catalog, m *Monster) i18n.Noun {
	n := monsterNoun(loc, m)
	if named(m) {
		return n
	}
	return i18n.Noun{Text: loc.T("ref.a", "name", n), Gender: n.Gender}
}

// modsText lists a champion's or unique's modifiers.
func modsText(loc *i18n.Catalog, m *Monster) string {
	parts := make([]string, len(m.Mods))
	for i, md := range m.Mods {
		parts[i] = loc.T("mod." + slug(modNames[md]))
	}
	return strings.Join(parts, loc.T("ui.list_sep"))
}

// ------------------------------------------------------------ items

// itemNoun is an item's name in the player's language, built from its
// parts so adjectives agree with the base: "Gezackter Dolch".
func itemNoun(loc *i18n.Catalog, it *Item) i18n.Noun {
	switch it.Kind {
	case IKGold:
		return i18n.Noun{Text: loc.T("item.gold", "n", it.Amount)}
	case IKHealth:
		return loc.Noun("item.healing_potion")
	case IKMana:
		return loc.Noun("item.mana_potion")
	case IKScroll:
		return loc.Noun("item.town_portal_scroll")
	case IKGamble:
		b := loc.Noun("gamble." + slug(it.Base.Name))
		return i18n.Noun{Text: loc.T("item.unidentified", "base", b), Gender: b.Gender}
	case IKReroll:
		return loc.Noun("item.fresh_stock")
	}
	if it.Base == nil || !loc.Has("base."+slug(it.Base.Name)) {
		return i18n.Noun{Text: it.Name}
	}
	base := loc.Noun("base." + slug(it.Base.Name))
	switch it.Rarity {
	case RUnique:
		if k := "unique." + slug(it.Name) + ".name"; loc.Has(k) {
			return loc.Noun(k)
		}
		return i18n.Noun{Text: it.Name}
	case RRare:
		if it.Pre == "" {
			return i18n.Noun{Text: it.Name}
		}
		second := loc.Noun("rare.second." + slug(it.Suf))
		if !loc.Has("rare.second." + slug(it.Suf)) {
			second = i18n.Noun{Text: it.Suf}
		}
		first := part(loc, "rare.first.", it.Pre, second)
		return i18n.Noun{Text: loc.T("item.rare", "first", first, "second", second), Gender: second.Gender}
	case RMagic:
		var pre, suf string
		if it.Pre != "" {
			pre = part(loc, "prefix.", it.Pre, base)
		}
		if it.Suf != "" {
			suf = part(loc, "suffix.", it.Suf, base)
		}
		if pre == "" && suf == "" {
			return base
		}
		s := loc.T("item.magic", "prefix", pre, "base", base, "suffix", suf)
		return i18n.Noun{Text: strings.Join(strings.Fields(s), " "), Gender: base.Gender}
	}
	return base
}

// part is a name part agreeing with noun, or its English when the
// catalog has no line for it.
func part(loc *i18n.Catalog, ns, english string, noun i18n.Noun) string {
	if k := ns + slug(english); loc.Has(k) {
		return loc.T(k, "noun", noun)
	}
	return english
}

// iname is itemNoun's text.
func iname(loc *i18n.Catalog, it *Item) string { return itemNoun(loc, it).Text }

// statKeys name each stat's line in the catalog: stat.<key>.
var statKeys = [StCount]string{
	StStr: "str", StDex: "dex", StVit: "vit", StEne: "ene", StLife: "life", StMana: "mana",
	StDmgPct: "dmg_pct", StFlatDmg: "flat_dmg", StArmor: "armor", StArmorPct: "armor_pct",
	StCrit: "crit", StLifeSteal: "life_steal", StLight: "light", StSpellPct: "spell_pct",
	StLifeRegen: "life_regen", StManaRegen: "mana_regen", StMF: "mf", StThorns: "thorns",
	StToHit: "to_hit", StAllAttr: "all_attr", StGoldFind: "gold_find",
}

func statLine(loc *i18n.Catalog, a Affix) string { return loc.T("stat."+statKeys[a.S], "n", a.V) }

// ------------------------------------------------------------ places

// areaName is a level's name, as the panel and the log show it.
func areaName(loc *i18n.Catalog, id string) string {
	num := func(prefix string) int { n, _ := strconv.Atoi(strings.TrimPrefix(id, prefix)); return n }
	switch {
	case id == "town", id == "fields", id == "marsh":
		return loc.T("area." + id)
	case strings.HasPrefix(id, "crypt"):
		if n := num("crypt"); n < 4 {
			return loc.T("area.crypt", "n", n)
		}
		return loc.T("area.throne")
	case strings.HasPrefix(id, "grotto"):
		if n := num("grotto"); n < 3 {
			return loc.T("area.grotto", "n", n)
		}
		return loc.T("area.oracle_pool")
	case strings.HasPrefix(id, "abyss"):
		if n := num("abyss"); n != hearthFloor {
			return loc.T("area.abyss", "n", n)
		}
		return loc.T("area.hearth")
	}
	return id
}

// loreKey is the line logged on first entering a level.
func loreKey(id string) string {
	switch {
	case id == "town", id == "fields", id == "marsh":
		return "lore." + id
	case strings.HasPrefix(id, "crypt"):
		return "lore.crypt"
	case strings.HasPrefix(id, "grotto"):
		return "lore.grotto"
	case id == "abyss"+strconv.Itoa(hearthFloor):
		return "lore.hearth"
	}
	return "lore.abyss"
}

// regionName is where a link leads, as hover text: "the Ashen Fields".
func regionName(loc *i18n.Catalog, id string) string {
	for _, r := range []string{"town", "fields", "marsh", "crypt", "grotto", "abyss"} {
		if strings.HasPrefix(id, r) {
			return loc.T("region." + r)
		}
	}
	return id
}

func tileName(loc *i18n.Catalog, t Tile) string { return loc.T("tile." + slug(tdefs[t].Name)) }
