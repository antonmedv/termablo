package main

import "math"

// Aspect compensates for terminal cells being about twice as tall as wide,
// so light pools and sight radii look round on screen.
const Aspect = 2.0

// castRays marches rays outward from (ox,oy) and calls visit once per cell
// reached. Rays stop at (and include) the first opaque cell, and cannot slip
// between two diagonally touching walls. dist is in visual (aspect-corrected) units.
func castRays(l *Level, ox, oy int, radius float64, stamp []uint32, gen uint32, visit func(i int, d float32)) {
	if !l.In(ox, oy) {
		return
	}
	oi := oy*l.W + ox
	stamp[oi] = gen
	if visit != nil {
		visit(oi, 0)
	}
	n := int(2*math.Pi*radius*Aspect*1.4) + 24
	const step = 0.2
	cx, cy := float64(ox)+0.5, float64(oy)+0.5
	W, H := l.W, l.H
	T := l.T
	for r := 0; r < n; r++ {
		a := 2 * math.Pi * (float64(r) + 0.5) / float64(n)
		dx, dy := math.Cos(a)*Aspect, math.Sin(a)
		lx, ly := ox, oy
		for t := step; t <= radius; t += step {
			x := int(math.Floor(cx + dx*t))
			y := int(math.Floor(cy + dy*t))
			if x == lx && y == ly {
				continue
			}
			if x < 0 || y < 0 || x >= W || y >= H {
				break
			}
			if x != lx && y != ly && tdefs[T[y*W+lx]].BlockSight && tdefs[T[ly*W+x]].BlockSight {
				break
			}
			lx, ly = x, y
			i := y*W + x
			if stamp[i] != gen {
				stamp[i] = gen
				if visit != nil {
					fx, fy := float64(x-ox)/Aspect, float64(y-oy)
					visit(i, float32(math.Sqrt(fx*fx+fy*fy)))
				}
			}
			if tdefs[T[i]].BlockSight {
				break
			}
		}
	}
}

type litCell struct {
	i int32
	w float32
}

// Light is a colored point light with cached, occlusion-aware coverage.
type Light struct {
	X, Y      int
	Color     RGB
	Radius    float32
	Intensity float32
	Flicker   float32
	Pulse     float32
	Seed      float32
	Off       bool

	cells  []litCell
	cx, cy int
	ver    int
	level  *Level
}

func NewLight(x, y int, s *LightSpec, seed float32) *Light {
	return &Light{X: x, Y: y, Color: s.Color, Radius: s.Radius, Intensity: s.Intensity,
		Flicker: s.Flicker, Pulse: s.Pulse, Seed: seed, ver: -1}
}

// ensure recomputes coverage if the light moved or level geometry changed.
func (lt *Light) ensure(l *Level, stamp []uint32, gen *uint32) {
	if lt.level == l && lt.ver == l.LightVer && lt.cx == lt.X && lt.cy == lt.Y {
		return
	}
	lt.level, lt.ver, lt.cx, lt.cy = l, l.LightVer, lt.X, lt.Y
	lt.cells = lt.cells[:0]
	*gen++
	R := lt.Radius
	castRays(l, lt.X, lt.Y, float64(R), stamp, *gen, func(i int, d float32) {
		t := 1 - d/R
		if t <= 0 {
			return
		}
		// Soft quadratic falloff with a gentle hot core.
		w := float32(math.Pow(float64(t), 1.5))
		lt.cells = append(lt.cells, litCell{int32(i), w})
	})
}

// factor returns the animated intensity multiplier at time t (seconds).
func (lt *Light) factor(t float64) float32 {
	s := float64(lt.Seed)
	f := float32(1)
	if lt.Flicker > 0 {
		n := 0.5*math.Sin(t*7.3+s) + 0.3*math.Sin(t*13.7+s*2.1) + 0.2*math.Sin(t*27.1+s*0.7)
		f += lt.Flicker * float32(n)
	}
	if lt.Pulse > 0 {
		f += lt.Pulse * float32(math.Sin(t*1.7+s))
	}
	return f
}
