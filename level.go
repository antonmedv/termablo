package main

import "math/rand"

type LevelKind int

const (
	KTown LevelKind = iota
	KSurface
	KDungeon
)

type Pos struct{ X, Y int }

// Link connects a rectangle of tiles to another level. AX/AY is where a
// traveller arriving through this link appears (-1 = next to the link).
type Link struct {
	X0, Y0, X1, Y1 int
	To             string
	AX, AY         int
}

func (k Link) Contains(x, y int) bool { return x >= k.X0 && x <= k.X1 && y >= k.Y0 && y <= k.Y1 }

type Level struct {
	ID, Name string
	Kind     LevelKind
	Depth    int // area level: drives monster and loot level
	W, H     int
	T        []Tile
	V        []uint8
	Seen     []bool
	Decal    []uint8
	Lights   []*Light
	Monsters []*Monster
	Items    []*FloorItem
	Links    []Link
	Ambient  RGB
	Start    Pos // default arrival
	PortalAt Pos // town only: where the town portal appears
	LightVer int
	Lore     string
	visited  bool
	rng      *rand.Rand
}

const (
	DecalNone uint8 = iota
	DecalBlood
	DecalCorpse
	DecalScorch
)

func newLevel(id, name string, kind LevelKind, w, h, depth int, seed int64) *Level {
	l := &Level{ID: id, Name: name, Kind: kind, W: w, H: h, Depth: depth}
	l.T = make([]Tile, w*h)
	l.V = make([]uint8, w*h)
	l.Seen = make([]bool, w*h)
	l.Decal = make([]uint8, w*h)
	l.rng = rand.New(rand.NewSource(seed))
	for i := range l.V {
		l.V[i] = uint8(l.rng.Intn(256))
	}
	return l
}

func (l *Level) In(x, y int) bool { return x >= 0 && y >= 0 && x < l.W && y < l.H }
func (l *Level) Idx(x, y int) int { return y*l.W + x }
func (l *Level) At(x, y int) Tile {
	if !l.In(x, y) {
		return TNone
	}
	return l.T[y*l.W+x]
}
func (l *Level) Set(x, y int, t Tile) {
	if l.In(x, y) {
		l.T[y*l.W+x] = t
	}
}
func (l *Level) Opaque(x, y int) bool {
	if !l.In(x, y) {
		return true
	}
	return tdefs[l.T[y*l.W+x]].BlockSight
}
func (l *Level) Walkable(x, y int) bool {
	if !l.In(x, y) {
		return false
	}
	return !tdefs[l.T[y*l.W+x]].BlockMove
}

func (l *Level) Fill(x0, y0, x1, y1 int, t Tile) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			l.Set(x, y, t)
		}
	}
}

func (l *Level) MonsterAt(x, y int) *Monster {
	for _, m := range l.Monsters {
		if !m.Dead && m.X == x && m.Y == y {
			return m
		}
	}
	return nil
}

func (l *Level) ItemsAt(x, y int) []*FloorItem {
	var r []*FloorItem
	for _, it := range l.Items {
		if it.X == x && it.Y == y {
			r = append(r, it)
		}
	}
	return r
}

func (l *Level) LinkAt(x, y int) *Link {
	for i := range l.Links {
		if l.Links[i].Contains(x, y) {
			return &l.Links[i]
		}
	}
	return nil
}

func (l *Level) LinkTo(id string) *Link {
	for i := range l.Links {
		if l.Links[i].To == id {
			return &l.Links[i]
		}
	}
	return nil
}

// FreeNear finds the closest walkable, unoccupied, non-link cell to (x,y).
func (l *Level) FreeNear(x, y int, px, py int) (int, int) {
	best, bx, by := 1<<30, x, y
	for r := 0; r < 12; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				nx, ny := x+dx, y+dy
				if !l.Walkable(nx, ny) || l.MonsterAt(nx, ny) != nil || l.LinkAt(nx, ny) != nil {
					continue
				}
				if nx == px && ny == py {
					continue
				}
				t := l.At(nx, ny)
				if t == TDoor || t == TStairsDown || t == TStairsUp {
					continue
				}
				d := dx*dx + dy*dy
				if d < best {
					best, bx, by = d, nx, ny
				}
			}
		}
		if best < 1<<30 {
			return bx, by
		}
	}
	return bx, by
}

// finalize creates static lights for every light-emitting tile.
func (l *Level) finalize() {
	lavaCount := 0
	for y := 0; y < l.H; y++ {
		for x := 0; x < l.W; x++ {
			t := l.At(x, y)
			spec := tdefs[t].Light
			if spec == nil {
				continue
			}
			if t == TLava {
				// Lava is emissive everywhere, but only sparsely casts light.
				lavaCount++
				if (x+y*3)%5 != 0 || l.rng.Intn(3) == 0 {
					continue
				}
			}
			l.Lights = append(l.Lights, NewLight(x, y, spec, l.rng.Float32()*100))
		}
	}
}

// reachable returns a flood fill mask of walkable cells from (x,y).
func (l *Level) reachable(x, y int) []bool {
	mask := make([]bool, l.W*l.H)
	if !l.In(x, y) {
		return mask
	}
	q := []int{l.Idx(x, y)}
	mask[q[0]] = true
	for len(q) > 0 {
		i := q[0]
		q = q[1:]
		cx, cy := i%l.W, i/l.W
		for _, d := range dirs8 {
			nx, ny := cx+d.X, cy+d.Y
			if !l.In(nx, ny) {
				continue
			}
			ni := l.Idx(nx, ny)
			if mask[ni] || !l.Walkable(nx, ny) {
				continue
			}
			mask[ni] = true
			q = append(q, ni)
		}
	}
	return mask
}

var dirs8 = []Pos{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}
var dirs4 = []Pos{{0, -1}, {-1, 0}, {1, 0}, {0, 1}}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func cheb(x0, y0, x1, y1 int) int {
	dx, dy := abs(x1-x0), abs(y1-y0)
	if dx > dy {
		return dx
	}
	return dy
}

func maxi(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func mini(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
