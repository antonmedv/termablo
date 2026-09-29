package main

import (
	"os"
	"testing"
)

func TestShotsUI(t *testing.T) {
	if os.Getenv("SHOTDIR") == "" {
		t.Skip()
	}
	g := NewGame(5)
	g.Mode = ModePlay
	s := NewScreen(120, 36)
	for i, r := range []Rarity{RMagic, RRare, RUnique, RNormal, RMagic, RRare} {
		g.P.Inv = append(g.P.Inv, GenItem(g.rng, 5+i, r, SlotNone))
	}
	g.time = 1
	g.Mode, g.pane, g.cur = ModeInv, 1, 2
	g.Draw(s)
	shot(t, s, "ui_inv")
	g.Mode, g.shop, g.tab, g.cur = ModeShop, g.shops[0], 0, 1
	g.Draw(s)
	shot(t, s, "ui_shop")
	g.P.Points = 5
	g.Mode = ModeChar
	g.Draw(s)
	shot(t, s, "ui_char")
	// combat moment with a firebolt in flight
	g.Mode = ModePlay
	g.changeLevel("crypt2", "", nil)
	l := g.Lv
	m := placeMonster(l, "skel", g.P.X+6, g.P.Y, 3, RankChampion)
	_ = m
	g.computeVisibility()
	g.Target = m
	g.castFirebolt()
	g.time += 0.08
	g.Draw(s)
	shot(t, s, "ui_bolt")
}
