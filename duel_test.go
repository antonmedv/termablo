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

func newArena(t testing.TB) *arena {
	return &arena{newTestGame(t, withMap(`
		#######
		#.....#
		#..@..#
		#.....#
		#######`))}
}

// sample pits a copy of hero against a copy of mon: n kills by sword, n
// by firebolt, n deaths, and returns the means. The hero never levels,
// the monster never acts while bolted (frozen) and cannot die of Thorns
// while it is the one attacking.
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
	var d duel
	for range n {
		reset()
		for !m.Dead {
			g.meleeAttack(&m)
			d.Swings++
		}
	}
	for range n {
		reset()
		m.Frozen = 1 << 30
		for !m.Dead {
			p.MP = float64(p.MaxMP())
			g.Target = &m
			g.castFirebolt()
			d.Bolts++
		}
	}
	taken, attacks, maxHP := g.Stats.DmgTaken, 0, m.MaxHP
	for range n {
		reset()
		m.HP, m.MaxHP = 1<<30, 1<<30
		for p.HP > 0 {
			g.monsterMelee(&m)
			attacks++
		}
	}
	m.MaxHP = maxHP
	reset()
	fn := float64(n)
	d.Swings, d.Bolts = d.Swings/fn, d.Bolts/fn
	d.Turns = float64(attacks) / fn * 100 / float64(m.Speed)
	d.DPT = float64(g.Stats.DmgTaken-taken) / float64(attacks)
	return d
}
