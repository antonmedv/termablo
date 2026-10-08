package i18n

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/maml-dev/go-maml"
)

// Slug is the key segment for an English name: "Executioner's Axe" is
// executioners_axe. Data tables (item bases, affixes, monsters, tiles)
// are keyed by the slug of their English name, so a new row needs no key
// of its own, only a line in en.maml.
func Slug(s string) string {
	var b strings.Builder
	under := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r == '\'' || r == '’':
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			if under && b.Len() > 0 {
				b.WriteByte('_')
			}
			under = false
			b.WriteRune(r)
		default:
			under = true
		}
	}
	return b.String()
}

// Hash fingerprints a source entry, so a translation can tell when the
// English it was made from has changed.
func Hash(m *Msg) string {
	h := sha256.New()
	writeMsg(h, m)
	return hex.EncodeToString(h.Sum(nil))[:8]
}

func writeMsg(h interface{ Write([]byte) (int, error) }, m *Msg) {
	if m.Vars == nil {
		_, _ = h.Write([]byte(m.Text + "\x00" + m.Gender + "\x00"))
		return
	}
	for _, k := range m.Order {
		_, _ = h.Write([]byte(k + "\x01"))
		writeMsg(h, m.Vars[k])
	}
}

// Marshal writes entries as a MAML catalog, nesting dotted keys into
// objects in the order given.
func Marshal(keys []string, msgs map[string]*Msg) []byte {
	type node struct {
		order []string
		kids  map[string]*node
		msg   *Msg
	}
	root := &node{kids: map[string]*node{}}
	for _, k := range keys {
		n := root
		for _, seg := range strings.Split(k, ".") {
			c, ok := n.kids[seg]
			if !ok {
				c = &node{kids: map[string]*node{}}
				n.kids[seg] = c
				n.order = append(n.order, seg)
			}
			n = c
		}
		n.msg = msgs[k]
	}
	var b strings.Builder
	var write func(n *node, depth int)
	write = func(n *node, depth int) {
		ind := strings.Repeat("  ", depth)
		for i, seg := range n.order {
			c := n.kids[seg]
			if depth == 0 && i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(ind + key(seg) + ": ")
			if c.msg != nil {
				writeLeaf(&b, c.msg, depth+1)
			} else {
				b.WriteString("{\n")
				write(c, depth+1)
				b.WriteString(ind + "}")
			}
			b.WriteByte('\n')
		}
	}
	b.WriteString("{\n")
	write(root, 1)
	b.WriteString("}\n")
	return []byte(b.String())
}

func key(k string) string {
	if maml.IsIdentifierKey(k) {
		return k
	}
	return maml.QuoteString(k)
}

func writeLeaf(b *strings.Builder, m *Msg, depth int) {
	switch {
	case m.Vars == nil && m.Gender == "":
		b.WriteString(maml.QuoteString(m.Text))
	case m.Vars == nil:
		b.WriteString("{ text: " + maml.QuoteString(m.Text) + ", gender: " + maml.QuoteString(m.Gender) + " }")
	default:
		ind := strings.Repeat("  ", depth)
		b.WriteString("{\n")
		for _, k := range m.Order {
			b.WriteString(ind + "  " + key(k) + ": ")
			writeLeaf(b, m.Vars[k], depth+1)
			b.WriteByte('\n')
		}
		b.WriteString(ind + "}")
	}
}
