package main

// The quest line is the way through the game: the quests in the order of
// the table, each to its boss's level by the shortest walk over the
// levels' links, and past the last one, on down. The bot walks it, and
// docs/quests.md is drawn from it (make docs).

// questAhead is the first quest on the line whose boss still stands, or
// nil once the line is walked.
func (g *Game) questAhead() *Quest {
	for i := range quests {
		if !g.done(&quests[i]) {
			return &quests[i]
		}
	}
	return nil
}

// way is the shortest walk from one level to another over their links,
// both ends included, or nil when there is none within reach. Levels on
// the way are built as it looks.
func (g *Game) way(from, to string) []string {
	const reach = 64 // levels looked at; past the Hearth the Abyss never ends
	prev := map[string]string{from: ""}
	for q := []string{from}; len(q) > 0 && len(prev) <= reach; q = q[1:] {
		id := q[0]
		if id == to {
			var w []string
			for ; id != ""; id = prev[id] {
				w = append([]string{id}, w...)
			}
			return w
		}
		for _, lk := range g.getLevel(id).Links {
			if _, ok := prev[lk.To]; !ok {
				prev[lk.To] = id
				q = append(q, lk.To)
			}
		}
	}
	return nil
}

// step is the next level on the way from one level to another, or ""
// when already there or there is no way.
func (g *Game) step(from, to string) string {
	if w := g.way(from, to); len(w) > 1 {
		return w[1]
	}
	return ""
}

// routeNext is where the quest line leads from a level: a step toward
// the first boss still standing, "" on its level, and once the line is
// walked, down the deepest link past the last boss's level.
func (g *Game) routeNext(id string) string {
	if q := g.questAhead(); q != nil {
		return g.step(id, q.Level)
	}
	last := quests[len(quests)-1].Level
	l := g.getLevel(id)
	if l.Depth < g.getLevel(last).Depth {
		return g.step(id, last)
	}
	next, depth := "", l.Depth
	for _, lk := range l.Links {
		if d := g.getLevel(lk.To).Depth; d > depth {
			next, depth = lk.To, d
		}
	}
	return next
}
