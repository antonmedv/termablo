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

// townGame puts the hero in Emberhold with the Last Wanderer loose in it.
func townGame(t *testing.T) (*Game, *Monster) {
	t.Helper()
	g, boss := hearthGame(t)
	g.changeLevel("town", "", nil)
	g.wait()
	g.wait()
	if !on(g.Lv, boss) {
		t.Fatal("he did not follow to town")
	}
	return g, boss
}

func townsperson(g *Game, id string) *Monster {
	for _, m := range g.Lv.Monsters {
		if m.T.ID == id && !m.Dead {
			return m
		}
	}
	return nil
}

// With him in town Emberhold goes dark: no ambient light, the panel says
// so, and the lights he passes stay out until he dies.
func TestTownGoesDark(t *testing.T) {
	g, boss := townGame(t)
	l := g.Lv
	if !g.townHunted() {
		t.Fatal("town is not hunted")
	}
	g.composeLight(0, false)
	far := l.Idx(1, 1)
	if g.light[far] != (RGB{}) {
		t.Errorf("the town's corner is still lit: %v", g.light[far])
	}
	var near *Light
	for _, lt := range l.Lights {
		if drinks([]Pos{{boss.X, boss.Y}}, lt.X, lt.Y) {
			near = lt
		}
	}
	if near == nil {
		t.Skip("no town light near the portal")
	}
	g.snuffAround(boss)
	boss.X, boss.Y = l.FreeNear(boss.X+30, boss.Y, -1, -1)
	g.gatherLights()
	if !near.Off {
		t.Error("a lantern he passed came back while he lives")
	}
	s := NewScreen(110, 40)
	g.Draw(s)
	if !screenHas(s, "Not safe") {
		t.Error("the panel does not warn")
	}
	g.killMonster(boss)
	g.cleanup()
	if near.Off {
		t.Error("the lantern stays out after he dies")
	}
	if g.townHunted() {
		t.Error("town is still hunted after he dies")
	}
}

// He goes for the nearest townsperson and kills them in three cuts.
func TestWandererHuntsTownsfolk(t *testing.T) {
	g, boss := townGame(t)
	l := g.Lv
	mirela := townsperson(g, "alch")
	boss.X, boss.Y = l.FreeNear(mirela.X+1, mirela.Y, -1, -1)
	g.P.X, g.P.Y = l.FreeNear(boss.X+20, boss.Y, -1, -1)
	if g.prey(boss) != mirela {
		t.Fatalf("he goes for %v, not Mirela", g.prey(boss))
	}
	for range 3 {
		g.huntStep(boss, mirela)
	}
	if !mirela.Dead {
		t.Fatalf("Mirela survives three cuts with %d life", mirela.HP)
	}
	if len(g.Fallen) != 1 || g.Fallen[0] != "alch" {
		t.Errorf("fallen: %v", g.Fallen)
	}
}

// Standing nearer than any townsperson makes the hero his target.
func TestWandererPreysOnNearestHero(t *testing.T) {
	g, boss := townGame(t)
	g.P.X, g.P.Y = g.Lv.FreeNear(boss.X+1, boss.Y, -1, -1)
	if p := g.prey(boss); p != nil && cheb(p.X, p.Y, boss.X, boss.Y) > 1 {
		t.Errorf("he goes for %s past the hero", p.Name)
	}
}

// Townsfolk near him run, and will not talk or trade while he is in town.
func TestTownsfolkFlee(t *testing.T) {
	g, boss := townGame(t)
	l := g.Lv
	voss := townsperson(g, "captain")
	boss.X, boss.Y = l.FreeNear(voss.X+2, voss.Y, -1, -1)
	d0 := cheb(voss.X, voss.Y, boss.X, boss.Y)
	for range 6 {
		g.monsterTurn(voss)
	}
	if cheb(voss.X, voss.Y, boss.X, boss.Y) <= d0 {
		t.Error("Voss does not run")
	}
	g.talkTo(voss)
	if g.Mode == ModeTalk {
		t.Error("Voss stops to talk")
	}
}

// With Hadrik the forge dies for good: it stays cold after he does.
func TestForgeGoesCold(t *testing.T) {
	g, boss := townGame(t)
	l := g.Lv
	g.townspersonDies(townsperson(g, "smith"))
	f := l.Forge
	if l.At(f.X, f.Y) != TColdBrazier || !g.ForgeOut {
		t.Fatal("the forge still burns")
	}
	g.killMonster(boss)
	for _, lt := range l.Lights {
		if lt.X == f.X && lt.Y == f.Y {
			t.Fatal("the forge lit again")
		}
	}
}

// Left alone in town he keeps hunting: he does not doze off when the hero
// is out of sight, and he paths through the streets to them.
func TestWandererHuntsUnwatched(t *testing.T) {
	g, _ := townGame(t)
	l := g.Lv
	for range 100 {
		g.P.X, g.P.Y = l.FreeNear(l.W-6, 4, -1, -1)
		g.wait()
	}
	if len(g.Fallen) < 2 {
		t.Errorf("only %v fell in 100 turns", g.Fallen)
	}
}

// His death drops his shroud, marks the quest and play goes on.
func TestWandererDeathDropsShroud(t *testing.T) {
	g, boss := hearthGame(t)
	g.killMonster(boss)
	if g.Mode != ModePlay {
		t.Errorf("mode %v after his death, want play", g.Mode)
	}
	if g.Quests[2] != 1 {
		t.Error("the quest is not done")
	}
	found := false
	for _, fi := range g.Lv.Items {
		if fi.It.Name == lastShroud.Name {
			found = true
		}
	}
	if !found {
		t.Error("no Last Wanderer's Shroud on the ground")
	}
}

// Voss's reward speech for both bosses at once still fits 80x24.
func TestVossRewardsFit(t *testing.T) {
	g := NewGame(1)
	g.Mode = ModePlay
	g.Quests = [3]int{1, 1, 0}
	for _, m := range g.Lv.Monsters {
		if m.T.ID == "captain" {
			g.talkTo(m)
		}
	}
	s := NewScreen(80, 24)
	g.Draw(s)
	if !screenHas(s, "press any key") || !screenHas(s, "counting the days.") {
		t.Error("Voss's rewards run off the screen")
	}
}

// -level readies the hero for a deep start: level, points spent, a full
// kit for the area, a full belt, and the quests on the way done.
func TestReady(t *testing.T) {
	for _, build := range []string{"fighter", "caster"} {
		g := NewGame(3)
		g.changeLevel("abyss3", "", nil)
		if err := g.Ready(21, build); err != nil {
			t.Fatal(err)
		}
		p := g.P
		if p.Lvl != 21 || p.Points != 0 {
			t.Errorf("%s: level %d with %d points unspent", build, p.Lvl, p.Points)
		}
		for slot, it := range p.Eq {
			twoHanded := slot == EqOffhand && p.Eq[EqWeapon].Base.TwoHanded
			if it == nil && !twoHanded {
				t.Errorf("%s: %s is empty", build, eqNames[slot])
			}
		}
		if p.HPot != beltMax || p.Scrolls != 2 || p.HP != float64(p.MaxHP()) {
			t.Errorf("%s: belt %d, scrolls %d, life %.0f/%d", build, p.HPot, p.Scrolls, p.HP, p.MaxHP())
		}
		if g.Quests != [3]int{2, 2, 0} {
			t.Errorf("%s: quests %v", build, g.Quests)
		}
	}
	if NewGame(3).Ready(5, "rogue") == nil {
		t.Error("an unknown build is accepted")
	}
}

// The bot does not try to trade with townsfolk running for their lives.
func TestBotWontTradeInHuntedTown(t *testing.T) {
	g, _ := townGame(t)
	b := NewBot(g, policyByName("caster"))
	if acted, found := b.visit("smith", func() { t.Fatal("traded during the hunt") }); acted || found {
		t.Errorf("visit acted %v, found %v", acted, found)
	}
}

// Townsfolk who ran from him walk back home afterwards.
func TestTownsfolkWalkHome(t *testing.T) {
	g := NewGame(1)
	g.Mode = ModePlay
	l := g.Lv
	v := townsperson(g, "villager")
	v.X, v.Y = l.FreeNear(v.HomeX-10, v.HomeY, -1, -1)
	for range 40 {
		g.monsterTurn(v)
	}
	if d := cheb(v.X, v.Y, v.HomeX, v.HomeY); d > 3 {
		t.Errorf("the villager is still %d cells from home", d)
	}
}

// A street lamp he put out draws cold after he walks on, not lit.
func TestSnuffedLampDrawsCold(t *testing.T) {
	g, boss := townGame(t)
	l := g.Lv
	var lamp *Light
	for _, lt := range l.Lights {
		if l.At(lt.X, lt.Y) == TLamp {
			lamp = lt
		}
	}
	boss.X, boss.Y = l.FreeNear(lamp.X+1, lamp.Y, -1, -1)
	g.snuffAround(boss)
	boss.X, boss.Y = l.FreeNear(lamp.X+40, lamp.Y, -1, -1)
	g.P.X, g.P.Y = l.FreeNear(lamp.X, lamp.Y+1, -1, -1)
	g.computeVisibility()
	s := NewScreen(110, 40)
	g.Draw(s)
	cx, cy := g.camera(s.W-panelW, s.H-logH)
	c := s.C[(lamp.Y-cy)*s.W+lamp.X-cx]
	if c.Ch != '¥' {
		t.Fatalf("no lamp drawn at its cell: %q", c.Ch)
	}
	if c.FG.R > 128 {
		t.Errorf("the put-out lamp still glows: %v", c.FG)
	}
}

// Back in town by the stairs, he does not collapse a portal left open
// on another floor.
func TestStairsArrivalKeepsPortal(t *testing.T) {
	g, boss := hearthGame(t)
	g.Portal = &Portal{"crypt2", 5, 5}
	g.changeLevel("town", "", nil)
	g.wait()
	g.wait()
	if !on(g.Lv, boss) {
		t.Fatal("he did not follow")
	}
	if g.Portal == nil {
		t.Error("he collapsed a portal the hero never used")
	}
}

// A -level start in town restocks the shops for the depth.
func TestReadyInTownRestocks(t *testing.T) {
	g := NewGame(2)
	if err := g.Ready(30, "fighter"); err != nil {
		t.Fatal(err)
	}
	if g.stocked != g.Deepest {
		t.Errorf("stock for depth %d, deepest %d", g.stocked, g.Deepest)
	}
}
