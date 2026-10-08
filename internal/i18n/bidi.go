package i18n

// A terminal draws one character per cell, left to right, and most
// terminals neither join Arabic letters nor reorder right-to-left text.
// Visual does both before the text reaches the screen: it picks each
// letter's contextual form (Arabic Presentation Forms-B) and lays a line
// out in display order with a simplified Unicode Bidirectional
// Algorithm, enough for one line of game text: runs of Arabic, Latin
// words and numbers, and the punctuation between them.

// forms are an Arabic letter's isolated, final, initial and medial
// presentation forms; 0 where the letter has none. A letter with no
// initial form joins only to the letter before it.
type forms [4]rune

var arabic = map[rune]forms{
	0x0621: {0xFE80, 0, 0, 0},
	0x0622: {0xFE81, 0xFE82, 0, 0},
	0x0623: {0xFE83, 0xFE84, 0, 0},
	0x0624: {0xFE85, 0xFE86, 0, 0},
	0x0625: {0xFE87, 0xFE88, 0, 0},
	0x0626: {0xFE89, 0xFE8A, 0xFE8B, 0xFE8C},
	0x0627: {0xFE8D, 0xFE8E, 0, 0},
	0x0628: {0xFE8F, 0xFE90, 0xFE91, 0xFE92},
	0x0629: {0xFE93, 0xFE94, 0, 0},
	0x062A: {0xFE95, 0xFE96, 0xFE97, 0xFE98},
	0x062B: {0xFE99, 0xFE9A, 0xFE9B, 0xFE9C},
	0x062C: {0xFE9D, 0xFE9E, 0xFE9F, 0xFEA0},
	0x062D: {0xFEA1, 0xFEA2, 0xFEA3, 0xFEA4},
	0x062E: {0xFEA5, 0xFEA6, 0xFEA7, 0xFEA8},
	0x062F: {0xFEA9, 0xFEAA, 0, 0},
	0x0630: {0xFEAB, 0xFEAC, 0, 0},
	0x0631: {0xFEAD, 0xFEAE, 0, 0},
	0x0632: {0xFEAF, 0xFEB0, 0, 0},
	0x0633: {0xFEB1, 0xFEB2, 0xFEB3, 0xFEB4},
	0x0634: {0xFEB5, 0xFEB6, 0xFEB7, 0xFEB8},
	0x0635: {0xFEB9, 0xFEBA, 0xFEBB, 0xFEBC},
	0x0636: {0xFEBD, 0xFEBE, 0xFEBF, 0xFEC0},
	0x0637: {0xFEC1, 0xFEC2, 0xFEC3, 0xFEC4},
	0x0638: {0xFEC5, 0xFEC6, 0xFEC7, 0xFEC8},
	0x0639: {0xFEC9, 0xFECA, 0xFECB, 0xFECC},
	0x063A: {0xFECD, 0xFECE, 0xFECF, 0xFED0},
	0x0640: {0x0640, 0x0640, 0x0640, 0x0640}, // tatweel joins both ways
	0x0641: {0xFED1, 0xFED2, 0xFED3, 0xFED4},
	0x0642: {0xFED5, 0xFED6, 0xFED7, 0xFED8},
	0x0643: {0xFED9, 0xFEDA, 0xFEDB, 0xFEDC},
	0x0644: {0xFEDD, 0xFEDE, 0xFEDF, 0xFEE0},
	0x0645: {0xFEE1, 0xFEE2, 0xFEE3, 0xFEE4},
	0x0646: {0xFEE5, 0xFEE6, 0xFEE7, 0xFEE8},
	0x0647: {0xFEE9, 0xFEEA, 0xFEEB, 0xFEEC},
	0x0648: {0xFEED, 0xFEEE, 0, 0},
	0x0649: {0xFEEF, 0xFEF0, 0, 0},
	0x064A: {0xFEF1, 0xFEF2, 0xFEF3, 0xFEF4},
}

// lamAlef is the lam-alef ligature for each alef: isolated, final.
var lamAlef = map[rune][2]rune{
	0x0622: {0xFEF5, 0xFEF6},
	0x0623: {0xFEF7, 0xFEF8},
	0x0625: {0xFEF9, 0xFEFA},
	0x0627: {0xFEFB, 0xFEFC},
}

// harakat are the vowel marks; the cell grid has no room for them.
func harakah(r rune) bool { return r >= 0x064B && r <= 0x0652 || r == 0x0670 }

func joinsNext(r rune) bool { f, ok := arabic[r]; return ok && f[2] != 0 }
func joinsPrev(r rune) bool { f, ok := arabic[r]; return ok && f[1] != 0 }

// shape replaces Arabic letters with their contextual forms.
func shape(rs []rune) []rune {
	in := rs[:0:0]
	for _, r := range rs {
		if !harakah(r) {
			in = append(in, r)
		}
	}
	out := make([]rune, 0, len(in))
	for i := 0; i < len(in); i++ {
		r := in[i]
		f, ok := arabic[r]
		if !ok {
			out = append(out, r)
			continue
		}
		prev := i > 0 && joinsNext(in[i-1])
		if r == 0x0644 && i+1 < len(in) {
			if lig, ok := lamAlef[in[i+1]]; ok {
				if prev {
					out = append(out, lig[1])
				} else {
					out = append(out, lig[0])
				}
				i++
				continue
			}
		}
		next := joinsNext(r) && i+1 < len(in) && joinsPrev(in[i+1])
		switch {
		case prev && next:
			out = append(out, f[3])
		case prev && f[1] != 0:
			out = append(out, f[1])
		case next:
			out = append(out, f[2])
		default:
			out = append(out, f[0])
		}
	}
	return out
}

// bidi classes, simplified.
const (
	bL  = iota // strong left-to-right
	bR         // strong right-to-left
	bEN        // number
	bN         // neutral: spaces, punctuation
)

func isRTL(r rune) bool {
	return r >= 0x0590 && r <= 0x08FF || r >= 0xFB1D && r <= 0xFDFF || r >= 0xFE70 && r <= 0xFEFF
}

func class(r rune) int {
	switch {
	case isRTL(r):
		return bR
	case r >= '0' && r <= '9':
		return bEN
	case r < 0x80 && (r < 'a' || r > 'z') && (r < 'A' || r > 'Z'):
		return bN
	case r >= 0x2000 && r <= 0x2BFF || r >= 0x3000 && r <= 0x303F || r == 0xB7 || r == 0xA0:
		return bN // punctuation, arrows, box drawing, symbols
	}
	return bL
}

// HasRTL reports whether s holds right-to-left letters.
func HasRTL(s string) bool {
	for _, r := range s {
		if isRTL(r) {
			return true
		}
	}
	return false
}

// mirror swaps paired characters in right-to-left runs so they still
// face the right way. Arrows are mirrored too: in this game they name
// keys, and "←→" must still read left, right.
var mirror = map[rune]rune{'(': ')', ')': '(', '[': ']', ']': '[', '{': '}', '}': '{', '<': '>', '>': '<', '«': '»', '»': '«', '‹': '›', '›': '‹', '←': '→', '→': '←', '◂': '▸', '▸': '◂'}

// Visual is a line of text in display order with Arabic shaped. rtl is
// the paragraph direction: true for a right-to-left language's text,
// false to only fix up Arabic words inside left-to-right text. A line
// with nothing right-to-left in a left-to-right paragraph comes back as
// it was.
func Visual(s string, rtl bool) string {
	if !rtl && !HasRTL(s) {
		return s
	}
	rs := shape([]rune(s))
	n := len(rs)
	cls := make([]int, n)
	for i, r := range rs {
		cls[i] = class(r)
	}
	// Signs and separators that belong to a number travel with it:
	// "+12%", "3-7", "12/40", "1.5".
	for i, r := range rs {
		if cls[i] != bN {
			continue
		}
		before := i > 0 && cls[i-1] == bEN
		after := i+1 < n && cls[i+1] == bEN
		switch {
		case (r == '+' || r == '-') && after:
			cls[i] = bEN
		case (r == '.' || r == ',' || r == ':' || r == '/' || r == '-') && before && after:
			cls[i] = bEN
		case r == '%' && before:
			cls[i] = bEN
		}
	}
	para := bL
	if rtl {
		para = bR
	}
	// A number counts as right-to-left when it decides a neutral's side.
	side := func(c int) int {
		if c == bEN {
			return bR
		}
		return c
	}
	for i := 0; i < n; {
		if cls[i] != bN {
			i++
			continue
		}
		j := i
		for j < n && cls[j] == bN {
			j++
		}
		prev, next := para, para
		if i > 0 {
			prev = side(cls[i-1])
		}
		if j < n {
			next = side(cls[j])
		}
		d := para
		if prev == next {
			d = prev
		}
		for k := i; k < j; k++ {
			cls[k] = d
		}
		i = j
	}
	lv := make([]int, n)
	base := 0
	if rtl {
		base = 1
	}
	maxLv := base
	for i, c := range cls {
		switch {
		case c == bR:
			lv[i] = 1
		case base == 1: // L and numbers inside a right-to-left line
			lv[i] = 2
		case c == bEN && i > 0 && lv[i-1] == 1: // a number after Arabic
			lv[i] = 2
		default:
			lv[i] = 0
		}
		maxLv = max(maxLv, lv[i])
	}
	for l := maxLv; l >= 1; l-- {
		for i := 0; i < n; {
			if lv[i] < l {
				i++
				continue
			}
			j := i
			for j < n && lv[j] >= l {
				j++
			}
			for a, b := i, j-1; a < b; a, b = a+1, b-1 {
				rs[a], rs[b] = rs[b], rs[a]
				lv[a], lv[b] = lv[b], lv[a]
			}
			i = j
		}
	}
	for i, r := range rs {
		if lv[i]%2 == 1 {
			if m, ok := mirror[r]; ok {
				rs[i] = m
			}
		}
	}
	return string(rs)
}
