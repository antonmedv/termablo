package main

import "math"

type EffKind int

const (
	EffBolt EffKind = iota
	EffNova
	EffText
)

// Effect is a purely visual animation. Game logic resolves instantly; effects
// replay it with moving colored light.
type Effect struct {
	Kind  EffKind
	Path  []Pos
	X, Y  int
	T0    float64
	Dur   float64
	Glyph rune
	Col   RGB
	Text  string
	R     float64
	Light *Light
	Row   int
	lvl   *Level
}

func (g *Game) boltFx(path []Pos, col RGB, glyph rune, lit bool) {
	if len(path) == 0 {
		return
	}
	e := &Effect{Kind: EffBolt, Path: path, T0: g.time, Dur: math.Max(0.12, float64(len(path))/40), Glyph: glyph, Col: col, lvl: g.Lv}
	if lit {
		e.Light = NewLight(path[0].X, path[0].Y, &LightSpec{col, 4, 1.1, .15, 0}, float32(g.rng.Intn(100)))
	}
	g.Effects = append(g.Effects, e)
}

func (g *Game) novaFx(x, y int, col RGB, r float64) {
	e := &Effect{Kind: EffNova, X: x, Y: y, T0: g.time, Dur: 0.45, Col: col, R: r, lvl: g.Lv}
	e.Light = NewLight(x, y, &LightSpec{col, float32(r + 2), 1.2, 0, 0}, 0)
	g.Effects = append(g.Effects, e)
}

func (g *Game) textFx(x, y int, s string, col RGB) {
	row := 0
	for _, e := range g.Effects {
		if e.Kind == EffText && e.X == x && e.Y == y && g.time-e.T0 < 0.4 {
			row++
		}
	}
	g.Effects = append(g.Effects, &Effect{Kind: EffText, X: x, Y: y, T0: g.time, Dur: 0.9, Col: col, Text: s, Row: row, lvl: g.Lv})
}

func (e *Effect) progress(t float64) float64 {
	p := (t - e.T0) / e.Dur
	if p < 0 {
		p = 0
	}
	return p
}

func (e *Effect) boltPos(t float64) Pos {
	p := e.progress(t)
	i := int(p * float64(len(e.Path)))
	if i >= len(e.Path) {
		i = len(e.Path) - 1
	}
	return e.Path[i]
}

func (g *Game) updateEffects() {
	keep := g.Effects[:0]
	for _, e := range g.Effects {
		if e.lvl != g.Lv || e.progress(g.time) >= 1 {
			continue
		}
		switch e.Kind {
		case EffBolt:
			if e.Light != nil {
				p := e.boltPos(g.time)
				e.Light.X, e.Light.Y = p.X, p.Y
			}
		case EffNova:
			p := e.progress(g.time)
			e.Light.Radius = float32(1.5 + e.R*1.3*p)
			e.Light.Intensity = float32(1.4 * (1 - p))
			e.Light.ver = -1
		}
		keep = append(keep, e)
	}
	g.Effects = keep
}
