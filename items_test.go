package main

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"
)

var equipSlots = []Slot{SlotWeapon, SlotOffhand, SlotHelm, SlotArmor, SlotGloves, SlotBoots, SlotRing, SlotAmulet}

func TestRollBaseSlot(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, ilvl := range []int{1, 10, 19, 30} {
		for _, s := range equipSlots {
			for range 20 {
				if b := rollBase(rng, ilvl, s); b.Slot != s {
					t.Fatalf("ilvl %d slot %d got %s", ilvl, s, b.Name)
				}
			}
		}
		slots := map[Slot]bool{}
		for range 400 {
			slots[rollBase(rng, ilvl, SlotNone).Slot] = true
		}
		if len(slots) < len(equipSlots) {
			t.Errorf("ilvl %d: only %d slots in unconstrained rolls", ilvl, len(slots))
		}
	}
}

// Every generated item must be well-formed, whatever it rolled.
func TestGenItemInvariants(t *testing.T) {
	rng, rules := rand.New(rand.NewSource(2)), DefaultRules()
	affixRange := map[Rarity][2]int{RNormal: {0, 0}, RMagic: {1, 2}, RRare: {1, 5}}
	for ilvl := 1; ilvl <= 30; ilvl++ {
		for _, r := range []Rarity{RNormal, RMagic, RRare, RUnique} {
			for _, s := range append([]Slot{SlotNone}, equipSlots...) {
				for range 5 {
					it := GenItem(rng, ilvl, r, s, rules)
					checkItem(t, it, s)
					if rg, ok := affixRange[it.Rarity]; ok && (len(it.Aff) < rg[0] || len(it.Aff) > rg[1]) {
						t.Errorf("%s %s: %d affixes", rarityName[it.Rarity], it.Name, len(it.Aff))
					}
				}
			}
		}
	}
}

func checkItem(t *testing.T, it *Item, want Slot) {
	t.Helper()
	switch {
	case it.Kind != IKEquip || it.Base == nil:
		t.Fatalf("%q: not equipment", it.Name)
	case want != SlotNone && it.Slot() != want:
		t.Errorf("%s: slot %d, want %d", it.Name, it.Slot(), want)
	case it.Name == "":
		t.Errorf("unnamed %s", it.Base.Name)
	case it.MinD > it.MaxD || it.MinD < 0:
		t.Errorf("%s: damage %d-%d", it.Name, it.MinD, it.MaxD)
	case it.Slot() == SlotWeapon && it.MaxD == 0:
		t.Errorf("%s: weapon with no damage", it.Name)
	case it.Armor < 0:
		t.Errorf("%s: armor %d", it.Name, it.Armor)
	case it.Value() <= 0:
		t.Errorf("%s: value %d", it.Name, it.Value())
	}
	seen := map[Stat]bool{}
	for _, a := range it.Aff {
		if seen[a.S] && it.Rarity != RUnique {
			t.Errorf("%s: stat %d rolled twice", it.Name, a.S)
		}
		seen[a.S] = true
		if a.S < 0 || a.S >= StCount {
			t.Errorf("%s: bad stat %d", it.Name, a.S)
		}
	}
}

// Rarer items must be rarer, and bonus must shift odds upward.
func TestRollRarityDistribution(t *testing.T) {
	count := func(bonus float64) [4]int {
		rng := rand.New(rand.NewSource(3))
		var n [4]int
		for range 20000 {
			n[RollRarity(rng, bonus)]++
		}
		return n
	}
	base, boosted := count(0), count(1)
	if base[RNormal] <= base[RMagic] || base[RMagic] <= base[RRare] || base[RRare] <= base[RUnique] || base[RUnique] == 0 {
		t.Errorf("rarity counts not descending: %v", base)
	}
	if boosted[RUnique] <= base[RUnique] || boosted[RRare] <= base[RRare] {
		t.Errorf("bonus did not raise rare drops: %v -> %v", base, boosted)
	}
}

func TestItemValueOrdering(t *testing.T) {
	rng, rules := rand.New(rand.NewSource(4)), DefaultRules()
	avg := func(r Rarity) int {
		sum := 0
		for range 200 {
			sum += GenItem(rng, 10, r, SlotArmor, rules).Value()
		}
		return sum / 200
	}
	n, m, ra := avg(RNormal), avg(RMagic), avg(RRare)
	if n >= m || m >= ra {
		t.Errorf("average value normal %d, magic %d, rare %d", n, m, ra)
	}
}

// The item budget: the smallest BudgetMul that clips none of today's
// Magic and Rare rolls is printed, so DefaultRules can start there; a
// low BudgetMul must hold every roll to its budget.
func TestItemBudget(t *testing.T) {
	rng, r := rand.New(rand.NewSource(5)), DefaultRules()
	worst, at := 0.0, ""
	for ilvl := 1; ilvl <= 30; ilvl++ {
		for _, rar := range []Rarity{RMagic, RRare} {
			for range 2000 {
				it := rollItem(rng, ilvl, rar, SlotNone, r)
				if q := it.power() / (expected(ilvl, r) * budgetFactor[rar]); q > worst {
					worst, at = q, fmt.Sprintf("%s ilvl %d %s (%.0f power)", rarityName[rar], ilvl, it.Name, it.power())
				}
			}
		}
	}
	t.Logf("expected Magic affix power at ilvl 1/5/10/20: %.1f/%.1f/%.1f/%.1f", expected(1, r), expected(5, r), expected(10, r), expected(20, r))
	t.Logf("smallest BudgetMul that clips nothing: %.2f, at %s; DefaultRules has %v", worst, at, r.BudgetMul)
	tight := *r
	tight.BudgetMul = 1
	for ilvl := 1; ilvl <= 30; ilvl += 3 {
		for _, rar := range []Rarity{RMagic, RRare} {
			for range 200 {
				it := GenItem(rng, ilvl, rar, SlotNone, &tight)
				slack := 0.0 // an affix clipped to 1 may still be worth more than its share
				for _, a := range it.Aff {
					if a.V == 1 {
						slack += a.power()
					}
				}
				if b := budget(ilvl, rar, &tight); it.power() > b+slack {
					t.Errorf("%s ilvl %d %s: power %.1f over budget %.1f", rarityName[rar], ilvl, it.Name, it.power(), b)
				}
				checkItem(t, it, SlotNone)
			}
		}
	}
}

// Every unique against the budget at its Lvl (BudgetMul 1, unique factor):
// the over-budget ones are printed, not changed.
func TestUniquesBudget(t *testing.T) {
	r := DefaultRules()
	r.BudgetMul = 1
	over := 0
	all := slices.Concat(uniques, []UniqueDef{lastShroud, stolenCoal})
	for _, u := range all {
		it := &Item{Kind: IKEquip, Base: baseByName(u.Base), Aff: u.Aff}
		b := budget(u.Lvl, RUnique, r)
		mark := ""
		if it.power() > b {
			mark, over = "  over", over+1
		}
		t.Logf("%-20s lvl %2d  power %6.1f  budget %6.1f  ×%.2f%s", u.Name, u.Lvl, it.power(), b, it.power()/b, mark)
	}
	t.Logf("%d of %d uniques over budget", over, len(all))
}
