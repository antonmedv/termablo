package main

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// Shared fixtures for tests.
//
// newTestGame builds a game on a hand-made level. By default it is an open,
// moonlit 120x30 grass field with no monsters; withMap replaces it with an
// ASCII layout:
//
//	g := newTestGame(t, withMap(`
//	    #########
//	    #@..z...#
//	    #...*...#
//	    #########`))

// Tiles by map character. Anything not listed (and not a monster or '@')
// is a wall; short rows are padded with walls.
var mapTiles = map[rune]Tile{
	'#': TWall,
	'.': TFloor,
	',': TGrass,
	'+': TDoor,
	'~': TWater,
	'=': TDeepWater,
	'>': TStairsDown,
	'<': TStairsUp,
	'*': TBrazier,
	'^': TCrystal,
	'T': TTree,
	'&': TChest,
	'F': TFountain,
	'A': TAltar,
}

// Monsters by map character; they stand on stone floor.
var mapMonsters = map[rune]string{
	'r': "rat",
	'b': "bat",
	'f': "fallen",
	'z': "zombie",
	's': "skel",
	'a': "archer",
	'w': "wolf",
}

type testOpts struct {
	seed    int64
	layout  string
	ambient RGB
	awake   bool
	rules   *Rules
}

type testOpt func(*testOpts)

func withMap(m string) testOpt   { return func(o *testOpts) { o.layout = m } }
func withAmbient(c RGB) testOpt  { return func(o *testOpts) { o.ambient = c } }
func withAwakeMonsters() testOpt { return func(o *testOpts) { o.awake = true } }
func withRules(r *Rules) testOpt { return func(o *testOpts) { o.rules = r } }

func newTestGame(t testing.TB, opts ...testOpt) *Game {
	t.Helper()
	o := testOpts{seed: 7, ambient: C(.05, .06, .12)}
	for _, f := range opts {
		f(&o)
	}
	if o.rules == nil {
		o.rules = DefaultRules()
	}
	g := NewGameWith(o.seed, o.rules)
	g.Mode = ModePlay
	var l *Level
	if o.layout == "" {
		l = newLevel("test", "Test Field", KSurface, 120, 30, 1, o.seed, g.Rules)
		l.Fill(0, 0, l.W-1, l.H-1, TGrass)
		l.Start = Pos{5, 15}
	} else {
		l = parseMap(t, g, o.layout, o.seed, o.awake)
	}
	l.Ambient = o.ambient
	l.finalize()
	g.Levels[l.ID] = l
	g.changeLevel(l.ID, "", nil)
	if o.layout != "" && (g.P.X != l.Start.X || g.P.Y != l.Start.Y) {
		t.Fatalf("player placed at %d,%d, map wants %d,%d", g.P.X, g.P.Y, l.Start.X, l.Start.Y)
	}
	return g
}

func parseMap(t testing.TB, g *Game, layout string, seed int64, awake bool) *Level {
	t.Helper()
	var rows [][]rune
	for _, ln := range strings.Split(layout, "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			rows = append(rows, []rune(ln))
		}
	}
	w := 0
	for _, r := range rows {
		w = maxi(w, len(r))
	}
	l := newLevel("test", "Test Map", KDungeon, w, len(rows), 1, seed, g.Rules)
	l.Fill(0, 0, w-1, len(rows)-1, TWall)
	player := false
	for y, r := range rows {
		for x, ch := range r {
			switch {
			case ch == '@':
				l.Set(x, y, TFloor)
				l.Start = Pos{x, y}
				player = true
			case mapMonsters[ch] != "":
				l.Set(x, y, TFloor)
				m := NewMonster(g.rng, mtemps[mapMonsters[ch]], 1, RankNormal, g.Rules)
				m.X, m.Y, m.HomeX, m.HomeY, m.Awake = x, y, x, y, awake
				l.Monsters = append(l.Monsters, m)
			default:
				tile, ok := mapTiles[ch]
				if !ok {
					t.Fatalf("map: unknown character %q", ch)
				}
				l.Set(x, y, tile)
			}
		}
	}
	if !player {
		t.Fatal("map: no '@'")
	}
	return l
}

// monsters returns living monsters of a template, in map (row-major) order.
func monsters(g *Game, id string) []*Monster {
	var r []*Monster
	for _, m := range g.Lv.Monsters {
		if !m.Dead && m.T.ID == id {
			r = append(r, m)
		}
	}
	return r
}

// spawnAt places a monster near an offset from the player.
func spawnAt(g *Game, id string, dx, dy int, awake bool) *Monster {
	l, p := g.Lv, g.P
	m := NewMonster(g.rng, mtemps[id], 1, RankNormal, g.Rules)
	m.X, m.Y = l.FreeNear(p.X+dx, p.Y+dy, p.X, p.Y)
	m.Awake = awake
	l.Monsters = append(l.Monsters, m)
	g.computeVisibility()
	return m
}

func lastLog(g *Game) string { return g.Log[len(g.Log)-1].Text }

// newTestModel wraps a game in the Bubble Tea model, to drive it by keys.
func newTestModel(g *Game) *model { return &model{g: g, scr: NewScreen(100, 32), seed: g.Seed} }

// press sends keys through the real key handler.
func press(m *model, keys ...string) {
	for _, k := range keys {
		m.key(k)
	}
}

// walkInto stands the player next to (x,y) and steps onto it. It reports
// false when no free cell borders the target.
func walkInto(g *Game, x, y int) bool {
	l := g.Lv
	for _, d := range dirs8 {
		sx, sy := x-d.X, y-d.Y
		if !l.Walkable(sx, sy) || l.MonsterAt(sx, sy) != nil || l.LinkAt(sx, sy) != nil {
			continue
		}
		if t := l.At(sx, sy); t == TDoor || tdefs[t].BlockMove {
			continue
		}
		g.P.X, g.P.Y = sx, sy
		g.computeVisibility()
		g.move(d.X, d.Y)
		return true
	}
	return false
}

// npc finds a townsperson by template ID.
func npc(t *testing.T, g *Game, id string) *Monster {
	t.Helper()
	for _, m := range g.getLevel("town").Monsters {
		if m.T.ID == id {
			return m
		}
	}
	t.Fatalf("no %s in town", id)
	return nil
}

// ------------------------------------------------------------ generated world

var worldIDs = []string{"town", "fields", "marsh", "crypt1", "crypt2", "crypt3", "crypt4", "barrow1", "barrow2", "grotto1", "grotto2", "grotto3", "sanctum1", "sanctum2", "abyss1", "abyss2"}

const worldSeeds = 40

var (
	worldMu    sync.Mutex
	worldCache = map[int64]*Game{}
)

// world returns a game with every level generated for seed. It is shared
// between tests: treat it as read-only.
func world(seed int64) *Game {
	worldMu.Lock()
	defer worldMu.Unlock()
	if g, ok := worldCache[seed]; ok {
		return g
	}
	g := NewGame(seed)
	for _, id := range worldIDs {
		g.getLevel(id)
	}
	worldCache[seed] = g
	return g
}

// eachLevel runs f as a subtest for every level of every test seed.
func eachLevel(t *testing.T, f func(t *testing.T, g *Game, l *Level)) {
	eachLevelSeeds(t, worldSeeds, f)
}

// eachLevelSeeds is eachLevel over seeds 1..n (3 under -short).
func eachLevelSeeds(t *testing.T, n int, f func(t *testing.T, g *Game, l *Level)) {
	if testing.Short() {
		n = 3
	}
	for seed := int64(1); seed <= int64(n); seed++ {
		g := world(seed)
		for _, id := range worldIDs {
			l := g.Levels[id]
			t.Run(fmt.Sprintf("%s/seed%02d", id, seed), func(t *testing.T) { f(t, g, l) })
		}
	}
}
