package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

// Rules are the numbers inside the game's formulas: what a level is worth,
// how monsters grow, what gold buys. DefaultRules holds today's values;
// knobs describes each one with the range a balance pass may search. A
// Game owns one; Player, Level and NewMonster reach it through a pointer
// set at creation. No package-level copy: a balance run evaluates many at
// once (BOTRULES, eval_test.go).
type Rules struct {
	// the hero
	HPPerLvl    float64 // MaxHP per level past the first
	StrDiv      float64 // DmgRange: 1 + STR/StrDiv
	ToHitPerLvl float64 // ToHit per level past the first
	BoltMul     float64 // FireboltDmg
	NovaMul     float64 // NovaDmg
	BoltEnePow  float64 // exponent on Energy in FireboltDmg
	PotionHeal  float64 // drinkHealth: fraction of MaxHP
	ManaPerEne  float64 // MaxMP: 14 + ManaPerEne·ENE + ManaPerLvl·(lvl−1)
	ManaPerLvl  float64
	BoltCostLvl float64 // FireboltCost: costFirebolt + BoltCostLvl·(lvl−1)
	ManaPotion  float64 // drinkMana: fraction of MaxMP, plus 5

	// monsters
	HpLin, HpQuad   float64 // NewMonster hpMul = 1 + HpLin·l + HpQuad·l²
	DmgLin, DmgQuad float64 // dMul likewise
	BossHpLin       float64 // boss hpMul = 1 + BossHpLin·l
	ChampHp         float64 // champion life factor
	ChampDmg        float64 // champion damage factor
	ChampBase       float64 // populate: champion pack chance, percent
	ChampPerDepth   float64 // ...plus this per depth
	PackMul         float64 // scales Pack sizes, at least 1
	MonArmorPerLvl  float64 // NewMonster armor per level
	MonToHitPerLvl  float64 // NewMonster to-hit per level

	// armor
	ArmorCap    float64 // monsterDamage: most the hero's armor takes off
	ArmorK      float64 // monsterDamage: a/(a+ArmorK+ArmorPerLvl·lvl)
	ArmorPerLvl float64
	MonArmorK   float64 // meleeAttack: a/(a+MonArmorK) off the hero's blow

	// loot
	AffixLvlScale float64 // multiplies the PerLvl term in rollAffix
	RareBonus     float64 // Rares roll this much stronger
	BudgetMul     float64 // item budget: this many mean Magic rolls, times the rarity factor
	DropChance    float64 // dropLoot: percent chance of an item
	GoldChance    float64 // dropLoot: percent chance of gold
	GoldPerLvl    float64 // dropLoot: gold per monster level

	// economy
	PotionPrice       float64
	PotionPricePerLvl float64
	ScrollPrice       float64
	ScrollPricePerLvl float64
	HealCost          float64 // Mirela, per point of life or mana
	SellDiv           float64 // sellPrice: Value/SellDiv for Magic and up, plain gear at Value/(SellDiv·5/3)
	QuestGoldPerLvl   float64 // Voss, per character level
	RerollPrice       float64 // Hadrik's fresh stock: RerollPrice + RerollPerDepth·Deepest
	RerollPerDepth    float64
	GamblePrice       float64 // an unidentified item: GamblePrice + GamblePerDepth·Deepest
	GamblePerDepth    float64
	GambleBonus       float64 // RollRarity bonus on a gamble

	// growth
	XPBase float64 // xpNext = XPBase · lvl^XPExp
	XPExp  float64
}

// DefaultRules is today's game. A balance pass that keeps a change edits
// this literal.
func DefaultRules() *Rules {
	return &Rules{
		HPPerLvl:          4,
		StrDiv:            60,
		ToHitPerLvl:       0,
		BoltMul:           1,
		NovaMul:           1,
		BoltEnePow:        1,
		PotionHeal:        0.5,
		ManaPerEne:        1.2,
		ManaPerLvl:        2,
		BoltCostLvl:       0.667,
		ManaPotion:        0.4,
		HpLin:             0.32,
		HpQuad:            0.05,
		DmgLin:            0.24,
		DmgQuad:           0.02,
		BossHpLin:         0.22,
		ChampHp:           2.2,
		ChampDmg:          1.3,
		ChampBase:         8,
		ChampPerDepth:     3,
		PackMul:           1,
		MonArmorPerLvl:    2,
		MonToHitPerLvl:    3,
		ArmorCap:          0.5,
		ArmorK:            50,
		ArmorPerLvl:       10,
		MonArmorK:         120,
		AffixLvlScale:     1,
		RareBonus:         0.2,
		BudgetMul:         7,
		DropChance:        12,
		GoldChance:        30,
		GoldPerLvl:        3,
		PotionPrice:       20,
		PotionPricePerLvl: 4,
		ScrollPrice:       40,
		ScrollPricePerLvl: 6,
		HealCost:          0.5,
		SellDiv:           12,
		QuestGoldPerLvl:   250,
		RerollPrice:       50,
		RerollPerDepth:    10,
		GamblePrice:       120,
		GamblePerDepth:    30,
		GambleBonus:       1,
		XPBase:            35,
		XPExp:             1.8,
	}
}

// xpNext is the experience a level needs to end.
func (r *Rules) xpNext(lvl int) int { return int(r.XPBase * math.Pow(float64(lvl), r.XPExp)) }

// A Knob is one tunable number: its field, what it does, the range a
// balance pass may search, and a sensible step for one adjustment. Int
// marks a knob the code reads as a whole number. The registry is what a
// balancing agent reads (make knobs) and what BOTRULES overlays check
// against (rulesFromFile).
type Knob struct {
	Name   string
	Group  string // combat, economy, loot
	Desc   string
	Lo, Hi float64
	Step   float64
	Int    bool
	Ptr    func(*Rules) *float64
}

// knobs is the registry: every number in Rules, once.
var knobs = []Knob{
	{"HPPerLvl", "combat",
		"hero max life per character level past the first",
		3, 10, 0.5, false, func(r *Rules) *float64 { return &r.HPPerLvl }},
	{"StrDiv", "combat",
		"melee damage multiplier is 1 + STR/StrDiv: lower makes Strength worth more",
		30, 90, 5, false, func(r *Rules) *float64 { return &r.StrDiv }},
	{"ToHitPerLvl", "combat",
		"hero to-hit per character level past the first",
		0, 3, 0.25, false, func(r *Rules) *float64 { return &r.ToHitPerLvl }},
	{"BoltMul", "combat",
		"multiplies Firebolt damage",
		0.7, 2, 0.1, false, func(r *Rules) *float64 { return &r.BoltMul }},
	{"NovaMul", "combat",
		"multiplies Frost Nova damage",
		0.7, 2, 0.1, false, func(r *Rules) *float64 { return &r.NovaMul }},
	{"BoltEnePow", "combat",
		"exponent on Energy in Firebolt damage: above 1 the caster scales faster",
		1, 1.4, 0.05, false, func(r *Rules) *float64 { return &r.BoltEnePow }},
	{"PotionHeal", "combat",
		"healing potion restores this fraction of max life, plus 12, over a few turns",
		0.3, 0.7, 0.05, false, func(r *Rules) *float64 { return &r.PotionHeal }},
	{"ManaPerEne", "combat",
		"mana pool per point of Energy",
		0.8, 2.5, 0.1, false, func(r *Rules) *float64 { return &r.ManaPerEne }},
	{"ManaPerLvl", "combat",
		"mana pool per character level past the first",
		0, 8, 0.5, false, func(r *Rules) *float64 { return &r.ManaPerLvl }},
	{"BoltCostLvl", "combat",
		"Firebolt costs costFirebolt (6) plus this per character level past the first; Nova grows by 1 a level regardless",
		0, 1.5, 0.1, false, func(r *Rules) *float64 { return &r.BoltCostLvl }},
	{"ManaPotion", "combat",
		"a mana potion restores this fraction of the pool, plus 5, over a few turns",
		0.2, 0.8, 0.05, false, func(r *Rules) *float64 { return &r.ManaPotion }},
	{"HpLin", "combat",
		"monster life multiplier: 1 + HpLin*(lvl-1) + HpQuad*(lvl-1)^2",
		0.2, 0.5, 0.02, false, func(r *Rules) *float64 { return &r.HpLin }},
	{"HpQuad", "combat",
		"quadratic term of monster life growth",
		0, 0.08, 0.005, false, func(r *Rules) *float64 { return &r.HpQuad }},
	{"DmgLin", "combat",
		"monster damage multiplier: 1 + DmgLin*(lvl-1) + DmgQuad*(lvl-1)^2",
		0.15, 0.4, 0.02, false, func(r *Rules) *float64 { return &r.DmgLin }},
	{"DmgQuad", "combat",
		"quadratic term of monster damage growth",
		0, 0.04, 0.005, false, func(r *Rules) *float64 { return &r.DmgQuad }},
	{"BossHpLin", "combat",
		"boss life multiplier is 1 + BossHpLin*(lvl-1), no quadratic term",
		0.15, 0.5, 0.02, false, func(r *Rules) *float64 { return &r.BossHpLin }},
	{"ChampHp", "combat",
		"champion life factor over a normal of its level",
		1.5, 3, 0.1, false, func(r *Rules) *float64 { return &r.ChampHp }},
	{"ChampDmg", "combat",
		"champion damage factor over a normal of its level",
		1.1, 1.6, 0.05, false, func(r *Rules) *float64 { return &r.ChampDmg }},
	{"ChampBase", "combat",
		"percent chance a pack is a champion pack at depth 0",
		4, 15, 1, false, func(r *Rules) *float64 { return &r.ChampBase }},
	{"ChampPerDepth", "combat",
		"champion pack chance grows this many percent per depth",
		1, 5, 0.5, false, func(r *Rules) *float64 { return &r.ChampPerDepth }},
	{"PackMul", "combat",
		"scales pack sizes from the monster tables, at least 1 monster",
		0.6, 1.2, 0.05, false, func(r *Rules) *float64 { return &r.PackMul }},
	{"MonArmorPerLvl", "combat",
		"monster armor per monster level, on top of the template",
		1, 4, 0.25, false, func(r *Rules) *float64 { return &r.MonArmorPerLvl }},
	{"MonToHitPerLvl", "combat",
		"monster to-hit per monster level, base 55",
		2, 4, 0.25, false, func(r *Rules) *float64 { return &r.MonToHitPerLvl }},
	{"ArmorCap", "combat",
		"most damage the hero's armor can take off a blow, as a fraction",
		0.4, 0.7, 0.05, false, func(r *Rules) *float64 { return &r.ArmorCap }},
	{"ArmorK", "combat",
		"hero armor reduction is a/(a+ArmorK+ArmorPerLvl*lvl): higher weakens armor",
		30, 80, 5, false, func(r *Rules) *float64 { return &r.ArmorK }},
	{"ArmorPerLvl", "combat",
		"armor reduction denominator grows this much per monster level",
		5, 15, 1, false, func(r *Rules) *float64 { return &r.ArmorPerLvl }},
	{"MonArmorK", "combat",
		"monster armor reduction is a/(a+MonArmorK) off the hero's blow: higher weakens it",
		80, 200, 10, false, func(r *Rules) *float64 { return &r.MonArmorK }},
	{"XPBase", "combat",
		"experience to finish level 1; a level needs XPBase*lvl^XPExp",
		25, 50, 2.5, false, func(r *Rules) *float64 { return &r.XPBase }},
	{"XPExp", "combat",
		"exponent of the experience curve",
		1.5, 2.1, 0.05, false, func(r *Rules) *float64 { return &r.XPExp }},
	{"AffixLvlScale", "loot",
		"multiplies the per-ilvl growth of every affix roll",
		0.4, 1.2, 0.1, false, func(r *Rules) *float64 { return &r.AffixLvlScale }},
	{"RareBonus", "loot",
		"Rare affixes roll this fraction stronger",
		0, 0.3, 0.05, false, func(r *Rules) *float64 { return &r.RareBonus }},
	{"BudgetMul", "loot",
		"item budget in mean Magic rolls at the ilvl, times the rarity factor; 7 = never clips",
		0.7, 1.5, 0.1, false, func(r *Rules) *float64 { return &r.BudgetMul }},
	{"DropChance", "loot",
		"percent chance a normal monster drops an item",
		8, 20, 1, true, func(r *Rules) *float64 { return &r.DropChance }},
	{"GoldChance", "economy",
		"percent chance a normal monster drops gold",
		20, 50, 2, true, func(r *Rules) *float64 { return &r.GoldChance }},
	{"GoldPerLvl", "economy",
		"base gold drop per monster level; the drop is base + rand(2*base+6)",
		2, 5, 0.25, false, func(r *Rules) *float64 { return &r.GoldPerLvl }},
	{"PotionPrice", "economy",
		"potion price at character level 1",
		10, 40, 2, false, func(r *Rules) *float64 { return &r.PotionPrice }},
	{"PotionPricePerLvl", "economy",
		"potion price grows this much per character level",
		2, 8, 0.5, false, func(r *Rules) *float64 { return &r.PotionPricePerLvl }},
	{"ScrollPrice", "economy",
		"portal scroll price at character level 1",
		20, 80, 5, false, func(r *Rules) *float64 { return &r.ScrollPrice }},
	{"ScrollPricePerLvl", "economy",
		"scroll price grows this much per character level",
		3, 12, 0.5, false, func(r *Rules) *float64 { return &r.ScrollPricePerLvl }},
	{"HealCost", "economy",
		"Mirela charges this per point of life or mana past level 3",
		0.2, 1, 0.05, false, func(r *Rules) *float64 { return &r.HealCost }},
	{"SellDiv", "economy",
		"merchants pay Value/SellDiv for Magic and better gear, Value/(SellDiv*5/3) for plain: higher pays less",
		8, 40, 2, false, func(r *Rules) *float64 { return &r.SellDiv }},
	{"QuestGoldPerLvl", "economy",
		"Voss pays this per character level for a bounty",
		100, 400, 25, false, func(r *Rules) *float64 { return &r.QuestGoldPerLvl }},
	{"RerollPrice", "economy",
		"Hadrik's fresh stock costs RerollPrice + RerollPerDepth*Deepest",
		20, 150, 10, false, func(r *Rules) *float64 { return &r.RerollPrice }},
	{"RerollPerDepth", "economy",
		"fresh stock price per deepest depth reached",
		5, 30, 2, false, func(r *Rules) *float64 { return &r.RerollPerDepth }},
	{"GamblePrice", "economy",
		"an unidentified item costs GamblePrice + GamblePerDepth*Deepest",
		60, 300, 20, false, func(r *Rules) *float64 { return &r.GamblePrice }},
	{"GamblePerDepth", "economy",
		"gamble price per deepest depth reached",
		10, 60, 5, false, func(r *Rules) *float64 { return &r.GamblePerDepth }},
	{"GambleBonus", "economy",
		"RollRarity bonus on a gamble: higher, rarer",
		0, 2, 0.2, false, func(r *Rules) *float64 { return &r.GambleBonus }},
}

// knobByName finds a registry entry.
func knobByName(name string) *Knob {
	for i := range knobs {
		if knobs[i].Name == name {
			return &knobs[i]
		}
	}
	return nil
}

// rulesFromFile is DefaultRules with the knobs in a JSON file laid over
// it: {"HpLin": 0.3, "GoldPerLvl": 2}. An unknown name is an error; a
// value outside the knob's range is allowed and returned in warnings, so
// a balance pass can search past the range on purpose but never by
// accident. An empty path is the defaults.
func rulesFromFile(path string) (r *Rules, warnings []string, err error) {
	r = DefaultRules()
	if path == "" {
		return r, nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var vals map[string]float64
	if err := json.Unmarshal(data, &vals); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	names := make([]string, 0, len(vals))
	for n := range vals {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		k := knobByName(n)
		if k == nil {
			return nil, nil, fmt.Errorf("%s: unknown knob %q", path, n)
		}
		v := vals[n]
		if k.Int && v != math.Trunc(v) {
			return nil, nil, fmt.Errorf("%s: %s is read as a whole number, got %v", path, n, v)
		}
		if v < k.Lo || v > k.Hi {
			warnings = append(warnings, fmt.Sprintf("%s = %v is outside %v–%v", n, v, k.Lo, k.Hi))
		}
		*k.Ptr(r) = v
	}
	return r, warnings, nil
}

// knobValues is every knob by name.
func knobValues(r *Rules) map[string]float64 {
	m := make(map[string]float64, len(knobs))
	for _, k := range knobs {
		m[k.Name] = *k.Ptr(r)
	}
	return m
}

// knobDiff lists the knobs where r differs from base, in registry order,
// as "Name: base → r".
func knobDiff(base, r *Rules) []string {
	var out []string
	for _, k := range knobs {
		if a, b := *k.Ptr(base), *k.Ptr(r); a != b {
			out = append(out, fmt.Sprintf("%s: %g → %g", k.Name, a, b))
		}
	}
	return out
}
