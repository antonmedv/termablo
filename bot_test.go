package main

import "testing"

func botTurn(g *Game) {
	p := g.P
	if p.HP < float64(p.MaxHP())*0.4 && p.HPot > 0 {
		g.drinkHealth()
		return
	}
	adj := 0
	for _, m := range g.visibleHostiles() {
		if cheb(m.X, m.Y, p.X, p.Y) <= 1 {
			adj++
		}
	}
	if adj >= 3 && p.MP >= costNova {
		g.castNova()
		return
	}
	if t := g.nearestHostile(); t != nil {
		if cheb(t.X, t.Y, p.X, p.Y) == 1 {
			g.move(t.X-p.X, t.Y-p.Y)
			return
		}
		if p.MP >= costFirebolt {
			g.Target = t
			g.castFirebolt()
			return
		}
		ox, oy, ot := p.X, p.Y, g.Turn
		g.move(sign(t.X-p.X), sign(t.Y-p.Y))
		if ox == p.X && oy == p.Y && ot == g.Turn {
			g.wait()
		}
		return
	}
	for _, fi := range g.Lv.ItemsAt(p.X, p.Y) {
		if fi.It.Kind == IKEquip {
			g.pickup()
			return
		}
	}
	if !g.autoStep() {
		// go down
		for _, lk := range g.Lv.Links {
			_ = lk
		}
		d := dirs8[g.rng.Intn(8)]
		g.move(d.X, d.Y)
	}
	// equip better stuff
	for i := len(p.Inv) - 1; i >= 0; i-- {
		if g.upgradeHint(p.Inv[i]) != "" {
			g.equip(i)
		}
	}
	for p.Points > 0 {
		p.Vit++
		p.Str++
		p.Points -= 2
	}
}

func TestBot(t *testing.T) {
	for seed := int64(1); seed <= 6; seed++ {
		g := NewGame(seed)
		g.Mode = ModePlay
		g.changeLevel("fields", "town", nil)
		turns := 0
		for ; turns < 3000 && g.Mode != ModeDead; turns++ {
			botTurn(g)
			if g.Mode == ModeTalk || g.Mode == ModeShop {
				g.Mode = ModePlay
			}
		}
		t.Logf("gturn %d area %s", g.Turn, g.Lv.ID)
		t.Logf("seed %d: turns %d dead=%v lvl %d kills %d gold %d inv %d killedBy=%q hpot %d", seed, turns, g.Mode == ModeDead, g.P.Lvl, g.P.Kills, g.P.Gold, len(g.P.Inv), g.P.KilledBy, g.P.HPot)
	}
}
