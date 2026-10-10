package main

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/antonmedv/termablo/internal/i18n"
)

const (
	panelW = 32
	logH   = 6
)

var bgDark = colPanelBG.C8()

func (g *Game) Draw(s *Screen) {
	s.Clear()
	s.RTL = g.L.Lang.RTL
	if s.W < 80 || s.H < 24 {
		s.Text(1, 1, g.L.T("ui.too_small"), colWhite.C8())
		s.Text(1, 2, g.L.T("ui.too_small_now", "w", s.W, "h", s.H), colGray.C8())
		return
	}
	if g.Mode == ModeTitle {
		g.drawTitle(s)
		return
	}
	g.updateEffects()
	g.composeLight(g.time, true)
	mapW, mapH := s.W-panelW, s.H-logH
	g.drawMap(s, 0, 0, mapW, mapH)
	g.drawLog(s, 0, mapH, mapW, logH)
	g.hoverLines = g.currentHover(mapW, mapH)
	g.drawPanel(s, mapW, 0, panelW, s.H)
	g.drawHover(s, mapW, mapH)
	switch g.Mode {
	case ModeInv:
		g.drawInventory(s, mapW, mapH)
	case ModeChar:
		g.drawChar(s, mapW, mapH)
	case ModeShop:
		g.drawShop(s, mapW, mapH)
	case ModeHelp:
		g.drawHelp(s, mapW, mapH)
	case ModeMap:
		g.drawOverview(s, mapW, mapH)
	case ModeTalk:
		g.drawTalk(s, mapW, mapH)
	case ModeQuests:
		g.drawJournal(s, mapW, mapH)
	case ModeDead:
		g.drawDead(s, mapW, mapH)
	}
}

func (g *Game) camera(w, h int) (int, int) {
	l, p := g.Lv, g.P
	var cx, cy int
	if l.W <= w {
		cx = -(w - l.W) / 2
	} else {
		cx = clampi(p.X-w/2, 0, l.W-w)
	}
	if l.H <= h {
		cy = -(h - l.H) / 2
	} else {
		cy = clampi(p.Y-h/2, 0, l.H-h)
	}
	return cx, cy
}

func gammaRGB(c RGB) RGB {
	f := func(x float32) float32 {
		if x <= 0 {
			return 0
		}
		return float32(math.Pow(float64(x), 0.75))
	}
	return RGB{f(c.R), f(c.G), f(c.B)}
}

// litAt reports whether a map cell is currently seen and returns its tone.
func (g *Game) litAt(i int) (bool, RGB) {
	if g.fov[i] != g.fovGen {
		return false, RGB{}
	}
	L := g.light[i]
	if !g.vis[i] && L.Max() <= visThresh {
		return false, RGB{}
	}
	return true, gammaRGB(toneRGB(L))
}

func (g *Game) drawMap(s *Screen, x0, y0, w, h int) {
	l := g.Lv
	camX, camY := g.camera(w, h)
	t := g.time
	for sy := range h {
		my := camY + sy
		for sx := range w {
			mx := camX + sx
			if !l.In(mx, my) {
				continue
			}
			i := my*l.W + mx
			tile := l.T[i]
			td := &tdefs[tile]
			v := l.V[i]
			glyph := td.Glyphs[int(v)%len(td.Glyphs)]
			lit, tl := g.litAt(i)
			if !lit {
				if l.Seen[i] {
					alb := td.Albedo.Lum()
					mem := C(.07, .08, .12).Scale(0.5 + alb)
					if td.Solid {
						mem = C(.1, .11, .16).Scale(0.6 + alb)
					}
					if td.Anim {
						glyph = td.Glyphs[0]
					}
					s.Set(x0+sx, y0+sy, glyph, mem.C8(), colBlack)
				}
				continue
			}
			if td.Anim {
				k := int(t*1.3+float64(v)/50+float64(mx)*0.21+float64(my)*0.13) % len(td.Glyphs)
				glyph = td.Glyphs[k]
			}
			jit := 0.86 + float32(v)/255*0.28
			alb := td.Albedo.Scale(jit)
			lightC := alb.Mul(tl)
			fg := lightC.Scale(1.6)
			bg := lightC.Scale(td.BG)
			if td.Emit {
				if drinks(g.dark, mx, my) || (l.Kind == KTown && g.snuffedAt(mx, my)) {
					// put out: cold ash in his red light
					fg = C(.3, .27, .26).Mul(tl).Scale(1.6)
					bg = RGB{}
				} else {
					f := float32(0.88 + 0.12*math.Sin(t*6+float64(v)))
					fg = td.Emissive.Scale(f).Add(tl.Scale(.1))
					bg = td.Emissive.Scale(td.BG * 0.35 * f)
				}
			}
			if td.Anim && tile != TLava {
				// water glints where it is lit
				sh := float32(0.85 + 0.25*math.Sin(t*2.2+float64(mx)*0.7+float64(my)*1.3))
				fg = fg.Scale(sh)
			}
			switch l.Decal[i] {
			case DecalBlood:
				fg = fg.Lerp(C(.7, .06, .05).Mul(tl).Scale(1.5), .75)
				bg = bg.Add(C(.06, 0, 0).Mul(tl))
			case DecalCorpse:
				glyph = '%'
				fg = C(.65, .22, .16).Mul(tl).Scale(1.5)
			case DecalScorch:
				fg = fg.Scale(.4).Add(C(.12, .05, 0).Mul(tl))
				bg = bg.Scale(.3)
			}
			s.Set(x0+sx, y0+sy, glyph, fg.C8(), bg.C8())
		}
	}
	put := func(mx, my int, ch rune, fg RGB, bgOverride *RGB, bold bool) {
		sx, sy := mx-camX, my-camY
		if sx < 0 || sy < 0 || sx >= w || sy >= h {
			return
		}
		c := &s.C[(y0+sy)*s.W+x0+sx]
		c.Ch, c.FG, c.Bold = ch, fg.C8(), bold
		if bgOverride != nil {
			c.BG = bgOverride.C8()
		}
	}
	entity := func(base RGB, tl RGB) RGB {
		b := clampf(tl.Max()*1.4, 0, 1)
		return base.Scale(0.5+0.6*b).Lerp(base.Mul(tl).Scale(1.6), 0.2)
	}
	// Portals
	portalGlyph := func(x, y int) {
		i := l.Idx(x, y)
		if lit, _ := g.litAt(i); lit {
			f := float32(0.75 + 0.25*math.Sin(t*4))
			bg := C(.05, .1, .35).Scale(f)
			put(x, y, 'Ω', C(.55, .75, 1).Scale(f+.2), &bg, true)
		}
	}
	if g.Portal != nil {
		if g.Portal.Level == l.ID {
			portalGlyph(g.Portal.X, g.Portal.Y)
		}
		if l.Kind == KTown {
			portalGlyph(l.PortalAt.X, l.PortalAt.Y)
		}
	}
	// Items
	for _, fi := range l.Items {
		i := l.Idx(fi.X, fi.Y)
		if lit, tl := g.litAt(i); lit {
			c := fi.It.Color()
			fg := c.Scale(0.55 + 0.6*clampf(tl.Max()*1.5, 0, 1))
			if fi.It.Kind == IKEquip && fi.It.Rarity >= RRare {
				fg = c.Scale(float32(0.9 + 0.2*math.Sin(t*3+float64(fi.X))))
			}
			put(fi.X, fi.Y, fi.It.Glyph(), fg, nil, fi.It.Rarity >= RMagic)
		}
	}
	// Monsters
	for _, m := range l.Monsters {
		if m.Dead {
			continue
		}
		i := l.Idx(m.X, m.Y)
		lit, tl := g.litAt(i)
		if !lit {
			continue
		}
		col := entity(m.Color(), tl)
		if m.Light != nil {
			col = m.Color()
		}
		if m.Frozen > 0 {
			col = col.Lerp(C(.55, .85, 1), .6)
		}
		var bgp *RGB
		if t-m.Flash < 0.16 {
			b := C(.55, .08, .05)
			bgp = &b
		} else if m == g.Target {
			b := C(.2, .05, .04)
			bgp = &b
		}
		put(m.X, m.Y, m.T.Glyph, col, bgp, m.Rank > RankNormal || m.Friendly)
	}
	// Player
	p := g.P
	pc := C(1, .96, .88)
	var pbg *RGB
	if t-p.Flash < 0.2 {
		b := C(.6, .05, .05)
		pbg = &b
	}
	put(p.X, p.Y, '@', pc, pbg, true)
	// Effects
	for _, e := range g.Effects {
		switch e.Kind {
		case EffBolt:
			bp := e.boltPos(t)
			bg := e.Col.Scale(.25)
			put(bp.X, bp.Y, e.Glyph, e.Col.Scale(1.2), &bg, true)
		case EffNova:
			pr := e.progress(t)
			r := e.R * pr
			ir := int(math.Ceil(e.R * Aspect))
			for dy := -int(e.R) - 1; dy <= int(e.R)+1; dy++ {
				for dx := -ir - 1; dx <= ir+1; dx++ {
					x, y := e.X+dx, e.Y+dy
					if !l.In(x, y) || g.fov[l.Idx(x, y)] != g.fovGen {
						continue
					}
					d := math.Sqrt(float64(dx*dx)/(Aspect*Aspect) + float64(dy*dy))
					if math.Abs(d-r) < 0.55 && !l.Opaque(x, y) {
						ch := '*'
						if (dx+dy)%2 == 0 {
							ch = '·'
						}
						put(x, y, ch, e.Col.Scale(float32(1.2-pr*0.6)), nil, true)
					}
				}
			}
		case EffText:
			pr := e.progress(t)
			txt := fit(e.Text, w)
			tw := i18n.Width(txt)
			sx := clampi(e.X-camX-tw/2, 0, w-tw) // kept on the map
			sy := e.Y - camY - 1 - int(pr*1.8) - e.Row
			if sy < 0 || sy >= h {
				continue
			}
			s.TextBold(x0+sx, y0+sy, txt, e.Col.Scale(float32(1.1-pr*0.7)).C8())
		}
	}
}

// ------------------------------------------------------------ panel & log

// bar draws frac of w cells solid; pend more (a potion still working)
// as a pale ghost segment after them.
func bar(s *Screen, x, y, w int, frac, pend float64, full, empty RGB) {
	frac = math.Max(0, math.Min(1, frac))
	fill := frac * float64(w)
	ghost := math.Min(1, frac+math.Max(0, pend)) * float64(w)
	for i := range w {
		c := empty
		ch := '░'
		switch {
		case float64(i)+1 <= fill:
			k := float32(i) / float32(w)
			c = full.Scale(0.75 + 0.35*k)
			ch = '█'
		case float64(i) < fill:
			c = full.Scale(0.7)
			ch = '▓'
		case float64(i) < ghost:
			c = full.Scale(0.45).Add(C(.18, .18, .18))
			ch = '▒'
		}
		s.Put(x+i, y, ch, c.C8())
	}
}

func (g *Game) drawPanel(s *Screen, x, y, w, h int) {
	s.Fill(x, y, w, h, bgDark)
	for yy := range h {
		s.Set(x, yy, '│', colBorder.C8(), bgDark)
	}
	p, l, L := g.P, g.Lv, g.L
	cx := x + 2
	bw := w - 4
	row := 1
	sp := 2 // blank rows between groups; a short terminal gets one
	if h < 30 {
		sp = 1
	}
	title := "T E R M A B L O"
	for i, r := range title {
		f := float32(0.8 + 0.2*math.Sin(g.time*3+float64(i)*0.5))
		s.TextBold(cx+i, row, string(r), C(1, .55, .2).Scale(f).C8())
	}
	row += 2
	s.TextBold(cx, row, fit(areaName(L, l), bw), colGold.C8())
	row++
	area, areaCol := L.T("ui.panel.safe"), colDim
	if l.Kind != KTown {
		area = L.T("ui.panel.area_level", "n", l.Depth)
	} else if g.townHunted() {
		area, areaCol = L.T("ui.panel.not_safe"), colRed
	}
	s.Text(cx, row, fit(area, bw), areaCol.C8())
	row += sp
	s.Text(cx, row, fit(L.T("ui.panel.hero", "n", p.Lvl), bw-5), colWhite.C8())
	if p.Points > 0 {
		pts := fmt.Sprintf("+%d", p.Points)
		s.TextBold(cx+bw-i18n.Width(pts), row, pts, colGold.C8())
	}
	row++
	s.Text(cx, row, fit(L.T("ui.panel.life"), bw-12), C(.9, .4, .35).C8())
	s.Text(cx+bw-12, row, fmt.Sprintf("%12s", fmt.Sprintf("%d/%d", int(p.HP), p.MaxHP())), colWhite.C8())
	row++
	bar(s, cx, row, bw, p.HP/float64(p.MaxHP()), p.HealPool/float64(p.MaxHP()), C(.85, .12, .1), C(.2, .05, .05))
	row++
	s.Text(cx, row, fit(L.T("ui.panel.mana"), bw-12), C(.45, .6, 1).C8())
	s.Text(cx+bw-12, row, fmt.Sprintf("%12s", fmt.Sprintf("%d/%d", int(p.MP), p.MaxMP())), colWhite.C8())
	row++
	bar(s, cx, row, bw, p.MP/float64(p.MaxMP()), p.ManaPool/float64(p.MaxMP()), C(.2, .35, .95), C(.05, .07, .2))
	row++
	s.Text(cx, row, fit(L.T("ui.panel.experience"), bw), C(.8, .7, .4).C8())
	row++
	bar(s, cx, row, bw, float64(p.XP)/float64(g.Rules.xpNext(p.Lvl)), 0, C(.85, .7, .3), C(.15, .12, .05))
	row += sp
	// the belt: a well per potion slot, the scroll beside the gold
	s.Text(cx, row, fit(L.T("ui.panel.gold", "n", p.Gold), 13), colGold.C8())
	g.beltHit[2] = g.beltRow(s, cx+14, row, bw-14, "t", '?', p.Scrolls, 1, C(.85, .8, .65), L.T("ui.panel.belt_portal", "n", p.Scrolls))
	row++
	g.beltHit[0] = g.beltRow(s, cx, row, bw, "q", '!', p.HPot, beltMax, C(1, .25, .25), L.T("ui.panel.belt_heal"))
	row++
	g.beltHit[1] = g.beltRow(s, cx, row, bw, "w", '!', p.MPot, beltMax, C(.35, .5, 1), L.T("ui.panel.belt_mana"))
	row += sp
	lo, hi := p.DmgRange()
	s.Text(cx, row, fit(L.T("ui.panel.damage", "lo", lo, "hi", hi), 14), colGray.C8())
	s.Text(cx+15, row, fit(L.T("ui.panel.armor", "n", p.ArmorVal()), bw-15), colGray.C8())
	row++
	s.Text(cx, row, fit(L.T("ui.panel.crit", "n", p.Crit()), 14), colGray.C8())
	s.Text(cx+15, row, fit(L.T("ui.panel.light", "r", fmt.Sprintf("%.1f", p.Torch.Radius)), bw-15), C(1, .7, .4).C8())
	row += sp
	s.Text(cx, row, fit(L.T("ui.panel.skills"), bw), colDim.C8())
	row++
	fl, fh := p.FireboltDmg()
	skc := func(cost int) Col8 {
		if p.MP >= float64(cost) {
			return colWhite.C8()
		}
		return colDim.C8()
	}
	s.TextBold(cx, row, "f", colOrange.C8())
	s.Text(cx+2, row, fit(L.T("ui.panel.firebolt", "lo", fl, "hi", fh), bw-8), skc(p.FireboltCost()))
	s.TextRight(cx, row, bw, L.T("ui.panel.mp", "n", p.FireboltCost()), C(.45, .6, 1).C8())
	row++
	nl, nh := p.NovaDmg()
	s.TextBold(cx, row, "r", colCyan.C8())
	s.Text(cx+2, row, fit(L.T("ui.panel.nova", "lo", nl, "hi", nh), bw-8), skc(p.NovaCost()))
	s.TextRight(cx, row, bw, L.T("ui.panel.mp", "n", p.NovaCost()), C(.45, .6, 1).C8())
	row += sp
	// target
	if row < h-10 {
		t := g.Target
		if t == nil || t.Dead || !g.canSee(t.X, t.Y) {
			t = g.nearestHostile()
		}
		if t != nil {
			s.TextBold(cx, row, fit(monsterNoun(L, t).Text, bw), t.Color().C8())
			row++
			bar(s, cx, row, bw-8, float64(t.HP)/float64(t.MaxHP), 0, C(.7, .1, .1), C(.15, .04, .04))
			s.TextRight(cx+bw-7, row, 7, fit(L.T("ui.panel.target_lvl", "n", t.Level), 7), colDim.C8())
			row++
			if len(t.Mods) > 0 {
				s.Text(cx, row, fit(modsText(L, t), bw), C(.45, .6, 1).C8())
				row++
			}
			if t.Frozen > 0 {
				s.Text(cx, row, fit(L.T("ui.panel.frozen"), bw), colCyan.C8())
				row++
			}
		} else {
			s.Text(cx, row, fit(L.T("ui.panel.no_enemies"), bw), colDim.C8())
			row++
		}
	}
	// the quests taken, none until someone gives one; a click, or J,
	// opens the journal
	qrow := h - 6
	g.questsHit, g.questsRows = hitBox{}, 0
	if shown := g.questsShown(3); qrow > row && len(shown) > 0 {
		s.TextBold(cx, qrow, "J", colDim.C8())
		s.Text(cx+2, qrow, fit(L.T("ui.panel.quests"), bw-2), colDim.C8())
		for i, q := range shown {
			mark, col := g.questMark(q)
			s.Text(cx, qrow+1+i, fit(mark+" "+L.T(q.nameKey()), bw), col.C8())
		}
		g.questsHit, g.questsRows = hitBox{cx, cx + bw - 1, qrow}, len(shown)
	}
	s.Text(cx, h-2, fit(L.T("ui.panel.keys"), bw), colDim.C8())
}

// fit cuts s to at most w cells.
func fit(s string, w int) string { return i18n.Truncate(s, max(0, w)) }

// beltRow draws one belt line: the hotkey, slot wells holding a glyph per
// item left, and a right-aligned label. It returns the row's clickable
// area. A drink still working shows on the Life/Mana bar, not here.
func (g *Game) beltRow(s *Screen, x, y, w int, key string, glyph rune, n, slots int, col RGB, label string) hitBox {
	n = mini(n, slots)
	keyCol, labCol := col, col.Scale(.85)
	if n == 0 {
		keyCol, labCol = colDim, colDim
	}
	s.TextBold(x, y, key, keyCol.C8())
	empty, full := C(.09, .085, .09), col.Scale(.2).Add(C(.03, .03, .03))
	for i := range slots {
		wx := x + 2 + i*4
		bg, ch, fg := empty, '·', C(.24, .23, .27)
		if i < n {
			bg, ch, fg = full, glyph, col
		}
		for k := range 3 {
			s.Set(wx+k, y, ' ', bg.C8(), bg.C8())
		}
		s.SetBold(wx+1, y, ch, fg.C8(), bg.C8(), i < n)
	}
	label = fit(label, w-1-slots*4) // the cells after the last well
	s.TextRight(x, y, w, label, labCol.C8())
	return hitBox{x, x + w - 1, y}
}

func (g *Game) drawLog(s *Screen, x, y, w, h int) {
	s.Fill(x, y, w, h, bgDark)
	for xx := x; xx < x+w; xx++ {
		s.Set(xx, y, '─', colBorder.C8(), bgDark)
	}
	lines := h - 1
	start := len(g.Log) - lines
	if start < 0 {
		start = 0
	}
	for i, m := range g.Log[start:] {
		age := g.Turn - m.Turn
		f := float32(1)
		if age > 0 {
			f = float32(math.Max(0.45, 1-float64(age)*0.08))
		}
		txt := m.Text
		if m.N > 1 {
			txt += g.L.T("ui.log.repeat", "n", m.N)
		}
		s.TextStart(x+1, y+1+i, w-2, fit(txt, w-2), m.Col.Scale(f).C8())
	}
}

// ------------------------------------------------------------ overlays

func centerBox(s *Screen, mapW, mapH, w, h int, title string) (int, int) {
	w = mini(w, mapW-2)
	h = mini(h, s.H-2)
	x := (mapW - w) / 2
	y := maxi(0, (mapH-h)/2)
	if y+h > s.H {
		y = s.H - h
	}
	s.Box(x, y, w, h, colBorder.C8(), C(.03, .025, .025).C8(), title, colGold.C8())
	return x, y
}

// scrollHints marks a list that runs past its rows: how many are above,
// right-aligned on the row over the list, and how many below, set into
// the row under it. Both end at x+w.
func (g *Game) scrollHints(s *Screen, x, w, top, bottom, above, below int) {
	if above > 0 {
		s.TextRight(x, top, w, g.L.T("ui.more_above", "n", above), colDim.C8())
	}
	if below > 0 {
		s.TextRight(x, bottom, w, " "+g.L.T("ui.more_below", "n", below)+" ", colDim.C8())
	}
}

func (g *Game) itemLines(s *Screen, x, y, w, maxRows int, it *Item) {
	row := 0
	for _, ln := range it.Lines(g.L, g.Rules) {
		for _, txt := range i18n.Wrap(ln.S, w) { // long flavor text wraps
			if row >= maxRows {
				return
			}
			s.TextStart(x, y+row, w, txt, ln.C.C8())
			row++
		}
	}
}

func (g *Game) drawInventory(s *Screen, mapW, mapH int) {
	p, L := g.P, g.L
	bw, bh := 84, mini(s.H-2, 34)
	x, y := centerBox(s, mapW, mapH, bw, bh, L.T("ui.inv.title"))
	bw = mini(bw, mapW-2)
	// two columns, equipped and pack; the names split what the width
	// leaves, the longest base name whole when there is room
	labW := 6
	for _, n := range eqNames {
		labW = max(labW, i18n.Width(L.T("slot."+slug(n))))
	}
	labW = min(labW, 12) + 1
	eqW := mini(17, (bw-12-labW)/2+1)
	packW := bw - 12 - labW - eqW
	colL := x + 2
	colR := colL + labW + eqW + 2
	hdr := func(xx int, t string, active bool) {
		c := colDim
		if active {
			c = colGold
		}
		s.TextBold(xx, y+1, t, c.C8())
	}
	hdr(colL, fit(L.T("ui.inv.equipped"), labW+eqW), g.pane == 0)
	hdr(colR, fit(L.T("ui.inv.pack", "n", len(p.Inv), "max", invMax), packW), g.pane == 1)
	sel := func(xx, yy, w int) {
		for i := range w {
			if s.in(xx+i, yy) {
				s.C[yy*s.W+xx+i].BG = C(.18, .1, .05).C8()
			}
		}
	}
	for i := range EqCount {
		yy := y + 3 + i
		if g.pane == 0 && g.cur == i {
			sel(colL-1, yy, labW+eqW+1)
		}
		s.Text(colL, yy, fit(L.T("slot."+slug(eqNames[i])), labW-1), colDim.C8())
		if it := p.Eq[i]; it != nil {
			s.Text(colL+labW, yy, fit(iname(L, it), eqW), it.Color().C8())
		} else {
			s.Text(colL+labW, yy, "—", colDim.C8())
		}
	}
	// the pack runs down to the divider over the details
	dy := maxi(y+14, y+bh-12)
	maxRows := dy - 1 - (y + 3)
	off := 0
	if g.pane == 1 && g.cur >= maxRows {
		off = g.cur - maxRows + 1
	}
	for i := off; i < len(p.Inv) && i-off < maxRows; i++ {
		it := p.Inv[i]
		yy := y + 3 + i - off
		if g.pane == 1 && g.cur == i {
			sel(colR-1, yy, x+bw-colR)
		}
		s.Put(colR, yy, it.Glyph(), it.Color().C8())
		s.Text(colR+2, yy, fit(iname(L, it), packW), it.Color().C8())
		if up := g.upgradeHint(it); up != "" {
			s.Text(x+bw-4, yy, up, colGreen.C8())
		}
	}
	if len(p.Inv) == 0 {
		s.Text(colR, y+3, fit(L.T("ui.inv.empty"), x+bw-2-colR), colDim.C8())
	}
	// details
	detRows := y + bh - 1 - dy
	for xx := x + 1; xx < x+bw-1; xx++ {
		s.Put(xx, dy-1, '─', colBorder.Scale(.6).C8())
	}
	g.scrollHints(s, colR, x+bw-2-colR, y+2, dy-1, off, len(p.Inv)-off-maxRows)
	var it *Item
	if g.pane == 0 {
		it = p.Eq[clampi(g.cur, 0, EqCount-1)]
	} else if g.cur < len(p.Inv) {
		it = p.Inv[g.cur]
	}
	half := (bw - 6) / 2
	if it != nil {
		g.itemLines(s, colL, dy, half, detRows, it)
		if g.pane == 1 {
			if cur := g.equippedFor(it); cur != nil {
				s.Text(colL+half+2, dy, fit(L.T("ui.currently_equipped"), half), colDim.C8())
				g.itemLines(s, colL+half+2, dy+1, half, detRows-1, cur)
			}
		}
	}
	s.Text(colL, y+bh-1, fit(" "+L.T("ui.inv.keys")+" ", bw-4), colDim.C8())
}

func (g *Game) upgradeHint(it *Item) string {
	cur := g.equippedFor(it)
	if it.Kind != IKEquip {
		return ""
	}
	if cur == nil {
		return fit(g.L.T("ui.inv.new"), 3)
	}
	if it.Slot() == SlotWeapon && it.MaxD+it.MinD > cur.MaxD+cur.MinD {
		return "▲"
	}
	if it.Armor > cur.Armor+2 && it.Slot() != SlotWeapon {
		return "▲"
	}
	return ""
}

func (g *Game) drawChar(s *Screen, mapW, mapH int) {
	p, L := g.P, g.L
	const bw = 56
	x, y := centerBox(s, mapW, mapH, bw, 24, L.T("ui.char.title"))
	cx := x + 3
	s.TextBold(cx, y+2, fit(L.T("ui.char.hero", "n", p.Lvl), bw-6), colWhite.C8())
	s.Text(cx, y+3, fit(L.T("ui.char.experience", "xp", p.XP, "next", g.Rules.xpNext(p.Lvl)), bw-6), colDim.C8())
	attrs := []struct {
		k    string
		key  string
		base int
		tot  int
	}{
		{"1", "str", p.Str, p.STR()},
		{"2", "dex", p.Dex, p.DEX()},
		{"3", "vit", p.Vit, p.VIT()},
		{"4", "ene", p.Ene, p.ENE()},
	}
	nameW := 10
	for _, a := range attrs {
		nameW = max(nameW, i18n.Width(L.T("ui.char.attr."+a.key)))
	}
	nameW = min(nameW, 14)
	totX := cx + 4 + nameW + 1
	descX := totX + 3 + 7
	for i, a := range attrs {
		yy := y + 5 + i
		c := colWhite
		if p.Points > 0 {
			s.TextBold(cx, yy, "["+a.k+"]", colGold.C8())
		}
		s.Text(cx+4, yy, fit(L.T("ui.char.attr."+a.key), nameW), c.C8())
		s.Text(totX, yy, fmt.Sprintf("%3d", a.tot), c.C8())
		if a.tot != a.base {
			s.Text(totX+4, yy, fmt.Sprintf("(%d)", a.base), colDim.C8())
		}
		s.Text(descX, yy, fit(L.T("ui.char.attr_desc."+a.key), x+bw-2-descX), colDim.C8())
	}
	if p.Points > 0 {
		s.TextBold(cx, y+10, fit(L.T("ui.char.points", "n", p.Points), bw-6), colGold.C8())
	}
	lo, hi := p.DmgRange()
	fl, fh := p.FireboltDmg()
	stats := []struct{ key, val string }{
		{"life", strconv.Itoa(p.MaxHP())},
		{"mana", strconv.Itoa(p.MaxMP())},
		{"damage", fmt.Sprintf("%d-%d", lo, hi)},
		{"firebolt", fmt.Sprintf("%d-%d", fl, fh)},
		{"armor", strconv.Itoa(p.ArmorVal())},
		{"attack", strconv.Itoa(p.ToHit())},
		{"crit", fmt.Sprintf("%d%%", p.Crit())},
		{"life_steal", fmt.Sprintf("%d%%", p.S(StLifeSteal))},
		{"magic_find", fmt.Sprintf("%d%%", p.S(StMF))},
		{"kills", strconv.Itoa(p.Kills)},
	}
	labW := 11
	for _, st := range stats {
		labW = max(labW, i18n.Width(L.T("ui.char.stat."+st.key)))
	}
	labW = min(labW, 17) + 1
	for i, st := range stats {
		col := cx
		row := y + 12 + i%5
		if i >= 5 {
			col = cx + 26
		}
		s.Text(col, row, fit(L.T("ui.char.stat."+st.key), labW-1), colGray.C8())
		s.Text(col+labW, row, st.val, colGray.C8())
	}
	s.Text(cx, y+22, fit(L.T("ui.esc_close"), bw-6), colDim.C8())
}

func (g *Game) shopList() []*Item {
	if g.tab == 0 {
		return slices.Concat(g.shop.Items, g.shop.Services)
	}
	return g.P.Inv
}

func (g *Game) drawShop(s *Screen, mapW, mapH int) {
	p, L := g.P, g.L
	bw, bh := 80, mini(s.H-2, 32)
	x, y := centerBox(s, mapW, mapH, bw, bh, L.T([...]string{"shop.smith", "shop.alch"}[g.shop.Kind]))
	bw = mini(bw, mapW-2)
	cx := x + 2
	tabs := []string{" " + L.T("ui.shop.buy") + " ", " " + L.T("ui.shop.sell") + " "}
	tx := cx
	for i, t := range tabs {
		c, bg := colDim, C(.03, .025, .025)
		if g.tab == i {
			c, bg = colGold, C(.2, .1, .04)
		}
		tw := i18n.Width(t)
		s.Fill(tx, y+1, tw, 1, bg.C8())
		s.TextBold(tx, y+1, t, c.C8())
		tx += tw + 1
	}
	s.TextRight(tx, y+1, x+bw-2-tx, L.T("ui.shop.gold", "n", p.Gold), colGold.C8())
	items := g.shopList()
	maxRows := bh - 16
	off := 0
	if g.cur >= maxRows {
		off = g.cur - maxRows + 1
	}
	for i := off; i < len(items) && i-off < maxRows; i++ {
		it := items[i]
		yy := y + 3 + i - off
		if i == g.cur {
			for k := 1; k < bw-1; k++ {
				s.C[yy*s.W+x+k].BG = C(.18, .1, .05).C8()
			}
		}
		s.Put(cx, yy, it.Glyph(), it.Color().C8())
		s.Text(cx+2, yy, fit(iname(L, it), bw-24), it.Color().C8())
		price := g.buyPrice(it)
		if g.tab == 1 {
			price = sellPrice(it, g.Rules)
		}
		pc := colGold
		if g.tab == 0 && price > p.Gold {
			pc = colRed
		}
		s.Text(x+bw-10, yy, fmt.Sprintf("%6dg", price), pc.C8())
		if up := g.upgradeHint(it); up != "" && it.Kind == IKEquip {
			s.Text(x+bw-14, yy, up, colGreen.C8())
		}
	}
	if len(items) == 0 {
		s.Text(cx, y+3, fit(L.T("ui.shop.empty"), bw-4), colDim.C8())
	}
	dy := y + bh - 12
	for xx := x + 1; xx < x+bw-1; xx++ {
		s.Put(xx, dy-1, '─', colBorder.Scale(.6).C8())
	}
	g.scrollHints(s, cx, bw-4, y+2, dy-1, off, len(items)-off-maxRows)
	if g.cur < len(items) {
		it := items[g.cur]
		half := (bw - 6) / 2
		g.itemLines(s, cx, dy, half, 10, it)
		if cur := g.equippedFor(it); cur != nil && it.Kind == IKEquip {
			s.Text(cx+half+2, dy, fit(L.T("ui.currently_equipped"), half), colDim.C8())
			g.itemLines(s, cx+half+2, dy+1, half, 9, cur)
		}
	}
	s.Text(cx, y+bh-1, fit(" "+L.T("ui.shop.keys")+" ", bw-4), colDim.C8())
}

func (g *Game) drawHelp(s *Screen, mapW, mapH int) {
	L := g.L
	// lay the text out for the width at hand, then show the rows that fit
	type hline struct {
		k, d string
		dx   int
		c    RGB
	}
	bw := mini(78, mapW-2)
	keyW := 16
	for _, k := range helpKeys {
		keyW = max(keyW, i18n.Width(L.T("help.key."+k)))
	}
	keyW = min(keyW, 26)
	dx := 3 + keyW + 2
	var rows []hline
	for _, k := range helpKeys {
		for i, d := range i18n.Wrap(L.T("help.does."+k), bw-dx-2) {
			key := ""
			if i == 0 {
				key = fit(L.T("help.key."+k), keyW)
			}
			rows = append(rows, hline{key, d, dx, colGray})
		}
	}
	rows = append(rows, hline{})
	for _, t := range helpTips {
		for i, d := range i18n.Wrap(L.T("help.tip."+t), bw-7) {
			pre := "· "
			if i > 0 {
				pre = "  "
			}
			rows = append(rows, hline{"", pre + d, 3, colLore})
		}
	}
	bh := mini(len(rows)+5, s.H-2)
	x, y := centerBox(s, mapW, mapH, bw, bh, L.T("ui.help.title"))
	show := bh - 4
	g.helpOff = clampi(g.helpOff, 0, maxi(0, len(rows)-show))
	for i, l := range rows[g.helpOff:mini(len(rows), g.helpOff+show)] {
		s.TextBold(x+3, y+2+i, l.k, colOrange.C8())
		s.Text(x+l.dx, y+2+i, l.d, l.c.C8())
	}
	foot := L.T("ui.esc_close")
	if len(rows) > show {
		foot += " · " + L.T("ui.help.scroll")
	}
	s.Text(x+3, y+bh-2, foot, colDim.C8())
	g.scrollHints(s, x, bw-2, y+1, y+bh-2, g.helpOff, len(rows)-show-g.helpOff)
}

// helpKeys and helpTips are the help screen's rows: help.key.<k> and
// help.does.<k>, help.tip.<t>.
var helpKeys = []string{"move", "wait", "pickup", "firebolt", "nova", "target", "potions", "portal", "explore", "inventory", "character", "map", "quests", "quit"}
var helpTips = []string{"walk_into", "talk", "darkness", "names", "glow", "upgrade", "hover", "belt"}

func (g *Game) drawDead(s *Screen, mapW, mapH int) {
	const w = 50
	x, y := centerBox(s, mapW, mapH, w, 11, "")
	f := float32(0.75 + 0.25*math.Sin(g.time*2))
	L, p := g.L, g.P
	center := func(row int, str string, col Col8, bold bool) {
		str = fit(str, w-4)
		if bold {
			s.TextBold(x+(w-i18n.Width(str))/2, row, str, col)
		} else {
			s.Text(x+(w-i18n.Width(str))/2, row, str, col)
		}
	}
	center(y+2, L.T("ui.dead.title"), C(.9, .1, .08).Scale(f).C8(), true)
	killer := g.killer
	if killer == "" {
		killer = p.KilledBy
	}
	lines := []string{
		L.T("ui.dead.slain_by", "who", killer),
		L.T("ui.dead.summary", "lvl", p.Lvl, "kills", p.Kills, "gold", p.Gold),
		L.T("ui.dead.where", "area", areaName(L, g.Lv)),
	}
	for i, l := range lines {
		center(y+4+i, l, colGray.C8(), false)
	}
	center(y+8, L.T("ui.dead.keys"), colOrange.C8(), false)
}

func (g *Game) drawOverview(s *Screen, mapW, mapH int) {
	l := g.Lv
	aw, ah := mapW-4, mapH-4
	sx := int(math.Ceil(float64(l.W) / float64(aw)))
	sy := int(math.Ceil(float64(l.H) / float64(ah)))
	sc := maxi(sx, sy)
	if sc < 1 {
		sc = 1
	}
	w, h := (l.W+sc-1)/sc+4, (l.H+sc-1)/sc+3
	x, y := centerBox(s, mapW, mapH, w, h, areaName(g.L, l))
	for my := 0; my < l.H; my += sc {
		for mx := 0; mx < l.W; mx += sc {
			var best Tile
			pri := -1
			for dy := range sc {
				for dx := range sc {
					xx, yy := mx+dx, my+dy
					if !l.In(xx, yy) || !l.Seen[l.Idx(xx, yy)] {
						continue
					}
					t := l.At(xx, yy)
					pr := 1
					switch {
					case l.LinkAt(xx, yy) != nil:
						pr = 5
					case tdefs[t].Light != nil:
						pr = 4
					case tdefs[t].Solid:
						pr = 2
					case tdefs[t].BlockMove:
						pr = 3
					}
					if pr > pri {
						pri, best = pr, t
					}
				}
			}
			if pri < 0 {
				continue
			}
			td := tdefs[best]
			ch := td.Glyphs[0]
			c := td.Albedo.Scale(.55)
			if td.Emit {
				c = td.Emissive
			}
			if best == TFloor || best == TCaveFloor || best == TGrass || best == TDirt || best == TMud || best == TRoad {
				ch = '·'
			}
			s.Put(x+2+mx/sc, y+1+my/sc, ch, c.C8())
		}
	}
	if g.Portal != nil && g.Portal.Level == l.ID {
		s.Put(x+2+g.Portal.X/sc, y+1+g.Portal.Y/sc, 'Ω', colBlue.C8())
	}
	f := float32(0.7 + 0.3*math.Sin(g.time*6))
	s.SetBold(x+2+g.P.X/sc, y+1+g.P.Y/sc, '@', C(1, 1, .6).Scale(f).C8(), C(.3, .2, 0).C8(), true)
}

// ------------------------------------------------------------ title

var logo = []string{
	"▀█▀ █▀▀ █▀█ █▀▄▀█ ▄▀█ █▄▄ █   █▀█",
	" █  ██▄ █▀▄ █ ▀ █ █▀█ █▄█ █▄▄ █▄█",
}

func (g *Game) drawTitle(s *Screen) {
	t := g.time
	cx, cy := s.W/2, s.H/2-3
	// two braziers light the title; everything else is black
	lights := []struct {
		x, y float64
		c    RGB
		seed float64
	}{
		{float64(cx - 24), float64(cy + 1), C(1, .5, .18), 1},
		{float64(cx + 24), float64(cy + 1), C(1, .5, .18), 2.7},
		{float64(cx), float64(cy + 9), C(.3, .55, 1), 5},
	}
	lightAt := func(x, y int) RGB {
		var L RGB
		for _, lt := range lights {
			dx, dy := (float64(x)-lt.x)/Aspect, float64(y)-lt.y
			d := math.Sqrt(dx*dx + dy*dy)
			r := 14.0
			if lt.seed == 5 {
				r = 8
			}
			k := 1 - d/r
			if k <= 0 {
				continue
			}
			f := 1 + 0.2*(0.6*math.Sin(t*7.3+lt.seed)+0.4*math.Sin(t*13.1+lt.seed*2))
			L = L.Add(lt.c.Scale(float32(k * k * f)))
		}
		return L
	}
	// ground
	for y := cy + 2; y < s.H; y++ {
		for x := range s.W {
			L := lightAt(x, y)
			if L.Max() < 0.02 {
				continue
			}
			tl := gammaRGB(toneRGB(L))
			h := hash2(int32(x), int32(y), 9)
			ch := []rune("··.,·°")[h%6]
			s.Set(x, y, ch, C(.45, .42, .38).Mul(tl).Scale(1.4).C8(), C(.45, .42, .38).Mul(tl).Scale(.08).C8())
		}
	}
	for _, lt := range lights[:2] {
		f := float32(0.85 + 0.15*math.Sin(t*9+lt.seed))
		s.SetBold(int(lt.x), int(lt.y), 'Ψ', C(1, .62, .22).Scale(f).C8(), C(.3, .12, 0).C8(), true)
	}
	s.SetBold(int(lights[2].x), int(lights[2].y), '♦', C(.5, .8, 1).C8(), C(.05, .1, .25).C8(), true)
	for row, line := range logo {
		rs := []rune(line)
		x0 := cx - len(rs)/2
		for i, r := range rs {
			if r == ' ' {
				continue
			}
			L := lightAt(x0+i, cy-2+row).Add(C(.12, .05, .02))
			tl := gammaRGB(toneRGB(L))
			s.SetBold(x0+i, cy-2+row, r, C(1, .8, .6).Mul(tl).Scale(1.5).C8(), colBlack, true)
		}
	}
	// embers rising from the braziers
	for k := range 14 {
		src := lights[k%2]
		ph := math.Mod(t*0.6+float64(k)*0.37, 1)
		ex := int(src.x + math.Sin(t*2+float64(k)*1.7)*3*ph)
		ey := int(src.y - ph*10)
		c := C(1, .5, .15).Scale(float32(1 - ph))
		s.Put(ex, ey, '·', c.C8())
	}
	sub := g.L.T("ui.title.subtitle")
	s.Text(cx-i18n.Width(sub)/2, cy+4, sub, C(.6, .5, .4).C8())
	g.drawLangs(s, s.H-5)
	pr := g.L.T("ui.title.begin")
	f := float32(0.55 + 0.45*math.Sin(t*3))
	s.Fill(cx-i18n.Width(pr)/2-2, s.H-3, i18n.Width(pr)+4, 1, colBlack)
	s.Text(cx-i18n.Width(pr)/2, s.H-3, pr, C(1, .7, .4).Scale(f).C8())
}

// drawLangs lays every language out on row y by its own name, the
// current one bold and underlined, and records where each one is for
// clicks. The row reads left to right whatever the language.
func (g *Game) drawLangs(s *Screen, y int) {
	const gap = 2
	w := -gap
	for _, l := range i18n.Langs {
		w += i18n.Width(l.Native) + gap
	}
	x := (s.W - w) / 2
	s.Fill(x-2, y, w+4, 2, colBlack) // a quiet band, out of the firelight
	g.langHit = g.langHit[:0]
	for _, l := range i18n.Langs {
		nw := i18n.Width(l.Native)
		if l.Code == g.L.Lang.Code {
			s.TextBold(x, y, l.Native, C(1, .8, .5).C8())
			for i := range nw {
				s.Put(x+i, y+1, '▔', C(1, .55, .2).C8())
			}
		} else {
			s.Text(x, y, l.Native, C(.5, .45, .4).C8())
		}
		g.langHit = append(g.langHit, hitBox{x, x + nw - 1, y})
		x += nw + gap
	}
}
