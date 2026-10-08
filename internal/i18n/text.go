package i18n

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// cells measures in terminal cells with East Asian ambiguous characters
// narrow, whatever the locale: the map's ·, ≈ and ♣ are one cell each,
// and a CJK locale must not make them two.
var cells = &runewidth.Condition{EastAsianWidth: false, StrictEmojiNeutral: true}

// RuneWidth is how many terminal cells a rune takes: 0, 1, or 2 for wide
// (CJK) characters.
func RuneWidth(r rune) int { return cells.RuneWidth(r) }

// Width is how many terminal cells a string takes.
func Width(s string) int {
	w := 0
	for _, r := range s {
		w += RuneWidth(r)
	}
	return w
}

// Truncate cuts s to at most w cells, never splitting a wide rune.
func Truncate(s string, w int) string {
	n := 0
	for i, r := range s {
		rw := RuneWidth(r)
		if n+rw > w {
			return s[:i]
		}
		n += rw
	}
	return s
}

// breakable reports whether a line may break before r without a space:
// CJK ideographs and kana stand alone.
func breakable(r rune) bool {
	return RuneWidth(r) == 2 && !strings.ContainsRune("，。、：；！？）」』】》", r)
}

// Wrap breaks s into lines of at most w cells, at spaces, or between
// wide characters in scripts written without spaces. A word longer than
// a line is cut.
func Wrap(s string, w int) []string {
	if w < 1 {
		w = 1
	}
	var lines []string
	var line strings.Builder
	lw := 0
	flush := func() {
		lines = append(lines, strings.TrimRight(line.String(), " "))
		line.Reset()
		lw = 0
	}
	for _, word := range tokens(s) {
		if word == "\n" {
			flush()
			continue
		}
		ww := Width(word)
		if word == " " {
			if lw > 0 && lw < w {
				line.WriteByte(' ')
				lw++
			}
			continue
		}
		if lw+ww > w && lw > 0 {
			flush()
		}
		for ww > w { // a word wider than the line
			head := Truncate(word, w-lw)
			if head == "" && lw > 0 {
				flush()
				continue
			}
			if head == "" { // one wide rune in a one-cell line
				_, n := utf8.DecodeRuneInString(word)
				head = word[:n]
			}
			line.WriteString(head)
			flush()
			word = word[len(head):]
			ww = Width(word)
		}
		line.WriteString(word)
		lw += ww
	}
	if lw > 0 || len(lines) == 0 {
		flush()
	}
	return lines
}

// tokens splits text into words, single spaces, newlines, and each wide
// character (with any closing punctuation after it) as its own word.
func tokens(s string) []string {
	var out []string
	rs := []rune(s)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case r == '\n':
			out = append(out, "\n")
			i++
		case unicode.IsSpace(r):
			out = append(out, " ")
			for i < len(rs) && rs[i] != '\n' && unicode.IsSpace(rs[i]) {
				i++
			}
		case breakable(r):
			j := i + 1
			for j < len(rs) && !unicode.IsSpace(rs[j]) && !breakable(rs[j]) && RuneWidth(rs[j]) == 2 {
				j++
			}
			out = append(out, string(rs[i:j]))
			i = j
		default:
			j := i
			for j < len(rs) && !unicode.IsSpace(rs[j]) && !breakable(rs[j]) {
				j++
			}
			out = append(out, string(rs[i:j]))
			i = j
		}
	}
	return out
}
