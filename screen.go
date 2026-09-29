package main

import (
	"strconv"
	"unicode/utf8"
)

// Cell is one terminal character with truecolor foreground/background.
type Cell struct {
	Ch     rune
	FG, BG Col8
	Bold   bool
}

// Screen is an off-screen cell buffer serialized to ANSI once per frame.
type Screen struct {
	W, H int
	C    []Cell
	buf  []byte
}

func NewScreen(w, h int) *Screen {
	s := &Screen{}
	s.Resize(w, h)
	return s
}

func (s *Screen) Resize(w, h int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	s.W, s.H = w, h
	s.C = make([]Cell, w*h)
	s.Clear()
}

func (s *Screen) Clear() {
	for i := range s.C {
		s.C[i] = Cell{Ch: ' '}
	}
}

func (s *Screen) in(x, y int) bool { return x >= 0 && y >= 0 && x < s.W && y < s.H }

func (s *Screen) Set(x, y int, ch rune, fg, bg Col8) {
	if s.in(x, y) {
		s.C[y*s.W+x] = Cell{Ch: ch, FG: fg, BG: bg}
	}
}

func (s *Screen) SetBold(x, y int, ch rune, fg, bg Col8, bold bool) {
	if s.in(x, y) {
		s.C[y*s.W+x] = Cell{Ch: ch, FG: fg, BG: bg, Bold: bold}
	}
}

// Put writes a glyph keeping the existing background.
func (s *Screen) Put(x, y int, ch rune, fg Col8) {
	if s.in(x, y) {
		c := &s.C[y*s.W+x]
		c.Ch, c.FG, c.Bold = ch, fg, false
	}
}

// Text writes a string keeping the background. Returns cells written.
func (s *Screen) Text(x, y int, str string, fg Col8) int {
	n := 0
	for _, r := range str {
		s.Put(x+n, y, r, fg)
		n++
	}
	return n
}

func (s *Screen) TextBold(x, y int, str string, fg Col8) int {
	n := 0
	for _, r := range str {
		if s.in(x+n, y) {
			c := &s.C[y*s.W+x+n]
			c.Ch, c.FG, c.Bold = r, fg, true
		}
		n++
	}
	return n
}

func (s *Screen) Fill(x, y, w, h int, bg Col8) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			s.Set(xx, yy, ' ', bg, bg)
		}
	}
}

func (s *Screen) Box(x, y, w, h int, border, bg Col8, title string, titleCol Col8) {
	s.Fill(x, y, w, h, bg)
	for xx := x + 1; xx < x+w-1; xx++ {
		s.Set(xx, y, '─', border, bg)
		s.Set(xx, y+h-1, '─', border, bg)
	}
	for yy := y + 1; yy < y+h-1; yy++ {
		s.Set(x, yy, '│', border, bg)
		s.Set(x+w-1, yy, '│', border, bg)
	}
	s.Set(x, y, '┌', border, bg)
	s.Set(x+w-1, y, '┐', border, bg)
	s.Set(x, y+h-1, '└', border, bg)
	s.Set(x+w-1, y+h-1, '┘', border, bg)
	if title != "" {
		t := " " + title + " "
		tx := x + (w-utf8.RuneCountInString(t))/2
		s.TextBold(tx, y, t, titleCol)
	}
}

func appendColor(b []byte, prefix string, c Col8) []byte {
	b = append(b, prefix...)
	b = strconv.AppendInt(b, int64(c.R), 10)
	b = append(b, ';')
	b = strconv.AppendInt(b, int64(c.G), 10)
	b = append(b, ';')
	b = strconv.AppendInt(b, int64(c.B), 10)
	return append(b, 'm')
}

// String serializes the buffer, emitting escapes only when colors change.
func (s *Screen) String() string {
	b := s.buf[:0]
	for y := 0; y < s.H; y++ {
		var fg, bg Col8
		haveFG, haveBG, bold := false, false, false
		row := s.C[y*s.W : (y+1)*s.W]
		for _, c := range row {
			if !haveBG || c.BG != bg {
				b = appendColor(b, "\x1b[48;2;", c.BG)
				bg, haveBG = c.BG, true
			}
			if c.Ch != ' ' {
				if !haveFG || c.FG != fg {
					b = appendColor(b, "\x1b[38;2;", c.FG)
					fg, haveFG = c.FG, true
				}
				if c.Bold != bold {
					if c.Bold {
						b = append(b, "\x1b[1m"...)
					} else {
						b = append(b, "\x1b[22m"...)
					}
					bold = c.Bold
				}
			}
			b = utf8.AppendRune(b, c.Ch)
		}
		b = append(b, "\x1b[0m"...)
		if y < s.H-1 {
			b = append(b, '\n')
		}
	}
	s.buf = b
	return string(b)
}
