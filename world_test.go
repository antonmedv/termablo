package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// TestShotWorld writes docs/world.md and docs/world.png. The page is the
// graph of every area and how they link, as Mermaid; the picture is the
// surface maps laid out where their edge exits meet, every tile one cell,
// and beside each stair down the chain of floors it leads to. Needs
// rsvg-convert.
//
//	make docs
func TestShotWorld(t *testing.T) {
	needShotDir(t)
	const cw, ch = 3, 6 // a terminal cell is twice as tall as wide
	const side = 300    // right margin for the dungeon stacks
	g := NewGame(42)

	// Lay the surface out from town, following edge exits: an exit on the
	// east edge puts the next map east of this one, its own exit back
	// against this one.
	off := map[string]Pos{"town": {}}
	order := []string{"town"}
	var stairs []struct {
		from string
		lk   Link
	}
	for i := 0; i < len(order); i++ {
		a := g.getLevel(order[i])
		for _, lk := range a.Links {
			b := g.getLevel(lk.To)
			if b.Kind == KDungeon {
				stairs = append(stairs, struct {
					from string
					lk   Link
				}{a.ID, lk})
				continue
			}
			if _, ok := off[b.ID]; ok {
				continue
			}
			back := b.LinkTo(a.ID)
			if back == nil {
				t.Fatalf("%s has no way back to %s", b.ID, a.ID)
			}
			var n Pos
			switch {
			case lk.X0 == a.W-1:
				n.X = 1
			case lk.X0 == 0:
				n.X = -1
			case lk.Y0 == a.H-1:
				n.Y = 1
			case lk.Y0 == 0:
				n.Y = -1
			}
			o := off[a.ID]
			off[b.ID] = Pos{
				o.X + (lk.X0+lk.X1)/2 + n.X - (back.X0+back.X1)/2,
				o.Y + (lk.Y0+lk.Y1)/2 + n.Y - (back.Y0+back.Y1)/2,
			}
			order = append(order, b.ID)
		}
	}
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1<<30, -1<<30
	for id, o := range off {
		l := g.Levels[id]
		minX, minY = min(minX, o.X), min(minY, o.Y)
		maxX, maxY = max(maxX, o.X+l.W), max(maxY, o.Y+l.H)
	}
	px := func(id string, x, y int) (int, int) {
		o := off[id]
		return (o.X + x - minX) * cw, (o.Y + y - minY) * ch
	}

	mw, mh := (maxX-minX)*cw, (maxY-minY)*ch
	img := image.NewRGBA(image.Rect(0, 0, mw, mh))
	for _, id := range order {
		l := g.Levels[id]
		for y := range l.H {
			for x := range l.W {
				td := &tdefs[l.At(x, y)]
				c := td.Albedo.Scale(.8)
				if td.Emit {
					c = td.Emissive
				}
				if l.LinkAt(x, y) != nil {
					c = C(1, .95, .85)
				}
				x0, y0 := px(id, x, y)
				for dy := range ch {
					for dx := range cw {
						img.Set(x0+dx, y0+dy, color.RGBA{u8(c.R), u8(c.G), u8(c.B), 255})
					}
				}
			}
		}
	}
	var pb bytes.Buffer
	if err := png.Encode(&pb, img); err != nil {
		t.Fatal(err)
	}

	var b strings.Builder
	W, H := mw+side, mh
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" font-family="Menlo" font-size="14">`, W, H)
	b.WriteString(`<rect width="100%" height="100%" fill="#0b0b0f"/>`)
	fmt.Fprintf(&b, `<image width="%d" height="%d" href="data:image/png;base64,%s"/>`, mw, mh, base64.StdEncoding.EncodeToString(pb.Bytes()))
	text := func(x, y int, size int, fill, anchor, s string) {
		fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="%d" fill="%s" text-anchor="%s" stroke="#000" stroke-width="3" paint-order="stroke">%s</text>`, x, y, size, fill, anchor, html.EscapeString(s))
	}
	for _, id := range order {
		l := g.Levels[id]
		x, y := px(id, 0, 0)
		fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" fill="none" stroke="#dcbb92" stroke-opacity=".35"/>`, x, y, l.W*cw, l.H*ch)
		text(x+8, y+22, 18, "#dcbb92", "start", l.Name)
		text(x+8, y+40, 13, "#a89880", "start", fmt.Sprintf("%d×%d · lvl %d", l.W, l.H, l.Depth))
	}

	// Each stair down: a marker on the map and, in the margin, the floors
	// below it, followed down until a dead end, a seal or a known floor.
	sort.Slice(stairs, func(i, j int) bool {
		_, yi := px(stairs[i].from, stairs[i].lk.X0, stairs[i].lk.Y0)
		_, yj := px(stairs[j].from, stairs[j].lk.X0, stairs[j].lk.Y0)
		return yi < yj
	})
	seen := map[string]bool{}
	sy := 20
	for _, s := range stairs {
		x, y := px(s.from, s.lk.X0, s.lk.Y0)
		x, y = x+cw/2, y+ch/2
		fmt.Fprintf(&b, `<circle cx="%d" cy="%d" r="9" fill="none" stroke="#ffd27a" stroke-width="2"/>`, x, y)
		var lines []string
		for id := s.lk.To; id != "" && !seen[id] && len(lines) < 6; {
			seen[id] = true
			l := g.getLevel(id)
			name := l.Name
			if l.SealedDown != "" {
				name += "  (sealed below)"
			}
			lines = append(lines, fmt.Sprintf("%-2d %s", l.Depth, name))
			next := ""
			for _, lk := range l.Links {
				if d := g.getLevel(lk.To); d.Kind == KDungeon && !seen[lk.To] && d.Depth > l.Depth {
					next = lk.To
				}
			}
			id = next
		}
		top := max(sy, y-len(lines)*10)
		lx := mw + 16
		fmt.Fprintf(&b, `<path d="M%d %d C%d %d %d %d %d %d" fill="none" stroke="#ffd27a" stroke-opacity=".6" stroke-width="1.5"/>`, x+9, y, (x+lx)/2, y, (x+lx)/2, top+4, lx-6, top+4)
		for i, ln := range lines {
			text(lx, top+8+i*20, 13, "#e8e0d0", "start", ln)
		}
		sy = top + len(lines)*20 + 30
	}
	text(mw+16, H-16, 12, "#a89880", "start", "lvl = area level · seed 42")
	b.WriteString("</svg>")

	dir := os.Getenv("SHOTDIR")
	svg := dir + "/world.svg"
	if err := os.WriteFile(svg, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("rsvg-convert", "-o", dir+"/world.png", svg).CombinedOutput(); err != nil {
		t.Fatal(string(out), err)
	}
	_ = os.Remove(svg)
	md := "# World map\n\n<!-- Generated by `make docs` (TestShotWorld in world_test.go); do not edit. -->\n\n" +
		"```mermaid\n" + worldGraph(g) + "```\n\n![World map](world.png)\n"
	if err := os.WriteFile(dir+"/world.md", []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

// worldGraph is a Mermaid flowchart of every area reachable from town:
// surface maps on their own, the floors of each dungeon grouped, an arrow
// from the shallower end of each link, and a dotted one where a way down
// opens only when its boss dies. Past a seal it stops at the first floor.
func worldGraph(g *Game) string {
	type edge struct{ a, b string }
	var order []string
	seen := map[string]bool{"town": true}
	beyond := map[string]bool{} // reached only through a seal
	var edges, sealed []edge
	for q := []string{"town"}; len(q) > 0; q = q[1:] {
		l := g.getLevel(q[0])
		order = append(order, l.ID)
		if beyond[l.ID] {
			continue
		}
		for _, lk := range l.Links {
			if !seen[lk.To] {
				seen[lk.To] = true
				q = append(q, lk.To)
			}
			if d := g.getLevel(lk.To); d.Depth > l.Depth {
				edges = append(edges, edge{l.ID, lk.To})
			}
		}
		if l.SealedDown != "" {
			sealed = append(sealed, edge{l.ID, l.SealedDown})
			seen[l.SealedDown], beyond[l.SealedDown] = true, true
			q = append(q, l.SealedDown)
		}
	}
	node := func(l *Level) string {
		label := fmt.Sprintf("<b>%s</b><br/>lvl %d", l.Name, l.Depth)
		for _, m := range l.Monsters {
			if m.Rank == RankBoss {
				label += "<br/>boss: " + m.T.Name
			}
		}
		return fmt.Sprintf("%s[\"%s\"]", l.ID, label)
	}
	var b strings.Builder
	b.WriteString("flowchart TD\n")
	group := func(id string) string { return strings.TrimRight(id, "0123456789") }
	var groups []string
	floors := map[string][]*Level{}
	for _, id := range order {
		l := g.Levels[id]
		if l.Kind != KDungeon {
			fmt.Fprintf(&b, "    %s\n", node(l))
			continue
		}
		grp := group(id)
		if floors[grp] == nil {
			groups = append(groups, grp)
		}
		floors[grp] = append(floors[grp], l)
	}
	for _, grp := range groups {
		first := floors[grp][0]
		fmt.Fprintf(&b, "    subgraph %s [%s]\n", grp, strings.TrimSuffix(first.Name, fmt.Sprintf(" %d", first.NameN)))
		for _, l := range floors[grp] {
			fmt.Fprintf(&b, "        %s\n", node(l))
		}
		b.WriteString("    end\n")
	}
	for _, e := range edges {
		fmt.Fprintf(&b, "    %s --> %s\n", e.a, e.b)
	}
	for _, e := range sealed {
		fmt.Fprintf(&b, "    %s -. sealed .-> %s\n", e.a, e.b)
		if l := g.Levels[e.b]; len(l.Links) > 1 {
			fmt.Fprintf(&b, "    %s --> %s_more[\"…\"]\n", e.b, e.b)
		}
	}
	return b.String()
}

func u8(f float32) uint8 { return uint8(min(max(f, 0), 1) * 255) }
