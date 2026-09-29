package main

import "testing"

// Rules that keep potions and spells from being spammed.

func TestPotionHealsOverTime(t *testing.T) {
	g := newTestGame(t)
	p := g.P
	p.HP = 1
	g.drinkHealth()
	after := p.HP
	if after >= float64(p.MaxHP())*0.5 {
		t.Fatalf("potion healed %v at once", after-1)
	}
	for range 8 {
		g.wait()
	}
	if p.HP <= after || p.HealPool > 0.01 {
		t.Errorf("life %v -> %v, pool left %v", after, p.HP, p.HealPool)
	}
}

func TestNoPotionAtFullLife(t *testing.T) {
	g := newTestGame(t)
	n, turn := g.P.HPot, g.Turn
	g.drinkHealth()
	if g.P.HPot != n || g.Turn != turn {
		t.Errorf("drank at full life: potions %d->%d", n, g.P.HPot)
	}
}

func TestBeltLimit(t *testing.T) {
	g := newTestGame(t)
	p := g.P
	p.HPot = beltMax
	g.dropItem(p.X, p.Y, NewPotion(IKHealth))
	g.autoPickup()
	if p.HPot != beltMax || len(g.Lv.ItemsAt(p.X, p.Y)) != 1 {
		t.Errorf("belt holds %d, floor %d", p.HPot, len(g.Lv.ItemsAt(p.X, p.Y)))
	}
	p.Gold = 1000
	g.buy(NewPotion(IKHealth))
	if p.HPot != beltMax || p.Gold != 1000 {
		t.Errorf("bought past the belt: %d potions, %d gold", p.HPot, p.Gold)
	}
}

func TestPotionPriceGrows(t *testing.T) {
	g := newTestGame(t)
	pot := NewPotion(IKMana)
	cheap := g.buyPrice(pot)
	g.P.Lvl = 10
	if g.buyPrice(pot) <= cheap {
		t.Errorf("potion costs %d at level 1 and %d at 10", cheap, g.buyPrice(pot))
	}
}

func TestPlainGearSellsForLittle(t *testing.T) {
	g := newTestGame(t)
	for _, r := range []Rarity{RNormal, RMagic, RRare} {
		it := GenItem(g.rng, 5, r, SlotArmor)
		if sp := sellPrice(it); sp < 1 || sp > it.Value()/12 {
			t.Errorf("%s sells for %d, worth %d", rarityName[r], sp, it.Value())
		}
	}
}

func TestSpellsScaleWithEnergyOnly(t *testing.T) {
	p := NewPlayer()
	lo, hi := p.FireboltDmg()
	p.Lvl = 20
	if l, h := p.FireboltDmg(); l != lo || h != hi {
		t.Errorf("level alone raised firebolt %d-%d to %d-%d", lo, hi, l, h)
	}
	p.Ene += 30
	if l, _ := p.FireboltDmg(); l <= lo {
		t.Errorf("energy did not raise firebolt")
	}
}

func TestNovaFreezeWearsOff(t *testing.T) {
	g := newTestGame(t, withMap(`
		#######
		#.....#
		#..@z.#
		#.....#
		#######`))
	z := monsters(g, "zombie")[0]
	z.MaxHP, z.HP = 1000, 1000
	g.P.MP = float64(g.P.NovaCost())
	g.castNova()
	if z.Frozen == 0 || z.FreezeCD == 0 {
		t.Fatalf("nova did not freeze: frozen %d, immune %d", z.Frozen, z.FreezeCD)
	}
	for z.Frozen > 0 {
		g.wait()
	}
	g.P.MP = float64(g.P.NovaCost())
	g.castNova()
	if z.Frozen != 0 {
		t.Errorf("refroze right after thawing: %d", z.Frozen)
	}
}

func TestEliteNotFrozen(t *testing.T) {
	g := newTestGame(t, withMap(`
		#####
		#@z.#
		#####`))
	z := monsters(g, "zombie")[0]
	z.Rank, z.MaxHP, z.HP = RankUnique, 1000, 1000
	g.castNova()
	if z.Frozen != 0 {
		t.Errorf("unique frozen for %d turns", z.Frozen)
	}
}

func TestFireboltCanMissDodgers(t *testing.T) {
	g := newTestGame(t, withMap(`
		########
		#@...b.#
		########`))
	b := monsters(g, "bat")[0]
	misses := 0
	for range 200 {
		g.P.MP = float64(g.P.FireboltCost())
		b.X, b.Y, b.MaxHP, b.HP, b.Frozen = 5, 1, 1e6, 1e6, 99
		hp := b.HP
		g.Target = b
		g.castFirebolt()
		if b.HP == hp {
			misses++
		}
	}
	if misses < 30 || misses > 110 {
		t.Errorf("bat dodged %d/200 firebolts, want about %d%%", misses, b.T.Dodge)
	}
}
