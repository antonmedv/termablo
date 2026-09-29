package main

import (
	"strings"
	"testing"
)

// open moonlit field with no monsters, for controlled setups
func quietLevel(t *testing.T) *Game {
	g := NewGame(7)
	g.Mode = ModePlay
	l := newLevel("test", "Test Field", KSurface, 120, 30, 1, 1)
	l.Fill(0, 0, l.W-1, l.H-1, TGrass)
	l.Ambient = C(.05, .06, .12)
	l.Start = Pos{5, 15}
	g.Levels["test"] = l
	g.changeLevel("test", "", nil)
	return g
}

func spawnAt(g *Game, id string, dx, dy int, awake bool) *Monster {
	l, p := g.Lv, g.P
	m := NewMonster(g.rng, mtemps[id], 1, RankNormal)
	m.X, m.Y = l.FreeNear(p.X+dx, p.Y+dy, p.X, p.Y)
	m.Awake = awake
	l.Monsters = append(l.Monsters, m)
	g.computeVisibility()
	return m
}

func lastLog(g *Game) string { return g.Log[len(g.Log)-1].Text }

func TestAutoExploreIgnoresDistantEnemies(t *testing.T) {
	g := quietLevel(t)
	far := spawnAt(g, "zombie", 30, 0, false)
	if !g.canSee(far.X, far.Y) {
		t.Skip("distant zombie not visible on this seed")
	}
	if !g.autoStep() {
		t.Fatalf("stopped for distant unaware enemy: %q", lastLog(g))
	}
	far.Awake = true
	far.X, far.Y = g.Lv.FreeNear(g.P.X+14, g.P.Y, g.P.X, g.P.Y)
	g.computeVisibility()
	if g.canSee(far.X, far.Y) && g.autoStep() {
		t.Fatal("did not stop for an aware enemy at 14 steps")
	}
	g2 := quietLevel(t)
	near := spawnAt(g2, "zombie", 4, 0, false)
	if g2.canSee(near.X, near.Y) && g2.autoStep() {
		t.Fatal("did not stop for a nearby enemy")
	}
	if !strings.Contains(lastLog(g2), "a Zombie") {
		t.Errorf("message = %q", lastLog(g2))
	}
}

func TestFireboltOutOfRangeKeepsMana(t *testing.T) {
	g := quietLevel(t)
	m := spawnAt(g, "zombie", 2*fireboltRange, 0, false)
	if !g.canSee(m.X, m.Y) || cheb(m.X, m.Y, g.P.X, g.P.Y) <= fireboltRange {
		t.Skip("setup: target not visible beyond range")
	}
	g.Target = m
	mp, turn := g.P.MP, g.Turn
	g.castFirebolt()
	if g.P.MP != mp || g.Turn != turn {
		t.Fatalf("mana %v->%v, turn %d->%d", mp, g.P.MP, turn, g.Turn)
	}
	if !strings.Contains(lastLog(g), "out of range") {
		t.Errorf("message = %q", lastLog(g))
	}
}

func TestCycleTargetNearestFirst(t *testing.T) {
	g := quietLevel(t)
	far := spawnAt(g, "zombie", 12, 0, true)
	near := spawnAt(g, "rat", 3, 0, true)
	mid := spawnAt(g, "bat", 7, 0, true)
	var order []*Monster
	for i := 0; i < 3; i++ {
		g.cycleTarget()
		order = append(order, g.Target)
	}
	if order[0] != near || order[1] != mid || order[2] != far {
		t.Fatalf("order = %v %v %v", order[0].Name, order[1].Name, order[2].Name)
	}
}

func TestHitsAreLogged(t *testing.T) {
	g := quietLevel(t)
	m := spawnAt(g, "zombie", 1, 0, true)
	m.ToHit = 1000
	g.monsterMelee(m)
	if got := lastLog(g); !strings.HasPrefix(got, "The Zombie claws you for ") {
		t.Fatalf("log = %q", got)
	}
}
