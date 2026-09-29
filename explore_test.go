package main

import (
	"strings"
	"testing"
)

func TestAutoExploreIgnoresDistantEnemies(t *testing.T) {
	g := newTestGame(t)
	far := spawnAt(g, "zombie", 30, 0, false)
	if !g.canSee(far.X, far.Y) {
		t.Fatal("setup: distant zombie not visible")
	}
	if !g.autoStep() {
		t.Fatalf("stopped for distant unaware enemy: %q", lastLog(g))
	}
	far.Awake = true
	far.X, far.Y = g.Lv.FreeNear(g.P.X+14, g.P.Y, g.P.X, g.P.Y)
	g.computeVisibility()
	if g.autoStep() {
		t.Fatal("did not stop for an aware enemy at 14 steps")
	}
}

func TestAutoExploreStopsForNearEnemy(t *testing.T) {
	g := newTestGame(t)
	spawnAt(g, "zombie", 4, 0, false)
	if g.autoStep() {
		t.Fatal("did not stop for a nearby enemy")
	}
	if !strings.Contains(lastLog(g), "a Zombie") {
		t.Errorf("message = %q", lastLog(g))
	}
}

// Auto-explore in a closed room must terminate with everything seen.
func TestAutoExploreFinishes(t *testing.T) {
	g := newTestGame(t, withMap(`
		############
		#@.........#
		#.####.###.#
		#....#.....#
		####.#.###.#
		#......#...#
		############`))
	steps := 0
	for ; steps < 500 && g.autoStep(); steps++ {
	}
	if steps == 500 {
		t.Fatal("auto-explore never stopped")
	}
	l := g.Lv
	for i, tt := range l.T {
		if tt == TFloor && !l.Seen[i] {
			t.Errorf("floor at %d,%d never seen", i%l.W, i/l.W)
		}
	}
}
