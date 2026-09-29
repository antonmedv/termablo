package main

import (
	"fmt"
	"testing"
)

// Every link (exit, stairs) must be reachable from each level's start.
func TestLinksReachable(t *testing.T) {
	ids := []string{"town", "fields", "marsh", "crypt1", "crypt2", "crypt3", "crypt4", "grotto1", "grotto2", "grotto3", "abyss1", "abyss2"}
	for seed := int64(1); seed <= 12; seed++ {
		g := NewGame(seed)
		for _, id := range ids {
			l := g.getLevel(id)
			reach := l.reachable(l.Start.X, l.Start.Y)
			for _, lk := range l.Links {
				ok := false
				for y := lk.Y0; y <= lk.Y1; y++ {
					for x := lk.X0; x <= lk.X1; x++ {
						if reach[l.Idx(x, y)] {
							ok = true
						}
					}
				}
				if !ok {
					t.Errorf("seed %d %s: link to %s unreachable", seed, id, lk.To)
				}
			}
		}
	}
}

func TestRollBaseSlot(t *testing.T) {
	g := NewGame(1)
	for _, ilvl := range []int{1, 10, 19, 30} {
		for _, s := range []Slot{SlotWeapon, SlotOffhand, SlotHelm, SlotArmor, SlotGloves, SlotBoots, SlotRing, SlotAmulet} {
			for i := 0; i < 20; i++ {
				if b := rollBase(g.rng, ilvl, s); b.Slot != s {
					t.Fatalf("ilvl %d slot %d got %s", ilvl, s, b.Name)
				}
			}
		}
		slots := map[Slot]bool{}
		for i := 0; i < 400; i++ {
			slots[rollBase(g.rng, ilvl, SlotNone).Slot] = true
		}
		if len(slots) < 8 {
			t.Errorf("ilvl %d: only %d slots in unconstrained rolls", ilvl, len(slots))
		}
	}
}

func TestEquipRespectsInvMax(t *testing.T) {
	g := NewGame(1)
	p := g.P
	p.Eq[EqOffhand] = GenItem(g.rng, 1, RNormal, SlotOffhand)
	for len(p.Inv) < invMax-1 {
		p.Inv = append(p.Inv, GenItem(g.rng, 1, RNormal, SlotRing))
	}
	p.Inv = append(p.Inv, &Item{Kind: IKEquip, Base: baseByName("War Hammer"), Name: "War Hammer", MinD: 7, MaxD: 17})
	g.equip(len(p.Inv) - 1)
	if len(p.Inv) > invMax {
		t.Fatalf("pack %d/%d", len(p.Inv), invMax)
	}
	_ = fmt.Sprint
}
