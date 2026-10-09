package main

import (
	"slices"
	"testing"

	"github.com/antonmedv/termablo/internal/i18n"
)

// slay kills bosses off the map.
func slay(g *Game, bosses ...string) {
	for _, b := range bosses {
		g.slay(b)
	}
}

// settle makes quests over: the boss dead, the giver paid.
func settle(g *Game, ids ...string) {
	for _, id := range ids {
		q := questByID(id)
		g.slay(q.Boss)
		if q.Giver != "" {
			g.Quests[id] = QuestRewarded
		}
	}
}

func taken(g *Game) []string {
	var ids []string
	for _, q := range quests {
		if g.Quests[q.ID] != QuestUnknown {
			ids = append(ids, q.ID)
		}
	}
	return ids
}

// The table holds together: every boss and giver is a monster, every
// line is in the catalog and is said by the giver it names.
func TestQuestTable(t *testing.T) {
	en := locales.Get(i18n.Source)
	seen := map[string]bool{}
	for _, q := range quests {
		if seen[q.ID] {
			t.Errorf("%s: twice", q.ID)
		}
		seen[q.ID] = true
		if mtemps[q.Boss] == nil {
			t.Errorf("%s: no monster %q", q.ID, q.Boss)
		}
		if !en.Has("monster." + q.Boss) {
			t.Errorf("%s: no name for the list", q.ID)
		}
		if !en.Has("area." + q.Area) {
			t.Errorf("%s: no area %q for the journal", q.ID, q.Area)
		}
		for _, st := range journalStages {
			if want := st != "rewarded" || q.Giver != ""; en.Has("journal."+q.ID+"."+st) != want {
				t.Errorf("%s: journal entry %s: want %v", q.ID, st, want)
			}
		}
		if q.Giver == "" {
			if q.Reward != "" {
				t.Errorf("%s: a reward and no one to pay it", q.ID)
			}
			continue
		}
		if mtemps[q.Giver] == nil {
			t.Errorf("%s: no giver %q", q.ID, q.Giver)
		}
		for _, k := range append([]string{q.Reward}, q.Offers...) {
			if !en.Has(k) {
				t.Errorf("%s: no line %s", q.ID, k)
			}
		}
	}
}

// A new hero has no quests; Voss gives the first on meeting, and the
// panel lists only what has been given.
func TestQuestsCollected(t *testing.T) {
	g := NewGame(1)
	g.Mode = ModePlay
	s := NewScreen(120, 40)
	g.Draw(s)
	if len(taken(g)) != 0 || screenHas(s, "Bone King") {
		t.Fatalf("quests before anyone gives one: %v", taken(g))
	}
	talkWith(t, g, "captain")
	if got := taken(g); !slices.Equal(got, []string{"boneking"}) {
		t.Errorf("after meeting Voss: %v", got)
	}
	press(newTestModel(g), "esc")
	g.Draw(s)
	if !screenHas(s, "Bone King") || screenHas(s, "Drowned Oracle") {
		t.Error("the panel does not list just the Bone King")
	}
	news := g.L.T("msg.quest_new", "name", g.L.T("monster.boneking"))
	if !slices.ContainsFunc(g.Log, func(l LogMsg) bool { return l.Text == news }) {
		t.Error("no word of the new quest")
	}
}

// Voss gives the Oracle once the Bone King is paid for, or sooner when
// asked about her.
func TestOracleGiven(t *testing.T) {
	g := NewGame(1)
	greetings(t, g, "captain", 1)
	slay(g, "boneking")
	greetings(t, g, "captain", 1) // the reward
	if g.Quests["oracle"] != QuestUnknown {
		t.Fatal("the Oracle given with the Bone King's reward")
	}
	greetings(t, g, "captain", 1)
	if g.Quests["oracle"] != QuestTaken {
		t.Error("the Oracle not given after the reward")
	}

	g = NewGame(1)
	g.Known["oracle"] = true
	talkWith(t, g, "captain")
	g.ask("oracle")
	if g.Quests["oracle"] != QuestTaken {
		t.Error("asking Voss about the Oracle does not give her")
	}
}

// Only the giver's lines give a quest: the town may talk of a boss.
func TestOthersDoNotGive(t *testing.T) {
	g := NewGame(1)
	g.Known["boneking"], g.Known["oracle"] = true, true
	talkWith(t, g, "villager")
	g.ask("boneking")
	g.ask("oracle")
	if got := taken(g); len(got) != 0 {
		t.Errorf("a villager gave %v", got)
	}
}

// A boss killed before its quest is given is still paid for.
func TestPaidUntaken(t *testing.T) {
	g := NewGame(1)
	slay(g, "boneking")
	gold := g.P.Gold
	got := greetings(t, g, "captain", 1)
	if got[0] != g.L.T("talk.voss.boneking_reward") || g.P.Gold <= gold || g.status(questByID("boneking")) != QuestOver {
		t.Errorf("Voss on an untaken quest: %q, gold %d->%d", got, gold, g.P.Gold)
	}
}

// The Oracle's death sets the hero on the Last Wanderer, and his own
// death ends it with nothing to collect.
func TestWandererQuest(t *testing.T) {
	g := NewGame(1)
	q := questByID("wanderer")
	g.changeLevel("grotto3", "", nil)
	for _, m := range g.Lv.Monsters {
		if m.T.ID == "oracle" {
			g.killMonster(m)
		}
	}
	if g.Quests["wanderer"] != QuestTaken || g.status(q) == QuestOver {
		t.Fatalf("after the Oracle: %v", g.Quests)
	}
	slay(g, "wanderer")
	if g.status(q) != QuestOver {
		t.Error("the Last Wanderer's death does not end his quest")
	}
}

// When the list is too long, the quests over make way.
func TestQuestsShown(t *testing.T) {
	g := NewGame(1)
	for _, q := range quests {
		g.Quests[q.ID] = QuestTaken
	}
	settle(g, "boneking")
	got := g.questsShown(2)
	if len(got) != 2 || got[0].ID != "oracle" || got[1].ID != "wanderer" {
		t.Errorf("shown %v", got)
	}
}

// Paid for a Bone King killed before they met, Voss skips the briefing on
// him and gives the Oracle on the next visit.
func TestOracleAfterUntakenKing(t *testing.T) {
	g := NewGame(1)
	slay(g, "boneking")
	got := greetings(t, g, "captain", 2)
	want := []string{g.L.T("talk.voss.boneking_reward"), g.L.T("talk.voss.greet_boneking")}
	if !slices.Equal(got, want) || g.Quests["oracle"] != QuestTaken {
		t.Errorf("Voss after an untaken Bone King: %q, want %q; quests %v", got, want, g.Quests)
	}
}

// A hero readied deep has every quest on the way: done above, given
// below, the Last Wanderer once the Oracle is dead.
func TestReadyQuests(t *testing.T) {
	for _, c := range []struct {
		depth int
		want  map[string]QuestState
	}{
		{3, map[string]QuestState{"boneking": QuestTaken, "oracle": QuestTaken}},
		{7, map[string]QuestState{"boneking": QuestRewarded, "oracle": QuestTaken}},
		{11, map[string]QuestState{"boneking": QuestRewarded, "oracle": QuestRewarded, "wanderer": QuestTaken}},
	} {
		g := NewGame(1)
		g.readyQuests(c.depth)
		if len(g.Quests) != len(c.want) {
			t.Errorf("depth %d: %v, want %v", c.depth, g.Quests, c.want)
		}
		for id, st := range c.want {
			if g.Quests[id] != st {
				t.Errorf("depth %d: %v, want %v", c.depth, g.Quests, c.want)
			}
		}
	}

	// Voss has nothing to brief on a king the hero is paid for: his
	// news comes first.
	g := NewGame(1)
	g.readyQuests(7)
	if got := greetings(t, g, "captain", 1)[0]; got != g.L.T("talk.voss.greet_boneking") {
		t.Errorf("Voss to a hero started deep: %q", got)
	}
}

// The journal opens on J or a click on the panel's quests, lists the
// open quests first, and tells the one picked: what to do, who gave it,
// where, and an entry for each stage it has reached.
func TestJournal(t *testing.T) {
	g := NewGame(1)
	g.Mode = ModePlay
	m := newTestModel(g)
	s := NewScreen(120, 40)
	press(m, "J")
	g.Draw(s)
	if g.Mode != ModeQuests || !screenHas(s, "No one has asked") {
		t.Fatalf("the journal with no quests: mode %v", g.Mode)
	}
	press(m, "esc")
	talkWith(t, g, "captain")
	press(m, "esc")
	g.Draw(s)
	g.Click(g.questsHit.X0, g.questsHit.Y+1, s.W, s.H)
	g.Draw(s)
	if g.Mode != ModeQuests || !screenHas(s, "must be destroyed") || !screenHas(s, "Given by Captain Voss") ||
		!screenHas(s, "Throne of the Bone King") || !screenHas(s, "Captain Voss wants") || screenHas(s, "is dust") {
		t.Errorf("the Bone King given: mode %v\n%s", g.Mode, screenText(s))
	}

	settle(g, "boneking")
	g.Quests["oracle"] = QuestTaken
	if got := g.journal(); got[0].ID != "oracle" || got[1].ID != "boneking" {
		t.Errorf("listed %s, %s: the open quest not first", got[0].ID, got[1].ID)
	}
	press(m, "down")
	g.Draw(s)
	if !screenHas(s, "Completed.") || !screenHas(s, "is dust") || !screenHas(s, "father's") || screenHas(s, "Where:") {
		t.Errorf("the Bone King over:\n%s", screenText(s))
	}
	press(m, "J")
	if g.Mode != ModePlay {
		t.Errorf("J left the journal in mode %v", g.Mode)
	}
}
