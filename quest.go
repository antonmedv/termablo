package main

import (
	"slices"
	"strconv"
)

// Quests are given by the townsfolk, in conversation. A quest is taken
// when the hero hears one of the lines that gives it, from whoever says
// it, and done when its boss dies, taken or not. Its giver pays for it
// the next time the hero talks to them, the reward line their greeting.
// A quest with no giver is taken by the story, when the boss it comes
// After dies, and asks no reward.

// Quest is a row of the quest table.
type Quest struct {
	ID     string   // the fact the town learns when it is settled
	Boss   string   // the monster template whose death completes it
	Depth  int      // where the boss sits: the reward's unique rolls there
	Giver  string   // the template ID of who pays for it, "" for none
	Offers []string // the lines that give it, talk.*
	Reward string   // what the giver says as they pay
	After  string   // the boss whose death gives it, for a quest no one gives
	Area   string   // where the boss waits, area.<Area>, for the journal
	Level  string   // the level the boss waits on: where the quest line leads
}

// quests are every quest, in the order they are listed and paid: the
// quest line, the order they are meant to be done in (route.go).
var quests = []Quest{
	{ID: "boneking", Boss: "boneking", Depth: 5, Area: "throne", Level: "crypt4", Giver: "captain", Reward: "talk.voss.boneking_reward",
		Offers: []string{"talk.voss.greet", "talk.voss.greet_again", "talk.voss.boneking"}},
	{ID: "barrow", Boss: "buried", Depth: 7, Area: "buried_watch", Level: "barrow2", Giver: "captain", Reward: "talk.voss.barrow_reward",
		Offers: []string{"talk.voss.barrow"}},
	{ID: "oracle", Boss: "oracle", Depth: 9, Area: "oracle_pool", Level: "grotto3", Giver: "captain", Reward: "talk.voss.oracle_reward",
		Offers: []string{"talk.voss.greet_boneking", "talk.voss.greet_boneking_again", "talk.voss.oracle"}},
	{ID: "wanderer", Boss: "wanderer", Area: "hearth", Level: "abyss" + strconv.Itoa(hearthFloor), After: "oracle"},
}

func questByID(id string) *Quest {
	for i := range quests {
		if quests[i].ID == id {
			return &quests[i]
		}
	}
	return nil
}

// QuestState is how far the hero has come with a quest: where they
// stand with its giver. Whether its boss is dead is Game.Slain.
type QuestState uint8

const (
	QuestUnknown QuestState = iota
	QuestTaken
	QuestRewarded
)

// done reports whether a quest's boss is dead.
func (g *Game) done(q *Quest) bool { return g.Slain[q.Boss] }

// QuestStatus is where a quest stands, read from the boss's death and
// the giver's pay: still to do, done with its giver yet to pay, or over.
// A boss killed before its quest is given is owed all the same.
type QuestStatus uint8

const (
	QuestOpen QuestStatus = iota
	QuestOwed
	QuestOver
)

// status is where a quest stands: over once paid for, or once done when
// there is no one to pay.
func (g *Game) status(q *Quest) QuestStatus {
	switch {
	case g.Quests[q.ID] == QuestRewarded, q.Giver == "" && g.done(q):
		return QuestOver
	case g.done(q):
		return QuestOwed
	}
	return QuestOpen
}

// bountyDue is the first quest a giver owes for, or nil.
func (g *Game) bountyDue() *Quest {
	for i := range quests {
		if g.status(&quests[i]) == QuestOwed {
			return &quests[i]
		}
	}
	return nil
}

// takeQuest gives the hero a quest, once.
func (g *Game) takeQuest(q *Quest) {
	if g.Quests[q.ID] != QuestUnknown {
		return
	}
	g.Quests[q.ID] = QuestTaken
	g.say(colGold, "msg.quest_new", "name", keyArg("monster."+q.Boss))
}

// offered takes the quests a line gives.
func (g *Game) offered(key string) {
	for i := range quests {
		if slices.Contains(quests[i].Offers, key) {
			g.takeQuest(&quests[i])
		}
	}
}

// slay records a boss's death, and takes the quests it gives.
func (g *Game) slay(boss string) {
	g.Slain[boss] = true
	for i := range quests {
		if quests[i].After == boss {
			g.takeQuest(&quests[i])
		}
	}
}

// briefed counts a giver's paying the hero they have not met as meeting
// them, when everything their greeting gives is over: it would brief the
// hero on work already done. Their news follows on the next visit.
func (g *Game) briefed(who string) {
	k := "talk." + who + ".greet"
	for i := range quests {
		if slices.Contains(quests[i].Offers, k) && g.status(&quests[i]) != QuestOver {
			return
		}
	}
	g.heard[who+" "+k] = true
}

// readyQuests is where the quests stand for a hero started at depth:
// those whose bosses sit above it done and paid, the rest given, as if
// the hero had met everyone on the way down.
func (g *Game) readyQuests(depth int) {
	for _, q := range quests {
		if q.Giver != "" && depth > q.Depth {
			g.Slain[q.Boss], g.Quests[q.ID] = true, QuestRewarded
			g.briefed(speaker(q.Giver))
		}
	}
	for _, q := range quests {
		if q.Giver != "" || g.Slain[q.After] {
			g.Quests[q.ID] = max(g.Quests[q.ID], QuestTaken)
		}
	}
}

// payQuests is what a giver pays for every quest they owe, taken or
// not: gold by the hero's level and a unique by the quest's depth, not
// the hero's. It returns the reward lines.
func (g *Game) payQuests(giver *Monster) []string {
	var said []string
	p := g.P
	for i := range quests {
		q := &quests[i]
		if q.Giver != giver.T.ID || g.status(q) != QuestOwed {
			continue
		}
		g.Quests[q.ID] = QuestRewarded
		said = append(said, q.Reward)
		gold := int(g.Rules.QuestGoldPerLvl * float64(p.Lvl))
		p.Gold += gold
		g.Stats.In[GoldQuest] += gold
		it := GenItem(g.rng, q.Depth+2, RUnique, SlotNone, g.Rules)
		g.dropItem(p.X, p.Y, it)
		// the item first: its gender picks the line's form, not the giver's
		g.say(colGold, "msg.quest_reward", "item", itemNoun(g.L, it), "who", monsterNoun(g.L, giver), "n", gold)
	}
	return said
}

// questsShown are the quests the panel lists: the journal's first n,
// those over last.
func (g *Game) questsShown(n int) []*Quest {
	shown := g.journal()
	return shown[:min(n, len(shown))]
}
