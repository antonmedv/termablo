package main

import "math"

// Noise is a small seeded value-noise generator used by the map generators.
type Noise struct{ seed uint32 }

func hash2(x, y int32, seed uint32) uint32 {
	h := uint32(x)*374761393 + uint32(y)*668265263 + seed*2246822519
	h = (h ^ (h >> 13)) * 1274126177
	return h ^ (h >> 16)
}

func (n Noise) val(x, y int32) float64 {
	return float64(hash2(x, y, n.seed)&0xffffff) / float64(0xffffff)
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func (n Noise) At(x, y float64) float64 {
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := x-x0, y-y0
	sx, sy := fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	ix, iy := int32(x0), int32(y0)
	a, b := n.val(ix, iy), n.val(ix+1, iy)
	c, d := n.val(ix, iy+1), n.val(ix+1, iy+1)
	return lerp(lerp(a, b, sx), lerp(c, d, sx), sy)
}

func (n Noise) FBM(x, y float64, oct int) float64 {
	sum, amp, norm := 0.0, 1.0, 0.0
	for range oct {
		sum += n.At(x, y) * amp
		norm += amp
		amp *= 0.5
		x, y = x*2.03+17.1, y*2.03-9.7
	}
	return sum / norm
}
