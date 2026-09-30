package main

import (
	"testing"
)

// The sampler: one hero against one monster through the real combat code,
// many kills and deaths averaged. ref_test.go prints a table of it; the
// bot report runs it on the snapshots taken at level arrival. Ranged
// monsters fight in melee here too.

// A duel is what the sampler measures.
type duel struct {
	Swings float64 // melee swings to kill
	Bolts  float64 // firebolts to kill
	Turns  float64 // player turns survived from full life, no potions
	DPT    float64 // damage per monster attack, misses included
}

// An arena is a closed room: the hero in the middle, the monster beside it.
type arena struct {
	g *Game
}

func newArena(t testing.TB, r *Rules) *arena {
	return &arena{newTestGame(t, withRules(r), withMap(`
		#######
		#.....#
		#..@..#
		#.....#
		#######`))}
}

// minTrials is the fewest kills or deaths a measure averages, however
// many rolls each one takes.
const minTrials = 30

// sample pits a copy of hero against a copy of mon and returns the means
// of three measures: kills by sword, kills by firebolt, deaths. Each runs
// until it has made n attack rolls and minTrials trials. The hero never
// levels, the monster never acts while bolted (frozen) and cannot die of
// Thorns while it is the one attacking.
func (a *arena) sample(hero *Player, mon *Monster, n int) duel {
	g, l := a.g, a.g.Lv
	p := *hero
	p.X, p.Y = l.Start.X, l.Start.Y
	p.Torch = g.P.Torch
	p.XP, p.Points, p.HealPool, p.ManaPool = 0, 0, 0, 0
	p.recalc()
	g.P = &p
	m := *mon
	m.X, m.Y, m.HomeX, m.HomeY = p.X+1, p.Y, p.X+1, p.Y
	m.Awake, m.XP, m.Minion = true, 0, true // Minion: no loot to roll
	g.computeVisibility()
	reset := func() {
		m.HP, m.Dead, m.Frozen, m.FreezeCD = m.MaxHP, false, 0, 0
		p.HP, p.MP = float64(p.MaxHP()), float64(p.MaxMP())
		l.Monsters = append(l.Monsters[:0], &m)
		l.Items = nil
		g.Effects = nil
		g.Mode = ModePlay
	}
	// measure runs trials of one action until the budget is spent and
	// returns the mean rolls per trial and the number of trials.
	measure := func(done func() bool, roll func()) (float64, int) {
		rolls, trials := 0, 0
		for ; trials < minTrials || rolls < n; trials++ {
			reset()
			for !done() {
				roll()
				rolls++
			}
		}
		return float64(rolls) / float64(trials), trials
	}
	var d duel
	d.Swings, _ = measure(func() bool { return m.Dead }, func() { g.meleeAttack(&m) })
	d.Bolts, _ = measure(func() bool { return m.Dead }, func() {
		m.Frozen = 1 << 30
		p.MP = float64(p.MaxMP())
		g.Target = &m
		g.castFirebolt()
	})
	taken, maxHP := g.Stats.DmgTaken, m.MaxHP
	m.MaxHP = 1 << 30
	attacks, deaths := measure(func() bool { return p.HP <= 0 }, func() {
		m.HP = 1 << 30
		g.monsterMelee(&m)
	})
	m.MaxHP = maxHP
	reset()
	d.Turns = attacks * 100 / float64(m.Speed)
	d.DPT = float64(g.Stats.DmgTaken-taken) / (attacks * float64(deaths))
	return d
}
