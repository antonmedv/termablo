package main

import (
	"strings"
	"testing"
)

func TestFireboltOutOfRangeKeepsMana(t *testing.T) {
	g := newTestGame(t)
	m := spawnAt(g, "zombie", 2*fireboltRange, 0, false)
	if !g.canSee(m.X, m.Y) || cheb(m.X, m.Y, g.P.X, g.P.Y) <= fireboltRange {
		t.Fatal("setup: target not visible beyond range")
	}
	g.Target = m
	mp, turn := g.P.MP, g.Turn
	g.castFirebolt()
	if g.P.MP != mp || g.Turn != turn {
		t.Fatalf("mana %v->%v, turn %d->%d", mp, g.P.MP, turn, g.Turn)
	}
	if !strings.Contains(lastLog(g), "out of range") {
		t.Errorf("message = %q", lastLog(g))
	}
}

func TestFireboltSpendsManaInRange(t *testing.T) {
	g := newTestGame(t, withMap(`
		##########
		#@.....z.#
		##########`))
	z := monsters(g, "zombie")[0]
	g.Target = z
	mp, hp := g.P.MP, z.HP
	g.castFirebolt()
	if spent := mp - g.P.MP; spent != float64(g.P.FireboltCost()) {
		t.Errorf("mana %v -> %v, want -%d", mp, g.P.MP, g.P.FireboltCost())
	}
	for i := 0; i < 20 && len(g.Effects) > 0; i++ {
		g.time += 0.1
		g.Draw(NewScreen(80, 30))
	}
	if z.HP >= hp && !z.Dead {
		t.Errorf("zombie unhurt: %d -> %d", hp, z.HP)
	}
}

func TestCycleTargetNearestFirst(t *testing.T) {
	g := newTestGame(t, withAwakeMonsters(), withMap(`
		################
		#@.r...b....z..#
		################`))
	near, mid, far := monsters(g, "rat")[0], monsters(g, "bat")[0], monsters(g, "zombie")[0]
	var order []*Monster
	for range 4 {
		g.cycleTarget()
		order = append(order, g.Target)
	}
	if order[0] != near || order[1] != mid || order[2] != far || order[3] != near {
		t.Fatalf("order = %v", order)
	}
}

func TestHitsAreLogged(t *testing.T) {
	g := newTestGame(t, withAwakeMonsters(), withMap(`
		#####
		#@z.#
		#####`))
	z := monsters(g, "zombie")[0]
	z.ToHit = 1000
	hp := g.P.HP
	g.monsterMelee(z)
	if got := lastLog(g); !strings.HasPrefix(got, "The Zombie claws you for ") {
		t.Fatalf("log = %q", got)
	}
	if g.P.HP >= hp {
		t.Errorf("hp %v -> %v", hp, g.P.HP)
	}
}

func TestMeleeKillGivesXP(t *testing.T) {
	g := newTestGame(t, withMap(`
		#####
		#@r.#
		#####`))
	r := monsters(g, "rat")[0]
	xp := g.P.XP
	for i := 0; i < 50 && !r.Dead; i++ {
		g.P.HP = float64(g.P.MaxHP())
		g.move(1, 0)
	}
	if !r.Dead {
		t.Fatal("rat survived 50 swings")
	}
	if g.P.XP <= xp || g.P.Kills != 1 {
		t.Errorf("xp %d -> %d, kills %d", xp, g.P.XP, g.P.Kills)
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
}
