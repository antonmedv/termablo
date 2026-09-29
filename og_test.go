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

// TestShotZones renders demo/zones.png for the README: six postcards, one per
// region, each from the spot with the best view of what that region is known
// for.
//
//	make zones
func TestShotZones(t *testing.T) {
	needShotDir(t)
	const cols, rows, gap = 47, 13, 8
	const tw, th = cols * 9, rows * 18
	type zone struct {
		id, title string
		seed      int64
		tiles     []Tile // the view to look for...
		monster   string // ...or stand next to this monster, if present
	}
	zones := []zone{
		{"town", "Emberhold", 42, []Tile{TFountain}, ""},
		{"fields", "Ashen Fields", 148, []Tile{TCampfire}, ""},
		{"crypt4", "Throne of the Bone King", 42, []Tile{TBrazier}, "boneking"},
		{"marsh", "Blackmarsh", 42, []Tile{TCrystal}, "wisp"},
		{"grotto1", "Sunken Grotto", 42, []Tile{TCrystal}, ""},
		{"abyss1", "The Burning Abyss", 99, []Tile{TLava}, ""},
	}
	var b strings.Builder
	W, H := tw*2+gap, th*3+gap*2
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, W, H, W, H)
	b.WriteString(`<defs><linearGradient id="bottom" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#000" stop-opacity="0"/><stop offset="1" stop-color="#000" stop-opacity=".9"/></linearGradient></defs>`)
	b.WriteString(`<rect width="100%" height="100%" fill="#000"/>`)
	for i, z := range zones {
		g := NewGame(z.seed)
		g.Mode = ModePlay
		g.changeLevel(z.id, "", nil)
		l := g.Lv
		free := func(x, y int) bool {
			return l.Walkable(x, y) && l.MonsterAt(x, y) == nil && l.LinkAt(x, y) == nil
		}
		px, py := -1, -1
		if z.monster != "" {
			for _, m := range l.Monsters {
				if m.T.ID != z.monster || m.Dead {
					continue
				}
				// a few cells to the side, on the same rows, so it stays in view
				best := 1 << 30
				for dy := -2; dy <= 2; dy++ {
					for dx := -12; dx <= 12; dx++ {
						d := abs(dx) + 4*abs(dy)
						if abs(dx) >= 3 && d < best && free(m.X+dx, m.Y+dy) {
							px, py, best = m.X+dx, m.Y+dy, d
						}
					}
				}
				break
			}
		}
		if px < 0 {
			// the walkable cell that sees the most of the wanted tiles, close up
			stamp := make([]uint32, l.W*l.H)
			var gen uint32
			best := -1.0
			for j := range l.T {
				x, y := j%l.W, j/l.W
				if !free(x, y) {
					continue
				}
				gen++
				score := 0.0
				castRays(l, x, y, 10, stamp, gen, func(k int, d float32) {
					if abs(k%l.W-x) > cols/2-2 || abs(k/l.W-y) > rows/2-1 {
						return
					}
					for _, want := range z.tiles {
						if l.T[k] == want {
							score += 12 - float64(d)
						}
					}
				})
				if x < cols/2 || x > l.W-cols/2 || y < rows/2 || y > l.H-rows/2 {
					score /= 2 // near the edge the camera cannot center on it
				}
				if score > best {
					px, py, best = x, y, score
				}
			}
		}
		if px < 0 {
			t.Fatalf("%s: nowhere to stand", z.id)
		}
		g.P.X, g.P.Y = px, py
		for _, m := range l.Monsters {
			if cheb(m.X, m.Y, px, py) <= 8 {
				m.Awake = true
			}
		}
		g.computeVisibility()
		g.time = 2.0 + float64(i)*0.7
		g.composeLight(g.time, true)
		s := NewScreen(cols, rows)
		g.drawMap(s, 0, 0, cols, rows)
		x, y := (i%2)*(tw+gap), (i/2)*(th+gap)
		fmt.Fprintf(&b, `<svg x="%d" y="%d" width="%d" height="%d" viewBox="0 0 %d %d">%s</svg>`, x, y, tw, th, tw, th, s.SVG())
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="44" fill="url(#bottom)"/>`, x, y+th-44, tw)
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-family="Menlo" font-size="15" fill="#dcbb92">%s</text>`, x+10, y+th-12, z.title)
	}
	b.WriteString("</svg>")
	dir := os.Getenv("SHOTDIR")
	svg := dir + "/zones.svg"
	if err := os.WriteFile(svg, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("rsvg-convert", "-o", dir+"/zones.png", svg).CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
	_ = os.Remove(svg)
}
