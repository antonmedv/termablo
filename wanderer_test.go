package main

import "testing"

// The third Abyss floor is the Hearth Below: the Last Wanderer in a
// chamber of braziers, reachable from the stairs, with no way down yet.
func TestHearthFloor(t *testing.T) {
	for seed := int64(1); seed <= 40; seed++ {
		g := NewGame(seed)
		l := g.getLevel("abyss3")
		if l.Name != "The Hearth Below" {
			t.Fatalf("seed %d: abyss3 is %q", seed, l.Name)
		}
		var boss *Monster
		for _, m := range l.Monsters {
			if m.T.ID == "wanderer" {
				boss = m
			}
		}
		if boss == nil || boss.Rank != RankBoss {
			t.Fatalf("seed %d: no Last Wanderer boss", seed)
		}
		for _, k := range l.Links {
			if k.To == "abyss4" {
				t.Fatalf("seed %d: a way down before the kill", seed)
			}
		}
		if l.SealedDown != "abyss4" {
			t.Fatalf("seed %d: sealed down is %q", seed, l.SealedDown)
		}
		if !l.reachable(l.Start.X, l.Start.Y)[l.Idx(boss.X, boss.Y)] {
			t.Errorf("seed %d: the boss cannot be reached from the stairs", seed)
		}
		braziers := 0
		for i, tt := range l.T {
			// the ring burns while he stands in the middle
			x, y := i%l.W, i/l.W
			if tt == TBrazier && cheb(x, y, boss.X, boss.Y) <= 16 && !drinks([]Pos{{boss.X, boss.Y}}, x, y) {
				braziers++
			}
		}
		if braziers < 4 {
			t.Errorf("seed %d: %d braziers burning around the boss, want the ring", seed, braziers)
		}
		for _, m := range l.Monsters {
			if m != boss && abs(m.X-boss.X) <= 17 && abs(m.Y-boss.Y) <= 9 {
				t.Errorf("seed %d: %s waits in the chamber", seed, m.Name)
			}
		}
	}
	if NewGame(1).getLevel("abyss2").Name == "The Hearth Below" {
		t.Error("abyss2 is the Hearth Below")
	}
}

// Every light near the Last Wanderer goes out but his own: braziers,
// lava glyphs and the hero's torch. Lights beyond his reach still burn.
func TestWandererDrinksLight(t *testing.T) {
	g := newTestGame(t, withMap(`
		##################################################
		#*........................@.....................*#
		#................................................#
		##################################################`))
	l := g.Lv
	near, far := l.Lights[0], l.Lights[1]
	if near.X > far.X {
		near, far = far, near
	}
	has := func(ls []*Light, x *Light) bool {
		for _, lt := range ls {
			if lt == x {
				return true
			}
		}
		return false
	}
	ls := g.gatherLights()
	if !has(ls, near) || !has(ls, far) || !has(ls, g.P.Torch) {
		t.Fatal("lights missing before he comes")
	}
	boss := placeMonster(l, "wanderer", near.X+3, near.Y+1, 12, RankBoss)
	ls = g.gatherLights()
	if has(ls, near) {
		t.Error("the brazier beside him still burns")
	}
	if !has(ls, far) {
		t.Error("the far brazier went out")
	}
	if !has(ls, boss.Light) {
		t.Error("his own glow went out")
	}
	if !has(ls, g.P.Torch) {
		t.Error("the torch went out beyond his reach")
	}
	boss.X = g.P.X - 2
	ls = g.gatherLights()
	if has(ls, g.P.Torch) {
		t.Error("the torch still burns beside him")
	}
	g.computeVisibility()
	if !g.canSee(boss.X, boss.Y) {
		t.Error("his glow does not show him")
	}
	boss.Dead = true
	if ls = g.gatherLights(); !has(ls, near) || !has(ls, g.P.Torch) {
		t.Error("the lights stay out after he dies")
	}
}

// Killing him opens the sealed way down in the middle of his chamber.
func TestWandererKillOpensWayDown(t *testing.T) {
	g := newTestGame(t, withMap(`
		##############
		#@...........#
		##############`))
	l := g.Lv
	l.SealedDown, l.SealedAt = "abyss4", Pos{8, 1}
	boss := placeMonster(l, "wanderer", 8, 1, 12, RankBoss)
	boss.X = 6
	g.killMonster(boss)
	if l.At(8, 1) != TStairsDown {
		t.Fatalf("no stairs in the chamber: %v", l.At(8, 1))
	}
	if k := l.LinkAt(8, 1); k == nil || k.To != "abyss4" {
		t.Fatalf("the stairs lead to %v", k)
	}
	if l.SealedDown != "" {
		t.Error("the way down is still sealed")
	}
}

// hearthGame starts a game on the Hearth Below with the Last Wanderer
// awake and the hero across the chamber from him.
func hearthGame(t *testing.T) (*Game, *Monster) {
	t.Helper()
	g := NewGame(4)
	g.Mode = ModePlay
	g.changeLevel("abyss3", "", nil)
	var boss *Monster
	for _, m := range g.Lv.Monsters {
		if m.T.ID == "wanderer" {
			boss = m
		}
	}
	g.P.HP = 1e6
	g.P.X, g.P.Y = g.Lv.FreeNear(boss.X-12, boss.Y, -1, -1)
	boss.Awake = true
	return g, boss
}

func on(l *Level, m *Monster) bool {
	for _, o := range l.Monsters {
		if o == m {
			return true
		}
	}
	return false
}

// Home is no refuge: he comes through the town portal two turns after
// the hero, and the portal collapses behind him.
func TestWandererFollowsThroughPortal(t *testing.T) {
	g, boss := hearthGame(t)
	g.P.Scrolls = 1
	g.readPortal()
	pt := *g.Portal
	g.P.X, g.P.Y = pt.X-1, pt.Y
	g.move(1, 0)
	town := g.Lv
	if town.Kind != KTown {
		t.Fatalf("the portal led to %s", town.ID)
	}
	if on(g.getLevel("abyss3"), boss) {
		t.Fatal("he stayed in the Hearth")
	}
	g.wait()
	if on(town, boss) {
		t.Fatal("he arrived at once")
	}
	g.wait()
	if !on(town, boss) || !boss.Awake {
		t.Fatal("he did not follow through the portal")
	}
	if cheb(boss.X, boss.Y, town.PortalAt.X, town.PortalAt.Y) > 3 {
		t.Errorf("he stepped out at %d,%d, far from the portal", boss.X, boss.Y)
	}
	if g.Portal != nil {
		t.Error("the portal is still open behind him")
	}
}

// After that he follows any way out, stairs included.
func TestWandererFollowsStairs(t *testing.T) {
	g, boss := hearthGame(t)
	g.changeLevel("town", "", nil)
	g.wait()
	g.wait()
	g.changeLevel("fields", "town", nil)
	g.wait()
	g.wait()
	if !on(g.Lv, boss) {
		t.Fatal("he did not follow to the Fields")
	}
	if cheb(boss.X, boss.Y, g.P.X, g.P.Y) > 4 {
		t.Errorf("he arrived %d cells away", cheb(boss.X, boss.Y, g.P.X, g.P.Y))
	}
}

// A sleeping Last Wanderer does not follow.
func TestWandererAsleepStays(t *testing.T) {
	g, boss := hearthGame(t)
	boss.Awake = false
	g.changeLevel("town", "", nil)
	for range 4 {
		g.wait()
	}
	if on(g.Lv, boss) || !on(g.getLevel("abyss3"), boss) {
		t.Fatal("he left the Hearth asleep")
	}
}

// At half life on his own floor he opens a red portal and goes home
// ahead of the hero; the hero follows through it and finds him there.
func TestWandererOpensRift(t *testing.T) {
	g, boss := hearthGame(t)
	boss.HP = boss.MaxHP / 2
	g.monsterTurn(boss)
	g.cleanup()
	if g.Rift == nil || on(g.Lv, boss) {
		t.Fatal("no red portal, or he is still here")
	}
	g.P.X, g.P.Y = g.Rift.X-1, g.Rift.Y
	if !g.Lv.Walkable(g.P.X, g.P.Y) {
		g.P.X, g.P.Y = g.Rift.X+1, g.Rift.Y
		g.move(-1, 0)
	} else {
		g.move(1, 0)
	}
	if g.Lv.Kind != KTown {
		t.Fatalf("the red portal led to %s", g.Lv.ID)
	}
	if g.Rift != nil {
		t.Error("the red portal stays open after use")
	}
	if !on(g.Lv, boss) {
		t.Fatal("he is not in town")
	}
}

// Wherever he dies, the Hearth's way down opens where his chamber is.
func TestWandererDiesAwayFromHearth(t *testing.T) {
	g, boss := hearthGame(t)
	hearth := g.Lv
	at := hearth.SealedAt
	g.changeLevel("town", "", nil)
	g.wait()
	g.wait()
	g.killMonster(boss)
	if hearth.At(at.X, at.Y) != TStairsDown {
		t.Fatal("the Hearth is still sealed")
	}
	if k := hearth.LinkAt(at.X, at.Y); k == nil || k.To != "abyss4" {
		t.Fatalf("its stairs lead to %v", k)
	}
}
