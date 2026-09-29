package main

import (
	"fmt"
	"testing"
)

// Gameplay scenarios, driven through the real key handler where possible.

func TestBuyPotion(t *testing.T) {
	g := NewGame(1)
	m := newTestModel(g)
	g.Mode = ModePlay
	alch := npc(t, g, "alch")
	walkInto(g, alch.X, alch.Y)
	if g.Mode != ModeShop || g.shop != g.shops[1] {
		t.Fatalf("mode %v after walking into Mirela", g.Mode)
	}
	gold, pots := g.P.Gold, g.P.HPot
	price := g.shopList()[0].Value()
	press(m, "enter")
	if g.P.HPot != pots+1 || g.P.Gold != gold-price {
		t.Fatalf("potions %d->%d, gold %d->%d (price %d)", pots, g.P.HPot, gold, g.P.Gold, price)
	}
	g.P.Gold = 0
	press(m, "enter")
	if g.P.HPot != pots+1 || lastLog(g) != "You cannot afford that." {
		t.Errorf("bought with no gold: potions %d, log %q", g.P.HPot, lastLog(g))
	}
	press(m, "esc")
	if g.Mode != ModePlay {
		t.Errorf("esc left mode %v", g.Mode)
	}
}

func TestBuyAndSellGear(t *testing.T) {
	g := NewGame(2)
	m := newTestModel(g)
	g.Mode = ModePlay
	smith := npc(t, g, "smith")
	walkInto(g, smith.X, smith.Y)
	it := g.shop.Items[0]
	g.P.Gold = it.Value()
	stock := len(g.shop.Items)
	press(m, "enter")
	if g.P.Gold != 0 || len(g.P.Inv) != 1 || g.P.Inv[0] != it || len(g.shop.Items) != stock-1 {
		t.Fatalf("buy: gold %d, pack %d, stock %d->%d", g.P.Gold, len(g.P.Inv), stock, len(g.shop.Items))
	}
	press(m, "tab", "enter")
	if len(g.P.Inv) != 0 || g.P.Gold != it.Value()/4 || len(g.shop.Items) != stock {
		t.Fatalf("sell: gold %d want %d, pack %d, stock %d", g.P.Gold, it.Value()/4, len(g.P.Inv), len(g.shop.Items))
	}
}

func TestTownPortalRoundTrip(t *testing.T) {
	g := NewGame(3)
	m := newTestModel(g)
	g.Mode = ModePlay
	g.changeLevel("crypt1", "", nil)
	scrolls := g.P.Scrolls
	press(m, "t")
	pt := g.Portal
	if pt == nil || g.P.Scrolls != scrolls-1 || pt.Level != "crypt1" {
		t.Fatalf("portal %+v, scrolls %d->%d", pt, scrolls, g.P.Scrolls)
	}
	walkInto(g, pt.X, pt.Y)
	if g.Lv.ID != "town" {
		t.Fatalf("stepped into portal, now in %s", g.Lv.ID)
	}
	press(m, "t")
	if g.P.Scrolls != scrolls-1 {
		t.Error("read a scroll in town")
	}
	tp := g.Lv.PortalAt
	walkInto(g, tp.X, tp.Y)
	if g.Lv.ID != "crypt1" || cheb(g.P.X, g.P.Y, pt.X, pt.Y) > 2 {
		t.Fatalf("returned to %s at %d,%d; portal was at %d,%d", g.Lv.ID, g.P.X, g.P.Y, pt.X, pt.Y)
	}
	if g.Portal != nil {
		t.Error("portal still open after the return trip")
	}
}

// Every link leads where it says, lands the player off any link (no
// bouncing), and the way back returns to the level it came from.
func TestLinkTravel(t *testing.T) {
	known := map[string]bool{}
	for _, id := range worldIDs {
		known[id] = true
	}
	for seed := int64(1); seed <= 3; seed++ {
		g := NewGame(seed)
		g.Mode = ModePlay
		for _, id := range worldIDs {
			g.changeLevel(id, "", nil)
			for _, lk := range append([]Link(nil), g.Lv.Links...) {
				name := fmt.Sprintf("seed %d %s->%s", seed, id, lk.To)
				g.changeLevel(id, "", nil)
				g.P.HP = 1e6
				clearAround(g.Lv, lk.X0, lk.Y0) // bosses guard some stairs
				if !walkInto(g, lk.X0, lk.Y0) {
					t.Errorf("%s: no way onto the link", name)
					continue
				}
				if g.Lv.ID != lk.To {
					t.Errorf("%s: arrived in %s", name, g.Lv.ID)
					continue
				}
				if g.Lv.LinkAt(g.P.X, g.P.Y) != nil || !g.Lv.Walkable(g.P.X, g.P.Y) {
					t.Errorf("%s: arrived on a link or blocked cell at %d,%d", name, g.P.X, g.P.Y)
				}
				back := g.Lv.LinkTo(id)
				if !known[lk.To] || back == nil {
					continue
				}
				clearAround(g.Lv, back.X0, back.Y0)
				walkInto(g, back.X0, back.Y0)
				if g.Lv.ID != id {
					t.Errorf("%s: way back led to %s", name, g.Lv.ID)
				} else if d := cheb(g.P.X, g.P.Y, lk.X0, lk.Y0); d > 3+lk.X1-lk.X0+lk.Y1-lk.Y0 && lk.AX < 0 {
					t.Errorf("%s: came back %d cells from the link", name, d)
				}
			}
		}
	}
}

func TestDeathAndNewGame(t *testing.T) {
	g := newTestGame(t)
	m := newTestModel(g)
	g.hurtPlayer(1e6, "a test", "smites")
	if g.Mode != ModeDead || g.P.KilledBy != "a test" || g.P.HP != 0 {
		t.Fatalf("mode %v, killed by %q, hp %v", g.Mode, g.P.KilledBy, g.P.HP)
	}
	press(m, "up", "q")
	if m.g != g || g.Mode != ModeDead {
		t.Fatal("keys other than n/Q/esc should do nothing when dead")
	}
	press(m, "n")
	if m.g == g || m.g.Mode != ModePlay || m.g.P.HP != float64(m.g.P.MaxHP()) || m.g.Lv.ID != "town" {
		t.Fatalf("no fresh game after n: mode %v", m.g.Mode)
	}
}

func TestLevelUpAndSpendPoints(t *testing.T) {
	g := newTestGame(t)
	m := newTestModel(g)
	g.P.HP = 1
	g.gainXP(xpNext(1))
	p := g.P
	if p.Lvl != 2 || p.Points != 5 || p.HP != float64(p.MaxHP()) {
		t.Fatalf("lvl %d points %d hp %v/%d", p.Lvl, p.Points, p.HP, p.MaxHP())
	}
	str, vit, maxHP := p.Str, p.Vit, p.MaxHP()
	press(m, "c", "1", "3", "3")
	if p.Str != str+1 || p.Vit != vit+2 || p.Points != 2 || p.MaxHP() != maxHP+4 {
		t.Fatalf("str %d vit %d points %d maxhp %d", p.Str, p.Vit, p.Points, p.MaxHP())
	}
	press(m, "1", "1", "1")
	if p.Points != 0 || p.Str != str+3 {
		t.Fatalf("spent past zero: points %d str %d", p.Points, p.Str)
	}
	press(m, "esc")
	if g.Mode != ModePlay {
		t.Errorf("mode %v", g.Mode)
	}
}

func TestEquipAndUnequipByKeys(t *testing.T) {
	g := newTestGame(t)
	m := newTestModel(g)
	ring := GenItem(g.rng, 5, RMagic, SlotRing)
	g.P.Inv = []*Item{ring}
	press(m, "i", "enter")
	if g.P.Eq[EqRing1] != ring || len(g.P.Inv) != 0 {
		t.Fatalf("ring not equipped: eq %v, pack %d", g.P.Eq[EqRing1], len(g.P.Inv))
	}
	press(m, "tab")
	for i := 0; i < EqRing1; i++ {
		press(m, "down")
	}
	press(m, "enter")
	if g.P.Eq[EqRing1] != nil || len(g.P.Inv) != 1 {
		t.Fatalf("ring not unequipped")
	}
	press(m, "tab", "d")
	if len(g.P.Inv) != 0 || len(g.Lv.ItemsAt(g.P.X, g.P.Y)) != 1 {
		t.Fatalf("drop: pack %d, floor %d", len(g.P.Inv), len(g.Lv.ItemsAt(g.P.X, g.P.Y)))
	}
	press(m, "esc", "g")
	if len(g.P.Inv) != 1 {
		t.Fatalf("pickup: pack %d", len(g.P.Inv))
	}
}

func TestPotionsByKeys(t *testing.T) {
	g := newTestGame(t)
	m := newTestModel(g)
	p := g.P
	p.HP, p.MP = 1, 0
	hp, mp := p.HPot, p.MPot
	press(m, "q", "w")
	if p.HPot != hp-1 || p.MPot != mp-1 || p.HP <= 1 || p.MP <= 0 {
		t.Fatalf("potions %d/%d, life %v mana %v", p.HPot, p.MPot, p.HP, p.MP)
	}
	p.HPot = 0
	press(m, "q")
	if p.HPot != 0 || lastLog(g) != "You have no healing potions." {
		t.Errorf("drank a potion we did not have: %q", lastLog(g))
	}
}

func TestChestAndFountain(t *testing.T) {
	g := newTestGame(t, withMap(`
		#######
		#@.&.F#
		#######`))
	g.move(1, 0)
	g.move(1, 0) // bump the chest
	if g.Lv.At(3, 1) != TChestOpen {
		t.Fatal("chest did not open")
	}
	if len(g.Lv.Items) < 2 { // loot plus gold
		t.Fatalf("chest dropped %d items", len(g.Lv.Items))
	}
	g.P.HP, g.P.MP = 1, 0
	walkInto(g, 5, 1)
	if g.P.HP != float64(g.P.MaxHP()) || g.P.MP != float64(g.P.MaxMP()) {
		t.Errorf("fountain: life %v mana %v", g.P.HP, g.P.MP)
	}
}

func TestBossQuestReward(t *testing.T) {
	g := NewGame(5)
	g.Mode = ModePlay
	g.changeLevel("crypt4", "", nil)
	var king *Monster
	for _, m := range g.Lv.Monsters {
		if m.T.ID == "boneking" {
			king = m
		}
	}
	if king == nil {
		t.Fatal("no Bone King in crypt4")
	}
	g.killMonster(king)
	if g.Quests[0] != 1 {
		t.Fatalf("quest state %d after the kill", g.Quests[0])
	}
	g.changeLevel("town", "", nil)
	gold := g.P.Gold
	voss := npc(t, g, "captain")
	walkInto(g, voss.X, voss.Y)
	if g.Quests[0] != 2 || g.P.Gold != gold+250*g.P.Lvl || g.Mode != ModeTalk {
		t.Fatalf("quest %d, gold %d->%d, mode %v", g.Quests[0], gold, g.P.Gold, g.Mode)
	}
	press(newTestModel(g), "x")
	gold = g.P.Gold
	walkInto(g, voss.X, voss.Y)
	if g.P.Gold != gold {
		t.Error("rewarded twice")
	}
}

// clearAround removes monsters next to a cell.
func clearAround(l *Level, x, y int) {
	for _, m := range l.Monsters {
		if cheb(m.X, m.Y, x, y) <= 1 {
			m.Dead = true
		}
	}
}
