package main

// Stats counts what happens in a run: gold by source and sink, potions,
// damage, kills by rank, equips, turns per level. Counters only,
// incremented where the events already happen; the bot report reads them
// (BALANCE.md §2.2). The wallet reconciles: Gold == 60 + sum(In) - sum(Out).

type GoldSrc int

const (
	GoldDrop GoldSrc = iota // dropped by a monster
	GoldChest
	GoldSale
	GoldQuest
	GoldSrcCount
)

type GoldSink int

const (
	SinkPotion GoldSink = iota
	SinkScroll
	SinkHeal
	SinkGear
	SinkReroll // §4 E, nothing charges it yet
	SinkGamble
	SinkCount
)

var goldSrcNames = [GoldSrcCount]string{"drop", "chest", "sell", "quest"}
var goldSinkNames = [SinkCount]string{"potion", "scroll", "heal", "gear", "reroll", "gamble"}

type Stats struct {
	In       [GoldSrcCount]int
	Out      [SinkCount]int
	HPots    int // healing potions drunk
	MPots    int // mana potions drunk
	DmgDealt int
	DmgTaken int
	Kills    [RankBoss + 1]int // by rank
	Equips   int
	Turns    map[string]int // by level ID
}

func newStats() Stats { return Stats{Turns: map[string]int{}} }

// GoldIn and GoldOut total the sources and the sinks.
func (s *Stats) GoldIn() int { return sum(s.In[:]) }

func (s *Stats) GoldOut() int { return sum(s.Out[:]) }

// sinkOf is where the price of a shop item goes.
func sinkOf(it *Item) GoldSink {
	switch it.Kind {
	case IKHealth, IKMana:
		return SinkPotion
	case IKScroll:
		return SinkScroll
	}
	return SinkGear
}

func sum(xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t
}
