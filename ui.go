package main

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	panelW = 32
	logH   = 6
)

var bgDark = colPanelBG.C8()

func (g *Game) Draw(s *Screen) {
	s.Clear()
	if s.W < 80 || s.H < 24 {
		s.Text(1, 1, "Termablo needs at least 80x24.", colWhite.C8())
		s.Text(1, 2, fmt.Sprintf("Current: %dx%d", s.W, s.H), colGray.C8())
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
				if drinks(g.dark, mx, my) {
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
			sx := e.X - camX - utf8.RuneCountInString(e.Text)/2
			sy := e.Y - camY - 1 - int(pr*1.8) - e.Row
			if sy < 0 || sy >= h {
				continue
			}
			col := e.Col.Scale(float32(1.1 - pr*0.7)).C8()
			for k, r := range e.Text {
				xx := sx + k
				if xx >= 0 && xx < w {
					c := &s.C[(y0+sy)*s.W+x0+xx]
					c.Ch, c.FG, c.Bold = r, col, true
				}
			}
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
	p, l := g.P, g.Lv
	cx := x + 2
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
	s.TextBold(cx, row, l.Name, colGold.C8())
	row++
	area, areaCol := "Safe haven", colDim
	if l.Kind != KTown {
		area = fmt.Sprintf("Area level %d", l.Depth)
	} else if g.townHunted() {
		area, areaCol = "Not safe", colRed
	}
	s.Text(cx, row, area, areaCol.C8())
	row += sp
	s.Text(cx, row, fmt.Sprintf("Wanderer  ·  Level %d", p.Lvl), colWhite.C8())
	if p.Points > 0 {
		s.TextBold(cx+22, row, fmt.Sprintf("+%d", p.Points), colGold.C8())
	}
	row++
	bw := w - 4
	s.Text(cx, row, "Life", C(.9, .4, .35).C8())
	s.Text(cx+bw-12, row, fmt.Sprintf("%12s", fmt.Sprintf("%d/%d", int(p.HP), p.MaxHP())), colWhite.C8())
	row++
	bar(s, cx, row, bw, p.HP/float64(p.MaxHP()), p.HealPool/float64(p.MaxHP()), C(.85, .12, .1), C(.2, .05, .05))
	row++
	s.Text(cx, row, "Mana", C(.45, .6, 1).C8())
	s.Text(cx+bw-12, row, fmt.Sprintf("%12s", fmt.Sprintf("%d/%d", int(p.MP), p.MaxMP())), colWhite.C8())
	row++
	bar(s, cx, row, bw, p.MP/float64(p.MaxMP()), p.ManaPool/float64(p.MaxMP()), C(.2, .35, .95), C(.05, .07, .2))
	row++
	s.Text(cx, row, "Experience", C(.8, .7, .4).C8())
	row++
	bar(s, cx, row, bw, float64(p.XP)/float64(g.Rules.xpNext(p.Lvl)), 0, C(.85, .7, .3), C(.15, .12, .05))
	row += sp
	// the belt: a well per potion slot, the scroll beside the gold
	s.Text(cx, row, fmt.Sprintf("Gold %d", p.Gold), colGold.C8())
	g.beltHit[2] = g.beltRow(s, cx+14, row, bw-14, "t", '?', p.Scrolls, 1, C(.85, .8, .65), fmt.Sprintf("Portal %d", p.Scrolls))
	row++
	g.beltHit[0] = g.beltRow(s, cx, row, bw, "q", '!', p.HPot, beltMax, C(1, .25, .25), "Heal")
	row++
	g.beltHit[1] = g.beltRow(s, cx, row, bw, "w", '!', p.MPot, beltMax, C(.35, .5, 1), "Mana")
	row += sp
	lo, hi := p.DmgRange()
	s.Text(cx, row, fmt.Sprintf("Damage %d-%d", lo, hi), colGray.C8())
	s.Text(cx+15, row, fmt.Sprintf("Armor %d", p.ArmorVal()), colGray.C8())
	row++
	s.Text(cx, row, fmt.Sprintf("Crit %d%%", p.Crit()), colGray.C8())
	s.Text(cx+15, row, fmt.Sprintf("Light %.1f", p.Torch.Radius), C(1, .7, .4).C8())
	row += sp
	s.Text(cx, row, "Skills", colDim.C8())
	row++
	fl, fh := p.FireboltDmg()
	skc := func(cost int) Col8 {
		if p.MP >= float64(cost) {
			return colWhite.C8()
		}
		return colDim.C8()
	}
	s.TextBold(cx, row, "f", colOrange.C8())
	s.Text(cx+2, row, fmt.Sprintf("Firebolt %d-%d", fl, fh), skc(p.FireboltCost()))
	s.Text(cx+bw-4, row, fmt.Sprintf("%2dmp", p.FireboltCost()), C(.45, .6, 1).C8())
	row++
	nl, nh := p.NovaDmg()
	s.TextBold(cx, row, "r", colCyan.C8())
	s.Text(cx+2, row, fmt.Sprintf("Frost Nova %d-%d", nl, nh), skc(p.NovaCost()))
	s.Text(cx+bw-4, row, fmt.Sprintf("%2dmp", p.NovaCost()), C(.45, .6, 1).C8())
	row += sp
	// target
	if row < h-10 {
		t := g.Target
		if t == nil || t.Dead || !g.canSee(t.X, t.Y) {
			t = g.nearestHostile()
		}
		if t != nil {
			name := t.Name
			if len(name) > bw {
				name = name[:bw]
			}
			s.TextBold(cx, row, name, t.Color().C8())
			row++
			bar(s, cx, row, bw-8, float64(t.HP)/float64(t.MaxHP), 0, C(.7, .1, .1), C(.15, .04, .04))
			s.Text(cx+bw-7, row, fmt.Sprintf("Lv %d", t.Level), colDim.C8())
			row++
			if d := t.Describe(); d != "" {
				if len(d) > bw {
					d = d[:bw]
				}
				s.Text(cx, row, d, C(.45, .6, 1).C8())
				row++
			}
			if t.Frozen > 0 {
				s.Text(cx, row, "Frozen", colCyan.C8())
				row++
			}
		} else {
			s.Text(cx, row, "No enemies in sight", colDim.C8())
			row++
		}
	}
	// quests
	qrow := h - 6
	if qrow > row {
		s.Text(cx, qrow, "Quests", colDim.C8())
		qn := []string{"The Bone King", "The Drowned Oracle"}
		if g.Quests[1] > 0 || g.homeYet {
			qn = append(qn, "The Last Wanderer")
		}
		for i, q := range qn {
			mark, col := "○", colGray
			switch {
			case g.Quests[i] == 2 || i == 2 && g.Quests[i] == 1:
				mark, col = "✓", colGreen
			case g.Quests[i] == 1:
				mark, col = "◉", colGold
			}
			s.Text(cx, qrow+1+i, mark+" "+q, col.C8())
		}
	}
	s.Text(cx, h-2, "? help  i inv  c char  m map", colDim.C8())
}

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
	s.Text(x+w-utf8.RuneCountInString(label), y, label, labCol.C8())
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
			txt += fmt.Sprintf(" (x%d)", m.N)
		}
		if utf8.RuneCountInString(txt) > w-2 {
			txt = string([]rune(txt)[:w-2])
		}
		s.Text(x+1, y+1+i, txt, m.Col.Scale(f).C8())
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

// clipStr cuts s to at most n runes.
func clipStr(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}

// wrapText breaks s into lines of at most w runes, at spaces when it can.
func wrapText(s string, w int) []string {
	w = maxi(1, w)
	var out []string
	for utf8.RuneCountInString(s) > w {
		r := []rune(s)
		cut := strings.LastIndex(string(r[:w+1]), " ")
		if cut <= 0 {
			cut = len(string(r[:w]))
		}
		out = append(out, strings.TrimRight(s[:cut], " "))
		s = strings.TrimLeft(s[cut:], " ")
	}
	return append(out, s)
}

// scrollHints marks a list that runs past its rows: how many are above,
// right-aligned on the row over the list, and how many below, set into
// the row under it. Both end at x+w.
func scrollHints(s *Screen, x, w, top, bottom, above, below int) {
	if above > 0 {
		t := fmt.Sprintf("↑ %d more", above)
		s.Text(x+w-utf8.RuneCountInString(t), top, t, colDim.C8())
	}
	if below > 0 {
		t := fmt.Sprintf(" ↓ %d more ", below)
		s.Text(x+w-utf8.RuneCountInString(t), bottom, t, colDim.C8())
	}
}

func (g *Game) itemLines(s *Screen, x, y, w, maxRows int, it *Item) {
	row := 0
	for _, ln := range it.Lines(g.Rules) {
		if row >= maxRows {
			break
		}
		txt := ln.S
		for utf8.RuneCountInString(txt) > w {
			// wrap long flavor text
			head := string([]rune(txt)[:w])
			cut := strings.LastIndex(head, " ")
			if cut <= 0 {
				cut = len(head)
			}
			s.Text(x, y+row, txt[:cut], ln.C.C8())
			txt = strings.TrimSpace(txt[cut:])
			row++
		}
		s.Text(x, y+row, txt, ln.C.C8())
		row++
	}
}

func (g *Game) drawInventory(s *Screen, mapW, mapH int) {
	p := g.P
	bw, bh := 84, mini(s.H-2, 34)
	x, y := centerBox(s, mapW, mapH, bw, bh, "Inventory")
	bw = mini(bw, mapW-2)
	// two columns, equipped and pack; the names split what the width
	// leaves, the longest base name whole when there is room
	eqW := mini(17, (bw-21)/2+1)
	packW := bw - 21 - eqW
	colL := x + 2
	colR := colL + 9 + eqW + 2
	hdr := func(xx int, t string, active bool) {
		c := colDim
		if active {
			c = colGold
		}
		s.TextBold(xx, y+1, t, c.C8())
	}
	hdr(colL, "Equipped", g.pane == 0)
	hdr(colR, fmt.Sprintf("Pack %d/%d", len(p.Inv), invMax), g.pane == 1)
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
			sel(colL-1, yy, 9+eqW+1)
		}
		s.Text(colL, yy, fmt.Sprintf("%-8s", eqNames[i]), colDim.C8())
		if it := p.Eq[i]; it != nil {
			s.Text(colL+9, yy, clipStr(it.Name, eqW), it.Color().C8())
		} else {
			s.Text(colL+9, yy, "—", colDim.C8())
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
		s.Text(colR+2, yy, clipStr(it.Name, packW), it.Color().C8())
		if up := g.upgradeHint(it); up != "" {
			s.Text(x+bw-4, yy, up, colGreen.C8())
		}
	}
	if len(p.Inv) == 0 {
		s.Text(colR, y+3, "Your pack is empty.", colDim.C8())
	}
	// details
	detRows := y + bh - 1 - dy
	for xx := x + 1; xx < x+bw-1; xx++ {
		s.Put(xx, dy-1, '─', colBorder.Scale(.6).C8())
	}
	scrollHints(s, colR, x+bw-2-colR, y+2, dy-1, off, len(p.Inv)-off-maxRows)
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
				s.Text(colL+half+2, dy, "Currently equipped:", colDim.C8())
				g.itemLines(s, colL+half+2, dy+1, half, detRows-1, cur)
			}
		}
	}
	s.Text(colL, y+bh-1, " ←→/tab switch · ↑↓ select · enter equip/remove · d drop · esc ", colDim.C8())
}

func (g *Game) upgradeHint(it *Item) string {
	cur := g.equippedFor(it)
	if it.Kind != IKEquip {
		return ""
	}
	if cur == nil {
		return "new"
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
	p := g.P
	x, y := centerBox(s, mapW, mapH, 56, 24, "Character")
	cx := x + 3
	s.TextBold(cx, y+2, fmt.Sprintf("Wanderer · Level %d", p.Lvl), colWhite.C8())
	s.Text(cx, y+3, fmt.Sprintf("Experience %d / %d", p.XP, g.Rules.xpNext(p.Lvl)), colDim.C8())
	attrs := []struct {
		k    string
		name string
		base int
		tot  int
		desc string
	}{
		{"1", "Strength", p.Str, p.STR(), "+melee damage"},
		{"2", "Dexterity", p.Dex, p.DEX(), "+hit, crit, dodge"},
		{"3", "Vitality", p.Vit, p.VIT(), "+life"},
		{"4", "Energy", p.Ene, p.ENE(), "+mana, spell damage"},
	}
	for i, a := range attrs {
		yy := y + 5 + i
		c := colWhite
		if p.Points > 0 {
			s.TextBold(cx, yy, "["+a.k+"]", colGold.C8())
		}
		s.Text(cx+4, yy, fmt.Sprintf("%-10s %3d", a.name, a.tot), c.C8())
		if a.tot != a.base {
			s.Text(cx+19, yy, fmt.Sprintf("(%d)", a.base), colDim.C8())
		}
		s.Text(cx+26, yy, a.desc, colDim.C8())
	}
	if p.Points > 0 {
		s.TextBold(cx, y+10, fmt.Sprintf("%d points to spend — press 1-4", p.Points), colGold.C8())
	}
	lo, hi := p.DmgRange()
	fl, fh := p.FireboltDmg()
	stats := []string{
		fmt.Sprintf("Life        %d", p.MaxHP()),
		fmt.Sprintf("Mana        %d", p.MaxMP()),
		fmt.Sprintf("Damage      %d-%d", lo, hi),
		fmt.Sprintf("Firebolt    %d-%d", fl, fh),
		fmt.Sprintf("Armor       %d", p.ArmorVal()),
		fmt.Sprintf("Attack      %d", p.ToHit()),
		fmt.Sprintf("Critical    %d%%", p.Crit()),
		fmt.Sprintf("Life steal  %d%%", p.S(StLifeSteal)),
		fmt.Sprintf("Magic find  %d%%", p.S(StMF)),
		fmt.Sprintf("Kills       %d", p.Kills),
	}
	for i, st := range stats {
		col := cx
		row := y + 12 + i%5
		if i >= 5 {
			col = cx + 26
		}
		s.Text(col, row, st, colGray.C8())
	}
	s.Text(cx, y+22, "esc to close", colDim.C8())
}

func (g *Game) shopList() []*Item {
	if g.tab == 0 {
		return slices.Concat(g.shop.Items, g.shop.Services)
	}
	return g.P.Inv
}

func (g *Game) drawShop(s *Screen, mapW, mapH int) {
	p := g.P
	bw, bh := 80, mini(s.H-2, 32)
	x, y := centerBox(s, mapW, mapH, bw, bh, g.shop.Name)
	bw = mini(bw, mapW-2)
	cx := x + 2
	tabs := []string{" Buy ", " Sell "}
	tx := cx
	for i, t := range tabs {
		c, bg := colDim, C(.03, .025, .025)
		if g.tab == i {
			c, bg = colGold, C(.2, .1, .04)
		}
		for k, r := range t {
			s.SetBold(tx+k, y+1, r, c.C8(), bg.C8(), true)
		}
		tx += len(t) + 1
	}
	s.Text(x+bw-16, y+1, fmt.Sprintf("Gold: %d", p.Gold), colGold.C8())
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
		s.Text(cx+2, yy, clipStr(it.DisplayName(), bw-24), it.Color().C8())
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
		s.Text(cx, y+3, "Nothing here.", colDim.C8())
	}
	dy := y + bh - 12
	for xx := x + 1; xx < x+bw-1; xx++ {
		s.Put(xx, dy-1, '─', colBorder.Scale(.6).C8())
	}
	scrollHints(s, cx, bw-4, y+2, dy-1, off, len(items)-off-maxRows)
	if g.cur < len(items) {
		it := items[g.cur]
		half := (bw - 6) / 2
		g.itemLines(s, cx, dy, half, 10, it)
		if cur := g.equippedFor(it); cur != nil && it.Kind == IKEquip {
			s.Text(cx+half+2, dy, "Currently equipped:", colDim.C8())
			g.itemLines(s, cx+half+2, dy+1, half, 9, cur)
		}
	}
	s.Text(cx, y+bh-1, " tab buy/sell · ↑↓ select · enter confirm · esc leave ", colDim.C8())
}

func (g *Game) drawHelp(s *Screen, mapW, mapH int) {
	keys := []struct {
		k, d string
	}{
		{"arrows hjkl yubn", "move / attack (numpad works too)"},
		{". or 5", "wait a turn"},
		{"g or ,", "pick up equipment (gold & potions are automatic)"},
		{"f", "Firebolt at target (lights up the dark!)"},
		{"r", "Frost Nova: damage + brief freeze around you"},
		{"tab / shift+tab", "next / previous target (or click one)"},
		{"q / w", "drink healing / mana potion (works over a few turns)"},
		{"t", "read Scroll of Town Portal"},
		{"o", "auto-explore (stops when enemies appear)"},
		{"i", "inventory & equipment"},
		{"c", "character sheet, spend attribute points"},
		{"m", "map of explored area"},
		{"Q / ctrl+c", "quit"},
	}
	tips := []string{
		"Walk into doors, chests, altars and fountains to use them.",
		"Talk to townsfolk by walking into them.",
		"Monsters in darkness are invisible until light touches them.",
		"Blue names are champions; gold names are uniques.",
		"Glowing drops are rare or unique. Look for the light.",
		"Items with ▲ beat what you're wearing.",
		"Hover the mouse over the map to see what things are.",
		"Click a belt row to drink a potion or read the scroll.",
	}
	// lay the text out for the width at hand, then show the rows that fit
	type hline struct {
		k, d string
		dx   int
		c    RGB
	}
	bw := mini(78, mapW-2)
	var rows []hline
	for _, l := range keys {
		for i, d := range wrapText(l.d, bw-23) {
			k := ""
			if i == 0 {
				k = l.k
			}
			rows = append(rows, hline{k, d, 21, colGray})
		}
	}
	rows = append(rows, hline{})
	for _, t := range tips {
		for i, d := range wrapText(t, bw-7) {
			pre := "· "
			if i > 0 {
				pre = "  "
			}
			rows = append(rows, hline{"", pre + d, 3, colLore})
		}
	}
	bh := mini(len(rows)+5, s.H-2)
	x, y := centerBox(s, mapW, mapH, bw, bh, "Help")
	show := bh - 4
	g.helpOff = clampi(g.helpOff, 0, maxi(0, len(rows)-show))
	for i, l := range rows[g.helpOff:mini(len(rows), g.helpOff+show)] {
		s.TextBold(x+3, y+2+i, l.k, colOrange.C8())
		s.Text(x+l.dx, y+2+i, l.d, l.c.C8())
	}
	foot := "esc to close"
	if len(rows) > show {
		foot += " · ↑↓ scroll"
	}
	s.Text(x+3, y+bh-2, foot, colDim.C8())
	scrollHints(s, x, bw-2, y+1, y+bh-2, g.helpOff, len(rows)-show-g.helpOff)
}

func (g *Game) drawTalk(s *Screen, mapW, mapH int) {
	// size the box first: centerBox narrows it on small screens
	w := mini(64, mapW-2)
	var wrapped []string
	for _, ln := range g.talkLines {
		wrapped = append(wrapped, wrap(ln, w-6)...)
		wrapped = append(wrapped, "")
	}
	x, y := centerBox(s, mapW, mapH, w, len(wrapped)+4, g.talkName)
	for i, ln := range wrapped {
		s.Text(x+3, y+2+i, ln, g.talkCol.Lerp(colWhite, .5).C8())
	}
	s.Text(x+3, y+len(wrapped)+3, "press any key", colDim.C8())
}

func wrap(s string, w int) []string {
	var out []string
	words := strings.Fields(s)
	line := ""
	for _, wd := range words {
		if utf8.RuneCountInString(line)+utf8.RuneCountInString(wd)+1 > w && line != "" {
			out = append(out, line)
			line = wd
		} else if line == "" {
			line = wd
		} else {
			line += " " + wd
		}
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

func (g *Game) drawDead(s *Screen, mapW, mapH int) {
	x, y := centerBox(s, mapW, mapH, 50, 11, "")
	f := float32(0.75 + 0.25*math.Sin(g.time*2))
	msg := "YOU HAVE DIED"
	s.TextBold(x+(50-len(msg))/2, y+2, msg, C(.9, .1, .08).Scale(f).C8())
	p := g.P
	lines := []string{
		fmt.Sprintf("Slain by %s", p.KilledBy),
		fmt.Sprintf("Level %d · %d kills · %d gold", p.Lvl, p.Kills, p.Gold),
		fmt.Sprintf("in %s", g.Lv.Name),
	}
	for i, l := range lines {
		s.Text(x+(50-utf8.RuneCountInString(l))/2, y+4+i, l, colGray.C8())
	}
	h := "n: new game   Q: quit"
	s.Text(x+(50-len(h))/2, y+8, h, colOrange.C8())
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
	x, y := centerBox(s, mapW, mapH, w, h, l.Name)
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
	sub := "a descent into lit darkness"
	s.Text(cx-len(sub)/2, cy+4, sub, C(.6, .5, .4).C8())
	pr := "press any key to begin"
	f := float32(0.55 + 0.45*math.Sin(t*3))
	s.Text(cx-len(pr)/2, s.H-3, pr, C(1, .7, .4).Scale(f).C8())
}
