package main

import (
	"math"
	"testing"
)

var black = C(0, 0, 0)

func lightAt(g *Game, x, y int) float32 { return g.light[g.Lv.Idx(x, y)].Max() }

func TestWallsBlockSight(t *testing.T) {
	g := newTestGame(t, withMap(`
		###########
		#@...#..z.#
		#....+....#
		###########`))
	z := monsters(g, "zombie")[0]
	if g.canSee(z.X, z.Y) {
		t.Fatal("saw through a wall and a closed door")
	}
	g.Lv.Set(5, 2, TDoorOpen)
	g.P.X, g.P.Y = 4, 2
	g.computeVisibility()
	if !g.canSee(z.X, z.Y) {
		t.Fatal("open door did not reveal the zombie")
	}
}

func TestTorchFalloff(t *testing.T) {
	g := newTestGame(t, withAmbient(black), withMap(`
		##############################
		#@...........................#
		##############################`))
	r := int(math.Ceil(float64(g.P.Torch.Radius)))
	prev := lightAt(g, g.P.X, g.P.Y)
	if prev <= visThresh {
		t.Fatalf("player's own cell is dark: %v", prev)
	}
	for x := g.P.X + 1; x < g.Lv.W-1; x++ {
		v := lightAt(g, x, g.P.Y)
		if v > prev+1e-6 {
			t.Errorf("light rises at x=%d: %v -> %v", x, prev, v)
		}
		if x-g.P.X > r*2 && v != 0 {
			t.Errorf("torch reaches x=%d, beyond radius %d", x, r)
		}
		prev = v
	}
	if g.canSee(g.Lv.W-2, g.P.Y) {
		t.Error("far end of the dark corridor is visible")
	}
}

// Walls hold light in: a brazier in a sealed room must not leak outside.
func TestLightDoesNotLeakThroughWalls(t *testing.T) {
	g := newTestGame(t, withAmbient(black), withMap(`
		###############################
		#@............................#
		#.....................#########
		#.....................#...*...#
		#.....................#########`))
	if len(g.Lv.Lights) != 1 {
		t.Fatalf("%d lights, want the brazier", len(g.Lv.Lights))
	}
	for x := 17; x <= 21; x++ {
		for y := 2; y <= 4; y++ {
			if v := lightAt(g, x, y); v != 0 {
				t.Errorf("light %v leaked to %d,%d", v, x, y)
			}
		}
	}
	if lightAt(g, 26, 3) < 0.3 {
		t.Errorf("brazier cell is dim: %v", lightAt(g, 26, 3))
	}
}

func TestDarkMonsterUnseenUntilLit(t *testing.T) {
	g := newTestGame(t, withAmbient(black), withMap(`
		##############################
		#@......................z....#
		##############################`))
	z := monsters(g, "zombie")[0]
	if g.canSee(z.X, z.Y) {
		t.Fatal("unlit zombie visible in the dark")
	}
	z.Light = NewLight(z.X, z.Y, &LightSpec{C(1, 1, 1), 3, 1, 0, 0}, 1)
	g.computeVisibility()
	if !g.canSee(z.X, z.Y) {
		t.Fatal("glowing zombie in line of sight is invisible")
	}
}

// Real levels: light must be finite and non-negative everywhere.
func TestLightFinite(t *testing.T) {
	for seed := int64(1); seed <= 2; seed++ {
		g := NewGame(seed)
		for _, id := range worldIDs {
			g.changeLevel(id, "", nil)
			g.composeLight(1.7, true)
			for i, c := range g.light {
				for _, v := range []float32{c.R, c.G, c.B} {
					if v < 0 || math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
						t.Fatalf("seed %d %s: light %v at %d,%d", seed, id, c, i%g.Lv.W, i/g.Lv.W)
					}
				}
			}
		}
	}
}
