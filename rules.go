package main

import "math"

// Rules are the numbers inside the game's formulas: what a level is worth,
// how monsters grow, what gold buys. DefaultRules holds today's values;
// cmd/tune searches inside the ranges in knobs (BALANCE.md §2.6). A Game
// owns one; Player, Level and NewMonster reach it through a pointer set
// at creation. No package-level copy: the tuner evaluates many at once.
type Rules struct {
	// the hero
	HPPerLvl    float64 // MaxHP per level past the first
	StrDiv      float64 // DmgRange: 1 + STR/StrDiv
	ToHitPerLvl float64 // ToHit per level past the first
	BoltMul     float64 // FireboltDmg
	NovaMul     float64 // NovaDmg
	BoltEnePow  float64 // exponent on Energy in FireboltDmg
	PotionHeal  float64 // drinkHealth: fraction of MaxHP

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

// DefaultRules is today's game. cmd/tune -apply rewrites this literal.
func DefaultRules() *Rules {
	return &Rules{
		HPPerLvl:          4,
		StrDiv:            60,
		ToHitPerLvl:       0,
		BoltMul:           1,
		NovaMul:           1,
		BoltEnePow:        1,
		PotionHeal:        0.5,
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

// A Knob is one tunable number: its field, the range the tuner may
// search, and the set it belongs to. Step 1 marks a knob the code reads
// as a whole number.
type Knob struct {
	Name   string
	Group  string // combat, economy, loot
	Lo, Hi float64
	Step   float64
	Ptr    func(*Rules) *float64
}

// knobs is the registry the tuner and -apply read.
var knobs = []Knob{
	{"HPPerLvl", "combat", 3, 10, 0, func(r *Rules) *float64 { return &r.HPPerLvl }},
	{"StrDiv", "combat", 30, 90, 0, func(r *Rules) *float64 { return &r.StrDiv }},
	{"ToHitPerLvl", "combat", 0, 3, 0, func(r *Rules) *float64 { return &r.ToHitPerLvl }},
	{"BoltMul", "combat", 0.7, 2, 0, func(r *Rules) *float64 { return &r.BoltMul }},
	{"NovaMul", "combat", 0.7, 2, 0, func(r *Rules) *float64 { return &r.NovaMul }},
	{"BoltEnePow", "combat", 1, 1.4, 0, func(r *Rules) *float64 { return &r.BoltEnePow }},
	{"PotionHeal", "combat", 0.3, 0.7, 0, func(r *Rules) *float64 { return &r.PotionHeal }},
	{"HpLin", "combat", 0.2, 0.5, 0, func(r *Rules) *float64 { return &r.HpLin }},
	{"HpQuad", "combat", 0, 0.08, 0, func(r *Rules) *float64 { return &r.HpQuad }},
	{"DmgLin", "combat", 0.15, 0.4, 0, func(r *Rules) *float64 { return &r.DmgLin }},
	{"DmgQuad", "combat", 0, 0.04, 0, func(r *Rules) *float64 { return &r.DmgQuad }},
	{"BossHpLin", "combat", 0.15, 0.5, 0, func(r *Rules) *float64 { return &r.BossHpLin }},
	{"ChampHp", "combat", 1.5, 3, 0, func(r *Rules) *float64 { return &r.ChampHp }},
	{"ChampDmg", "combat", 1.1, 1.6, 0, func(r *Rules) *float64 { return &r.ChampDmg }},
	{"ChampBase", "combat", 4, 15, 0, func(r *Rules) *float64 { return &r.ChampBase }},
	{"ChampPerDepth", "combat", 1, 5, 0, func(r *Rules) *float64 { return &r.ChampPerDepth }},
	{"PackMul", "combat", 0.6, 1.2, 0, func(r *Rules) *float64 { return &r.PackMul }},
	{"MonArmorPerLvl", "combat", 1, 4, 0, func(r *Rules) *float64 { return &r.MonArmorPerLvl }},
	{"MonToHitPerLvl", "combat", 2, 4, 0, func(r *Rules) *float64 { return &r.MonToHitPerLvl }},
	{"ArmorCap", "combat", 0.4, 0.7, 0, func(r *Rules) *float64 { return &r.ArmorCap }},
	{"ArmorK", "combat", 30, 80, 0, func(r *Rules) *float64 { return &r.ArmorK }},
	{"ArmorPerLvl", "combat", 5, 15, 0, func(r *Rules) *float64 { return &r.ArmorPerLvl }},
	{"MonArmorK", "combat", 80, 200, 0, func(r *Rules) *float64 { return &r.MonArmorK }},
	{"XPBase", "combat", 25, 50, 0, func(r *Rules) *float64 { return &r.XPBase }},
	{"XPExp", "combat", 1.5, 2.1, 0, func(r *Rules) *float64 { return &r.XPExp }},
	{"AffixLvlScale", "loot", 0.4, 1.2, 0, func(r *Rules) *float64 { return &r.AffixLvlScale }},
	{"RareBonus", "loot", 0, 0.3, 0, func(r *Rules) *float64 { return &r.RareBonus }},
	{"BudgetMul", "loot", 0.7, 1.5, 0, func(r *Rules) *float64 { return &r.BudgetMul }},
	{"DropChance", "loot", 8, 20, 1, func(r *Rules) *float64 { return &r.DropChance }},
	{"GoldChance", "economy", 20, 50, 1, func(r *Rules) *float64 { return &r.GoldChance }},
	{"GoldPerLvl", "economy", 2, 5, 0, func(r *Rules) *float64 { return &r.GoldPerLvl }},
	{"PotionPrice", "economy", 10, 40, 0, func(r *Rules) *float64 { return &r.PotionPrice }},
	{"PotionPricePerLvl", "economy", 2, 8, 0, func(r *Rules) *float64 { return &r.PotionPricePerLvl }},
	{"ScrollPrice", "economy", 20, 80, 0, func(r *Rules) *float64 { return &r.ScrollPrice }},
	{"ScrollPricePerLvl", "economy", 3, 12, 0, func(r *Rules) *float64 { return &r.ScrollPricePerLvl }},
	{"HealCost", "economy", 0.2, 1, 0, func(r *Rules) *float64 { return &r.HealCost }},
	{"QuestGoldPerLvl", "economy", 100, 400, 0, func(r *Rules) *float64 { return &r.QuestGoldPerLvl }},
	{"RerollPrice", "economy", 20, 150, 0, func(r *Rules) *float64 { return &r.RerollPrice }},
	{"RerollPerDepth", "economy", 5, 30, 0, func(r *Rules) *float64 { return &r.RerollPerDepth }},
	{"GamblePrice", "economy", 60, 300, 0, func(r *Rules) *float64 { return &r.GamblePrice }},
	{"GamblePerDepth", "economy", 10, 60, 0, func(r *Rules) *float64 { return &r.GamblePerDepth }},
	{"GambleBonus", "economy", 0, 2, 0, func(r *Rules) *float64 { return &r.GambleBonus }},
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
