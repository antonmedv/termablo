package main

import (
	"slices"
	"strings"
	"unicode"

	"github.com/antonmedv/termablo/internal/i18n"
)

// Conversations, the way Morrowind holds them: what has been said runs
// down the left, the topics the hero knows down the right. A topic is
// learned by hearing it, a [words](topic) link in a line, and from then
// on anyone can be asked about it.

// Talk is an open conversation.
type Talk struct {
	Who    string // the speaker in the catalog: talk.<who>.<topic>
	Name   string
	Col    RGB
	Barter bool // a trader: the list opens with Barter
	Log    []talkEntry
	Cur    int // the row picked in the topic list
	off    int // the first history row shown
	follow bool
	rumor  string    // the last rumor told here
	hits   []talkHit // this frame's topic rows and links
}

// talkEntry is one answer: the topic asked, or none for a greeting, and
// the text as the catalog has it, links and all.
type talkEntry struct{ Head, Text string }

// talkHit is a place to click: a row of the topic list, or a link.
type talkHit struct {
	hitBox
	topic string
	link  bool
}

// in is hitBox.in that takes a box one cell wide: a link may be.
func (h talkHit) in(x, y int) bool { return y == h.Y && x >= h.X0 && x <= h.X1 }

// topics are every topic there is, topic.<id> in the catalog. An answer
// is talk.<who>.<id>, else the town's talk.any.<id>; while topicDone,
// <id>_after is asked first. Rumors are the villagers', rumor.<key>.
var topics = []string{"rumors", "ember", "edran", "altars", "forge", "portals", "crypt", "boneking", "father", "blackmarsh", "oracle", "abyss", "cult", "stranger"}

// rumors are what a villager tells, rumor.<key>, one picked at random.
var rumors = []string{"braziers", "brother", "torch", "forge", "glimmer", "crystal", "scroll", "stranger", "edran", "uncle", "holding"}

func (g *Game) rumor() string { return "rumor." + rumors[g.rng.Intn(len(rumors))] }

// anotherRumor is a rumor other than the last one heard here.
func (g *Game) anotherRumor() string {
	i := g.rng.Intn(len(rumors) - 1)
	if last := slices.Index(rumors, strings.TrimPrefix(g.talk.rumor, "rumor.")); last >= 0 && i >= last {
		i++
	}
	return "rumor." + rumors[i]
}

// topicDone reports whether what a topic is about has been settled: its
// boss is dead.
func (g *Game) topicDone(id string) bool {
	switch id {
	case "boneking", "father":
		return g.Quests[0] > 0
	case "oracle", "abyss":
		return g.Quests[1] > 0
	case "stranger":
		return g.Quests[2] > 0
	}
	return false
}

// answer is the catalog key of what who says about a topic, or "".
func (g *Game) answer(who, id string) string {
	if id == "rumors" {
		if who == "villager" {
			return "rumor"
		}
		return ""
	}
	var keys []string
	for _, w := range []string{who, "any"} {
		if g.topicDone(id) {
			keys = append(keys, "talk."+w+"."+id+"_after")
		}
		keys = append(keys, "talk."+w+"."+id)
	}
	for _, k := range keys {
		if g.L.Has(k) {
			return k
		}
	}
	return ""
}

// hear adds a line to the conversation and learns the topics it links:
// the English links, so that what the hero knows does not hang on a
// translation keeping every one.
func (g *Game) hear(head, key string) {
	t := g.talk
	g.asked[key] = true
	text := g.L.T(key)
	for _, id := range i18n.Links(locales.Get(i18n.Source).T(key)) {
		g.Known[id] = true
	}
	if strings.HasPrefix(key, "rumor.") {
		t.rumor = key
	}
	t.Log = append(t.Log, talkEntry{head, text})
	t.follow = true
}

// talkOpt is a row of the topic list: Barter, a topic, or Goodbye.
type talkOpt struct {
	id, label string
	asked     bool
	special   bool // Barter or Goodbye, not a topic
}

const (
	optBarter  = "barter"
	optGoodbye = "goodbye"
)

// options are the topic list: Barter for a trader, then every known topic
// the speaker has an answer to, by name, then Goodbye.
func (t *Talk) options(g *Game) []talkOpt {
	var opts []talkOpt
	for _, id := range topics {
		if k := g.answer(t.Who, id); g.Known[id] && k != "" {
			opts = append(opts, talkOpt{id: id, label: g.L.T("topic." + id), asked: k != "rumor" && g.asked[k]})
		}
	}
	slices.SortFunc(opts, func(a, b talkOpt) int { return strings.Compare(a.label, b.label) })
	if t.Barter {
		opts = append([]talkOpt{{id: optBarter, label: g.L.T("ui.talk.barter"), special: true}}, opts...)
	}
	return append(opts, talkOpt{id: optGoodbye, label: g.L.T("ui.talk.goodbye"), special: true})
}

// firstTopic is the row the list opens on: Barter with a trader, else
// the first topic not yet asked.
func (t *Talk) firstTopic(g *Game) int {
	if t.Barter {
		return 0
	}
	for i, o := range t.options(g) {
		if !o.special && !o.asked {
			return i
		}
	}
	return 0
}

// ask picks a row of the topic list, or a link.
func (g *Game) ask(id string) {
	t := g.talk
	switch id {
	case optBarter:
		g.Mode, g.cur, g.tab = ModeShop, 0, 0
	case optGoodbye:
		g.talk, g.Mode = nil, ModePlay
	default:
		k := g.answer(t.Who, id)
		if k == "" {
			return // a link to a topic this speaker has nothing on
		}
		if k == "rumor" {
			k = g.anotherRumor()
		}
		g.hear(g.L.T("topic."+id), k)
		if i := slices.IndexFunc(t.options(g), func(o talkOpt) bool { return o.id == id }); i >= 0 {
			t.Cur = i
		}
	}
}

// talkKey: ↑↓ pick a topic, enter asks it, pgup/pgdn scroll what was
// said, esc says goodbye.
func (g *Game) talkKey(k string) {
	t := g.talk
	opts := t.options(g)
	switch k {
	case "esc", "q":
		g.ask(optGoodbye)
	case "up", "k":
		t.Cur--
	case "down", "j":
		t.Cur++
	case "home":
		t.Cur = 0
	case "end":
		t.Cur = len(opts) - 1
	case "pgup":
		t.scroll(-5)
	case "pgdown":
		t.scroll(5)
	case "enter", " ":
		g.ask(opts[clampi(t.Cur, 0, len(opts)-1)].id)
		return
	}
	t.Cur = clampi(t.Cur, 0, len(opts)-1)
}

func (t *Talk) scroll(n int) { t.off, t.follow = maxi(0, t.off+n), false }

// clickTalk asks the topic or link under the mouse.
func (g *Game) clickTalk(x, y int) {
	// by id, not row: the list may have changed since it was drawn
	for _, h := range g.talk.hits {
		if h.in(x, y) {
			g.ask(h.topic)
			return
		}
	}
}

// talkSpan is a run of a history row in one style: plain, or a link.
type talkSpan struct{ text, topic string }

// talkRow is a row of the history: a topic's heading, or text.
type talkRow struct {
	head  bool
	spans []talkSpan
}

// wrapLinks wraps a line with links to w cells, keeping which words
// belong to which link.
func wrapLinks(text string, w int) [][]talkSpan {
	plain, links := i18n.ParseLinks(text)
	rs := []rune(plain)
	of := make([]string, len(rs)) // the topic each rune links to
	at := 0
	for i := range rs {
		for _, l := range links {
			if at >= l.Start && at < l.End {
				of[i] = l.Topic
			}
		}
		at += len(string(rs[i]))
	}
	// Wrap only drops and folds spaces: the rest come out in order.
	var rows [][]talkSpan
	j := 0
	for _, line := range i18n.Wrap(plain, w) {
		var row []talkSpan
		for _, r := range line {
			topic := ""
			if r == ' ' { // any space of the text, folded
				if j < len(rs) && unicode.IsSpace(rs[j]) {
					topic = of[j]
				}
				for j < len(rs) && unicode.IsSpace(rs[j]) {
					j++
				}
			} else {
				for j < len(rs) && rs[j] != r {
					j++
				}
				if j < len(rs) {
					topic = of[j]
					j++
				}
			}
			if n := len(row); n > 0 && row[n-1].topic == topic {
				row[n-1].text += string(r)
			} else {
				row = append(row, talkSpan{string(r), topic})
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// rows lays out the history w cells wide. start is where the last entry
// begins.
func (t *Talk) rows(w int) (rows []talkRow, start int) {
	for i, e := range t.Log {
		if i > 0 {
			rows = append(rows, talkRow{})
		}
		start = len(rows)
		if e.Head != "" {
			rows = append(rows, talkRow{head: true, spans: []talkSpan{{text: i18n.Truncate(e.Head, w)}}})
		}
		for _, para := range strings.Split(e.Text, "\n") {
			for _, r := range wrapLinks(para, w) {
				rows = append(rows, talkRow{spans: r})
			}
		}
	}
	return rows, start
}

var (
	colTalkBG   = C(.03, .025, .025)
	colTalkPick = C(.18, .1, .05)
	colTalkLink = C(1, .72, .3)
)

func (g *Game) drawTalk(s *Screen, mapW, mapH int) {
	t, L := g.talk, g.L
	opts := t.options(g)
	w, h := mini(76, mapW-2), mini(s.H-2, 26)
	x, y := centerBox(s, mapW, mapH, w, h, "")
	t.hits = t.hits[:0]

	// the topic list takes what its longest name needs, up to two fifths
	// of the box, or 20 cells: the 18 of the longest name, and a margin
	lw := 0
	for _, o := range opts {
		lw = maxi(lw, i18n.Width(o.label))
	}
	lw = clampi(lw+2, 12, maxi(20, w*2/5))
	tw := w - lw - 5 // the history: │ text │ list │
	sep := x + w - lw - 2
	tx, lx := x+2, sep+1
	if s.RTL {
		sep = x + lw + 1
		tx, lx = sep+2, x+1
	}
	top, view := y+1, h-2
	for yy := y + 1; yy < y+h-1; yy++ {
		s.Set(sep, yy, '│', colBorder.C8(), colTalkBG.C8())
	}
	s.Set(sep, y, '┬', colBorder.C8(), colTalkBG.C8())
	s.Set(sep, y+h-1, '┴', colBorder.C8(), colTalkBG.C8())

	// what was said, the newest answer from its start
	rows, start := t.rows(tw)
	most := maxi(0, len(rows)-view)
	if t.follow {
		t.off, t.follow = mini(start, most), false
	}
	t.off = clampi(t.off, 0, most)
	text := t.Col.Lerp(colWhite, .5)
	for i, r := range rows[t.off:mini(len(rows), t.off+view)] {
		yy := top + i
		if r.head {
			textStartBold(s, tx, yy, tw, r.spans[0].text, colOrange.C8())
			continue
		}
		g.drawSpans(s, tx, yy, tw, r.spans, text)
	}
	above, below := t.off, maxi(0, len(rows)-view-t.off)

	// the topics, scrolled to keep the pick in sight
	loff := clampi(t.Cur-view+1, 0, maxi(0, len(opts)-view))
	for i := loff; i < len(opts) && i-loff < view; i++ {
		o, yy := opts[i], top+i-loff
		c := colWhite
		switch {
		case o.special:
			c = colLore
		case o.asked:
			c = colGray
		}
		hb := hitBox{lx, lx + lw - 1, yy}
		if i == t.Cur {
			s.Fill(lx, yy, lw, 1, colTalkPick.C8())
			c = colGold
		} else if g.hoverOn && hb.in(g.hoverX, g.hoverY) {
			s.Fill(lx, yy, lw, 1, colTalkPick.Scale(.6).C8())
		}
		s.TextStart(lx+1, yy, lw-2, i18n.Truncate(o.label, lw-2), c.C8())
		t.hits = append(t.hits, talkHit{hb, o.id, false})
	}
	// In the border, at the start of the history: how much was said above
	// and below what is shown. The keys go at its far end, or across the
	// whole border when they do not fit there and nothing is below.
	c := colDim.C8()
	name := i18n.Truncate(" "+t.Name+" ", tw)
	textStartBold(s, tx, y, tw, name, colGold.C8())
	if more := " " + L.T("ui.more_above", "n", above) + " "; above > 0 && i18n.Width(name)+i18n.Width(more) < tw {
		if s.RTL {
			s.Text(tx, y, more, c)
		} else {
			s.TextRight(tx, y, tw, more, c)
		}
	}
	foot, room := " "+L.T("ui.talk.keys")+" ", tw
	if below > 0 {
		more := " " + L.T("ui.more_below", "n", below) + " "
		s.TextStart(tx, y+h-1, tw, more, c)
		room -= i18n.Width(more) + 1
	}
	switch {
	case i18n.Width(foot) <= room && s.RTL:
		s.Text(tx, y+h-1, foot, c)
	case i18n.Width(foot) <= room:
		s.TextRight(tx, y+h-1, tw, foot, c)
	case below == 0:
		s.TextStart(x+2, y+h-1, w-4, i18n.Truncate(foot, w-4), c)
	}
}

// textStartBold is Screen.TextStart in bold.
func textStartBold(s *Screen, x, y, w int, str string, fg Col8) {
	if s.RTL {
		x += max(0, w-i18n.Width(str))
	}
	s.TextBold(x, y, str, fg)
}

// drawSpans writes a history row, links in their own color, and marks
// where the links are for the mouse. Right to left, the row is drawn
// whole and each link found again where it landed.
func (g *Game) drawSpans(s *Screen, x, y, w int, spans []talkSpan, col RGB) {
	if s.RTL {
		var line strings.Builder
		for _, sp := range spans {
			line.WriteString(sp.text)
		}
		s.TextStart(x, y, w, line.String(), col.C8())
		vis := i18n.Visual(line.String(), true)
		x0 := x + w - i18n.Width(line.String())
		// The row reads from the right: each span, plain ones too, is the
		// nearest match left of the one before, so a word said twice is
		// found where it was said.
		end := len(vis)
		for _, sp := range spans {
			word := strings.TrimSpace(sp.text)
			if word == "" {
				continue
			}
			i := strings.LastIndex(vis[:end], i18n.Visual(word, true))
			if i < 0 {
				continue
			}
			end = i
			if sp.topic != "" {
				g.linkCells(s, x0+i18n.Width(vis[:i]), y, i18n.Width(word), sp.topic)
			}
		}
		return
	}
	for _, sp := range spans {
		c := col
		if sp.topic != "" {
			c = colTalkLink
		}
		n := s.Text(x, y, sp.text, c.C8())
		if sp.topic != "" {
			g.linkCells(s, x, y, n, sp.topic)
		}
		x += n
	}
}

// linkCells colors a link's cells and makes them clickable.
func (g *Game) linkCells(s *Screen, x, y, w int, topic string) {
	hb := hitBox{x, x + w - 1, y}
	hover := g.hoverOn && hb.in(g.hoverX, g.hoverY)
	for xx := x; xx < x+w && xx < s.W; xx++ {
		c := &s.C[y*s.W+xx]
		c.FG, c.Bold = colTalkLink.C8(), true
		if hover {
			c.BG = colTalkPick.C8()
		}
	}
	g.talk.hits = append(g.talk.hits, talkHit{hb, topic, true})
}
