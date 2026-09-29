package main

import (
	"testing"
)

// TestPowerCurve follows an idealized caster down the crypt: every monster
// killed, every chest opened, every point in Energy, the best spell gear
// worn. After each level it reports how many firebolts the next level's
// monsters take. go test -v -run PowerCurve shows the table.
func TestPowerCurve(t *testing.T) {
	route := []string{"fields", "crypt1", "crypt2", "crypt3", "crypt4"}
	seeds := []int64{1, 2, 3, 4, 5, 6}
	if testing.Short() {
		seeds = seeds[:2]
	}
	type row struct{ lvl, ene, spell, lo, hi, mp, cost, zombie, ghoul float64 }
	sum := make([]row, len(route))
	for _, seed := range seeds {
		g := NewGame(seed)
		g.Mode = ModePlay
		p := g.P
		var loot []*Item
		for i, id := range route {
			g.changeLevel(id, "", nil)
			l := g.Lv
			for _, m := range l.Monsters {
				if !m.Friendly {
					g.dropLoot(m)
					p.XP += m.XP
				}
			}
			for y := range l.H {
				for x := range l.W {
					if l.At(x, y) == TChest {
						g.openChest(x, y)
					}
				}
			}
			for _, fi := range l.Items {
				if fi.It.Kind == IKEquip {
					loot = append(loot, fi.It)
				}
			}
			l.Items = nil
			for p.XP >= xpNext(p.Lvl) {
				p.XP -= xpNext(p.Lvl)
				p.Lvl++
				p.Ene += 5
			}
			wearBestSpellGear(p, loot)
			lo, hi := p.FireboltDmg()
			next := l.Depth + 1
			z := NewMonster(g.rng, mtemps["zombie"], next, RankNormal)
			gh := NewMonster(g.rng, mtemps["ghoul"], next, RankNormal)
			r := &sum[i]
			r.lvl += float64(p.Lvl)
			r.ene += float64(p.ENE())
			r.spell += float64(p.S(StSpellPct))
			r.lo += float64(lo)
			r.hi += float64(hi)
			r.mp += float64(p.MaxMP())
			r.cost += float64(p.FireboltCost())
			r.zombie += float64(z.MaxHP)
			r.ghoul += float64(gh.MaxHP)
		}
	}
	n := float64(len(seeds))
	t.Logf("%-8s %5s %4s %6s %9s %8s %7s  %s", "after", "clvl", "ene", "spell", "firebolt", "casts", "zombie", "ghoul (bolts to kill)")
	for i, id := range route {
		r := sum[i]
		avg := (r.lo + r.hi) / 2 / n
		t.Logf("%-8s %5.1f %4.0f %5.0f%% %4.0f-%-4.0f %8.1f %4.0f (%.1f)  %4.0f (%.1f)", id, r.lvl/n, r.ene/n, r.spell/n,
			r.lo/n, r.hi/n, r.mp/r.cost, r.zombie/n, r.zombie/n/avg, r.ghoul/n, r.ghoul/n/avg)
		// A normal crypt monster should never fall to one bolt.
		if id != "fields" && r.zombie/n/avg < 1.3 {
			t.Errorf("after %s, a firebolt takes %.1f of a zombie's life", id, avg/(r.zombie/n))
		}
	}
}

// wearBestSpellGear greedily fills each slot with the item giving the most
// firebolt damage, then the most mana.
func wearBestSpellGear(p *Player, loot []*Item) {
	score := func() int {
		lo, hi := p.FireboltDmg()
		return (lo+hi)*1000 + p.MaxMP()
	}
	for _, it := range loot {
		if it.Base.TwoHanded {
			continue
		}
		for s := range EqCount {
			if !eqSlotOK(s, it) || p.Eq[s] == it {
				continue
			}
			old, before := p.Eq[s], score()
			p.Eq[s] = it
			p.recalc()
			if score() <= before {
				p.Eq[s] = old
				p.recalc()
			}
		}
	}
}
