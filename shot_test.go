package main

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func (s *Screen) SVG() string {
	cw, ch := 9.0, 18.0
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d"><rect width="100%%" height="100%%" fill="#000"/>`, int(float64(s.W)*cw), int(float64(s.H)*ch))
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
			c := s.C[y*s.W+x]
			if c.BG != (Col8{}) {
				fmt.Fprintf(&b, `<rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" fill="rgb(%d,%d,%d)"/>`, float64(x)*cw, float64(y)*ch, cw+0.5, ch+0.5, c.BG.R, c.BG.G, c.BG.B)
			}
		}
	}
	fmt.Fprintf(&b, `<g font-family="Menlo" font-size="15">`)
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W; x++ {
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
	os.WriteFile(svg, []byte(s.SVG()), 0644)
	if out, err := exec.Command("rsvg-convert", "-o", dir+"/"+name+".png", svg).CombinedOutput(); err != nil {
		t.Log(string(out), err)
	}
}

func TestShots(t *testing.T) {
	if os.Getenv("SHOTDIR") == "" {
		t.Skip()
	}
	g := NewGame(42)
	g.Mode = ModePlay
	s := NewScreen(110, 34)
	g.time = 1.3
	g.Draw(s)
	shot(t, s, "town")
	for _, id := range []string{"fields", "crypt1", "marsh", "grotto1", "abyss1"} {
		g.changeLevel(id, "", nil)
		g.P.HP = 1e9
		for i := 0; i < 60; i++ {
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
