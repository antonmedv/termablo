package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/antonmedv/termablo/internal/i18n"
)

// talkWith opens a conversation with a townsperson, as walking into them.
func talkWith(t *testing.T, g *Game, id string) {
	t.Helper()
	g.Mode = ModePlay
	m := npc(t, g, id)
	if !walkInto(g, m.X, m.Y) || g.Mode != ModeTalk {
		t.Fatalf("no conversation with %s: mode %v", id, g.Mode)
	}
}

func optIDs(g *Game) []string {
	var ids []string
	for _, o := range g.talk.options(g) {
		ids = append(ids, o.id)
	}
	return ids
}

// A topic heard from one speaker can be asked of another.
func TestTopicsAreLearnedAndShared(t *testing.T) {
	g := NewGame(1)
	if g.Known["ember"] {
		t.Fatal("the Ember is known before anyone mentions it")
	}
	talkWith(t, g, "captain")
	for _, id := range []string{"ember", "crypt", "boneking", "blackmarsh", "oracle", "portals"} {
		if !g.Known[id] {
			t.Errorf("Voss's briefing does not teach %s", id)
		}
	}
	m := newTestModel(g)
	press(m, "esc")
	if g.Mode != ModePlay || g.talk != nil {
		t.Fatalf("goodbye left mode %v", g.Mode)
	}
	talkWith(t, g, "smith")
	ids := optIDs(g)
	if ids[0] != optBarter || ids[len(ids)-1] != optGoodbye || !slices.Contains(ids, "boneking") {
		t.Errorf("Hadrik's topics: %q", ids)
	}
	if slices.Contains(ids, "rumors") {
		t.Error("Hadrik trades in rumors")
	}
	g.ask("ember")
	last := g.talk.Log[len(g.talk.Log)-1]
	if last.Head != "Ember" || last.Text != g.L.T("talk.hadrik.ember") {
		t.Errorf("Hadrik on the Ember: %+v", last)
	}
	g.ask("boneking")
	if last := g.talk.Log[len(g.talk.Log)-1]; last.Text != g.L.T("talk.any.boneking") {
		t.Errorf("Hadrik on the Bone King, not the town's answer: %q", last.Text)
	}
	if o := g.talk.options(g)[g.talk.Cur]; o.id != "boneking" || !o.asked {
		t.Errorf("the pick is %+v after asking the Bone King", o)
	}
}

// What is said of a boss changes once the boss is dead.
func TestTopicAfterTheBoss(t *testing.T) {
	g := NewGame(1)
	if k := g.answer("voss", "boneking"); k != "talk.voss.boneking" {
		t.Errorf("before: %s", k)
	}
	g.Quests[0] = 2
	if k := g.answer("voss", "boneking"); k != "talk.voss.boneking_after" {
		t.Errorf("after: %s", k)
	}
	if k := g.answer("villager", "boneking"); k != "talk.any.boneking_after" {
		t.Errorf("a villager after: %s", k)
	}
}

// Villagers greet with a rumor and tell another when asked.
func TestVillagerRumors(t *testing.T) {
	g := NewGame(1)
	talkWith(t, g, "villager")
	if len(g.talk.Log) != 1 || g.talk.Log[0].Head != "" {
		t.Fatalf("greeting %+v", g.talk.Log)
	}
	if !slices.Contains(optIDs(g), "rumors") {
		t.Fatal("no Latest rumors")
	}
	g.ask("rumors")
	if len(g.talk.Log) != 2 || g.talk.Log[1].Head != "Latest rumors" {
		t.Errorf("asked for rumors: %+v", g.talk.Log)
	}
}

// Barter opens the shop, and leaving it comes back to the conversation.
func TestBarter(t *testing.T) {
	g := NewGame(1)
	m := newTestModel(g)
	talkWith(t, g, "smith")
	if g.talk.Cur != 0 {
		t.Errorf("Hadrik's list opens on row %d, not Barter", g.talk.Cur)
	}
	press(m, "enter")
	if g.Mode != ModeShop || g.shop != g.shops[0] {
		t.Fatalf("mode %v after Barter", g.Mode)
	}
	press(m, "esc")
	if g.Mode != ModeTalk {
		t.Errorf("mode %v after the shop", g.Mode)
	}
}

// A link keeps its words through wrapping, even when they break across
// lines, and the text reads the same as without the markup.
func TestWrapLinks(t *testing.T) {
	text := "Go north to [the Drowned Oracle](oracle), then home."
	rows := wrapLinks(text, 16)
	var lines []string
	var linked []string
	for _, r := range rows {
		var b strings.Builder
		for _, sp := range r {
			b.WriteString(sp.text)
			if sp.topic != "" {
				linked = append(linked, strings.TrimSpace(sp.text))
			}
		}
		lines = append(lines, b.String())
	}
	plain, _ := i18n.ParseLinks(text)
	if want := i18n.Wrap(plain, 16); !slices.Equal(lines, want) {
		t.Errorf("rows %q, want %q", lines, want)
	}
	if got := strings.Join(linked, " "); got != "the Drowned Oracle" {
		t.Errorf("linked words %q", got)
	}
}

// Clicking a link asks about it.
func TestClickLink(t *testing.T) {
	g := NewGame(1)
	talkWith(t, g, "captain")
	s := NewScreen(120, 36)
	g.Draw(s)
	for _, h := range g.talk.hits {
		if h.row < 0 && h.topic == "crypt" {
			g.Click(h.X0, h.Y, s.W, s.H)
			if last := g.talk.Log[len(g.talk.Log)-1]; last.Text != g.L.T("talk.voss.crypt") {
				t.Errorf("clicked the crypt, heard %q", last.Text)
			}
			return
		}
	}
	t.Error("no crypt link on screen")
}

// Every link names a topic, every topic but rumors has the town's answer,
// and every topic can be learned somewhere.
func TestTopicCatalog(t *testing.T) {
	en := locales.Get(i18n.Source)
	linked := map[string]bool{"rumors": true}
	for _, k := range en.Keys() {
		for _, id := range i18n.Links(en.T(k)) {
			linked[id] = true
			if !slices.Contains(topics, id) {
				t.Errorf("%s links to (%s), which is not a topic", k, id)
			}
		}
	}
	for _, id := range topics {
		if !linked[id] {
			t.Errorf("nothing teaches the topic %s", id)
		}
		if id != "rumors" && !en.Has("talk.any."+id) {
			t.Errorf("talk.any.%s is missing", id)
		}
	}
}
