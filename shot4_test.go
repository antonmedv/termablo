package main

import (
	"os"
	"testing"
)

func TestShotHover(t *testing.T) {
	if os.Getenv("SHOTDIR") == "" {
		t.Skip()
	}
	g := NewGame(42)
	g.Mode = ModePlay
	g.changeLevel("crypt1", "", nil)
	s := NewScreen(110, 34)
	l, p := g.Lv, g.P
	mx, my := l.FreeNear(p.X+2, p.Y, p.X, p.Y)
	m := placeMonsterAvoid(l, "skel", mx, my, 3, RankChampion, p.X, p.Y)
	ix, iy := l.FreeNear(p.X-1, p.Y+1, p.X, p.Y)
	g.dropItem(ix, iy, GenItem(g.rng, 5, RUnique, SlotNone))
	g.computeVisibility()
	g.time = 3
	mapW, mapH := s.W-panelW, s.H-logH
	camX, camY := g.camera(mapW, mapH)
	for _, c := range []struct {
		name string
		x, y int
	}{{"hover_monster", m.X, m.Y}, {"hover_item", l.Items[len(l.Items)-1].X, l.Items[len(l.Items)-1].Y}, {"hover_stairs", p.X - 1, p.Y}} {
		g.SetHover(c.x-camX, c.y-camY)
		t.Log(c.name, g.hoverInfo(c.x, c.y))
		g.Draw(s)
		shot(t, s, c.name)
	}
}
