package main

import "math"

// RGB is a linear-ish float color used for lighting math.
type RGB struct{ R, G, B float32 }

func C(r, g, b float32) RGB { return RGB{r, g, b} }

func (c RGB) Add(o RGB) RGB       { return RGB{c.R + o.R, c.G + o.G, c.B + o.B} }
func (c RGB) Mul(o RGB) RGB       { return RGB{c.R * o.R, c.G * o.G, c.B * o.B} }
func (c RGB) Scale(f float32) RGB { return RGB{c.R * f, c.G * f, c.B * f} }
func (c RGB) Lerp(o RGB, t float32) RGB {
	return RGB{c.R + (o.R-c.R)*t, c.G + (o.G-c.G)*t, c.B + (o.B-c.B)*t}
}
func (c RGB) Max() float32 {
	m := c.R
	if c.G > m {
		m = c.G
	}
	if c.B > m {
		m = c.B
	}
	return m
}
func (c RGB) Lum() float32 { return 0.3*c.R + 0.55*c.G + 0.15*c.B }

// Col8 is a terminal truecolor value.
type Col8 struct{ R, G, B uint8 }

func clamp8(f float32) uint8 {
	if f <= 0 {
		return 0
	}
	if f >= 1 {
		return 255
	}
	return uint8(f*255 + 0.5)
}

func (c RGB) C8() Col8 { return Col8{clamp8(c.R), clamp8(c.G), clamp8(c.B)} }

// tone maps accumulated light energy to a 0..1 brightness with soft saturation,
// so overlapping lights blend instead of clipping.
func tone(x float32) float32 {
	if x <= 0 {
		return 0
	}
	return 1 - float32(math.Exp(float64(-x*1.6)))
}

func toneRGB(c RGB) RGB { return RGB{tone(c.R), tone(c.G), tone(c.B)} }

func clampf(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Common UI colors.
var (
	colBlack   = Col8{0, 0, 0}
	colWhite   = C(.92, .9, .86)
	colGray    = C(.55, .55, .58)
	colDim     = C(.35, .35, .4)
	colRed     = C(.9, .25, .2)
	colGreen   = C(.45, .85, .4)
	colBlue    = C(.4, .6, 1)
	colGold    = C(1, .82, .35)
	colOrange  = C(1, .55, .2)
	colYellow  = C(1, 1, .45)
	colCyan    = C(.45, .9, 1)
	colPurple  = C(.75, .45, 1)
	colBot     = C(.55, .95, .8) // the scripted player's own log lines
	colBorder  = C(.45, .32, .2)
	colPanelBG = C(.035, .03, .03)
)
