package main

import (
	"slices"
	"strconv"
)

// Quests are given by the townsfolk, in conversation. A quest is taken
// when the hero hears one of the lines that gives it, from whoever says
// it, and done when its boss dies, or the thing it is after is picked
// up, taken or not. Its giver pays for it the next time the hero talks
// to them, the reward line their greeting, and takes back what was
// brought. A quest with no giver is taken by the story, when the boss
// it comes After dies, and asks no reward.

// Quest is a row of the quest table.
type Quest struct {
	ID     string   // the fact the town learns when it is settled
	Boss   string   // the monster template whose death completes it, or
	Item   string   // the unique whose pickup does: a thing to bring back
	Depth  int      // where the boss or the thing sits: the reward's unique rolls there
	Giver  string   // the template ID of who pays for it, "" for none
	Offers []string // the lines that give it, talk.*
	Reward string   // what the giver says as they pay
	After  string   // the boss whose death gives it, for a quest no one gives
	Alarm  string   // what is said as the thing is picked up, msg.*, which wakes the floor: a called-for thing
	Area   string   // where the boss or the thing waits, area.<Area>, for the journal
	Level  string   // the level it waits on: where the quest line leads
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
	{ID: "coal", Item: stolenCoal.Name, Alarm: "msg.coal_taken", Depth: 10, Area: "sanctum", Level: "sanctum1", Giver: "smith", Reward: "talk.hadrik.coal_reward",
		Offers: []string{"talk.hadrik.greet_oracle", "talk.hadrik.coal"}},
	{ID: "prior", Boss: "prior", Depth: 11, Area: "kindling", Level: "sanctum2", Giver: "exile", Reward: "talk.aldous.prior_reward",
		Offers: []string{"talk.aldous.greet_oracle", "talk.aldous.prior"}},
	{ID: "wanderer", Boss: "wanderer", Area: "hearth", Level: "abyss" + strconv.Itoa(hearthFloor), After: "oracle"},
}

// questByItem is the quest after a unique, by name, or nil.
func questByItem(name string) *Quest {
	for i := range quests {
		if quests[i].Item == name {
			return &quests[i]
		}
	}
	return nil
}

func questByID(id string) *Quest {
	for i := range quests {
		if quests[i].ID == id {
			return &quests[i]
		}
	}
	return nil
}

// nameKey is the catalog key of what a quest is after, as the lists
// name it: its boss, or the thing to bring back.
func (q *Quest) nameKey() string {
	if q.Item != "" {
		return "unique." + slug(q.Item) + ".name"
	}
	return "monster." + q.Boss
}

// QuestState is how far the hero has come with a quest: where they
// stand with its giver. Whether its boss is dead is Game.Slain, and
// whether its thing was picked up Game.Found.
type QuestState uint8

const (
	QuestUnknown QuestState = iota
	QuestTaken
	QuestRewarded
)

// done reports whether a quest's boss is dead, or its thing picked up.
func (g *Game) done(q *Quest) bool {
	if q.Item != "" {
		return g.Found[q.Item]
	}
	return g.Slain[q.Boss]
}

// complete makes a quest done off the map: its boss dead, or its thing
// found.
func (g *Game) complete(q *Quest) {
	if q.Item != "" {
		g.Found[q.Item] = true
		return
	}
	g.slay(q.Boss)
}

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
	g.say(colGold, "msg.quest_new", "name", keyArg(q.nameKey()))
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

// found records a unique come into the pack, and what finding it does:
// a quest's thing with an alarm is a called-for thing, and taking it up
// wakes the floor.
func (g *Game) found(it *Item) {
	if g.Found[it.Name] {
		return
	}
	g.Found[it.Name] = true
	if q := questByItem(it.Name); q != nil && q.Alarm != "" {
		g.stir()
		g.say(colOrange, q.Alarm)
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
	for i := range quests {
		if q := &quests[i]; q.Giver != "" && depth > q.Depth {
			// done off the map, quietly: what their bosses give is given below
			if q.Item != "" {
				g.Found[q.Item] = true
			} else {
				g.Slain[q.Boss] = true
			}
			g.Quests[q.ID] = QuestRewarded
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
// the hero's, taking back what the quest brought. It returns the reward
// lines.
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
		if it := g.takeBack(q.Item); it != nil {
			g.say(colGold, "msg.quest_take", "who", monsterNoun(g.L, giver), "item", itemNoun(g.L, it))
		}
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

// takeBack takes a unique out of the hero's pack or off their body, and
// returns it, or nil when they no longer have it.
func (g *Game) takeBack(name string) *Item {
	p := g.P
	if name == "" {
		return nil
	}
	for i, it := range p.Inv {
		if it.Rarity == RUnique && it.Name == name {
			p.Inv = slices.Delete(p.Inv, i, i+1)
			return it
		}
	}
	for i, it := range p.Eq {
		if it != nil && it.Rarity == RUnique && it.Name == name {
			p.Eq[i] = nil
			p.recalc()
			return it
		}
	}
	return nil
}

// questsShown are the quests the panel lists: the journal's first n,
// those over last.
func (g *Game) questsShown(n int) []*Quest {
	shown := g.journal()
	return shown[:min(n, len(shown))]
}
