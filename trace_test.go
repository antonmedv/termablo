package main

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestBotTrace plays one policy on one seed and prints the game log
// around the end, for reading a death without the screen:
//
//	BOTTRACE=fighter:7 go test -count=1 -v -run BotTrace .
//	BOTTRACE=caster:3:60  (the last 60 lines; 40 by default)
//
// BOTRULES applies. The lines are what the player would have read,
// with the bot's own state changes among them.
func TestBotTrace(t *testing.T) {
	spec := os.Getenv("BOTTRACE")
	if spec == "" {
		t.Skip("set BOTTRACE=policy:seed[:lines]")
	}
	parts := strings.Split(spec, ":")
	if len(parts) < 2 {
		t.Fatalf("BOTTRACE=%q: want policy:seed[:lines]", spec)
	}
	pol := policyByName(parts[0])
	seed, err := strconv.ParseInt(parts[1], 10, 64)
	if pol == nil || err != nil {
		t.Fatalf("BOTTRACE=%q: want fighter|caster and a seed", spec)
	}
	lines := 40
	if len(parts) > 2 {
		lines, _ = strconv.Atoi(parts[2])
	}
	r, _, err := rulesFromFile(os.Getenv("BOTRULES"))
	if err != nil {
		t.Fatal(err)
	}
	g := NewGameWith(seed, r)
	g.Mode = ModePlay
	b := NewBot(g, pol)
	var log []string
	seen := 0
	last := ""
	for calls := 0; g.Turn < botTurns && calls < 2*botTurns && g.Mode != ModeDead && b.State() != BotStuck; calls++ {
		b.turn()
		// the bot's own state and reason, whenever either changes
		if now := b.State().String() + ": " + b.Why(); now != last {
			last = now
			log = append(log, "turn "+strconv.Itoa(g.Turn)+" "+g.Lv.ID+" @"+strconv.Itoa(g.P.X)+","+strconv.Itoa(g.P.Y)+" | bot "+now)
		}
		for fresh := min(g.logN-seen, len(g.Log)); fresh > 0; fresh-- {
			m := g.Log[len(g.Log)-fresh]
			log = append(log, "turn "+strconv.Itoa(m.Turn)+" "+g.Lv.ID+" hp "+strconv.Itoa(int(g.P.HP))+"/"+strconv.Itoa(g.P.MaxHP())+" pots "+strconv.Itoa(g.P.HPot)+" scrolls "+strconv.Itoa(g.P.Scrolls)+": "+m.Text)
		}
		seen = g.logN
		if len(log) > 4*lines {
			log = log[len(log)-2*lines:]
		}
	}
	if len(log) > lines {
		log = log[len(log)-lines:]
	}
	p := g.P
	t.Logf("%s seed %d: clvl %d, %s, turn %d, deepest %s, killed by %q", pol.name, seed, p.Lvl, b.State(), g.Turn, b.Stats.Deepest, p.KilledBy)
	for _, l := range log {
		t.Log(l)
	}
}
