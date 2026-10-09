package i18n

import (
	"slices"
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

// cellWidth is the cells r takes after prev, as drawn: an alef after a
// lam joins it in one ligature cell (see shape).
func cellWidth(prev, r rune) int {
	if prev == 0x0644 {
		if _, ok := lamAlef[r]; ok {
			return 0
		}
	}
	return RuneWidth(r)
}

// Width is how many terminal cells a string takes once drawn.
func Width(s string) int {
	w, prev := 0, rune(0)
	for _, r := range s {
		w += cellWidth(prev, r)
		prev = r
	}
	return w
}

// Truncate cuts s to at most w cells, never splitting a wide rune.
func Truncate(s string, w int) string {
	n, prev := 0, rune(0)
	for i, r := range s {
		rw := cellWidth(prev, r)
		if n+rw > w {
			return s[:i]
		}
		n += rw
		prev = r
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

// Link is a span of dialog text that names a topic, written
// [words](topic) in the catalog: Start and End are byte offsets into the
// text with the markup taken out.
type Link struct {
	Start, End int
	Topic      string
}

// ParseLinks takes the [words](topic) markup out of a text and reports
// where each link's words are.
func ParseLinks(s string) (string, []Link) {
	var b strings.Builder
	var links []Link
	for {
		i := strings.IndexByte(s, '[')
		if i < 0 {
			break
		}
		j := strings.Index(s[i:], "](")
		k := -1
		if j > 0 {
			k = strings.IndexByte(s[i+j:], ')')
		}
		if k < 0 {
			break
		}
		words, topic := s[i+1:i+j], s[i+j+2:i+j+k]
		b.WriteString(s[:i])
		links = append(links, Link{Start: b.Len(), End: b.Len() + len(words), Topic: topic})
		b.WriteString(words)
		s = s[i+j+k+1:]
	}
	b.WriteString(s)
	return b.String(), links
}

// Links is the set of topics a text links to, sorted.
func Links(s string) []string {
	_, links := ParseLinks(s)
	var out []string
	for _, l := range links {
		if !slices.Contains(out, l.Topic) {
			out = append(out, l.Topic)
		}
	}
	slices.Sort(out)
	return out
}
