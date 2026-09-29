package main

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Screenshot tests render scenes to SVG/PNG for eyeballing. They check
// nothing and only run with SHOTDIR set:
//
//	SHOTDIR=/tmp/shots go test -run Shot

func (s *Screen) SVG() string {
	cw, ch := 9.0, 18.0
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d"><rect width="100%%" height="100%%" fill="#000"/>`, int(float64(s.W)*cw), int(float64(s.H)*ch))
	for y := range s.H {
		for x := range s.W {
			c := s.C[y*s.W+x]
			if c.BG != (Col8{}) {
				fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" fill="rgb(%d,%d,%d)"/>`, float64(x)*cw, float64(y)*ch, cw+0.5, ch+0.5, c.BG.R, c.BG.G, c.BG.B)
			}
		}
	}
	fmt.Fprintf(&b, `<g font-family="Menlo" font-size="15">`)
	for y := range s.H {
		for x := range s.W {
			c := s.C[y*s.W+x]
			if c.Ch == ' ' {
				continue
			}
			w := ""
			if c.Bold {
				w = ` font-weight="bold"`
			}
			fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" fill="rgb(%d,%d,%d)"%s>%s</text>`, float64(x)*cw, float64(y)*ch+14, c.FG.R, c.FG.G, c.FG.B, w, html.EscapeString(string(c.Ch)))
		}
	}
	b.WriteString("</g></svg>")
	return b.String()
}

func shot(t *testing.T, s *Screen, name string) {
	dir := os.Getenv("SHOTDIR")
	svg := dir + "/" + name + ".svg"
	if err := os.WriteFile(svg, []byte(s.SVG()), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("rsvg-convert", "-o", dir+"/"+name+".png", svg).CombinedOutput(); err != nil {
		t.Log(string(out), err)
	}
}

func needShotDir(t *testing.T) {
	if os.Getenv("SHOTDIR") == "" {
		t.Skip("set SHOTDIR to write screenshots")
	}
}

func TestShots(t *testing.T) {
	needShotDir(t)
	g := NewGame(42)
	g.Mode = ModePlay
	s := NewScreen(110, 34)
	g.time = 1.3
	g.Draw(s)
	shot(t, s, "town")
	for _, id := range []string{"fields", "crypt1", "marsh", "grotto1", "abyss1"} {
		g.changeLevel(id, "", nil)
		g.P.HP = 1e9
		for range 60 {
			if !g.autoStep() {
				break
			}
		}
		g.time += 2.1
		g.Draw(s)
		shot(t, s, id)
		// teleport near a light feature
		l := g.Lv
		for i, tt := range l.T {
			if tt == TBrazier || tt == TCrystal || tt == TCampfire || tt == TLava {
				x, y := l.FreeNear(i%l.W+3, i/l.W+1, -1, -1)
				if l.MonsterAt(x, y) == nil {
					g.P.X, g.P.Y = x, y
					if l.Kind == KDungeon && tt == TBrazier && i%3 != 0 {
						continue
					}
					break
				}
			}
		}
		g.computeVisibility()
		g.Draw(s)
		shot(t, s, id+"_lit")
	}
	g.Mode = ModeTitle
	g.Draw(s)
	shot(t, s, "title")
}

func TestShotsUI(t *testing.T) {
	needShotDir(t)
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

func TestShotHover(t *testing.T) {
	needShotDir(t)
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
