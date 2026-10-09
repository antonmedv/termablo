package main

import (
	"slices"
	"strings"
	"unicode"

	"github.com/antonmedv/termablo/internal/i18n"
)

// Conversations, the way Morrowind holds them: what has been said runs
// down the left, the topics the hero knows down the right. A topic is
// learned by hearing it, a [words](topic) link in a line, or by coming
// upon the thing itself (sights), and from then on anyone can be asked
// about it.

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
	looks  []string  // how the hero looked as the talk opened
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

// topics are every topic there is, topic.<id> in the catalog. What
// a speaker says on one is found by says. Rumors are the villagers',
// rumor.<key>.
var topics = []string{"rumors", "ember", "edran", "altars", "forge", "portals", "crypt", "boneking", "father", "blackmarsh", "oracle", "abyss", "cult", "stranger"}

// rumorDef is a rumor villagers tell, rumor.<Key>: from the start, or
// once the fact After holds, and no more once Until does. A fact holds
// once its quest is settled: the town hears from the giver, as they pay.
type rumorDef struct{ Key, After, Until string }

// rumors are what villagers tell: the town's talk follows the hero down.
var rumors = []rumorDef{
	{Key: "braziers"}, {Key: "brother", Until: "oracle"}, {Key: "torch"}, {Key: "forge"},
	{Key: "glimmer"}, {Key: "crystal", Until: "oracle"}, {Key: "scroll"},
	{Key: "stranger", Until: "wanderer"}, {Key: "edran"}, {Key: "uncle"}, {Key: "holding", Until: "oracle"},
	{Key: "bells", After: "boneking"}, {Key: "face", After: "boneking"},
	{Key: "lights", After: "oracle"}, {Key: "wind", After: "oracle"},
	{Key: "lanterns", After: "wanderer"},
}

// rumor is a rumor told now, other than last: one the hero has not heard
// from any villager while there are some, then any.
func (g *Game) rumor(last string) string {
	var fresh, old []string
	for _, r := range rumors {
		k := "rumor." + r.Key
		if k == last || r.After != "" && !g.fact(r.After) || r.Until != "" && g.fact(r.Until) {
			continue
		}
		if g.saidBy("villager", k) {
			old = append(old, k)
		} else {
			fresh = append(fresh, k)
		}
	}
	if len(fresh) == 0 {
		fresh = old
	}
	return fresh[g.rng.Intn(len(fresh))]
}

// sights teach the hero a topic when they meet the thing itself: a place
// entered, area.<lore key>; a monster killed, kill.<template>; a unique
// picked up, item.<name>; an altar bled on; a portal opened.
var sights = map[string]string{
	"area.crypt": "crypt", "area.marsh": "blackmarsh", "area.grotto": "oracle", "area.abyss": "abyss",
	"kill.cultist": "cult", "kill.boneking": "edran",
	"item.Kingsbane": "father", "item.Hollow Crown": "edran",
	"altar": "altars", "portal": "portals",
}

// see learns the topic a sight teaches, and says so: something to bring
// back to town.
func (g *Game) see(what string) {
	id, ok := sights[what]
	if !ok || g.Known[id] {
		return
	}
	g.Known[id] = true
	g.say(colLore, "msg.topic_new", "topic", keyArg("topic."+id))
}

// looks are what a speaker may see in the hero as they meet, in the
// order they are remarked on. A line may have a variant for each,
// <line>_<look>, said while it holds: hurt, wearing Kingsbane or the
// Hollow Crown, no healing potion on the belt, or not the coin for one.
var looks = []string{"hurt", "kingsbane", "crown", "nopots", "broke"}

func (g *Game) looking(look string) bool {
	p := g.P
	wears := func(name string) bool {
		return slices.ContainsFunc(p.Eq[:], func(it *Item) bool { return it != nil && it.Rarity == RUnique && it.Name == name })
	}
	switch look {
	case "hurt":
		return p.HP < float64(p.MaxHP())*.4
	case "kingsbane":
		return wears("Kingsbane")
	case "crown":
		return wears("Hollow Crown")
	case "nopots":
		return p.HPot == 0
	case "broke":
		return p.Gold < g.buyPrice(&Item{Kind: IKHealth})
	}
	return false
}

// looksNow are the looks that hold.
func (g *Game) looksNow() []string {
	var l []string
	for _, id := range looks {
		if g.looking(id) {
			l = append(l, id)
		}
	}
	return l
}

// facts are what the town remembers has happened, newest first: the
// quests settled, later quests in the table newer. A line may have a
// variant for each, <line>_<fact>, said instead while the fact holds.
// The town hears of a quest's boss from its giver, as they pay.
func (g *Game) facts() []string {
	var ids []string
	for i := len(quests) - 1; i >= 0; i-- {
		if g.status(&quests[i]) == QuestOver {
			ids = append(ids, quests[i].ID)
		}
	}
	return ids
}

// topicFact is the fact that settles a topic: its <id>_after is said
// instead once that boss is dead.
var topicFact = map[string]string{"boneking": "boneking", "father": "boneking", "oracle": "oracle", "abyss": "oracle", "stranger": "wanderer"}

func (g *Game) fact(id string) bool {
	q := questByID(id)
	return q != nil && g.status(q) == QuestOver
}

// lineKind is what a variant of a line is: the plain line or its
// _after, news of a fact, or a remark on how the hero looks.
type lineKind uint8

const (
	linePlain lineKind = iota
	lineNews
	lineLook
)

// variant is a line a speaker may say on a topic, and its kind.
type variant struct {
	key  string
	kind lineKind
}

// variants are the lines a speaker has on a topic, in the order tried:
// their own before the town's, and for each the settled <id>_after, the
// news of the newest fact they have a line for, <id>_<fact>, the first
// look they have a line for, <id>_<look>, and the plain <id>. Older
// news is passed over: it is old.
func (g *Game) variants(who, id string) []variant {
	var vs []variant
	add := func(k string, kind lineKind) bool {
		if !g.L.Has(k) {
			return false
		}
		vs = append(vs, variant{k, kind})
		return true
	}
	var seen []string
	if g.talk != nil {
		seen = g.talk.looks
	}
	for _, w := range []string{who, "any"} {
		base := "talk." + w + "." + id
		if f, ok := topicFact[id]; ok && g.fact(f) {
			add(base+"_after", linePlain)
		}
		for _, f := range g.facts() {
			if add(base+"_"+f, lineNews) {
				break
			}
		}
		for _, l := range seen {
			if add(base+"_"+l, lineLook) {
				break
			}
		}
		add(base, linePlain)
	}
	return vs
}

// answer is the catalog key of the first line a speaker has on a topic,
// or "" when they have none.
func (g *Game) answer(who, id string) string {
	if id == "rumors" {
		if who == "villager" {
			return "rumor"
		}
		return ""
	}
	vs := g.variants(who, id)
	if len(vs) == 0 {
		return ""
	}
	return vs[0].key
}

// says is the catalog key of what a speaker says on a topic now, with
// what they remember saying. Before any news they say the line under
// it: the hero is met before being told the news. A line said before
// gives way to its <line>_again; without one, news is told once and
// passed over, and anything else is said again. How the hero looks is
// remarked on over anything said before, news told and all.
func (g *Game) says(who, id string) string {
	if id == "rumors" {
		return g.answer(who, id)
	}
	vs := g.variants(who, id)
	// the hero is met before being told the news: the first plain line
	// comes before anything, until it is said
	if i := slices.IndexFunc(vs, func(v variant) bool { return v.kind == linePlain }); i >= 0 && !g.saidBy(who, vs[i].key) {
		return vs[i].key
	}
	looked := slices.ContainsFunc(vs, func(v variant) bool { return v.kind == lineLook })
	last := ""
	for _, v := range vs {
		said, again := g.saidBy(who, v.key), v.key+"_again"
		switch {
		case v.kind == lineLook:
			// how the hero looks is remarked on every time, the second
			// time on in its _again when there is one
			if said && g.L.Has(again) {
				return again
			}
			return v.key
		case !said:
			return v.key
		case v.kind == lineNews && looked:
			// news told gives way to a look still to come
		case g.L.Has(again):
			return again
		case v.kind == linePlain:
			return v.key
		}
		last = v.key
	}
	return last
}

// fresh reports whether a speaker has something on a topic they have not
// said.
func (g *Game) fresh(who, id string) bool {
	k := g.says(who, id)
	return k != "" && !g.saidBy(who, k) && !strings.HasSuffix(k, "_again")
}

// saidBy reports whether a speaker has said a line to the hero.
func (g *Game) saidBy(who, key string) bool { return g.heard[who+" "+key] }

// hear adds a line to the conversation, remembers who said it, and
// learns the topics it links: the English links, so that what the hero
// knows does not hang on a translation keeping every one.
func (g *Game) hear(head, key string) {
	t := g.talk
	g.heard[t.Who+" "+key] = true
	g.offered(key)
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

// speakers are who the townsfolk are in the catalog, talk.<speaker>.*,
// by template ID; anyone else is a villager.
var speakers = map[string]string{"smith": "hadrik", "alch": "mirela", "captain": "voss"}

func speaker(template string) string {
	if s, ok := speakers[template]; ok {
		return s
	}
	return "villager"
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
			opts = append(opts, talkOpt{id: id, label: g.L.T("topic." + id), asked: k != "rumor" && !g.fresh(t.Who, id)})
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
		k := g.says(t.Who, id)
		if k == "" {
			return // a link to a topic this speaker has nothing on
		}
		if k == "rumor" {
			k = g.rumor(t.rumor)
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
