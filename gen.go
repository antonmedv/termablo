package main

import (
	"math"
	"sort"
)

// carvePath walks a wobbly path from a to b calling paint on a brush around
// each step. Returns the visited center points. A brush of width 1 paints
// the step itself; wider brushes keep the footprint the roads were tuned
// on (2 paints three across, 3 paints two).
func (l *Level) carvePath(x0, y0, x1, y1, width int, paint func(x, y int)) []Pos {
	x, y := x0, y0
	var pts []Pos
	r := width / 2
	hi := maxi(r+(width+1)%2-1, 0)
	for range 8000 {
		pts = append(pts, Pos{x, y})
		for dy := -r; dy <= hi; dy++ {
			for dx := -r; dx <= hi; dx++ {
				if l.In(x+dx, y+dy) {
					paint(x+dx, y+dy)
				}
			}
		}
		if x == x1 && y == y1 {
			break
		}
		ddx, ddy := x1-x, y1-y
		if l.rng.Float64() < 0.22 {
			// wander perpendicular to the dominant direction
			if abs(ddx) > abs(ddy) {
				y += l.rng.Intn(3) - 1
			} else {
				x += l.rng.Intn(3) - 1
			}
		} else if l.rng.Intn(abs(ddx)+abs(ddy)+1) < abs(ddx) {
			if ddx > 0 {
				x++
			} else {
				x--
			}
		} else if ddy != 0 {
			if ddy > 0 {
				y++
			} else {
				y--
			}
		}
		x = clampi(x, 1, l.W-2)
		y = clampi(y, 1, l.H-2)
	}
	return pts
}

func (l *Level) circle(cx, cy int, r float64, f func(x, y int, d float64)) {
	ir := int(math.Ceil(r * Aspect))
	for y := cy - int(math.Ceil(r)); y <= cy+int(math.Ceil(r)); y++ {
		for x := cx - ir; x <= cx+ir; x++ {
			dx, dy := float64(x-cx)/Aspect, float64(y-cy)
			d := math.Sqrt(dx*dx + dy*dy)
			if d <= r && l.In(x, y) {
				f(x, y, d)
			}
		}
	}
}

func (l *Level) building(x0, y0, x1, y1, doorX, doorY int) {
	l.Fill(x0, y0, x1, y1, TWall)
	l.Fill(x0+1, y0+1, x1-1, y1-1, TWoodFloor)
	l.Set(doorX, doorY, TDoor)
}

// ---------------------------------------------------------------- town

func genTown(seed int64, r *Rules) *Level {
	W, H := 84, 46
	l := newLevel("town", "Emberhold", KTown, W, H, 0, seed, r)
	l.Ambient = C(.07, .075, .12)
	l.NameKey, l.LoreKey = "town", "town"
	nz := Noise{uint32(seed)}
	for y := range H {
		for x := range W {
			l.Set(x, y, TGrass)
			n := nz.FBM(float64(x)*0.12, float64(y)*0.24, 3)
			if x < 4 || x > 79 || y < 3 || y > 42 {
				if n > 0.52 || x < 1 || y < 1 || y > H-2 {
					l.Set(x, y, TTree)
				}
			} else if n > 0.62 {
				l.Set(x, y, TDirt)
			}
		}
	}
	cx, cy := 42, 23
	// Wall ring
	for x := 4; x <= 79; x++ {
		l.Set(x, 3, TTownWall)
		l.Set(x, 42, TTownWall)
	}
	for y := 3; y <= 42; y++ {
		l.Set(4, y, TTownWall)
		l.Set(79, y, TTownWall)
	}
	// Roads
	l.Fill(5, cy-1, W-1, cy+1, TRoad)
	l.Fill(cx-1, 4, cx+1, 41, TRoad)
	l.circle(cx, cy, 4.2, func(x, y int, d float64) { l.Set(x, y, TRoad) })
	l.Set(cx, cy, TFountain)
	l.Set(cx-1, cy, TFountain)
	l.Set(cx+1, cy, TFountain)
	// East gate
	l.Fill(79, cy-1, W-1, cy+1, TRoad)
	l.Set(79, cy-2, TBrazier)
	l.Set(79, cy+2, TBrazier)
	for y := cy - 1; y <= cy+1; y++ {
		l.Set(W-1, y, TExit)
	}
	l.Links = append(l.Links, Link{W - 1, cy - 1, W - 1, cy + 1, "fields", W - 3, cy})

	// Smithy with a forge
	l.building(48, 11, 61, 19, 54, 19)
	l.Set(50, 13, TBrazier)
	l.Forge = Pos{50, 13}
	l.Set(59, 13, TShelf)
	l.Set(59, 14, TShelf)
	// Alchemist with a crystal lamp
	l.building(23, 11, 36, 19, 30, 19)
	l.Set(25, 13, TCrystal)
	for x := 28; x <= 34; x++ {
		l.Set(x, 12, TShelf)
	}
	// Tavern
	l.building(22, 27, 37, 36, 30, 27)
	l.Fill(25, 30, 34, 34, TCarpet)
	l.Set(23, 35, TBrazier)
	l.Set(36, 35, TBrazier)
	// Houses
	l.building(48, 28, 56, 34, 52, 28)
	l.Set(49, 33, TBrazier)
	l.building(60, 28, 67, 35, 63, 28)
	l.building(8, 6, 16, 12, 12, 12)
	l.building(8, 32, 16, 39, 12, 32)
	// Graveyard
	l.Fill(64, 5, 76, 16, TDirt)
	for y := 6; y <= 15; y += 3 {
		for x := 65; x <= 75; x += 2 {
			if l.rng.Intn(4) != 0 {
				l.Set(x, y, TGrave)
			}
		}
	}
	l.Set(70, 16, TDeadTree)
	l.Set(66, 11, TDeadTree)
	l.Set(75, 5, TBrazier)
	// Street lamps
	for x := 8; x < 78; x += 9 {
		for _, y := range []int{cy - 2, cy + 2} {
			if l.At(x, y) == TGrass || l.At(x, y) == TDirt {
				l.Set(x, y, TLamp)
			}
		}
	}
	for y := 7; y < 41; y += 8 {
		for _, x := range []int{cx - 2, cx + 2} {
			if l.At(x, y) == TGrass || l.At(x, y) == TDirt {
				l.Set(x, y, TLamp)
			}
		}
	}
	l.Set(cx-4, cy-3, TLamp)
	l.Set(cx+4, cy+3, TLamp)

	l.Start = Pos{cx - 3, cy + 1}
	l.PortalAt = Pos{cx + 5, cy - 2}
	l.Set(l.PortalAt.X, l.PortalAt.Y, TRoad)

	npc := func(id string, x, y int) {
		m := NewMonster(l.rng, mtemps[id], 1, RankNormal, r)
		m.X, m.Y, m.HomeX, m.HomeY = x, y, x, y
		l.Monsters = append(l.Monsters, m)
	}
	npc("smith", 56, 20)
	npc("alch", 32, 20)
	npc("captain", 33, 26)
	npc("exile", 70, 13) // among the graves
	npc("boy", 44, 25)   // by the fountain
	npc("villager", 60, 24)
	npc("villager", 20, 22)
	l.finalize()
	return l
}

// ---------------------------------------------------------------- fields

func (l *Level) ruin(x0, y0, w, h int, keep float64) {
	for y := y0; y <= y0+h; y++ {
		for x := x0; x <= x0+w; x++ {
			edge := x == x0 || y == y0 || x == x0+w || y == y0+h
			if edge {
				if l.rng.Float64() < keep {
					l.Set(x, y, TWall)
				} else {
					l.Set(x, y, TRubble)
				}
			} else {
				if l.rng.Intn(6) == 0 {
					l.Set(x, y, TRubble)
				} else {
					l.Set(x, y, TFloor)
				}
			}
		}
	}
}

func genFields(seed int64, r *Rules) *Level {
	W, H := 180, 108
	l := newLevel("fields", "Ashen Fields", KSurface, W, H, 1, seed, r)
	l.Ambient = C(.05, .06, .12)
	l.NameKey, l.LoreKey = "fields", "fields"
	nz := Noise{uint32(seed) + 7}
	nz2 := Noise{uint32(seed) + 99}
	for y := range H {
		for x := range W {
			n := nz.FBM(float64(x)*0.07, float64(y)*0.14, 4)
			d := nz2.FBM(float64(x)*0.1, float64(y)*0.2, 2)
			t := TGrass
			switch {
			case x < 2 || y < 2 || x > W-3 || y > H-3:
				t = TTree
			case n > 0.6:
				t = TTree
			case n > 0.56 && l.rng.Intn(3) == 0:
				t = TTree
			case d > 0.66:
				t = TDirt
			case l.rng.Intn(120) == 0:
				t = TRock
			case l.rng.Intn(200) == 0:
				t = TDeadTree
			}
			l.Set(x, y, t)
		}
	}
	cy := H / 2
	road := func(x, y int) {
		if t := l.At(x, y); t != TWall && t != TEarthWall && t != TFloor && t != TBrazier && t != TStairsDown {
			l.Set(x, y, TDirt)
		}
	}
	// Crypt entrance ruin
	ex, ey := 142, 77
	l.ruin(ex-6, ey-4, 12, 8, 0.85)
	for y := ey - 1; y <= ey+1; y++ {
		l.Set(ex-6, y, TFloor)
	}
	l.Set(ex, ey, TStairsDown)
	for _, p := range []Pos{{ex - 4, ey - 2}, {ex + 4, ey - 2}, {ex - 4, ey + 2}, {ex + 4, ey + 2}} {
		l.Set(p.X, p.Y, TBrazier)
	}
	l.Links = append(l.Links, Link{ex, ey, ex, ey, "crypt1", -1, -1})
	for range 26 {
		gx, gy := ex-16+l.rng.Intn(32), ey-14+l.rng.Intn(26)
		if l.At(gx, gy) == TGrass && cheb(gx, gy, ex, ey) > 7 {
			l.Set(gx, gy, TGrave)
		}
	}
	// The Gravewardens' barrow: an earth ring south of the crypt road, open
	// to the north, its watch buried around it
	bx, by := 104, 96
	l.circle(bx, by, 4.6, func(x, y int, d float64) {
		if d > 3.4 {
			l.Set(x, y, TEarthWall)
		} else {
			l.Set(x, y, TDirt)
		}
	})
	for y := by - 5; y <= by-3; y++ {
		l.Fill(bx-1, y, bx+1, y, TDirt)
	}
	l.Set(bx, by, TStairsDown)
	l.Links = append(l.Links, Link{bx, by, bx, by, "barrow1", -1, -1})
	l.circle(bx, by, 8, func(x, y int, d float64) {
		if d > 6.5 && l.At(x, y) == TGrass && y > by-5 && (x+y)%3 == 0 {
			l.Set(x, y, TGrave)
		}
	})
	// Roads: town -> crypt, branch -> marsh, branch -> barrow
	main := l.carvePath(1, cy, ex-8, ey, 2, road)
	l.carvePath(ex-8, ey, ex-6, ey, 3, road)
	mid := main[len(main)/2]
	north := l.carvePath(mid.X, mid.Y, 120, 1, 2, road)
	fork := main[len(main)*3/4]
	south := l.carvePath(fork.X, fork.Y, bx, by-6, 2, road)
	roadPts := append(append(append([]Pos{}, main...), north...), south...)
	trail := func(x, y int) {
		switch l.At(x, y) {
		case TTree, TRock, TDeadTree, TGrass:
			l.Set(x, y, TDirt)
		case TWall:
			l.Set(x, y, TRubble)
		}
	}
	connect := func(x, y int) {
		best, bd := roadPts[0], 1<<30
		for _, p := range roadPts {
			if d := (p.X-x)*(p.X-x) + (p.Y-y)*(p.Y-y); d < bd {
				best, bd = p, d
			}
		}
		l.carvePath(x, y, best.X, best.Y, 1, trail)
	}
	// Exits; the gate road is painted, not carved, so the start is never
	// a tree the road wandered past
	for y := cy - 1; y <= cy+1; y++ {
		l.Set(0, y, TExit)
		l.Fill(1, y, 3, y, TDirt)
	}
	l.Links = append(l.Links, Link{0, cy - 1, 0, cy + 1, "town", 2, cy})
	for x := 119; x <= 121; x++ {
		l.Set(x, 0, TExit)
		l.Set(x, 1, TDirt)
	}
	l.Links = append(l.Links, Link{119, 0, 121, 0, "marsh", 120, 2})

	l.Start = Pos{3, cy}
	safe := []Pos{{2, cy}}
	// Ruins scattered in the fields
	for range 10 {
		rx, ry := 20+l.rng.Intn(W-50), 8+l.rng.Intn(H-20)
		if cheb(rx, ry, ex, ey) < 16 || cheb(rx, ry, 2, cy) < 15 || cheb(rx, ry, bx, by) < 26 {
			continue
		}
		w, h := 6+l.rng.Intn(8), 4+l.rng.Intn(4)
		l.ruin(rx, ry, w, h, 0.55)
		connect(rx+w/2, ry+h+1)
		if l.rng.Intn(2) == 0 {
			l.Set(rx+1+l.rng.Intn(w-1), ry+1+l.rng.Intn(h-1), TChest)
		}
		if l.rng.Intn(3) == 0 {
			l.Set(rx+w/2, ry+h/2, TBrazier)
		}
	}
	// Campfires of the Fallen, kept apart: a fire set inside an earlier
	// camp lands on one of its Fallen.
	var fires []Pos
	for tries := 0; tries < 60 && len(fires) < 7; tries++ {
		fx, fy := 20+l.rng.Intn(W-40), 10+l.rng.Intn(H-20)
		if cheb(fx, fy, 2, cy) < 20 || cheb(fx, fy, ex, ey) < 12 || cheb(fx, fy, bx, by) < 24 {
			continue
		}
		apart := true
		for _, f := range fires {
			apart = apart && cheb(fx, fy, f.X, f.Y) >= 8
		}
		if !apart {
			continue
		}
		l.circle(fx, fy, 3.2, func(x, y int, d float64) {
			if l.At(x, y) == TTree || l.At(x, y) == TGrass || l.At(x, y) == TRock {
				l.Set(x, y, TDirt)
			}
		})
		connect(fx, fy+3)
		l.Set(fx, fy, TCampfire)
		fires = append(fires, Pos{fx, fy})
		lvl := 1 + l.rng.Intn(2)
		for range 3 + l.rng.Intn(3) {
			placeMonster(l, "fallen", fx+l.rng.Intn(5)-2, fy+l.rng.Intn(3)-1, lvl, RankNormal)
		}
		placeMonster(l, "shaman", fx, fy+1, lvl, RankNormal)
	}
	// Guarantee a way through: re-carve if trees block crypt reach
	populate(l, "fields", 29, safe)
	l.finalize()
	return l
}

// ---------------------------------------------------------------- marsh

func genMarsh(seed int64, r *Rules) *Level {
	W, H := 156, 96
	l := newLevel("marsh", "Blackmarsh", KSurface, W, H, 6, seed, r)
	l.Ambient = C(.04, .07, .09)
	l.NameKey, l.LoreKey = "marsh", "marsh"
	nz := Noise{uint32(seed) + 3}
	nz2 := Noise{uint32(seed) + 71}
	for y := range H {
		for x := range W {
			n := nz.FBM(float64(x)*0.06, float64(y)*0.12, 4)
			m := nz2.FBM(float64(x)*0.15, float64(y)*0.3, 2)
			t := TMud
			switch {
			case x < 2 || y < 2 || x > W-3 || y > H-3:
				t = TTree
			case n < 0.32:
				t = TDeepWater
			case n < 0.42:
				t = TWater
			case n < 0.46:
				t = TReeds
			case n > 0.72:
				t = TTree
			case m > 0.55:
				t = TGrass
			}
			if (t == TMud || t == TGrass) && l.rng.Intn(28) == 0 {
				t = TDeadTree
			}
			l.Set(x, y, t)
		}
	}
	// Grotto mouth
	gx, gy := 122, 19
	l.circle(gx, gy, 5, func(x, y int, d float64) {
		if d > 3.6 {
			l.Set(x, y, TCaveWall)
			if l.rng.Intn(5) == 0 {
				l.Set(x, y, TCrystal)
			}
		} else {
			l.Set(x, y, TCaveFloor)
		}
	})
	// open the mouth on the south side so the road can reach the stairs
	l.Fill(gx-1, gy+3, gx+1, gy+6, TCaveFloor)
	l.Set(gx, gy, TStairsDown)
	l.Links = append(l.Links, Link{gx, gy, gx, gy, "grotto1", -1, -1})
	path := func(x, y int) {
		switch l.At(x, y) {
		case TDeepWater, TWater:
			l.Set(x, y, TBridge)
		case TCaveWall, TCrystal, TCaveFloor, TStairsDown:
		default:
			l.Set(x, y, TMud)
		}
	}
	// The road ends just outside the opening, never inside the ring: the
	// carver walks through cave wall without painting it, so a road aimed
	// at the stairs could tunnel in through the wall and leave deep water
	// in front of the opening.
	l.carvePath(48, H-2, gx, gy+7, 2, path)
	for x := 47; x <= 49; x++ {
		l.Set(x, H-1, TExit)
		l.Set(x, H-2, TMud)
	}
	l.Links = append(l.Links, Link{47, H - 1, 49, H - 1, "fields", 48, H - 3})
	l.Start = Pos{48, H - 3}

	// Standing stones with crystals, clear of the start and of the grotto
	// mouth: a ring of boulders over its opening would seal the way down.
	for range 8 {
		sx, sy := 10+l.rng.Intn(W-20), 8+l.rng.Intn(H-16)
		if cheb(sx, sy, 48, H-3) < 10 || cheb(sx, sy, gx, gy) < 12 {
			continue
		}
		l.circle(sx, sy, 2.6, func(x, y int, d float64) {
			if d < 2.0 {
				l.Set(x, y, TGrass)
			} else if l.rng.Intn(2) == 0 {
				l.Set(x, y, TRock)
			}
		})
		l.Set(sx, sy, TCrystal)
	}
	// Drowned shrine
	sx, sy := 30+l.rng.Intn(48), 12+l.rng.Intn(24)
	l.ruin(sx, sy, 10, 6, 0.8)
	l.Set(sx+5, sy+3, TAltar)
	l.Set(sx+2, sy+2, TChest)
	l.Set(sx, sy+3, TFloor)
	l.carvePath(sx, sy+3, 48, H/2, 1, path)

	populate(l, "marsh", 35, []Pos{l.Start})
	l.finalize()
	return l
}

// ---------------------------------------------------------------- dungeons

type Room struct{ X0, Y0, X1, Y1 int }

func (r Room) Center() Pos { return Pos{(r.X0 + r.X1) / 2, (r.Y0 + r.Y1) / 2} }

type DungeonSpec struct {
	ID, Name   string
	NameKey    string // area.<NameKey> with {n} = NameN, the name the player reads
	NameN      int
	Depth      int
	Style      int // 0 crypt, 1 grotto, 2 abyss, 3 barrow
	Up, Down   string
	SealedDown string // a down link that opens when the boss dies
	Boss       string
	SpawnTable string
	Rules      *Rules
}

func genDungeon(s DungeonSpec, seed int64) *Level {
	if s.Style == 0 || s.Style == 3 {
		return genCrypt(s, seed)
	}
	return genCave(s, seed)
}

func genCrypt(s DungeonSpec, seed int64) *Level {
	W, H := 92, 56
	l := newLevel(s.ID, s.Name, KDungeon, W, H, s.Depth, seed, s.Rules)
	l.NameKey, l.NameN = s.NameKey, s.NameN
	l.Ambient = C(0, 0, 0)
	// a barrow is the crypt dug in earth: dirt between earthen walls, and
	// no fire but the watch's own
	barrow := s.Style == 3
	wall, floor := TWall, TFloor
	if barrow {
		wall, floor = TEarthWall, TDirt
	}
	l.Fill(0, 0, W-1, H-1, wall)
	var rooms []Room
	for tries := 0; tries < 400 && len(rooms) < 17; tries++ {
		w, h := 6+l.rng.Intn(11), 4+l.rng.Intn(7)
		x, y := 1+l.rng.Intn(W-w-2), 1+l.rng.Intn(H-h-2)
		r := Room{x, y, x + w, y + h}
		ok := true
		for _, o := range rooms {
			if r.X0-2 <= o.X1 && r.X1+2 >= o.X0 && r.Y0-2 <= o.Y1 && r.Y1+2 >= o.Y0 {
				ok = false
				break
			}
		}
		if ok {
			rooms = append(rooms, r)
		}
	}
	sort.Slice(rooms, func(i, j int) bool { return rooms[i].X0+rooms[i].Y0/2 < rooms[j].X0+rooms[j].Y0/2 })
	for _, r := range rooms {
		l.Fill(r.X0, r.Y0, r.X1, r.Y1, floor)
	}
	corridor := func(a, b Pos) {
		x, y := a.X, a.Y
		horizFirst := l.rng.Intn(2) == 0
		step := func(tx, ty int) {
			for x != tx || y != ty {
				if x < tx {
					x++
				} else if x > tx {
					x--
				} else if y < ty {
					y++
				} else if y > ty {
					y--
				}
				if l.At(x, y) == wall {
					l.Set(x, y, floor)
				}
			}
		}
		if horizFirst {
			step(b.X, a.Y)
			step(b.X, b.Y)
		} else {
			step(a.X, b.Y)
			step(b.X, b.Y)
		}
	}
	for i := 1; i < len(rooms); i++ {
		corridor(rooms[i-1].Center(), rooms[i].Center())
	}
	for range 4 {
		a, b := l.rng.Intn(len(rooms)), l.rng.Intn(len(rooms))
		if a != b {
			corridor(rooms[a].Center(), rooms[b].Center())
		}
	}
	inRoom := func(x, y int) bool {
		for _, r := range rooms {
			if x >= r.X0 && x <= r.X1 && y >= r.Y0 && y <= r.Y1 {
				return true
			}
		}
		return false
	}
	// Doors where corridors pierce room walls
	for _, r := range rooms {
		for x := r.X0 - 1; x <= r.X1+1; x++ {
			for _, y := range []int{r.Y0 - 1, r.Y1 + 1} {
				if l.At(x, y) == floor && !inRoom(x, y) && l.At(x-1, y) == wall && l.At(x+1, y) == wall && l.rng.Intn(3) == 0 {
					l.Set(x, y, TDoor)
				}
			}
		}
		for y := r.Y0 - 1; y <= r.Y1+1; y++ {
			for _, x := range []int{r.X0 - 1, r.X1 + 1} {
				if l.At(x, y) == floor && !inRoom(x, y) && l.At(x, y-1) == wall && l.At(x, y+1) == wall && l.rng.Intn(3) == 0 {
					l.Set(x, y, TDoor)
				}
			}
		}
	}
	// Decorate rooms
	for i, r := range rooms {
		w, h := r.X1-r.X0, r.Y1-r.Y0
		if w >= 10 && h >= 6 && l.rng.Intn(2) == 0 {
			for y := r.Y0 + 2; y <= r.Y1-2; y += 3 {
				for x := r.X0 + 2; x <= r.X1-2; x += 4 {
					l.Set(x, y, TPillar)
				}
			}
		}
		if barrow {
			if i == len(rooms)-1 {
				l.Set(r.X0, r.Y0, TBrazier)
				l.Set(r.X1, r.Y1, TBrazier)
			}
			// the watch, buried standing in rows
			for x := r.X0 + 1; x < r.X1; x += 3 {
				if l.At(x, r.Y0-1) == wall && l.rng.Intn(3) == 0 {
					l.Set(x, r.Y0, TGrave)
				}
			}
		} else if l.rng.Intn(100) < 55 || i == len(rooms)-1 {
			l.Set(r.X0, r.Y0, TBrazier)
			l.Set(r.X1, r.Y1, TBrazier)
		} else if l.rng.Intn(3) == 0 {
			l.Set(r.X1, r.Y0, TBrazier)
		}
		if l.rng.Intn(5) == 0 && s.Depth >= 3 {
			c := r.Center()
			l.Fill(c.X-2, c.Y, c.X+2, c.Y, TCarpet)
			l.Set(c.X, c.Y-1, TAltar)
		}
		if l.rng.Intn(4) == 0 {
			l.Set(r.X0+l.rng.Intn(w+1), r.Y1, TChest)
		}
		for range w * h / 12 {
			x, y := r.X0+l.rng.Intn(w+1), r.Y0+l.rng.Intn(h+1)
			if l.At(x, y) == floor {
				if l.rng.Intn(2) == 0 {
					l.Set(x, y, TBones)
				} else {
					l.Set(x, y, TRubble)
				}
			}
		}
	}
	first, last := rooms[0], rooms[len(rooms)-1]
	up := first.Center()
	l.Set(up.X, up.Y, TStairsUp)
	l.Links = append(l.Links, Link{up.X, up.Y, up.X, up.Y, s.Up, -1, -1})
	l.Start = Pos{up.X + 1, up.Y}
	if !l.Walkable(l.Start.X, l.Start.Y) { // a pillar can stand there
		l.Start.X, l.Start.Y = l.FreeNear(up.X, up.Y, -1, -1)
	}
	if s.Down != "" {
		d := last.Center()
		l.Set(d.X, d.Y, TStairsDown)
		l.Links = append(l.Links, Link{d.X, d.Y, d.X, d.Y, s.Down, -1, -1})
	}
	if s.Boss != "" {
		c := last.Center()
		b := placeMonster(l, s.Boss, c.X-2, c.Y, s.Depth, RankBoss)
		b.Awake = false
		guard := "skel"
		if barrow {
			guard = "gravewarden"
		}
		for range 4 {
			placeMonster(l, guard, c.X+l.rng.Intn(5)-2, c.Y+l.rng.Intn(3)-1, s.Depth, RankNormal)
		}
	}
	l.LoreKey = "crypt"
	if barrow {
		l.LoreKey = "barrow"
	}
	populate(l, s.SpawnTable, 14+s.Depth*2, []Pos{up})
	l.finalize()
	return l
}

func genCave(s DungeonSpec, seed int64) *Level {
	W, H := 96, 60
	l := newLevel(s.ID, s.Name, KDungeon, W, H, s.Depth, seed, s.Rules)
	l.NameKey, l.NameN = s.NameKey, s.NameN
	l.Ambient = C(0, 0, 0)
	wall, floor := TCaveWall, TCaveFloor
	grid := make([]bool, W*H)
	for i := range grid {
		x, y := i%W, i/W
		grid[i] = x == 0 || y == 0 || x == W-1 || y == H-1 || l.rng.Float64() < 0.46
	}
	for it := range 5 {
		ng := make([]bool, W*H)
		for y := range H {
			for x := range W {
				if x == 0 || y == 0 || x == W-1 || y == H-1 {
					ng[y*W+x] = true
					continue
				}
				n := 0
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if grid[(y+dy)*W+x+dx] {
							n++
						}
					}
				}
				ng[y*W+x] = n >= 5 || (it < 2 && n <= 1)
			}
		}
		grid = ng
	}
	for i, g := range grid {
		if g {
			l.T[i] = wall
		} else {
			l.T[i] = floor
		}
	}
	// keep largest region
	seen := make([]bool, W*H)
	var best []int
	for i := range grid {
		if grid[i] || seen[i] {
			continue
		}
		var region []int
		q := []int{i}
		seen[i] = true
		for len(q) > 0 {
			c := q[0]
			q = q[1:]
			region = append(region, c)
			cx, cy := c%W, c/W
			for _, d := range dirs4 {
				nx, ny := cx+d.X, cy+d.Y
				ni := ny*W + nx
				if l.In(nx, ny) && !grid[ni] && !seen[ni] {
					seen[ni] = true
					q = append(q, ni)
				}
			}
		}
		if len(region) > len(best) {
			best = region
		}
	}
	keep := make([]bool, W*H)
	for _, i := range best {
		keep[i] = true
	}
	for i := range l.T {
		if !keep[i] {
			l.T[i] = wall
		}
	}
	// stairs: up somewhere, down at the farthest reachable point
	up := best[l.rng.Intn(len(best))]
	ux, uy := up%W, up/W
	dist := bfsDist(l, ux, uy)
	far, fd := up, 0
	for _, i := range best {
		if dist[i] > fd {
			far, fd = i, dist[i]
		}
	}
	fx, fy := far%W, far/W
	// protect the route between stairs from hazards
	protect := make([]bool, W*H)
	{
		back := bfsDist(l, fx, fy)
		x, y := ux, uy
		for steps := 0; steps < W*H && (x != fx || y != fy); steps++ {
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if l.In(x+dx, y+dy) {
						protect[(y+dy)*W+x+dx] = true
					}
				}
			}
			bx, by, bd := x, y, back[y*W+x]
			for _, d := range dirs8 {
				nx, ny := x+d.X, y+d.Y
				if l.In(nx, ny) && back[ny*W+nx] >= 0 && back[ny*W+nx] < bd {
					bx, by, bd = nx, ny, back[ny*W+nx]
				}
			}
			if bx == x && by == y {
				break
			}
			x, y = bx, by
		}
	}
	nz := Noise{uint32(seed) + 5}
	crystals := 0
	for y := 1; y < H-1; y++ {
		for x := 1; x < W-1; x++ {
			i := y*W + x
			n := nz.FBM(float64(x)*0.09, float64(y)*0.18, 3)
			if l.T[i] == floor && !protect[i] {
				if s.Style == 1 && n < 0.3 {
					l.T[i] = TWater
					if n < 0.24 {
						l.T[i] = TDeepWater
					}
				}
				if s.Style == 2 && n > 0.66 {
					l.T[i] = TLava
				}
				if l.T[i] == floor && l.rng.Intn(40) == 0 {
					if s.Style == 2 {
						l.T[i] = TBones
					} else {
						l.T[i] = TRubble
					}
				}
			}
			if l.T[i] == wall {
				nearFloor := false
				for _, d := range dirs4 {
					if l.At(x+d.X, y+d.Y) == floor {
						nearFloor = true
					}
				}
				if nearFloor && crystals < 55 && ((s.Style == 1 && l.rng.Intn(22) == 0) || (s.Style == 2 && l.rng.Intn(120) == 0)) {
					l.T[i] = TCrystal
					crystals++
				}
			}
		}
	}
	if s.Style == 2 {
		for range 10 {
			i := best[l.rng.Intn(len(best))]
			if l.T[i] == floor && !protect[i] {
				l.T[i] = TBrazier
			}
		}
	} else {
		for range 4 {
			i := best[l.rng.Intn(len(best))]
			if l.T[i] == floor && !protect[i] {
				l.T[i] = TChest
			}
		}
	}
	safe := []Pos{{ux, uy}}
	if s.Boss == "wanderer" {
		c := l.hearth(fx, fy)
		fx, fy = c.X, c.Y
		// no packs in the chamber: he fights alone
		safe = append(safe, c, Pos{c.X - 10, c.Y}, Pos{c.X + 10, c.Y})
	}
	l.Set(ux, uy, TStairsUp)
	l.Links = append(l.Links, Link{ux, uy, ux, uy, s.Up, -1, -1})
	l.Start = Pos{ux, uy}
	l.Start.X, l.Start.Y = l.FreeNear(ux, uy, -1, -1)
	if s.Down != "" {
		l.Set(fx, fy, TStairsDown)
		l.Links = append(l.Links, Link{fx, fy, fx, fy, s.Down, -1, -1})
	}
	l.SealedDown, l.SealedAt = s.SealedDown, Pos{fx, fy}
	if s.Boss != "" {
		b := placeMonster(l, s.Boss, fx, fy, s.Depth, RankBoss)
		b.Awake = false
		if s.Boss == "oracle" {
			for range 3 {
				placeMonster(l, "wisp", fx+l.rng.Intn(5)-2, fy+l.rng.Intn(3)-1, s.Depth, RankNormal)
			}
		}
	}
	switch {
	case s.Boss == "wanderer":
		l.LoreKey = "hearth"
	case s.Style == 1:
		l.LoreKey = "grotto"
	default:
		l.LoreKey = "abyss"
	}
	populate(l, s.SpawnTable, 16+s.Depth, safe)
	l.finalize()
	return l
}

// bfsDist returns step distances over walkable cells (-1 unreachable).
func bfsDist(l *Level, sx, sy int) []int {
	d := make([]int, l.W*l.H)
	for i := range d {
		d[i] = -1
	}
	q := []int{sy*l.W + sx}
	d[q[0]] = 0
	for len(q) > 0 {
		c := q[0]
		q = q[1:]
		cx, cy := c%l.W, c/l.W
		for _, dd := range dirs8 {
			nx, ny := cx+dd.X, cy+dd.Y
			if !l.In(nx, ny) {
				continue
			}
			ni := ny*l.W + nx
			if d[ni] >= 0 || !l.Walkable(nx, ny) {
				continue
			}
			d[ni] = d[c] + 1
			q = append(q, ni)
		}
	}
	return d
}

// hearth carves the Last Wanderer's chamber around (cx, cy): an open oval
// ringed by braziers, with lava pooled against the wall where no passage
// comes in. The ring sits just beyond his reach from the middle, so the
// braziers burn until he walks toward them. The chamber moves
// in from the map's edge far enough for the whole ring to fit, never so
// far that it leaves (cx, cy) outside, so it stays joined to the cave.
// It returns the chamber's center.
func (l *Level) hearth(cx, cy int) Pos {
	const rx, ry = 16.0, 8.0
	cx = mini(maxi(cx, 15), l.W-16)
	cy = mini(maxi(cy, 8), l.H-9)
	in := func(x, y float64, rx, ry float64) bool {
		dx, dy := (x-float64(cx))/rx, (y-float64(cy))/ry
		return dx*dx+dy*dy < 1
	}
	for y := 1; y < l.H-1; y++ {
		for x := 1; x < l.W-1; x++ {
			if in(float64(x), float64(y), rx, ry) {
				l.Set(x, y, TCaveFloor)
			}
		}
	}
	for k := range 6 {
		a := math.Pi/6 + float64(k)*math.Pi/3
		bx := cx + int(math.Round((rx-2.5)*math.Cos(a)))
		by := cy + int(math.Round((ry-1.5)*math.Sin(a)))
		if l.In(bx, by) && l.At(bx, by) == TCaveFloor {
			l.Set(bx, by, TBrazier)
		}
		// lava on the rim between braziers, only against solid rock
		a += math.Pi / 6
		lx := cx + int(math.Round((rx-1)*math.Cos(a)))
		ly := cy + int(math.Round((ry-1)*math.Sin(a)))
		if !l.In(lx, ly) || l.At(lx, ly) != TCaveFloor {
			continue
		}
		closed := true
		for _, d := range dirs8 {
			x, y := lx+d.X, ly+d.Y
			if !in(float64(x), float64(y), rx, ry) && l.At(x, y) != TCaveWall {
				closed = false
			}
		}
		if closed {
			l.Set(lx, ly, TLava)
		}
	}
	return Pos{cx, cy}
}
