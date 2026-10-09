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
		if h.link && h.topic == "crypt" {
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

// Topics are learned from the English links, whatever a translation kept.
func TestLearnedWhateverTheLanguage(t *testing.T) {
	g := NewGame(1)
	cat, err := i18n.Parse("de", []byte(`{ talk: { voss: { greet: "Geht zur Seherin." } } }`))
	if err != nil {
		t.Fatal(err)
	}
	g.L = cat
	g.talk = &Talk{Who: "voss"}
	g.hear("", "talk.voss.greet")
	if !g.Known["oracle"] || g.talk.Log[0].Text != "Geht zur Seherin." {
		t.Errorf("known %v, heard %q", g.Known, g.talk.Log[0].Text)
	}
}

// Right to left, a link whose words were also said plain earlier in the
// row is found where the link is, not at the first match.
func TestRTLLinkSaidTwice(t *testing.T) {
	g := NewGame(1)
	g.talk = &Talk{}
	s := NewScreen(40, 3)
	s.RTL = true
	g.drawSpans(s, 0, 1, 40, []talkSpan{{"باب ", ""}, {"باب", "door"}}, colWhite)
	if len(g.talk.hits) != 1 {
		t.Fatalf("hits %+v", g.talk.hits)
	}
	// the plain word reads first, so it is the right one; the link is left
	if h := g.talk.hits[0]; h.X0 != 40-7 {
		t.Errorf("link at %d..%d, want it at %d", h.X0, h.X1, 40-7)
	}
}

// Latest rumors never tells the rumor just told.
func TestRumorNotRepeated(t *testing.T) {
	g := NewGame(1)
	talkWith(t, g, "villager")
	for range 40 {
		before := g.talk.rumor
		g.ask("rumors")
		if g.talk.rumor == before {
			t.Fatalf("%s told twice running", before)
		}
	}
}

// A link one cell wide can still be clicked.
func TestOneCellLinkClicks(t *testing.T) {
	g := NewGame(1)
	talkWith(t, g, "captain")
	g.talk.hits = []talkHit{{hitBox{5, 5, 3}, "crypt", true}}
	g.clickTalk(5, 3)
	if last := g.talk.Log[len(g.talk.Log)-1]; last.Text != g.L.T("talk.voss.crypt") {
		t.Errorf("the click was lost: %q", last.Text)
	}
}

// lastText is the last line said.
func lastText(g *Game) string { return g.talk.Log[len(g.talk.Log)-1].Text }

// A speaker remembers greeting the hero, and greets them shorter after.
func TestGreetRemembered(t *testing.T) {
	g := NewGame(1)
	m := newTestModel(g)
	talkWith(t, g, "captain")
	if lastText(g) != g.L.T("talk.voss.greet") {
		t.Fatalf("first meeting: %q", lastText(g))
	}
	press(m, "esc")
	talkWith(t, g, "captain")
	if lastText(g) != g.L.T("talk.voss.greet_again") {
		t.Errorf("coming back: %q", lastText(g))
	}
}

// What has happened is news: told once, and then the speaker goes back
// to what they said before.
func TestNewsToldOnce(t *testing.T) {
	g := NewGame(1)
	m := newTestModel(g)
	talkWith(t, g, "smith")
	press(m, "esc")
	g.Quests[0] = 2
	var said []string
	for range 3 {
		talkWith(t, g, "smith")
		said = append(said, lastText(g))
		press(m, "esc")
	}
	want := []string{g.L.T("talk.hadrik.greet_boneking"), g.L.T("talk.hadrik.greet_again"), g.L.T("talk.hadrik.greet_again")}
	if !slices.Equal(said, want) {
		t.Errorf("Hadrik after the Bone King: %q, want %q", said, want)
	}
	g.Quests[1] = 2
	talkWith(t, g, "smith")
	if lastText(g) != g.L.T("talk.hadrik.greet_oracle") {
		t.Errorf("the newest news first: %q", lastText(g))
	}
}

// Voss's reward is his greeting; what he says of the quest after waits
// for the next visit, and keeps, shorter, after that.
func TestVossAfterReward(t *testing.T) {
	g := NewGame(1)
	m := newTestModel(g)
	g.Quests[0] = 1
	var said []string
	for range 3 {
		talkWith(t, g, "captain")
		said = append(said, lastText(g))
		press(m, "esc")
	}
	want := []string{g.L.T("talk.voss.boneking_reward"), g.L.T("talk.voss.greet_boneking"), g.L.T("talk.voss.greet_boneking_again")}
	if !slices.Equal(said, want) || g.Quests[0] != 2 {
		t.Errorf("Voss after the Bone King: %q, want %q", said, want)
	}
}

// Asked again, a speaker may say they have said it, and the topic shows
// as asked of them, not of everyone.
func TestTopicRemembered(t *testing.T) {
	g := NewGame(1)
	g.Known["boneking"] = true
	talkWith(t, g, "smith")
	g.ask("ember")
	g.ask("ember")
	if lastText(g) != g.L.T("talk.hadrik.ember_again") {
		t.Errorf("asked again: %q", lastText(g))
	}
	g.ask("boneking")
	g.talk, g.Mode = nil, ModePlay
	talkWith(t, g, "villager")
	for _, o := range g.talk.options(g) {
		if o.id == "boneking" && o.asked {
			t.Error("asking Hadrik about the Bone King counts as asking the villagers")
		}
	}
}

// Every talk line is a greeting, a topic or a reward, with known
// variants: a misspelled fact would never be said.
func TestTalkVariants(t *testing.T) {
	en := locales.Get(i18n.Source)
	var factIDs []string
	for _, f := range facts {
		factIDs = append(factIDs, f.id)
	}
	for _, k := range en.Keys() {
		rest, ok := strings.CutPrefix(k, "talk.")
		if !ok || slices.Contains(questRewards[:], k) {
			continue
		}
		who, id, _ := strings.Cut(rest, ".")
		if !slices.Contains([]string{"hadrik", "mirela", "voss", "any"}, who) {
			t.Errorf("%s: no one is %q", k, who)
		}
		id = strings.TrimSuffix(id, "_again")
		if base, f, ok := strings.Cut(id, "_"); ok {
			switch {
			case f == "after" && topicFact[base] != "":
			case slices.Contains(factIDs, f):
			default:
				t.Errorf("%s: %q is not a fact", k, f)
			}
			id = base
		}
		if id != "greet" && (id == "rumors" || !slices.Contains(topics, id)) {
			t.Errorf("%s: %q is not a topic", k, id)
		}
	}
}
