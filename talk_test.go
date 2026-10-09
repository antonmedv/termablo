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
	for _, id := range []string{"crypt", "boneking", "portals"} {
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
	g.ask("forge") // his greeting's link, which links the Ember
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
	settle(g, "boneking")
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
	for _, id := range sights {
		linked[id] = true
	}
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
	cat, err := i18n.Parse("de", []byte(`{ talk: { voss: { greet_boneking: "Geht zur Seherin." } } }`))
	if err != nil {
		t.Fatal(err)
	}
	g.L = cat
	g.talk = &Talk{Who: "voss"}
	g.hear("", "talk.voss.greet_boneking")
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
	settle(g, "boneking")
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
	settle(g, "oracle")
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
	talkWith(t, g, "captain")
	press(m, "esc")
	slay(g, "boneking")
	var said []string
	for range 3 {
		talkWith(t, g, "captain")
		said = append(said, lastText(g))
		press(m, "esc")
	}
	want := []string{g.L.T("talk.voss.boneking_reward"), g.L.T("talk.voss.greet_boneking"), g.L.T("talk.voss.greet_boneking_again")}
	if !slices.Equal(said, want) || g.Quests["boneking"] != QuestRewarded {
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
// variants: a misspelled fact or look would never be said.
func TestTalkVariants(t *testing.T) {
	en := locales.Get(i18n.Source)
	var factIDs, rewards []string
	for _, q := range quests {
		factIDs, rewards = append(factIDs, q.ID), append(rewards, q.Reward)
	}
	for _, k := range en.Keys() {
		rest, ok := strings.CutPrefix(k, "talk.")
		if !ok || slices.Contains(rewards, k) {
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
			case slices.Contains(factIDs, f), slices.Contains(looks, f):
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

// greetings is what a speaker says on each of n visits.
func greetings(t *testing.T, g *Game, id string, n int) []string {
	t.Helper()
	m := newTestModel(g)
	var said []string
	for range n {
		talkWith(t, g, id)
		said = append(said, lastText(g))
		press(m, "esc")
	}
	return said
}

// Bosses killed between visits are one piece of news, the newest: the
// older is old.
func TestOnlyNewestNews(t *testing.T) {
	g := NewGame(1)
	greetings(t, g, "smith", 1)
	settle(g, "boneking", "oracle", "wanderer")
	got := greetings(t, g, "smith", 3)
	want := []string{g.L.T("talk.hadrik.greet_wanderer"), g.L.T("talk.hadrik.greet_again"), g.L.T("talk.hadrik.greet_again")}
	if !slices.Equal(got, want) {
		t.Errorf("Hadrik after three bosses: %q, want %q", got, want)
	}
}

// A speaker meets the hero before telling them the news.
func TestMeetBeforeNews(t *testing.T) {
	g := NewGame(1)
	settle(g, "boneking")
	got := greetings(t, g, "alch", 3)
	want := []string{g.L.T("talk.mirela.greet"), g.L.T("talk.mirela.greet_boneking"), g.L.T("talk.mirela.greet_again")}
	if !slices.Equal(got, want) {
		t.Errorf("Mirela met after the Bone King: %q, want %q", got, want)
	}
}

// The Oracle first: Voss pays, then briefs the hero he has not met, so
// the topics of his briefing are still learned.
func TestOracleBeforeBoneKing(t *testing.T) {
	g := NewGame(1)
	slay(g, "oracle")
	got := greetings(t, g, "captain", 2)
	want := []string{g.L.T("talk.voss.oracle_reward"), g.L.T("talk.voss.greet")}
	if !slices.Equal(got, want) || !g.Known["crypt"] {
		t.Errorf("Voss after the Oracle: %q, want %q", got, want)
	}
}

// Both quests paid on one visit; the next brings only the newest news.
func TestBothRewards(t *testing.T) {
	g := NewGame(1)
	greetings(t, g, "captain", 1)
	slay(g, "boneking", "oracle")
	talkWith(t, g, "captain")
	if n := len(g.talk.Log); n != 2 || g.bountyDue() != nil {
		t.Fatalf("%d lines, quests %v", n, g.Quests)
	}
	g.talk, g.Mode = nil, ModePlay
	if got := greetings(t, g, "captain", 1); got[0] != g.L.T("talk.voss.greet_oracle") {
		t.Errorf("after both rewards: %q", got[0])
	}
}

// A topic asked shows as asked until there is something new on it.
func TestAskedUntilNews(t *testing.T) {
	g := NewGame(1)
	g.Known["boneking"] = true
	talkWith(t, g, "captain")
	g.ask("boneking")
	asked := func() bool {
		for _, o := range g.talk.options(g) {
			if o.id == "boneking" {
				return o.asked
			}
		}
		t.Fatal("no Bone King topic")
		return false
	}
	if !asked() {
		t.Error("the Bone King not asked after asking")
	}
	settle(g, "boneking")
	if asked() {
		t.Error("the Bone King's death is nothing new")
	}
}

// Everyone with lines of their own greets the hero.
func TestEveryoneGreets(t *testing.T) {
	en := locales.Get(i18n.Source)
	for _, k := range en.Keys() {
		if rest, ok := strings.CutPrefix(k, "talk."); ok {
			if who, _, _ := strings.Cut(rest, "."); who != "any" && !en.Has("talk."+who+".greet") {
				t.Errorf("%s has no greet", who)
			}
		}
	}
}

// logged reports whether a line is in the log.
func logged(g *Game, text string) bool {
	return slices.ContainsFunc(g.Log, func(l LogMsg) bool { return l.Text == text })
}

// What the hero comes upon gives them something to ask about, and the
// log says so once.
func TestSightsTeachTopics(t *testing.T) {
	g := NewGame(1)
	g.changeLevel("crypt1", "", nil)
	news := g.L.T("msg.topic_new", "topic", g.L.T("topic.crypt"))
	if !g.Known["crypt"] || !logged(g, news) {
		t.Fatalf("the crypt not learned on entering it: %v", g.Known)
	}
	n := len(g.Log)
	g.changeLevel("crypt2", "crypt1", nil)
	if slices.ContainsFunc(g.Log[n:], func(l LogMsg) bool { return l.Text == news }) {
		t.Error("the crypt learned twice")
	}
	g.Mode = ModePlay
	press(newTestModel(g), "t")
	if !g.Known["portals"] {
		t.Error("reading a portal scroll does not teach portals")
	}
	cultist := NewMonster(g.rng, mtemps["cultist"], 1, RankNormal, g.Rules)
	cultist.X, cultist.Y = g.Lv.FreeNear(g.P.X+2, g.P.Y, g.P.X, g.P.Y)
	g.Lv.Monsters = append(g.Lv.Monsters, cultist)
	g.killMonster(cultist)
	if !g.Known["cult"] {
		t.Error("killing a cultist does not teach the Ember Cult")
	}
	g.useAltar(1, 1)
	if !g.Known["altars"] {
		t.Error("an altar does not teach altars")
	}

	// a unique teaches however it comes, bought as well as found
	g.changeLevel("town", "", nil)
	g.shop = g.shops[0]
	for _, u := range uniques {
		if u.Name == "Hollow Crown" {
			g.shop.Items = append(g.shop.Items, uniqueItem(u, u.Lvl))
		}
	}
	g.P.Gold = 1 << 20
	g.buy(g.shop.Items[len(g.shop.Items)-1])
	if !g.Known["edran"] {
		t.Error("buying the Hollow Crown does not teach King Edran")
	}
}

// tellAll is every rumor a villager tells over n tellings.
func tellAll(t *testing.T, g *Game, n int) []string {
	t.Helper()
	talkWith(t, g, "villager")
	told := []string{g.talk.rumor}
	for range n - 1 {
		g.ask("rumors")
		told = append(told, g.talk.rumor)
	}
	press(newTestModel(g), "esc")
	return told
}

// Villagers tell what the hero has not heard first, and what they tell
// follows the story: news once a boss is dead, old talk dropped.
func TestRumorsFollowTheStory(t *testing.T) {
	g := NewGame(1)
	now := 0
	for _, r := range rumors {
		if r.After == "" {
			now++
		}
	}
	told := tellAll(t, g, now)
	if s := slices.Clone(told); len(slices.Compact(slices.Sorted(slices.Values(s)))) != now {
		t.Errorf("a rumor told twice before all were: %q", told)
	}
	if slices.Contains(told, "rumor.bells") {
		t.Error("the crypt bells ring before the Bone King is dead")
	}
	settle(g, "boneking")
	if got := slices.Sorted(slices.Values(tellAll(t, g, 2))); !slices.Equal(got, []string{"rumor.bells", "rumor.face"}) {
		t.Errorf("after the Bone King: %q", got)
	}
	settle(g, "oracle")
	for _, k := range tellAll(t, g, 30) {
		if k == "rumor.brother" || k == "rumor.holding" || k == "rumor.crystal" {
			t.Errorf("%s still told after the Oracle", k)
		}
	}
}

// visit is what a speaker greets the hero with, the hero at hp of their
// life.
func visit(t *testing.T, g *Game, id string, hp float64) string {
	t.Helper()
	g.P.HP = hp * float64(g.P.MaxHP())
	return greetings(t, g, id, 1)[0]
}

// Speakers remark on how the hero looks, once they have met: over what
// they said before, never over news.
func TestLooksRemarked(t *testing.T) {
	g := NewGame(1)
	L := g.L.T
	if got := visit(t, g, "smith", .1); got != L("talk.hadrik.greet") {
		t.Errorf("met hurt: %q", got)
	}
	if got := visit(t, g, "smith", .1); got != L("talk.hadrik.greet_hurt") {
		t.Errorf("back hurt: %q", got)
	}
	if got := visit(t, g, "smith", 1); got != L("talk.hadrik.greet_again") {
		t.Errorf("back whole: %q", got)
	}
	// Mirela sees the wounds before she tends them.
	visit(t, g, "alch", 1)
	if got := visit(t, g, "alch", .1); got != L("talk.mirela.greet_hurt") {
		t.Errorf("Mirela on a hurt hero: %q", got)
	}

	settle(g, "boneking")
	got := []string{visit(t, g, "smith", .1), visit(t, g, "smith", .1)}
	if want := []string{L("talk.hadrik.greet_boneking"), L("talk.hadrik.greet_hurt")}; !slices.Equal(got, want) {
		t.Errorf("news and wounds: %q, want %q", got, want)
	}

	// Voss's standing orders give way to the axe on the hero's back.
	g = NewGame(1)
	visit(t, g, "captain", 1)
	for _, u := range uniques {
		if u.Name == "Kingsbane" {
			g.P.Eq[EqWeapon] = uniqueItem(u, u.Lvl)
		}
	}
	got = []string{visit(t, g, "captain", 1), visit(t, g, "captain", 1)}
	if want := []string{L("talk.voss.greet_kingsbane"), L("talk.voss.greet_kingsbane_again")}; !slices.Equal(got, want) {
		t.Errorf("Voss on Kingsbane: %q, want %q", got, want)
	}
}
