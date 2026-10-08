package main

import (
	"strings"
	"testing"
)

func rowText(s *Screen, y int) string {
	var b strings.Builder
	for _, c := range s.C[y*s.W : (y+1)*s.W] {
		if c.Ch != wideTail {
			b.WriteRune(c.Ch)
		}
	}
	return b.String()
}

// In a right-to-left screen, padding stays where it was put: a padded
// number keeps its column, right-aligned Arabic reaches the edge.
func TestScreenRTLAlignment(t *testing.T) {
	s := NewScreen(20, 2)
	s.RTL = true
	s.Text(0, 0, "      12/40", colWhite.C8())
	if got := rowText(s, 0); !strings.HasPrefix(got, "      12/40") {
		t.Errorf("padded number moved: %q", got)
	}
	s.TextRight(0, 1, 20, "السلام", colWhite.C8())
	if got := rowText(s, 1); strings.HasSuffix(got, " ") {
		t.Errorf("right-aligned Arabic stops short of the edge: %q", got)
	}
}

// A wide character fills two cells, and one drawn over by a narrow
// character no longer pushes the rest of the row right.
func TestScreenWide(t *testing.T) {
	s := NewScreen(6, 1)
	if n := s.Text(0, 0, "你好", colWhite.C8()); n != 4 {
		t.Fatalf("wrote %d cells, want 4", n)
	}
	s.Text(1, 0, "x", colWhite.C8())
	out := s.String()
	if strings.Contains(out, "你") || !strings.Contains(out, "x") || !strings.Contains(out, "好") {
		t.Errorf("overdrawn wide character: %q", out)
	}
}
