package main

import "testing"

// The wallet reconciles against the counters on every turn of a bot run:
// every coin in and out is booked under a source or a sink.
func TestStatsGoldReconciles(t *testing.T) {
	for _, pol := range botPolicies {
		for seed := int64(1); seed <= 3; seed++ {
			g := NewGame(seed)
			g.Mode = ModePlay
			b := NewBot(g, pol)
			for calls := 0; g.Turn < 3000 && calls < 6000 && g.Mode != ModeDead && b.State() != BotStuck; calls++ {
				b.turn()
				s := &g.Stats
				if want := 60 + s.GoldIn() - s.GoldOut(); g.P.Gold != want {
					t.Fatalf("%s seed %d turn %d: gold %d, counters say %d (in %v, out %v)", pol.name, seed, g.Turn, g.P.Gold, want, s.In, s.Out)
				}
			}
			s := g.Stats
			t.Logf("%s seed %d: turn %d, gold %d = 60 +%v -%v · potions %d/%d · dealt %d taken %d · kills %v · equips %d · levels %d",
				pol.name, seed, g.Turn, g.P.Gold, s.In, s.Out, s.HPots, s.MPots, s.DmgDealt, s.DmgTaken, s.Kills, s.Equips, len(s.Turns))
			if s.GoldIn() == 0 || s.DmgDealt == 0 || s.Kills[RankNormal] == 0 || len(s.Turns) < 2 {
				t.Errorf("%s seed %d: counters never moved: %+v", pol.name, seed, s)
			}
		}
	}
}
