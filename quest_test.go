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
		g.complete(q)
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
		switch {
		case q.Boss != "" && q.Item != "":
			t.Errorf("%s: a boss and a thing both", q.ID)
		case q.Item != "" && q.Item != stolenCoal.Name:
			t.Errorf("%s: no unique %q", q.ID, q.Item)
		case q.Item == "" && mtemps[q.Boss] == nil:
			t.Errorf("%s: no monster %q", q.ID, q.Boss)
		}
		if !en.Has(q.nameKey()) {
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
		if q.Alarm != "" && (q.Item == "" || !en.Has(q.Alarm)) {
			t.Errorf("%s: an alarm %q with no thing to pick up, or no line", q.ID, q.Alarm)
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
	if len(got) != 2 || got[0].ID != "barrow" || got[1].ID != "oracle" {
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
		{3, map[string]QuestState{"boneking": QuestTaken, "barrow": QuestTaken, "oracle": QuestTaken, "coal": QuestTaken, "prior": QuestTaken}},
		{7, map[string]QuestState{"boneking": QuestRewarded, "barrow": QuestTaken, "oracle": QuestTaken, "coal": QuestTaken, "prior": QuestTaken}},
		{11, map[string]QuestState{"boneking": QuestRewarded, "barrow": QuestRewarded, "oracle": QuestRewarded, "coal": QuestRewarded, "prior": QuestTaken, "wanderer": QuestTaken}},
		{12, map[string]QuestState{"boneking": QuestRewarded, "barrow": QuestRewarded, "oracle": QuestRewarded, "coal": QuestRewarded, "prior": QuestRewarded, "wanderer": QuestTaken}},
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

// Voss gives the barrow when asked about it, and pays once the Buried
// Captain, waiting on the second floor, lowers his spear.
func TestBarrowQuest(t *testing.T) {
	g := NewGame(1)
	g.Known["barrow"] = true
	talkWith(t, g, "captain")
	g.ask("barrow")
	if g.Quests["barrow"] != QuestTaken {
		t.Fatal("asking Voss about the barrow does not give it")
	}
	g.Mode = ModePlay
	g.changeLevel("barrow2", "", nil)
	for _, m := range g.Lv.Monsters {
		if m.T.ID == "buried" {
			g.killMonster(m)
		}
	}
	g.changeLevel("town", "", nil)
	if g.status(questByID("barrow")) != QuestOwed {
		t.Fatalf("the Buried Captain is not on barrow2, or his death does not count: %v", g.Slain)
	}
	gold := g.P.Gold
	if got := greetings(t, g, "captain", 1); got[0] != g.L.T("talk.voss.barrow_reward") || g.P.Gold <= gold {
		t.Errorf("Voss on the barrow: %q, gold %d->%d", got, gold, g.P.Gold)
	}
}

// Hurt past half, the Buried Captain wakes his watch, every sleeper
// with a way to the hero, once.
func TestBuriedCaptainRallies(t *testing.T) {
	g := NewGame(1)
	g.changeLevel("barrow2", "", nil)
	var c *Monster
	for _, m := range g.Lv.Monsters {
		if m.T.ID == "buried" {
			c = m
		}
	}
	if c == nil {
		t.Fatal("no Buried Captain on barrow2")
	}
	g.P.X, g.P.Y = g.Lv.FreeNear(c.X+3, c.Y, -1, -1)
	g.computeDist()
	c.Awake, c.HP = true, c.MaxHP/3
	g.monsterTurn(c)
	for _, m := range g.Lv.Monsters {
		if g.dist[g.Lv.Idx(m.X, m.Y)] < 0 {
			continue // too far to come
		}
		if !m.Dead && !m.Awake {
			t.Fatalf("%s still sleeps after the rally", m.Name)
		}
	}
	if !c.Spent {
		t.Error("the rally is not remembered")
	}
}

// coalOn finds the Stolen Coal on a level.
func coalOn(t *testing.T, l *Level) *FloorItem {
	t.Helper()
	for _, fi := range l.Items {
		if fi.It.Name == stolenCoal.Name {
			return fi
		}
	}
	t.Fatalf("no Stolen Coal on %s", l.ID)
	return nil
}

// Hadrik gives the coal when asked about it; it lies in the Sanctum's
// reliquary, taking it up wakes the floor, and Hadrik takes it back as he
// pays, off the hero's neck if need be.
func TestCoalQuest(t *testing.T) {
	g := NewGame(1)
	g.Known["coal"] = true
	talkWith(t, g, "smith")
	g.ask("coal")
	if g.Quests["coal"] != QuestTaken {
		t.Fatal("asking Hadrik about the coal does not give it")
	}
	g.Mode = ModePlay
	g.changeLevel("sanctum1", "", nil)
	fi := coalOn(t, g.Lv)
	if fi.Light == nil {
		t.Error("the coal does not glow")
	}
	g.P.X, g.P.Y = fi.X, fi.Y
	g.computeVisibility()
	g.computeDist()
	asleep := 0
	for _, m := range g.Lv.Monsters {
		if !m.Awake && g.dist[g.Lv.Idx(m.X, m.Y)] >= 0 {
			asleep++
		}
	}
	if asleep == 0 {
		t.Fatal("nothing to wake")
	}
	g.pickup()
	if !g.Found[stolenCoal.Name] || g.status(questByID("coal")) != QuestOwed {
		t.Fatalf("the coal picked up: found %v, quests %v", g.Found, g.Quests)
	}
	if taken := g.L.T("msg.coal_taken"); !slices.ContainsFunc(g.Log, func(l LogMsg) bool { return l.Text == taken }) {
		t.Error("no word of the coal flaring")
	}
	for _, m := range g.Lv.Monsters {
		if !m.Dead && !m.Awake && g.dist[g.Lv.Idx(m.X, m.Y)] >= 0 {
			t.Fatalf("%s sleeps through the coal", m.Name)
		}
	}
	// worn, it still goes back
	for i, it := range g.P.Inv {
		if it.Name == stolenCoal.Name {
			g.equip(i)
		}
	}
	if g.P.Eq[EqAmulet] == nil || g.P.Eq[EqAmulet].Name != stolenCoal.Name {
		t.Fatal("the coal is not an amulet the hero can wear")
	}
	g.changeLevel("town", "", nil)
	gold := g.P.Gold
	got := greetings(t, g, "smith", 1)
	if got[0] != g.L.T("talk.hadrik.coal_reward") || g.P.Gold <= gold {
		t.Errorf("Hadrik on the coal: %q, gold %d->%d", got, gold, g.P.Gold)
	}
	if g.P.Eq[EqAmulet] != nil || slices.ContainsFunc(g.P.Inv, func(it *Item) bool { return it.Name == stolenCoal.Name }) {
		t.Error("Hadrik left the hero the coal")
	}
	if g.status(questByID("coal")) != QuestOver {
		t.Error("the coal quest is not over")
	}
}

// Aldous gives the Prior when asked about him, and pays once he is ash
// at the bottom of the Kindling.
func TestPriorQuest(t *testing.T) {
	g := NewGame(1)
	g.Known["prior"] = true
	talkWith(t, g, "exile")
	g.ask("prior")
	if g.Quests["prior"] != QuestTaken {
		t.Fatal("asking Aldous about the Prior does not give him")
	}
	g.Mode = ModePlay
	g.changeLevel("sanctum2", "", nil)
	for _, m := range g.Lv.Monsters {
		if m.T.ID == "prior" {
			g.killMonster(m)
		}
	}
	g.changeLevel("town", "", nil)
	if g.status(questByID("prior")) != QuestOwed {
		t.Fatalf("the Prior is not on sanctum2, or his death does not count: %v", g.Slain)
	}
	gold := g.P.Gold
	if got := greetings(t, g, "exile", 1); got[0] != g.L.T("talk.aldous.prior_reward") || g.P.Gold <= gold {
		t.Errorf("Aldous on the Prior: %q, gold %d->%d", got, gold, g.P.Gold)
	}
}

// After the Oracle is paid for, Hadrik's and Aldous's news give their
// quests.
func TestSanctumQuestsGivenAfterOracle(t *testing.T) {
	g := NewGame(1)
	greetings(t, g, "smith", 1)
	greetings(t, g, "exile", 1)
	settle(g, "oracle")
	greetings(t, g, "smith", 1)
	greetings(t, g, "exile", 1)
	if g.Quests["coal"] != QuestTaken || g.Quests["prior"] != QuestTaken {
		t.Errorf("after the Oracle: %v", g.Quests)
	}
}

// priorOn finds the Cinder Prior on the Kindling, the hero beside him.
func priorOn(t *testing.T, g *Game) *Monster {
	t.Helper()
	g.changeLevel("sanctum2", "", nil)
	for _, m := range g.Lv.Monsters {
		if m.T.ID == "prior" {
			g.P.X, g.P.Y = g.Lv.FreeNear(m.X+2, m.Y, -1, -1)
			g.computeVisibility()
			g.computeDist()
			return m
		}
	}
	t.Fatal("no Cinder Prior on sanctum2")
	return nil
}

// Every tenth turn in sight of the hero, the Prior kindles the braziers
// near them: Fire Imps climb out, his minions.
func TestPriorKindles(t *testing.T) {
	g := NewGame(1)
	m := priorOn(t, g)
	l, p := g.Lv, g.P
	for _, d := range []Pos{{2, 1}, {-2, -1}, {3, -1}} {
		x, y := l.FreeNear(p.X+d.X, p.Y+d.Y, p.X, p.Y)
		l.Set(x, y, TBrazier)
	}
	g.computeVisibility()
	m.Awake, m.Timer = true, 9
	g.monsterTurn(m)
	imps := 0
	for _, o := range g.Lv.Monsters {
		if o.T.ID == "imp" && o.Minion && o.Awake {
			imps++
		}
	}
	if imps != 2 || lastLog(g) != g.L.T("msg.prior_kindles") {
		t.Errorf("%d imps kindled; last line %q", imps, lastLog(g))
	}
}

// Hurt past half, the fire takes the Prior once: faster, burning, and
// his flock awake.
func TestPriorBurns(t *testing.T) {
	g := NewGame(1)
	m := priorOn(t, g)
	speed := m.Speed
	m.Awake, m.HP = true, m.MaxHP/3
	g.monsterTurn(m)
	if !m.Spent || m.Speed <= speed || !m.HasMod(ModFire) || lastLog(g) != g.L.T("msg.prior_burns") {
		t.Errorf("the Prior at a third: spent %v, speed %d->%d, mods %v, last line %q", m.Spent, speed, m.Speed, m.Mods, lastLog(g))
	}
	for _, o := range g.Lv.Monsters {
		if !o.Dead && !o.Awake && g.dist[g.Lv.Idx(o.X, o.Y)] >= 0 {
			t.Fatalf("%s sleeps through the burning", o.Name)
		}
	}
	m.Timer = 1
	g.monsterTurn(m)
	if m.HasMod(ModFire) && len(m.Mods) != 1 {
		t.Error("the fire took him twice")
	}
}
