package main

import (
	"hash/fnv"
	"testing"
)

// Invariants every generated level must hold, checked over many seeds.

func TestLevelStart(t *testing.T) {
	eachLevel(t, func(t *testing.T, g *Game, l *Level) {
		if !l.Walkable(l.Start.X, l.Start.Y) {
			t.Fatalf("start %v on %s", l.Start, tdefs[l.At(l.Start.X, l.Start.Y)].Name)
		}
		if len(l.T) != l.W*l.H || len(l.Seen) != l.W*l.H || len(l.Decal) != l.W*l.H {
			t.Fatal("tile arrays do not match level size")
		}
	})
}

// Every link (exit, stairs) must be reachable from the level's start.
func TestLinksReachable(t *testing.T) {
	eachLevel(t, func(t *testing.T, g *Game, l *Level) {
		reach := l.reachable(l.Start.X, l.Start.Y)
		for _, lk := range l.Links {
			ok := false
			for y := lk.Y0; y <= lk.Y1; y++ {
				for x := lk.X0; x <= lk.X1; x++ {
					ok = ok || (l.In(x, y) && reach[l.Idx(x, y)])
				}
			}
			if !ok {
				t.Errorf("link to %s unreachable", lk.To)
			}
		}
	})
}

// A link from A to B needs a way back from B to A.
func TestLinksReciprocal(t *testing.T) {
	known := map[string]bool{}
	for _, id := range worldIDs {
		known[id] = true
	}
	eachLevel(t, func(t *testing.T, g *Game, l *Level) {
		for _, lk := range l.Links {
			if !known[lk.To] {
				continue
			}
			if g.Levels[lk.To].LinkTo(l.ID) == nil {
				t.Errorf("%s links to %s, but not back", l.ID, lk.To)
			}
		}
	})
}

func TestMonsterPlacement(t *testing.T) {
	eachLevel(t, func(t *testing.T, g *Game, l *Level) {
		reach := l.reachable(l.Start.X, l.Start.Y)
		taken := map[Pos]string{}
		for _, m := range l.Monsters {
			at := Pos{m.X, m.Y}
			switch {
			case !l.Walkable(m.X, m.Y):
				t.Errorf("%s at %v stands in %s", m.Name, at, tdefs[l.At(m.X, m.Y)].Name)
			case l.LinkAt(m.X, m.Y) != nil:
				t.Errorf("%s at %v blocks a link", m.Name, at)
			case at == l.Start:
				t.Errorf("%s spawned on the start cell", m.Name)
			case !m.Friendly && !reach[l.Idx(m.X, m.Y)]:
				t.Errorf("%s at %v can never be reached", m.Name, at)
			}
			if other, ok := taken[at]; ok {
				t.Errorf("%s and %s share %v", m.Name, other, at)
			}
			taken[at] = m.Name
			if m.HP <= 0 || m.HP > m.MaxHP || m.MinD > m.MaxD {
				t.Errorf("%s has bad stats: hp %d/%d dmg %d-%d", m.Name, m.HP, m.MaxHP, m.MinD, m.MaxD)
			}
		}
	})
}

func TestFloorItemsAndLights(t *testing.T) {
	eachLevel(t, func(t *testing.T, g *Game, l *Level) {
		for _, fi := range l.Items {
			if !l.Walkable(fi.X, fi.Y) {
				t.Errorf("%s at %d,%d lies in %s", fi.It.DisplayName(), fi.X, fi.Y, tdefs[l.At(fi.X, fi.Y)].Name)
			}
		}
		for _, lt := range l.Lights {
			if !l.In(lt.X, lt.Y) || lt.Radius <= 0 || lt.Intensity <= 0 {
				t.Errorf("bad light at %d,%d r=%v i=%v", lt.X, lt.Y, lt.Radius, lt.Intensity)
			}
		}
	})
}

// The same seed must always build the same level.
func TestLevelDeterministic(t *testing.T) {
	for _, id := range worldIDs {
		a, b := levelHash(NewGame(3).getLevel(id)), levelHash(NewGame(3).getLevel(id))
		if a != b {
			t.Errorf("%s: two builds differ", id)
		}
		if c := levelHash(NewGame(4).getLevel(id)); id != "town" && c == a {
			t.Errorf("%s: seeds 3 and 4 built identical levels", id)
		}
	}
}

func levelHash(l *Level) uint64 {
	h := fnv.New64a()
	for _, t := range l.T {
		h.Write([]byte{byte(t)})
	}
	for _, m := range l.Monsters {
		h.Write([]byte(m.Name))
		h.Write([]byte{byte(m.X), byte(m.X >> 8), byte(m.Y), byte(m.Y >> 8), byte(m.HP)})
	}
	for _, fi := range l.Items {
		h.Write([]byte(fi.It.DisplayName()))
	}
	return h.Sum64()
}
