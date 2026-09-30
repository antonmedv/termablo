package main

import (
	"fmt"
	"math/rand"
	"sync"
)

type Slot int

const (
	SlotNone Slot = iota
	SlotWeapon
	SlotOffhand
	SlotHelm
	SlotArmor
	SlotGloves
	SlotBoots
	SlotRing
	SlotAmulet
)

const (
	EqWeapon = iota
	EqOffhand
	EqHelm
	EqArmor
	EqGloves
	EqBoots
	EqRing1
	EqRing2
	EqAmulet
	EqCount
)

var eqNames = [EqCount]string{"Weapon", "Off-hand", "Helm", "Armor", "Gloves", "Boots", "Ring", "Ring", "Amulet"}

type Rarity int

const (
	RNormal Rarity = iota
	RMagic
	RRare
	RUnique
)

var rarityColor = [...]RGB{C(.88, .86, .82), C(.45, .6, 1), C(1, 1, .4), C(.85, .65, .3)}
var rarityName = [...]string{"", "Magic", "Rare", "Unique"}

type Stat int

const (
	StStr Stat = iota
	StDex
	StVit
	StEne
	StLife
	StMana
	StDmgPct
	StFlatDmg
	StArmor
	StArmorPct
	StCrit
	StLifeSteal
	StLight
	StSpellPct
	StLifeRegen
	StManaRegen
	StMF
	StThorns
	StToHit
	StAllAttr
	StGoldFind
	StCount
)

var statFmt = [StCount]string{
	StStr: "+%d Strength", StDex: "+%d Dexterity", StVit: "+%d Vitality", StEne: "+%d Energy",
	StLife: "+%d Life", StMana: "+%d Mana", StDmgPct: "+%d%% Enhanced Damage", StFlatDmg: "+%d Damage",
	StArmor: "+%d Armor", StArmorPct: "+%d%% Enhanced Armor", StCrit: "+%d%% Critical Strike",
	StLifeSteal: "%d%% Life Stolen per Hit", StLight: "+%d Light Radius", StSpellPct: "+%d%% Spell Damage",
	StLifeRegen: "Regenerate %d Life", StManaRegen: "Regenerate %d Mana", StMF: "+%d%% Magic Find",
	StThorns: "Attackers take %d Damage", StToHit: "+%d Attack Rating", StAllAttr: "+%d All Attributes",
	StGoldFind: "+%d%% Extra Gold",
}

// statWeight is the power of one point of each stat in life-equivalents,
// policy-neutral: what an item is worth to nobody in particular. Item
// pricing (Value) and the item budget read it; the bot's gearScore
// multiplies it by a build's own preferences (botPolicy.mul). Armor%
// counts for little here because finishStats folds it into Armor. A new
// affix is one row here and one in each policy.
var statWeight = [StCount]float64{
	StStr: 1, StDex: 1, StVit: 2, StEne: 2, StLife: 1, StMana: .5, StDmgPct: .5, StFlatDmg: 4,
	StArmor: 1, StArmorPct: .25, StCrit: 2, StLifeSteal: 4, StLight: 3, StSpellPct: 2, StLifeRegen: 2,
	StManaRegen: 2, StMF: .3, StThorns: 1, StToHit: .25, StAllAttr: 6, StGoldFind: .2,
}

// affixGold is what a shop charges per life-equivalent of affix power,
// before the rarity multiplier.
const affixGold = 10

type Affix struct {
	S Stat
	V int
}

// power is the affix's worth in life-equivalents.
func (a Affix) power() float64 { return statWeight[a.S] * float64(a.V) }

// The item budget (BALANCE.md §4 B): an item's affixes may add up to at
// most budget(ilvl, rarity) life-equivalents, the mean Magic roll at that
// ilvl times a rarity factor times Rules.BudgetMul. GenItem rolls as it
// always did and then scales the affixes down to fit. Uniques are
// audited (TestUniquesBudget), not clipped.
var budgetFactor = [...]float64{RNormal: 1, RMagic: 1, RRare: 1.25, RUnique: 1.4}

// The expected table covers ilvl 1..budgetILvls; deeper reads the last.
const (
	budgetILvls = 64
	expectRolls = 500
)

// expectMagic caches the mean affix power of a Magic item per ilvl as two
// numbers, the part from the affixes' own ranges and the part
// AffixLvlScale scales, so one table serves every Rules: mean = a +
// AffixLvlScale·b. It is sampled under DefaultRules otherwise; a knob
// that changes how many affixes a Magic item rolls would have to be
// folded in here.
var expectMagic [budgetILvls + 1]struct {
	once sync.Once
	a, b float64
}

// expected is the mean affix power of a Magic item at ilvl under r.
func expected(ilvl int, r *Rules) float64 {
	e := &expectMagic[clampi(ilvl, 1, budgetILvls)]
	e.once.Do(func() {
		mean := func(scale float64) float64 {
			rng, rr := rand.New(rand.NewSource(int64(ilvl))), *DefaultRules()
			rr.AffixLvlScale = scale
			t := 0.0
			for range expectRolls {
				t += rollItem(rng, ilvl, RMagic, SlotNone, &rr).power()
			}
			return t / expectRolls
		}
		e.a = mean(0)
		e.b = mean(1) - e.a // the same rolls, so only the level term differs
	})
	return e.a + r.AffixLvlScale*e.b
}

// budget is the most affix power an item of this ilvl and rarity keeps.
func budget(ilvl int, rarity Rarity, r *Rules) float64 {
	return expected(ilvl, r) * budgetFactor[rarity] * r.BudgetMul
}

// power is the item's affix power in life-equivalents.
func (it *Item) power() float64 {
	t := 0.0
	for _, a := range it.Aff {
		t += a.power()
	}
	return t
}

// clip scales the affixes down in proportion to fit a budget, rounding
// down so the item lands under it, each affix at least 1.
func (it *Item) clip(budget float64) {
	p := it.power()
	if p <= budget {
		return
	}
	f := budget / p
	for i := range it.Aff {
		it.Aff[i].V = maxi(1, int(float64(it.Aff[i].V)*f))
	}
}

type Base struct {
	Name      string
	Slot      Slot
	Glyph     rune
	MinD      int
	MaxD      int
	Armor     int
	Lvl       int
	TwoHanded bool
}

var bases = []*Base{
	{"Dagger", SlotWeapon, ')', 1, 4, 0, 1, false},
	{"Short Sword", SlotWeapon, ')', 2, 6, 0, 1, false},
	{"Club", SlotWeapon, ')', 1, 7, 0, 1, false},
	{"Hand Axe", SlotWeapon, ')', 3, 7, 0, 2, false},
	{"Mace", SlotWeapon, ')', 3, 9, 0, 3, false},
	{"Long Sword", SlotWeapon, ')', 4, 10, 0, 4, false},
	{"Battle Axe", SlotWeapon, ')', 5, 13, 0, 6, false},
	{"War Hammer", SlotWeapon, ')', 7, 17, 0, 8, true},
	{"Great Sword", SlotWeapon, ')', 8, 20, 0, 10, true},
	{"Runeblade", SlotWeapon, ')', 9, 18, 0, 12, false},
	{"Executioner's Axe", SlotWeapon, ')', 12, 27, 0, 13, true},
	{"Doom Flail", SlotWeapon, ')', 13, 24, 0, 16, false},
	{"Buckler", SlotOffhand, ']', 0, 0, 4, 1, false},
	{"Kite Shield", SlotOffhand, ']', 0, 0, 8, 4, false},
	{"Tower Shield", SlotOffhand, ']', 0, 0, 13, 9, false},
	{"Bone Ward", SlotOffhand, ']', 0, 0, 18, 13, false},
	{"Rags", SlotArmor, '[', 0, 0, 2, 1, false},
	{"Leather Armor", SlotArmor, '[', 0, 0, 6, 1, false},
	{"Studded Leather", SlotArmor, '[', 0, 0, 10, 3, false},
	{"Ring Mail", SlotArmor, '[', 0, 0, 15, 5, false},
	{"Chain Mail", SlotArmor, '[', 0, 0, 21, 7, false},
	{"Scale Mail", SlotArmor, '[', 0, 0, 28, 9, false},
	{"Plate Mail", SlotArmor, '[', 0, 0, 36, 12, false},
	{"Gothic Plate", SlotArmor, '[', 0, 0, 47, 15, false},
	{"Cap", SlotHelm, '^', 0, 0, 2, 1, false},
	{"Skull Cap", SlotHelm, '^', 0, 0, 4, 3, false},
	{"Helm", SlotHelm, '^', 0, 0, 7, 6, false},
	{"Great Helm", SlotHelm, '^', 0, 0, 11, 10, false},
	{"Grim Visage", SlotHelm, '^', 0, 0, 15, 14, false},
	{"Leather Gloves", SlotGloves, '(', 0, 0, 2, 1, false},
	{"Chain Gloves", SlotGloves, '(', 0, 0, 5, 5, false},
	{"Gauntlets", SlotGloves, '(', 0, 0, 8, 10, false},
	{"Boots", SlotBoots, '[', 0, 0, 2, 1, false},
	{"Chain Boots", SlotBoots, '[', 0, 0, 5, 5, false},
	{"Greaves", SlotBoots, '[', 0, 0, 8, 10, false},
	{"Ring", SlotRing, '=', 0, 0, 0, 1, false},
	{"Amulet", SlotAmulet, '"', 0, 0, 0, 1, false},
}

func baseByName(n string) *Base {
	for _, b := range bases {
		if b.Name == n {
			return b
		}
	}
	return bases[0]
}

const (
	mWeapon = 1 << SlotWeapon
	mOff    = 1 << SlotOffhand
	mHelm   = 1 << SlotHelm
	mArmor  = 1 << SlotArmor
	mGloves = 1 << SlotGloves
	mBoots  = 1 << SlotBoots
	mRing   = 1 << SlotRing
	mAmulet = 1 << SlotAmulet
	mJewel  = mRing | mAmulet
	mDef    = mOff | mHelm | mArmor | mGloves | mBoots
	mAll    = 0xffff
)

type AffixDef struct {
	S      Stat
	Lo, Hi int
	PerLvl float64
	Prefix bool
	Slots  int
	Names  []string
}

var affixDefs = []AffixDef{
	{StDmgPct, 10, 25, 3, true, mWeapon, []string{"Jagged", "Deadly", "Vicious", "Brutal", "Savage", "Merciless"}},
	{StFlatDmg, 1, 3, .35, true, mWeapon | mGloves | mRing, []string{"Sharp", "Keen", "Cruel", "Wicked"}},
	{StToHit, 10, 25, 3, true, mWeapon | mGloves | mRing | mAmulet, []string{"Bronze", "Iron", "Steel", "Silver", "Gold", "Platinum"}},
	{StArmor, 3, 8, 1.5, true, mDef | mAmulet, []string{"Sturdy", "Strong", "Glorious", "Blessed", "Saintly", "Godly"}},
	{StArmorPct, 15, 35, 3, true, mDef, []string{"Fine", "Warrior's", "Soldier's", "Knight's", "Lord's", "King's"}},
	{StMana, 5, 12, 2, true, mAll &^ mWeapon, []string{"Lizard's", "Snake's", "Serpent's", "Drake's", "Dragon's"}},
	{StSpellPct, 4, 8, .6, true, mWeapon | mOff | mAmulet, []string{"Smoldering", "Fiery", "Blazing", "Infernal", "Hellfire"}},
	{StCrit, 2, 5, .35, true, mWeapon | mGloves | mRing, []string{"Honed", "Precise", "Lethal", "Assassin's"}},
	{StManaRegen, 1, 2, .2, true, mHelm | mRing | mAmulet | mOff, []string{"Humming", "Resonant", "Singing"}},
	{StLife, 5, 12, 3, false, mAll &^ mWeapon, []string{"of the Jackal", "of the Fox", "of the Wolf", "of the Tiger", "of the Mammoth", "of the Whale"}},
	{StStr, 2, 5, .6, false, mAll, []string{"of Strength", "of Might", "of Power", "of the Giant", "of the Titan"}},
	{StDex, 2, 5, .6, false, mAll, []string{"of Dexterity", "of Skill", "of Accuracy", "of Precision", "of Perfection"}},
	{StVit, 2, 5, .6, false, mAll, []string{"of Vigor", "of Vitality", "of Zest", "of Life"}},
	{StEne, 2, 5, .6, false, mAll, []string{"of the Mind", "of Energy", "of Brilliance", "of Sorcery", "of Wizardry"}},
	{StLifeSteal, 2, 3, .15, false, mWeapon | mGloves | mRing, []string{"of the Leech", "of the Bat", "of the Vampire"}},
	{StLight, 1, 2, .05, false, mHelm | mRing | mAmulet | mOff | mWeapon, []string{"of Light", "of Radiance", "of the Sun"}},
	{StLifeRegen, 1, 2, .2, false, mArmor | mHelm | mRing | mAmulet | mBoots, []string{"of Regeneration", "of Renewal", "of the Troll"}},
	{StMF, 5, 12, 1.2, false, mRing | mAmulet | mHelm | mBoots | mGloves, []string{"of Fortune", "of Luck", "of Greed"}},
	{StThorns, 2, 5, .8, false, mArmor | mOff, []string{"of Thorns", "of Spikes", "of Retribution"}},
	{StAllAttr, 1, 3, .3, false, mAmulet | mRing, []string{"of the Sky", "of the Moon", "of the Stars", "of the Heavens"}},
	{StGoldFind, 15, 40, 3, false, mRing | mAmulet | mGloves | mBoots, []string{"of Wealth", "of Avarice"}},
}

type UniqueDef struct {
	Name   string
	Base   string
	Lvl    int
	Aff    []Affix
	Flavor string
}

var uniques = []UniqueDef{
	{"Wanderer's Shroud", "Rags", 1, []Affix{{StAllAttr, 3}, {StLife, 20}, {StLight, 1}}, "Worn by one who never stopped walking."},
	{"Shard of Night", "Dagger", 2, []Affix{{StCrit, 15}, {StDmgPct, 60}, {StDex, 8}}, "It drinks the torchlight."},
	{"Hollow Crown", "Skull Cap", 3, []Affix{{StLife, 25}, {StToHit, 40}, {StStr, 6}}, "The king it belonged to did not need a skull."},
	{"Emberbrand", "Long Sword", 4, []Affix{{StDmgPct, 70}, {StSpellPct, 20}, {StLight, 2}, {StFlatDmg, 3}}, "Its edge never cools."},
	{"The Last Lantern", "Kite Shield", 4, []Affix{{StLight, 4}, {StLifeRegen, 3}, {StArmor, 12}}, "It burns when all else is dark."},
	{"Moonstride", "Chain Boots", 5, []Affix{{StDex, 12}, {StCrit, 5}, {StLife, 20}}, "Footsteps like falling snow."},
	{"Band of Ashes", "Ring", 5, []Affix{{StSpellPct, 25}, {StMana, 25}, {StLight, 1}}, "Warm to the touch, always."},
	{"Gravewarden's Coat", "Chain Mail", 6, []Affix{{StArmorPct, 80}, {StLife, 35}, {StThorns, 8}}, "Stitched from the shrouds of the unquiet."},
	{"Eye of the Deep", "Amulet", 7, []Affix{{StMF, 50}, {StAllAttr, 6}, {StManaRegen, 3}}, "It blinks when you are not looking."},
	{"Crown of the Drowned", "Great Helm", 9, []Affix{{StMana, 45}, {StManaRegen, 4}, {StEne, 12}, {StLight, -1}}, "Water still drips from its rim."},
	{"Bloodfist", "Gauntlets", 9, []Affix{{StLifeSteal, 7}, {StStr, 12}, {StFlatDmg, 5}}, "Never clean. Never dry."},
	{"Kingsbane", "Executioner's Axe", 12, []Affix{{StDmgPct, 130}, {StCrit, 10}, {StLifeSteal, 5}}, "Seven crowns have rolled beneath it."},
	{"Starfall Plate", "Gothic Plate", 14, []Affix{{StArmorPct, 110}, {StAllAttr, 10}, {StLight, 3}, {StLife, 60}}, "Forged from a fallen star; it remembers the sky."},
}

type ItemKind int

const (
	IKEquip ItemKind = iota
	IKGold
	IKHealth
	IKMana
	IKScroll
	IKGamble // Hadrik's unidentified item: bought, it rolls (BALANCE.md §4 E)
	IKReroll // Hadrik's fresh stock
)

// gambleBases stand in for the item a gamble will become: a slot and a
// name for the shop entry.
var gambleBases = []*Base{
	{Name: "Weapon", Slot: SlotWeapon, Glyph: ')'},
	{Name: "Shield", Slot: SlotOffhand, Glyph: ']'},
	{Name: "Helm", Slot: SlotHelm, Glyph: '^'},
	{Name: "Armor", Slot: SlotArmor, Glyph: '['},
	{Name: "Gloves", Slot: SlotGloves, Glyph: '('},
	{Name: "Boots", Slot: SlotBoots, Glyph: '['},
	{Name: "Ring", Slot: SlotRing, Glyph: '='},
	{Name: "Amulet", Slot: SlotAmulet, Glyph: '"'},
}

type Item struct {
	Kind   ItemKind
	Base   *Base
	Name   string
	Rarity Rarity
	ILvl   int
	MinD   int
	MaxD   int
	Armor  int
	Aff    []Affix
	Amount int
	Src    GoldSrc // gold only: where it came from
	Flavor string
}

type FloorItem struct {
	X, Y  int
	It    *Item
	Light *Light
}

func (it *Item) Slot() Slot {
	if it.Base == nil {
		return SlotNone
	}
	return it.Base.Slot
}

func (it *Item) Glyph() rune {
	switch it.Kind {
	case IKGold:
		return '$'
	case IKHealth, IKMana:
		return '!'
	case IKScroll:
		return '?'
	case IKReroll:
		return '*'
	}
	return it.Base.Glyph
}

func (it *Item) Color() RGB {
	switch it.Kind {
	case IKGold:
		return colGold
	case IKHealth:
		return C(1, .25, .25)
	case IKMana:
		return C(.35, .5, 1)
	case IKScroll:
		return C(.85, .8, .65)
	case IKGamble:
		return C(.7, .6, .85)
	case IKReroll:
		return C(1, .6, .3)
	}
	return rarityColor[it.Rarity]
}

func (it *Item) DisplayName() string {
	switch it.Kind {
	case IKGold:
		return fmt.Sprintf("%d gold", it.Amount)
	case IKHealth:
		return "Healing Potion"
	case IKMana:
		return "Mana Potion"
	case IKScroll:
		return "Scroll of Town Portal"
	}
	return it.Name
}

func (it *Item) Stat(s Stat) int {
	t := 0
	for _, a := range it.Aff {
		if a.S == s {
			t += a.V
		}
	}
	return t
}

// Value is the gold a shop charges for the item.
func (it *Item) Value() int {
	switch it.Kind {
	case IKGold:
		return it.Amount
	case IKHealth:
		return 30
	case IKMana:
		return 30
	case IKScroll:
		return 60
	case IKGamble, IKReroll:
		return 0 // priced by buyPrice from the depth reached
	}
	v := 20 + it.ILvl*12 + (it.MaxD+it.MinD)*6 + it.Armor*5
	for _, a := range it.Aff {
		v += int(a.power() * affixGold)
	}
	switch it.Rarity {
	case RMagic:
		v *= 2
	case RRare:
		v *= 4
	case RUnique:
		v *= 7
	}
	return v
}

// Lines describes the item for tooltips.
func (it *Item) Lines() []struct {
	S string
	C RGB
} {
	type ln = struct {
		S string
		C RGB
	}
	var out []ln
	out = append(out, ln{it.DisplayName(), it.Color()})
	if it.Kind != IKEquip {
		switch it.Kind {
		case IKHealth:
			out = append(out, ln{"Restores a large portion of life", colGray})
		case IKMana:
			out = append(out, ln{"Restores a large portion of mana", colGray})
		case IKScroll:
			out = append(out, ln{"Opens a portal back to Emberhold", colGray})
		case IKGamble:
			out = append(out, ln{"Hadrik's pick from the depth you have reached.", colGray}, ln{"Could be anything. No refunds.", colGray})
		case IKReroll:
			out = append(out, ln{"Hadrik hauls out new stock, and Mirela does too.", colGray})
		}
		return out
	}
	kind := it.Base.Name
	if it.Rarity != RNormal {
		kind = rarityName[it.Rarity] + " " + it.Base.Name
	}
	if it.Rarity == RRare || it.Rarity == RUnique {
		out = append(out, ln{kind, it.Color().Scale(.75)})
	}
	if it.Base.Slot == SlotWeapon {
		h := ""
		if it.Base.TwoHanded {
			h = " (two-handed)"
		}
		out = append(out, ln{fmt.Sprintf("Damage: %d-%d%s", it.MinD, it.MaxD, h), colWhite})
	}
	if it.Armor > 0 {
		out = append(out, ln{fmt.Sprintf("Armor: %d", it.Armor), colWhite})
	}
	for _, a := range it.Aff {
		out = append(out, ln{fmt.Sprintf(statFmt[a.S], a.V), C(.5, .65, 1)})
	}
	if it.Flavor != "" {
		out = append(out, ln{it.Flavor, C(.75, .6, .35)})
	}
	out = append(out, ln{fmt.Sprintf("Item level %d  ·  worth %dg", it.ILvl, sellPrice(it)), colDim})
	return out
}

func rollBase(rng *rand.Rand, ilvl int, slot Slot) *Base {
	// best available base level per slot, so every slot keeps a pool at high ilvl
	top := map[Slot]int{}
	for _, b := range bases {
		if b.Lvl <= ilvl+1 && b.Lvl > top[b.Slot] {
			top[b.Slot] = b.Lvl
		}
	}
	var pool []*Base
	for _, b := range bases {
		if b.Lvl <= ilvl+1 && (slot == SlotNone || b.Slot == slot) {
			// Prefer bases near the item level (or the slot's best if it tops out).
			if b.Lvl >= ilvl-8 || b.Lvl >= top[b.Slot]-8 {
				pool = append(pool, b)
			}
		}
	}
	if len(pool) == 0 {
		return bases[0]
	}
	return pool[rng.Intn(len(pool))]
}

func rollAffix(rng *rand.Rand, d *AffixDef, ilvl int, r *Rules) (Affix, string) {
	v := d.Lo + rng.Intn(d.Hi-d.Lo+1)
	v += int(float64(ilvl) * d.PerLvl * r.AffixLvlScale * (0.5 + rng.Float64()*0.5))
	maxV := float64(d.Hi) + 18*d.PerLvl*r.AffixLvlScale
	tier := int(float64(v) / maxV * float64(len(d.Names)))
	tier = clampi(tier, 0, len(d.Names)-1)
	return Affix{d.S, v}, d.Names[tier]
}

var rareFirst = []string{"Grim", "Doom", "Blood", "Storm", "Death", "Shadow", "Rune", "Gloom", "Bone", "Ash", "Raven", "Soul", "Dread", "Havoc", "Wraith", "Corpse", "Ember", "Venom", "Night", "Hollow", "Cinder", "Pale"}
var rareSecond = map[Slot][]string{
	SlotWeapon:  {"Bite", "Edge", "Fang", "Song", "Reaver", "Cleaver", "Thirst", "Kiss", "Needle"},
	SlotOffhand: {"Ward", "Bulwark", "Aegis", "Guard", "Wall"},
	SlotArmor:   {"Shell", "Coat", "Hide", "Carapace", "Mantle", "Shroud"},
	SlotHelm:    {"Crown", "Visor", "Cowl", "Brow", "Horn"},
	SlotGloves:  {"Grip", "Fist", "Hand", "Claw", "Knuckle"},
	SlotBoots:   {"Stride", "Track", "Trail", "Spur", "Tread"},
	SlotRing:    {"Loop", "Coil", "Band", "Circle", "Spiral"},
	SlotAmulet:  {"Eye", "Heart", "Charm", "Talisman", "Star"},
}

// GenItem creates an equipment item of a rarity for a slot (SlotNone: any),
// its affixes clipped to the item budget.
func GenItem(rng *rand.Rand, ilvl int, rarity Rarity, slot Slot, r *Rules) *Item {
	it := rollItem(rng, ilvl, rarity, slot, r)
	if it.Rarity != RUnique {
		it.clip(budget(it.ILvl, it.Rarity, r))
	}
	it.finishStats(rng)
	return it
}

// rollItem rolls an item's base, name and affixes; finishStats is left
// to the caller.
func rollItem(rng *rand.Rand, ilvl int, rarity Rarity, slot Slot, r *Rules) *Item {
	if ilvl < 1 {
		ilvl = 1
	}
	if rarity == RUnique {
		var pool []UniqueDef
		for _, u := range uniques {
			if u.Lvl <= ilvl+2 && (slot == SlotNone || baseByName(u.Base).Slot == slot) {
				pool = append(pool, u)
			}
		}
		if len(pool) > 0 {
			u := pool[rng.Intn(len(pool))]
			b := baseByName(u.Base)
			it := &Item{Kind: IKEquip, Base: b, Name: u.Name, Rarity: RUnique, ILvl: maxi(ilvl, u.Lvl), Flavor: u.Flavor}
			it.Aff = append(it.Aff, u.Aff...)
			return it
		}
		rarity = RRare
	}
	b := rollBase(rng, ilvl, slot)
	it := &Item{Kind: IKEquip, Base: b, Rarity: rarity, ILvl: ilvl, Name: b.Name}
	mask := 1 << b.Slot
	var pre, suf []*AffixDef
	for i := range affixDefs {
		d := &affixDefs[i]
		if d.Slots&mask == 0 {
			continue
		}
		if d.Prefix {
			pre = append(pre, d)
		} else {
			suf = append(suf, d)
		}
	}
	used := map[Stat]bool{}
	pick := func(pool []*AffixDef) (Affix, string, bool) {
		for tries := 0; tries < 12 && len(pool) > 0; tries++ {
			d := pool[rng.Intn(len(pool))]
			if used[d.S] {
				continue
			}
			used[d.S] = true
			a, n := rollAffix(rng, d, ilvl, r)
			return a, n, true
		}
		return Affix{}, "", false
	}
	switch rarity {
	case RMagic:
		hasPre, hasSuf := rng.Intn(2) == 0, rng.Intn(2) == 0
		if !hasPre && !hasSuf {
			hasPre = rng.Intn(2) == 0
			hasSuf = !hasPre
		}
		pn, sn := "", ""
		if hasPre {
			if a, n, ok := pick(pre); ok {
				it.Aff = append(it.Aff, a)
				pn = n + " "
			}
		}
		if hasSuf {
			if a, n, ok := pick(suf); ok {
				it.Aff = append(it.Aff, a)
				sn = " " + n
			}
		}
		it.Name = pn + b.Name + sn
	case RRare:
		n := 3 + rng.Intn(3)
		for i := range n {
			pool := pre
			if i%2 == 1 {
				pool = suf
			}
			if a, _, ok := pick(pool); ok {
				// rares roll a little stronger
				a.V += int(float64(a.V) * r.RareBonus)
				it.Aff = append(it.Aff, a)
			}
		}
		sec := rareSecond[b.Slot]
		it.Name = rareFirst[rng.Intn(len(rareFirst))] + " " + sec[rng.Intn(len(sec))]
	}
	return it
}

func (it *Item) finishStats(rng *rand.Rand) {
	b := it.Base
	if b.Slot == SlotWeapon {
		it.MinD, it.MaxD = b.MinD, b.MaxD
		if it.Rarity == RUnique || it.ILvl > b.Lvl+3 {
			it.MaxD += rng.Intn(3)
		}
	}
	if b.Armor > 0 {
		a := b.Armor + rng.Intn(b.Armor/3+2)
		a = a * (100 + it.Stat(StArmorPct)) / 100
		it.Armor = a
	}
}

func NewPotion(k ItemKind) *Item { return &Item{Kind: k, Amount: 1} }

// RollRarity picks a drop rarity. bonus shifts odds toward rarer items.
func RollRarity(rng *rand.Rand, bonus float64) Rarity {
	r := rng.Float64() * 100
	u := 1.2 * (1 + bonus)
	ra := 7 * (1 + bonus*0.8)
	m := 32 * (1 + bonus*0.5)
	switch {
	case r < u:
		return RUnique
	case r < u+ra:
		return RRare
	case r < u+ra+m:
		return RMagic
	}
	return RNormal
}
