package main

import (
	"testing"
	"time"
)

func TestSimulate(t *testing.T) {
	for seed := int64(1); seed <= 4; seed++ {
		g := NewGame(seed)
		g.Mode = ModePlay
		g.P.Vit, g.P.Ene = 1e7, 1e4 // immortal, endless mana
		g.P.recalc()
		s := NewScreen(140, 45)
		ids := []string{"town", "fields", "crypt1", "crypt2", "crypt3", "crypt4", "marsh", "grotto1", "grotto2", "grotto3", "abyss1", "abyss2"}
		for _, id := range ids {
			g.changeLevel(id, "", nil)
			g.P.HP = float64(g.P.MaxHP())
			for i := range 400 {
				g.time += 0.05
				switch i % 7 {
				case 0:
					g.castFirebolt()
				case 1:
					g.castNova()
				default:
					if !g.autoStep() {
						d := dirs8[g.rng.Intn(8)]
						g.move(d.X, d.Y)
					}
				}
				g.P.MP = float64(g.P.MaxMP())
				if g.Mode == ModeDead || g.Mode == ModeTalk || g.Mode == ModeShop {
					g.Mode = ModePlay
				}
				if i%20 == 0 {
					g.Draw(s)
					_ = s.String()
				}
			}
			for _, m := range []Mode{ModeInv, ModeChar, ModeHelp, ModeMap, ModeDead} {
				g.Mode = m
				g.Draw(s)
			}
			g.Mode = ModePlay
			if err := checkGame(g); err != nil {
				t.Fatalf("seed %d %s: %v", seed, id, err)
			}
		}
	}
}

func BenchmarkFrame(b *testing.B) {
	g := NewGame(3)
	g.Mode = ModePlay
	g.changeLevel("crypt2", "", nil)
	s := NewScreen(200, 60)
	b.ResetTimer()
	for i := range b.N {
		g.time = float64(i) * 0.05
		g.Draw(s)
		_ = s.String()
	}
}

func BenchmarkTurn(b *testing.B) {
	g := NewGame(3)
	g.Mode = ModePlay
	g.changeLevel("fields", "", nil)
	for range b.N {
		g.P.HP = 1e9
		g.endTurn()
	}
}

var _ = time.Now

func BenchmarkFrameAbyss(b *testing.B) {
	g := NewGame(3)
	g.Mode = ModePlay
	g.changeLevel("abyss1", "", nil)
	s := NewScreen(200, 60)
	b.ResetTimer()
	for i := range b.N {
		g.time = float64(i) * 0.05
		g.Draw(s)
		_ = s.String()
	}
}
