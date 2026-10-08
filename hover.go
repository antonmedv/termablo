package main

import (
	"fmt"
	"strings"

	"github.com/antonmedv/termablo/internal/i18n"
)

// Mouse hover: describe whatever is under the cursor on the map.

type hoverLine struct {
	S string
	C RGB
	// Detail lines stay out of the one-line summary.
	Detail bool
}

// SetHover records the mouse position in screen cells.
func (g *Game) SetHover(x, y int) {
	g.hoverX, g.hoverY, g.hoverOn = x, y, true
}

// clickLang picks a language on the title screen; clicking the one
// already picked begins the game.
func (g *Game) clickLang(x, y int) {
	for i, b := range g.langHit {
		if !b.in(x, y) {
			continue
		}
		if code := i18n.Langs[i].Code; code != g.L.Lang.Code {
			g.SetLang(code)
		} else {
			g.Mode = ModePlay
		}
		return
	}
}

// hitBox is a clickable run of cells on one screen row, as drawn last
// frame. The zero value matches nothing.
type hitBox struct{ X0, X1, Y int }

func (b hitBox) in(x, y int) bool { return b.X1 > b.X0 && y == b.Y && x >= b.X0 && x <= b.X1 }

// Click handles a left click at screen cell (x,y) on a w×h screen: an
// enemy under the cursor becomes the target, a belt row drinks or reads.
func (g *Game) Click(x, y, w, h int) {
	if g.Mode == ModeTitle {
		g.clickLang(x, y)
		return
	}
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

// levelName is where a link or portal leads: the level's own name once
// it exists, else its region's.
func (g *Game) levelName(id string) string {
	if l, ok := g.Levels[id]; ok {
		return areaName(g.L, l)
	}
	return regionName(g.L, id)
}

// tileNotes are the tiles with a hover note, tile_note.<slug> in the
// catalog.
var tileNotes = map[Tile]bool{
	TDoor: true, TChest: true, TFountain: true, TAltar: true, TBrazier: true, TColdBrazier: true,
	TLamp: true, TCampfire: true, TCrystal: true, TLava: true, TDeepWater: true, TWater: true,
	TTree: true, TChestOpen: true, TGrave: true,
}

func (g *Game) tileNote(t Tile) string {
	if !tileNotes[t] {
		return ""
	}
	return g.L.T("tile_note." + slug(tdefs[t].Name))
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
	L := g.L
	add := func(s string, c RGB) { out = append(out, hoverLine{S: s, C: c}) }

	if !lit {
		if !l.Seen[i] {
			return []hoverLine{{S: L.T("ui.hover.darkness"), C: colDim}, {S: L.T("ui.hover.unexplored"), C: colDim}}
		}
		add(capFirst(tileName(L, l.T[i])), colGray)
		add(L.T("ui.hover.remembered"), colDim)
		return out
	}

	if mx == p.X && my == p.Y {
		add(L.T("ui.hover.you"), colWhite)
		add(L.T("ui.hover.you_stats", "lvl", p.Lvl, "hp", int(p.HP), "maxhp", p.MaxHP(), "mp", int(p.MP), "maxmp", p.MaxMP()), colGray)
		add(L.T("ui.hover.torch", "r", fmt.Sprintf("%.1f", p.Torch.Radius)), C(1, .7, .4))
	}
	if m := l.MonsterAt(mx, my); m != nil {
		if len(out) > 0 {
			add("", colDim)
		}
		add(monsterNoun(L, m).Text, m.Color())
		if m.Friendly {
			add(L.T("ui.hover.townsfolk"), colGray)
		} else {
			kind := [...]string{"normal", "champion", "unique", "boss"}[mini(m.Rank, 3)]
			add(L.T("ui.hover.rank."+kind, "lvl", m.Level, "hp", maxi(0, m.HP), "maxhp", m.MaxHP), colGray)
			add(L.T("ui.hover.hits", "lo", m.MinD, "hi", m.MaxD, "armor", m.Armor), colGray)
			if len(m.Mods) > 0 {
				add(modsText(L, m), C(.45, .6, 1))
			}
			var st []string
			if m.Frozen > 0 {
				st = append(st, L.T("ui.hover.state.frozen"))
			}
			if !m.Awake {
				st = append(st, L.T("ui.hover.state.unaware"))
			}
			if m.Flee > 0 {
				st = append(st, L.T("ui.hover.state.fleeing"))
			}
			if m.T.AI == AIRanged || m.T.AI == AIOracle || m.T.AI == AIWanderer {
				st = append(st, L.T("ui.hover.state.ranged"))
			}
			if m.Light != nil {
				st = append(st, L.T("ui.hover.state.glows"))
			}
			if len(st) > 0 {
				add(capFirst(strings.Join(st, L.T("ui.list_sep"))), colCyan)
			}
		}
	}
	for _, fi := range l.ItemsAt(mx, my) {
		if len(out) > 0 {
			add("", colDim)
		}
		lines := fi.It.Lines(L, g.Rules)
		for k, ln := range lines {
			if k >= 8 {
				add("…", colDim)
				break
			}
			add(ln.S, ln.C)
			if k == len(lines)-1 && fi.It.Kind == IKEquip {
				out[len(out)-1].Detail = true // the item level and worth
			}
		}
		if fi.It.Kind == IKEquip {
			if cur := g.equippedFor(fi.It); cur != nil {
				add(L.T("ui.hover.replaces", "item", itemNoun(L, cur)), colDim)
			}
			out = append(out, hoverLine{S: L.T("ui.hover.pick_up"), C: colDim, Detail: true})
		}
	}
	if g.portalAt(mx, my) {
		if len(out) > 0 {
			add("", colDim)
		}
		add(L.T("ui.hover.portal"), colBlue)
		if l.Kind == KTown {
			add(L.T("ui.hover.leads_back", "place", g.levelName(g.Portal.Level)), colGray)
		} else {
			add(L.T("ui.hover.leads_to", "place", regionName(L, "town")), colGray)
		}
	}
	// the ground itself
	if len(out) == 0 || l.LinkAt(mx, my) != nil || tileNotes[l.T[i]] {
		if len(out) > 0 {
			add("", colDim)
		}
		t := l.T[i]
		name := capFirst(tileName(L, t))
		switch l.Decal[i] {
		case DecalCorpse:
			name += " · " + L.T("ui.hover.decal.corpse")
		case DecalBlood:
			name += " · " + L.T("ui.hover.decal.blood")
		case DecalScorch:
			name += " · " + L.T("ui.hover.decal.scorch")
		}
		col := tdefs[t].Albedo.Scale(1.3)
		if tdefs[t].Emit {
			col = tdefs[t].Emissive
		}
		add(name, col)
		if lk := l.LinkAt(mx, my); lk != nil {
			add(L.T("ui.hover.leads_to", "place", g.levelName(lk.To)), colGray)
		} else if n := g.tileNote(t); n != "" {
			add(n, colGray)
		}
	}
	return out
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
		if ln.S == "" || ln.Detail {
			continue
		}
		n := i18n.Width(ln.S) + 3
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
