package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Mouse hover: describe whatever is under the cursor on the map.

type hoverLine struct {
	S string
	C RGB
}

// SetHover records the mouse position in screen cells.
func (g *Game) SetHover(x, y int) {
	g.hoverX, g.hoverY, g.hoverOn = x, y, true
}

// hitBox is a clickable run of cells on one screen row, as drawn last
// frame. The zero value matches nothing.
type hitBox struct{ X0, X1, Y int }

func (b hitBox) in(x, y int) bool { return b.X1 > b.X0 && y == b.Y && x >= b.X0 && x <= b.X1 }

// Click handles a left click at screen cell (x,y) on a w×h screen: an
// enemy under the cursor becomes the target, a belt row drinks or reads.
func (g *Game) Click(x, y, w, h int) {
	if g.Mode != ModePlay || x < 0 || y < 0 {
		return
	}
	mapW, mapH := w-panelW, h-logH
	if x >= mapW {
		switch {
		case g.beltHit[0].in(x, y):
			g.drinkHealth()
		case g.beltHit[1].in(x, y):
			g.drinkMana()
		case g.beltHit[2].in(x, y):
			g.readPortal()
		}
		return
	}
	if y >= mapH {
		return
	}
	camX, camY := g.camera(mapW, mapH)
	g.targetAt(camX+x, camY+y)
}

func (g *Game) levelName(id string) string {
	if l, ok := g.Levels[id]; ok {
		return l.Name
	}
	switch {
	case id == "town":
		return "Emberhold"
	case id == "fields":
		return "the Ashen Fields"
	case id == "marsh":
		return "Blackmarsh"
	case strings.HasPrefix(id, "crypt"):
		return "the Crypt of the Fallen"
	case strings.HasPrefix(id, "grotto"):
		return "the Sunken Grotto"
	case strings.HasPrefix(id, "abyss"):
		return "the Burning Abyss"
	}
	return id
}

var tileNotes = map[Tile]string{
	TDoor:        "Walk into it to open.",
	TChest:       "Walk into it to open.",
	TFountain:    "Walk into it to restore life.",
	TAltar:       "Walk into it to make an offering.",
	TBrazier:     "Casts warm firelight.",
	TColdBrazier: "Hadrik's forge. It has gone out.",
	TLamp:        "Casts warm lamplight.",
	TCampfire:    "A camp of the Fallen is never far.",
	TCrystal:     "Glows with cold blue light.",
	TLava:        "Molten rock. Impassable, and bright.",
	TDeepWater:   "Too deep to wade.",
	TWater:       "Shallow enough to wade.",
	TTree:        "Blocks movement and sight.",
	TChestOpen:   "Already looted.",
	TGrave:       "Here lies someone unlucky.",
	TStairsDown:  "",
	TStairsUp:    "",
}

// hoverInfo returns tooltip lines for a map cell, or nil when nothing is known.
func (g *Game) hoverInfo(mx, my int) []hoverLine {
	l, p := g.Lv, g.P
	if !l.In(mx, my) {
		return nil
	}
	i := l.Idx(mx, my)
	lit, _ := g.litAt(i)
	var out []hoverLine
	add := func(s string, c RGB) { out = append(out, hoverLine{s, c}) }

	if !lit {
		if !l.Seen[i] {
			return []hoverLine{{"Darkness", colDim}, {"Unexplored, or beyond your light.", colDim}}
		}
		add(titleWord(tdefs[l.T[i]].Name), colGray)
		add("Remembered — not currently in sight.", colDim)
		return out
	}

	if mx == p.X && my == p.Y {
		add("You — the Wanderer", colWhite)
		add(fmt.Sprintf("Level %d · Life %d/%d · Mana %d/%d", p.Lvl, int(p.HP), p.MaxHP(), int(p.MP), p.MaxMP()), colGray)
		add(fmt.Sprintf("Torch radius %.1f", p.Torch.Radius), C(1, .7, .4))
	}
	if m := l.MonsterAt(mx, my); m != nil {
		if len(out) > 0 {
			add("", colDim)
		}
		add(m.Name, m.Color())
		if m.Friendly {
			add("Townsfolk · walk into them to talk", colGray)
		} else {
			kind := [...]string{"", "Champion · ", "Unique · ", "Boss · "}[mini(m.Rank, 3)]
			add(fmt.Sprintf("%sLevel %d · HP %d/%d", kind, m.Level, maxi(0, m.HP), m.MaxHP), colGray)
			add(fmt.Sprintf("Hits for %d-%d · armor %d", m.MinD, m.MaxD, m.Armor), colGray)
			if d := m.Describe(); d != "" {
				add(d, C(.45, .6, 1))
			}
			var st []string
			if m.Frozen > 0 {
				st = append(st, "frozen")
			}
			if !m.Awake {
				st = append(st, "unaware of you")
			}
			if m.Flee > 0 {
				st = append(st, "fleeing")
			}
			if m.T.AI == AIRanged || m.T.AI == AIOracle || m.T.AI == AIWanderer {
				st = append(st, "ranged")
			}
			if m.Light != nil {
				st = append(st, "glows")
			}
			if len(st) > 0 {
				add(titleWord(strings.Join(st, ", ")), colCyan)
			}
		}
	}
	for _, fi := range l.ItemsAt(mx, my) {
		if len(out) > 0 {
			add("", colDim)
		}
		lines := fi.It.Lines(g.Rules)
		for k, ln := range lines {
			if k >= 8 {
				add("…", colDim)
				break
			}
			add(ln.S, ln.C)
		}
		if fi.It.Kind == IKEquip {
			if cur := g.equippedFor(fi.It); cur != nil {
				add("Replaces "+cur.Name, colDim)
			}
			add("Step on it and press g to pick up", colDim)
		}
	}
	if g.portalAt(mx, my) {
		if len(out) > 0 {
			add("", colDim)
		}
		add("Town Portal", colBlue)
		if l.Kind == KTown {
			add("Leads back to "+g.levelName(g.Portal.Level), colGray)
		} else {
			add("Leads to Emberhold", colGray)
		}
	}
	// the ground itself
	if len(out) == 0 || l.LinkAt(mx, my) != nil || tileNotes[l.T[i]] != "" {
		if len(out) > 0 {
			add("", colDim)
		}
		t := l.T[i]
		name := titleWord(tdefs[t].Name)
		switch l.Decal[i] {
		case DecalCorpse:
			name += " · a corpse"
		case DecalBlood:
			name += " · bloodstained"
		case DecalScorch:
			name += " · scorched"
		}
		col := tdefs[t].Albedo.Scale(1.3)
		if tdefs[t].Emit {
			col = tdefs[t].Emissive
		}
		add(name, col)
		if lk := l.LinkAt(mx, my); lk != nil {
			add("Leads to "+g.levelName(lk.To), colGray)
		} else if n := tileNotes[t]; n != "" {
			add(n, colGray)
		}
	}
	return out
}

func titleWord(s string) string {
	if s == "" {
		return s
	}
	r, n := utf8.DecodeRuneInString(s)
	return strings.ToUpper(string(r)) + s[n:]
}

// currentHover returns info for the hovered map cell, or nil.
func (g *Game) currentHover(mapW, mapH int) []hoverLine {
	if !g.hoverOn || g.Mode != ModePlay || g.hoverX < 0 || g.hoverY < 0 || g.hoverX >= mapW || g.hoverY >= mapH {
		return nil
	}
	camX, camY := g.camera(mapW, mapH)
	lines := g.hoverInfo(camX+g.hoverX, camY+g.hoverY)
	if len(lines) == 0 {
		return nil
	}
	return lines
}

// hoverSummary condenses hover info into one line that fits w cells.
func hoverSummary(lines []hoverLine, w int) []hoverLine {
	var out []hoverLine
	used := 0
	for _, ln := range lines {
		if ln.S == "" || strings.HasPrefix(ln.S, "Step on it") || strings.HasPrefix(ln.S, "Item level") {
			continue
		}
		n := utf8.RuneCountInString(ln.S) + 3
		if used+n > w {
			break
		}
		out = append(out, ln)
		used += n
	}
	return out
}

// drawHover highlights the hovered cell and summarizes it in a status line.
func (g *Game) drawHover(s *Screen, mapW, mapH int) {
	lines := g.hoverLines
	if lines == nil {
		return
	}
	c := &s.C[g.hoverY*s.W+g.hoverX]
	c.BG = C(.25, .22, .3).C8()
	if c.Ch == ' ' {
		c.Ch, c.FG = '·', C(.5, .45, .6).C8()
	}
	// one-line summary along the bottom edge of the map
	drawStatus(s, mapH-1, mapW, hoverSummary(lines, mapW-2))
}

// drawStatus writes a status line across row y of the map: the first
// entry bold, the rest separated by dots.
func drawStatus(s *Screen, y, w int, lines []hoverLine) {
	bg := C(.04, .035, .06).C8()
	s.Fill(0, y, w, 1, bg)
	x := 1
	for k, ln := range lines {
		if k > 0 {
			x += s.Text(x, y, " · ", colDim.C8())
		}
		if k == 0 {
			x += s.TextBold(x, y, ln.S, ln.C.C8())
		} else {
			x += s.Text(x, y, ln.S, ln.C.C8())
		}
	}
}
