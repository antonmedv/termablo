package main

import (
	"strings"
	"testing"
)

// beltCells counts the cells in a belt row showing glyph r.
func beltCells(s *Screen, b hitBox, r rune) int {
	n := 0
	for x := b.X0; x <= b.X1; x++ {
		if s.C[b.Y*s.W+x].Ch == r {
			n++
		}
	}
	return n
}

func beltText(s *Screen, b hitBox) string {
	var sb strings.Builder
	for x := b.X0; x <= b.X1; x++ {
		sb.WriteRune(s.C[b.Y*s.W+x].Ch)
	}
	return sb.String()
}

func TestBeltDraw(t *testing.T) {
	g := newTestGame(t)
	p := g.P
	p.HPot, p.MPot, p.Scrolls = 3, 0, 2
	s := NewScreen(100, 32)
	g.Draw(s)
	heal, mana, portal := g.beltHit[0], g.beltHit[1], g.beltHit[2]
	if got := beltCells(s, heal, '!'); got != 3 {
		t.Errorf("heal belt shows %d potions, want 3", got)
	}
	if got := beltCells(s, heal, '·'); got != beltMax-3 {
		t.Errorf("heal belt shows %d empty wells, want %d", got, beltMax-3)
	}
	if got := beltCells(s, mana, '!'); got != 0 {
		t.Errorf("empty mana belt shows %d potions", got)
	}
	if got := beltCells(s, portal, '?'); got != 1 {
		t.Errorf("portal row shows %d scrolls, want 1 well", got)
	}
	if row := beltText(s, heal); !strings.HasSuffix(row, "Heal") {
		t.Errorf("heal row %q should end with its label", row)
	}
	if row := beltText(s, portal); !strings.HasSuffix(row, "Portal 2") {
		t.Errorf("portal row %q should end with the count", row)
	}
	// a working drink shows on the bar, not the belt
	p.HealPool = 37.2
	g.Draw(s)
	if row := beltText(s, heal); !strings.HasSuffix(row, "Heal") {
		t.Errorf("drinking row %q should not change", row)
	}
	// the rows sit inside the panel and never overlap the gold
	mapW := s.W - panelW
	if heal.X0 < mapW || portal.X0 <= mapW+2+len("Gold 12345") {
		t.Errorf("belt rows at %d and %d, panel starts at %d", heal.X0, portal.X0, mapW)
	}
}

func TestBeltClick(t *testing.T) {
	g := newTestGame(t)
	p := g.P
	p.HP = 1
	s := NewScreen(100, 32)
	g.Draw(s)
	heal, mana, portal := g.beltHit[0], g.beltHit[1], g.beltHit[2]
	g.Click(heal.X0+3, heal.Y, s.W, s.H)
	if p.HPot != 2 || p.HealPool <= 0 {
		t.Fatalf("click on the heal row: %d potions, pool %.0f", p.HPot, p.HealPool)
	}
	g.Click(mana.X1, mana.Y, s.W, s.H)
	if p.MPot != 2 {
		t.Errorf("mana at full still drank: %d potions", p.MPot)
	}
	g.Click(portal.X0-2, portal.Y, s.W, s.H) // the gold, not the scroll
	if p.Scrolls != 1 {
		t.Errorf("clicking the gold read a scroll")
	}
	g.Click(portal.X0+2, portal.Y, s.W, s.H)
	if p.Scrolls != 0 {
		t.Errorf("click on the portal row left %d scrolls", p.Scrolls)
	}
	g.Mode = ModeInv
	g.Click(heal.X0+3, heal.Y, s.W, s.H)
	if p.HPot != 2 {
		t.Errorf("clicked through the inventory")
	}
}

func barText(s *Screen, w int) string {
	var sb strings.Builder
	for x := range w {
		sb.WriteRune(s.C[x].Ch)
	}
	return sb.String()
}

func TestBarPending(t *testing.T) {
	s := NewScreen(20, 1)
	bar(s, 0, 0, 10, .5, .3, colRed, colDim)
	if got, want := barText(s, 10), "█████▒▒▒░░"; got != want {
		t.Errorf("bar %q, want %q", got, want)
	}
	bar(s, 0, 0, 10, .8, .5, colRed, colDim) // the ghost never overflows
	if got, want := barText(s, 10), "████████▒▒"; got != want {
		t.Errorf("bar %q, want %q", got, want)
	}
}
