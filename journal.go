package main

import "github.com/antonmedv/termablo/internal/i18n"

// The quest journal: the quests given down the left, the one picked on
// the right, with what to do now, who gave it, where its boss or its
// thing is and what it pays, and below a journal entry for each stage
// it has reached, journal.<id>.<stage> in the catalog.

// journalStages are the stages a quest's journal has entries for, in
// the order they are reached.
var journalStages = []string{"taken", "done", "rewarded"}

// reached reports whether a quest has reached a stage.
func (g *Game) reached(q *Quest, stage string) bool {
	switch stage {
	case "taken":
		return g.Quests[q.ID] != QuestUnknown
	case "done":
		return g.status(q) != QuestOpen
	case "rewarded":
		return g.Quests[q.ID] == QuestRewarded
	}
	return false
}

// journal is the quests the journal lists: those not over first, in the
// order of the table, then those over.
func (g *Game) journal() []*Quest {
	var open, over []*Quest
	for i := range quests {
		q := &quests[i]
		switch {
		case g.Quests[q.ID] == QuestUnknown:
		case g.status(q) == QuestOver:
			over = append(over, q)
		default:
			open = append(open, q)
		}
	}
	return append(open, over...)
}

// questMark is a quest's mark and color in a list: over, done and owed,
// or still to do.
func (g *Game) questMark(q *Quest) (string, RGB) {
	switch g.status(q) {
	case QuestOver:
		return "✓", colGreen
	case QuestOwed:
		return "◉", colGold
	}
	return "○", colGray
}

// openJournal opens the journal on the first quest.
func (g *Game) openJournal() { g.Mode, g.cur = ModeQuests, 0 }

// journalKey: ↑↓ pick a quest, esc or J closes.
func (g *Game) journalKey(k string) {
	switch k {
	case "esc", "J", "q", "enter", " ":
		g.Mode = ModePlay
	case "up", "k":
		g.cur--
	case "down", "j":
		g.cur++
	case "home":
		g.cur = 0
	case "end":
		g.cur = len(quests)
	}
	g.cur = clampi(g.cur, 0, maxi(0, len(g.journal())-1))
}

// clickJournal picks the quest under the mouse.
func (g *Game) clickJournal(x, y int) {
	for i, h := range g.journalHit {
		if h.in(x, y) {
			g.cur = i
		}
	}
}

func (g *Game) drawJournal(s *Screen, mapW, mapH int) {
	L := g.L
	w, h := mini(76, mapW-2), mini(s.H-2, 24)
	x, y := centerBox(s, mapW, mapH, w, h, L.T("ui.journal.title"))
	g.journalHit = g.journalHit[:0]
	list := g.journal()
	foot := " " + L.T("ui.journal.keys") + " "
	if len(list) == 0 {
		s.TextRight(x+2, y+h-1, w-4, fit(foot, w-4), colDim.C8())
		for i, ln := range i18n.Wrap(L.T("ui.journal.none"), w-6) {
			s.TextStart(x+3, y+2+i, w-6, ln, colGray.C8())
		}
		return
	}
	g.cur = clampi(g.cur, 0, len(list)-1)

	// the list takes what its longest name and mark need, up to two
	// fifths of the box
	lw := 0
	for _, q := range list {
		lw = maxi(lw, i18n.Width(L.T(q.nameKey()))+4)
	}
	lw = clampi(lw, 14, w*2/5)
	tw := w - lw - 5
	sep := x + lw + 1
	lx, tx := x+1, sep+2
	if s.RTL {
		sep = x + w - lw - 2
		lx, tx = sep+1, x+2
	}
	for yy := y + 1; yy < y+h-1; yy++ {
		s.Set(sep, yy, '│', colBorder.C8(), colTalkBG.C8())
	}
	s.Set(sep, y, '┬', colBorder.C8(), colTalkBG.C8())
	s.Set(sep, y+h-1, '┴', colBorder.C8(), colTalkBG.C8())
	// the keys in the border under the quest, at its far end
	if foot = fit(foot, tw); s.RTL {
		s.Text(tx, y+h-1, foot, colDim.C8())
	} else {
		s.TextRight(tx, y+h-1, tw, foot, colDim.C8())
	}
	for i, q := range list {
		yy := y + 1 + i
		if yy >= y+h-1 {
			break
		}
		mark, c := g.questMark(q)
		hb := hitBox{lx, lx + lw - 1, yy}
		if i == g.cur {
			s.Fill(lx, yy, lw, 1, colTalkPick.C8())
		} else if g.hoverOn && hb.in(g.hoverX, g.hoverY) {
			s.Fill(lx, yy, lw, 1, colTalkPick.Scale(.6).C8())
		}
		name := L.T(q.nameKey())
		nc := colWhite
		switch {
		case i == g.cur:
			nc = colGold
		case g.status(q) == QuestOver:
			nc = colGray
		}
		if s.RTL {
			s.TextStart(lx+1, yy, lw-4, fit(name, lw-4), nc.C8())
			s.Text(lx+lw-2, yy, mark, c.C8())
		} else {
			s.Text(lx+1, yy, mark, c.C8())
			s.Text(lx+3, yy, fit(name, lw-4), nc.C8())
		}
		g.journalHit = append(g.journalHit, hb)
	}

	// the quest picked: its name, what to do now, the facts, the entries
	q := list[g.cur]
	row, bottom := y+1, y+h-1
	line := func(str string, c RGB, bold bool) {
		for _, ln := range i18n.Wrap(str, tw) {
			if row >= bottom {
				return
			}
			if bold {
				textStartBold(s, tx, row, tw, ln, c.C8())
			} else {
				s.TextStart(tx, row, tw, ln, c.C8())
			}
			row++
		}
	}
	name := L.Noun(q.nameKey())
	line(name.Text, colGold, true)
	st := g.status(q)
	_, mc := g.questMark(q)
	switch {
	case st == QuestOver:
		line(L.T("ui.journal.over"), mc, false)
	case st == QuestOwed:
		line(L.T("ui.journal.return", "who", L.Noun("monster."+q.Giver)), mc, false)
	case q.Item != "":
		line(L.T("ui.journal.fetch", "name", name, "who", L.Noun("monster."+q.Giver)), colWhite, false)
	default:
		line(L.T("ui.journal.goal", "name", name), colWhite, false)
	}
	row++
	if q.Giver != "" {
		line(L.T("ui.journal.giver", "who", L.Noun("monster."+q.Giver)), colDim, false)
	}
	if st == QuestOpen && q.Area != "" {
		line(L.T("ui.journal.where", "area", L.T("area."+q.Area)), colDim, false)
	}
	if q.Giver != "" && st != QuestOver {
		line(L.T("ui.journal.reward"), colDim, false)
	}
	for _, stage := range journalStages {
		if k := "journal." + q.ID + "." + stage; g.reached(q, stage) && L.Has(k) {
			row++
			line(L.T(k), colLore, false)
		}
	}
}
