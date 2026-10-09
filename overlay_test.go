package main

import (
	"strings"
	"testing"
)

func screenRow(s *Screen, y int) string {
	var sb strings.Builder
	for x := range s.W {
		sb.WriteRune(s.C[y*s.W+x].Ch)
	}
	return sb.String()
}

func screenHas(s *Screen, sub string) bool {
	for y := range s.H {
		if strings.Contains(screenRow(s, y), sub) {
			return true
		}
	}
	return false
}

// boxRect finds the one box drawn on the map side of the screen.
func boxRect(t *testing.T, s *Screen) (x0, y0, x1, y1 int) {
	t.Helper()
	x0, y0, x1, y1 = -1, -1, -1, -1
	for y := range s.H {
		for x := range s.W - panelW {
			switch s.C[y*s.W+x].Ch {
			case '┌':
				x0, y0 = x, y
			case '┐':
				x1 = x
			case '└':
				y1 = y
			}
		}
	}
	if x0 < 0 || x1 < 0 || y1 < 0 {
		t.Fatalf("no box on screen")
	}
	return
}

// The help fits its box at any size and scrolls when it must.
func TestHelpStaysInBox(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {110, 34}, {120, 44}} {
		g := newTestGame(t)
		play := NewScreen(sz[0], sz[1])
		g.Draw(play)
		g.Mode, g.helpOff = ModeHelp, 0
		help := NewScreen(sz[0], sz[1])
		g.Draw(help)
		x0, y0, x1, y1 := boxRect(t, help)
		for y := range help.H {
			for x := range help.W {
				if x >= x0 && x <= x1 && y >= y0 && y <= y1 {
					continue
				}
				if help.C[y*help.W+x] != play.C[y*play.W+x] {
					t.Fatalf("%dx%d: help wrote outside its box at %d,%d: %q", sz[0], sz[1], x, y, help.C[y*help.W+x].Ch)
				}
			}
		}
		if !strings.Contains(screenRow(help, y1-1), "esc to close") {
			t.Errorf("%dx%d: footer missing: %q", sz[0], sz[1], screenRow(help, y1-1))
		}
		if !screenHas(help, "arrows hjkl yubn") {
			t.Errorf("%dx%d: first key row missing", sz[0], sz[1])
		}
		g.helpOff = 1000
		g.Draw(help)
		scrolls := g.helpOff > 0
		if small := sz[1] < 30; scrolls != small {
			t.Errorf("%dx%d: help scrolls=%v, want %v", sz[0], sz[1], scrolls, small)
		}
		if scrolls && !screenHas(help, "Click a belt row") {
			t.Errorf("%dx%d: scrolled to the end, last tip missing", sz[0], sz[1])
		}
	}
}

func TestOverviewAboveLog(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {110, 34}} {
		g := newTestGame(t)
		g.Mode = ModeMap
		s := NewScreen(sz[0], sz[1])
		g.Draw(s)
		_, _, _, y1 := boxRect(t, s)
		if y1 >= sz[1]-logH {
			t.Errorf("%dx%d: overview bottom at row %d covers the log (starts %d)", sz[0], sz[1], y1, sz[1]-logH)
		}
	}
}

func TestInventoryNarrow(t *testing.T) {
	g := newTestGame(t)
	it := GenItem(g.rng, 3, RNormal, SlotGloves, g.Rules)
	it.Rarity, it.Pre, it.Suf = RMagic, "Sturdy", "of Testing"
	it.Name = "Sturdy Leather Gloves of Testing"
	g.P.Inv = append(g.P.Inv, it)
	g.Mode, g.pane, g.cur = ModeInv, 1, 0
	s := NewScreen(80, 24)
	g.Draw(s)
	if !screenHas(s, "Sturdy Leath") {
		t.Errorf("pack name cut too short at 80 columns")
	}
	if !screenHas(s, "Short Sword") {
		t.Errorf("equipped name cut at 80 columns")
	}
	if screenHas(s, "more") {
		t.Errorf("one item should need no scroll hint")
	}
	for range 20 {
		g.P.Inv = append(g.P.Inv, GenItem(g.rng, 3, RNormal, SlotNone, g.Rules))
	}
	g.Draw(s)
	if !screenHas(s, "↓ ") || !screenHas(s, "more") {
		t.Errorf("long pack shows no hint of rows below")
	}
	g.cur = len(g.P.Inv) - 1
	g.Draw(s)
	if !screenHas(s, "↑ ") {
		t.Errorf("pack scrolled to the end shows no hint of rows above")
	}
}

func TestShopScrollHint(t *testing.T) {
	g := NewGame(42)
	g.Mode, g.shop, g.cur, g.tab = ModeShop, g.shops[0], 0, 0
	s := NewScreen(80, 24)
	g.Draw(s)
	if n := len(g.shopList()); n > 6 && !screenHas(s, "↓ ") {
		t.Errorf("%d shop items, no hint of rows below", n)
	}
}

// Talk text wraps to the box it is drawn in, at any size.
func TestTalkStaysInBox(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {110, 34}} {
		g := NewGame(1)
		g.Mode = ModePlay
		play := NewScreen(sz[0], sz[1])
		g.Draw(play)
		for _, m := range g.Lv.Monsters {
			if m.T.ID == "captain" {
				g.talkTo(m)
			}
		}
		talk := NewScreen(sz[0], sz[1])
		g.Draw(talk)
		x0, y0, x1, y1 := boxRect(t, talk)
		for y := range talk.H {
			for x := range talk.W {
				if x >= x0 && x <= x1 && y >= y0 && y <= y1 {
					continue
				}
				if talk.C[y*talk.W+x] != play.C[y*play.W+x] {
					t.Fatalf("%dx%d: talk wrote outside its box at %d,%d: %q", sz[0], sz[1], x, y, talk.C[y*talk.W+x].Ch)
				}
			}
		}
		if !strings.Contains(screenRow(talk, y1), "esc goodbye") {
			t.Errorf("%dx%d: footer missing: %q", sz[0], sz[1], screenRow(talk, y1))
		}
		if !screenHas(talk, "alive.") {
			t.Errorf("%dx%d: last line missing", sz[0], sz[1])
		}
	}
}
