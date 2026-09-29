package main

import (
	"math/rand"
	"testing"
)

var equipSlots = []Slot{SlotWeapon, SlotOffhand, SlotHelm, SlotArmor, SlotGloves, SlotBoots, SlotRing, SlotAmulet}

func TestRollBaseSlot(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, ilvl := range []int{1, 10, 19, 30} {
		for _, s := range equipSlots {
			for i := 0; i < 20; i++ {
				if b := rollBase(rng, ilvl, s); b.Slot != s {
					t.Fatalf("ilvl %d slot %d got %s", ilvl, s, b.Name)
				}
			}
		}
		slots := map[Slot]bool{}
		for i := 0; i < 400; i++ {
			slots[rollBase(rng, ilvl, SlotNone).Slot] = true
		}
		if len(slots) < len(equipSlots) {
			t.Errorf("ilvl %d: only %d slots in unconstrained rolls", ilvl, len(slots))
		}
	}
}

// Every generated item must be well-formed, whatever it rolled.
func TestGenItemInvariants(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	affixRange := map[Rarity][2]int{RNormal: {0, 0}, RMagic: {1, 2}, RRare: {1, 5}}
	for ilvl := 1; ilvl <= 30; ilvl++ {
		for _, r := range []Rarity{RNormal, RMagic, RRare, RUnique} {
			for _, s := range append([]Slot{SlotNone}, equipSlots...) {
				for i := 0; i < 5; i++ {
					it := GenItem(rng, ilvl, r, s)
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
		for i := 0; i < 20000; i++ {
			n[RollRarity(rng, bonus)]++
		}
		return n
	}
	base, boosted := count(0), count(1)
	if !(base[RNormal] > base[RMagic] && base[RMagic] > base[RRare] && base[RRare] > base[RUnique] && base[RUnique] > 0) {
		t.Errorf("rarity counts not descending: %v", base)
	}
	if boosted[RUnique] <= base[RUnique] || boosted[RRare] <= base[RRare] {
		t.Errorf("bonus did not raise rare drops: %v -> %v", base, boosted)
	}
}

func TestItemValueOrdering(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	avg := func(r Rarity) int {
		sum := 0
		for i := 0; i < 200; i++ {
			sum += GenItem(rng, 10, r, SlotArmor).Value()
		}
		return sum / 200
	}
	n, m, ra := avg(RNormal), avg(RMagic), avg(RRare)
	if !(n < m && m < ra) {
		t.Errorf("average value normal %d, magic %d, rare %d", n, m, ra)
	}
}
