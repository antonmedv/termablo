package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestShotOG renders the GitHub social preview: a Fallen camp in the Ashen
// Fields, mid Frost Nova with a Firebolt in flight, and the logo on top.
// Writes og.png (1280×640) into SHOTDIR; needs rsvg-convert.
//
//	make og
func TestShotOG(t *testing.T) {
	needShotDir(t)
	const cols, rows = 108, 27 // 4:1 cells of 9×18 px make a 2:1 image
	const spare = 6            // extra rows rendered below the crop so the action sits low
	g := NewGame(148)
	g.Mode = ModePlay
	g.changeLevel("fields", "", nil)
	l := g.Lv
	var camps []Pos
	for i, tt := range l.T {
		if tt == TCampfire {
			camps = append(camps, Pos{i % l.W, i / l.W})
		}
	}
	camp := camps[3] // the one in view of the crypt entrance
	px, py := l.FreeNear(camp.X-6, camp.Y, -1, -1)
	g.P.X, g.P.Y = px, py
	var target *Monster
	for _, m := range l.Monsters {
		if cheb(m.X, m.Y, camp.X, camp.Y) > 5 {
			continue
		}
		m.Awake = true
		if cheb(m.X, m.Y, px, py) <= 3 {
			m.Frozen = 2
		} else if target == nil || cheb(m.X, m.Y, px, py) > cheb(target.X, target.Y, px, py) {
			target = m
		}
	}
	g.computeVisibility()
	g.time = 3
	g.novaFx(px, py, C(.5, .85, 1), 3.4)
	if target != nil {
		g.time = 3.1
		path, _, _ := g.traceBolt(px, py, target.X, target.Y, fireboltRange, true)
		for i, p := range path { // keep the campfire visible
			if p == camp {
				path = path[:i]
				break
			}
		}
		g.boltFx(path, C(1, .5, .15), '*', true)
	}
	g.time = 3.2
	s := NewScreen(cols, rows+spare)
	g.updateEffects()
	g.composeLight(g.time, true)
	g.drawMap(s, 0, 0, s.W, s.H)

	dir := os.Getenv("SHOTDIR")
	svg := dir + "/og.svg"
	if err := os.WriteFile(svg, []byte(ogSVG(s, rows)), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("rsvg-convert", "-o", dir+"/og.png", svg).CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
	_ = os.Remove(svg)
}

// ogSVG wraps the scene (top `rows` rows of s) in a 1280×640 frame with the
// logo drawn as lit blocks, a tagline and the install command.
func ogSVG(s *Screen, rows int) string {
	const W, H = 1280, 640
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, W, H, W, H)
	b.WriteString(`<defs>
<linearGradient id="fire" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#ffe2b8"/><stop offset=".55" stop-color="#ff9a3c"/><stop offset="1" stop-color="#c8461a"/></linearGradient>
<linearGradient id="top" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#000" stop-opacity=".92"/><stop offset="1" stop-color="#000" stop-opacity="0"/></linearGradient>
<linearGradient id="bottom" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#000" stop-opacity="0"/><stop offset="1" stop-color="#000" stop-opacity=".85"/></linearGradient>
<filter id="glow" x="-20%" y="-50%" width="140%" height="200%"><feGaussianBlur stdDeviation="9"/></filter>
</defs>`)
	b.WriteString(`<rect width="100%" height="100%" fill="#000"/>`)
	// scene: 9×18 px cells scaled to fill the frame, cropped to the top rows
	fmt.Fprintf(&b, `<svg x="0" y="0" width="%d" height="%d" viewBox="0 0 %d %d">%s</svg>`, W, H, s.W*9, rows*18, s.SVG())
	fmt.Fprintf(&b, `<rect x="0" y="0" width="%d" height="250" fill="url(#top)"/>`, W)
	fmt.Fprintf(&b, `<rect x="0" y="%d" width="%d" height="90" fill="url(#bottom)"/>`, H-90, W)
	// logo: each glyph is a column of two square pixels
	const P = 20
	lw := len([]rune(logo[0])) * P
	x0, y0 := (W-lw)/2, 80
	var px strings.Builder
	for r, line := range logo {
		for i, ch := range []rune(line) {
			top, bot := ch == '█' || ch == '▀', ch == '█' || ch == '▄'
			if top {
				fmt.Fprintf(&px, `<rect x="%d" y="%d" width="%d" height="%d"/>`, x0+i*P, y0+r*2*P, P, P+1)
			}
			if bot {
				fmt.Fprintf(&px, `<rect x="%d" y="%d" width="%d" height="%d"/>`, x0+i*P, y0+r*2*P+P, P, P+1)
			}
		}
	}
	fmt.Fprintf(&b, `<g fill="#ff7a1e" opacity=".75" filter="url(#glow)">%s</g>`, px.String())
	fmt.Fprintf(&b, `<g fill="url(#fire)">%s</g>`, px.String())
	tag := "a Diablo-like roguelike for the terminal"
	fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="middle" font-family="Menlo" font-size="27" fill="#dcbb92">%s</text>`, W/2, y0+4*P+52, tag)
	cmd := "go run github.com/antonmedv/termablo@latest"
	fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="middle" font-family="Menlo" font-size="19" fill="#8c7a63">$ %s</text>`, W/2, H-30, cmd)
	b.WriteString("</svg>")
	return b.String()
}
