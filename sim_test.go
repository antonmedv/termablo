package main

import (
	"testing"
	"time"
)

func TestSimulate(t *testing.T) {
	for seed := int64(1); seed <= 4; seed++ {
		g := NewGame(seed)
		g.Mode = ModePlay
		s := NewScreen(140, 45)
		ids := []string{"town", "fields", "crypt1", "crypt2", "crypt3", "crypt4", "marsh", "grotto1", "grotto2", "grotto3", "abyss1", "abyss2"}
		for _, id := range ids {
			g.changeLevel(id, "", nil)
			g.P.HP = 1e9
			for i := 0; i < 400; i++ {
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
				g.P.MP = 100
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
		}
	}
}

func BenchmarkFrame(b *testing.B) {
	g := NewGame(3)
	g.Mode = ModePlay
	g.changeLevel("crypt2", "", nil)
	s := NewScreen(200, 60)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.time = float64(i) * 0.05
		g.Draw(s)
		_ = s.String()
	}
}

func BenchmarkTurn(b *testing.B) {
	g := NewGame(3)
	g.Mode = ModePlay
	g.changeLevel("fields", "", nil)
	for i := 0; i < b.N; i++ {
		g.P.HP = 1e9
		g.endTurn()
	}
}

var _ = time.Now
