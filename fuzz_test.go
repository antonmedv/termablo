package main

import (
	"fmt"
	"testing"
)

// Every key the game reacts to, plus a few it should ignore.
var fuzzKeys = []string{
	"up", "down", "left", "right", "y", "u", "b", "n", ".", "g",
	"f", "r", "q", "w", "t", "tab", "shift+tab", "o", "i", "c", "m", "?",
	"esc", "enter", " ", "e", "d", "s", "1", "2", "3", "4", "h", "l", "j", "k", "x",
}

// FuzzKeys drives the real key handler with random input and checks the
// game state stays sane. Run longer with: go test -fuzz FuzzKeys
func FuzzKeys(f *testing.F) {
	f.Add(int64(1), []byte{16, 16, 16, 16, 16, 16})             // auto-explore
	f.Add(int64(2), []byte{17, 21, 22, 22, 22, 20, 21, 29, 31}) // inventory
	f.Add(int64(3), []byte{14, 10, 10, 11, 12, 13, 14})         // spells, potions, portal
	f.Add(int64(4), []byte{0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9}) // walking
	f.Fuzz(func(t *testing.T, seed int64, keys []byte) {
		if len(keys) > 300 {
			keys = keys[:300]
		}
		m := &model{g: NewGame(seed % 64), scr: NewScreen(100, 32), seed: seed}
		m.g.Mode = ModePlay
		m.g.P.HP = 1e4 // stay alive long enough to reach odd states
		m.g.P.Vit = 5000
		m.g.P.recalc()
		for i, k := range keys {
			key := fuzzKeys[int(k)%len(fuzzKeys)]
			if m.key(key) {
				return // quit
			}
			for n := 0; m.g.auto && n < 40; n++ {
				if !m.g.autoStep() {
					m.g.auto = false
				}
			}
			if err := checkGame(m.g); err != nil {
				t.Fatalf("after key %d %q: %v", i, key, err)
			}
		}
		m.g.Draw(m.scr)
		_ = m.scr.String()
	})
}

// checkGame reports the first broken invariant of a running game.
func checkGame(g *Game) error {
	p, l := g.P, g.Lv
	switch {
	case l == nil:
		return fmt.Errorf("no current level")
	case g.Mode == ModeDead:
		return nil
	case !l.In(p.X, p.Y):
		return fmt.Errorf("player out of bounds at %d,%d", p.X, p.Y)
	case !l.Walkable(p.X, p.Y):
		return fmt.Errorf("player inside %s", tdefs[l.At(p.X, p.Y)].Name)
	case p.HP > float64(p.MaxHP()) || p.MP > float64(p.MaxMP()) || p.MP < 0:
		return fmt.Errorf("life %v/%d mana %v/%d", p.HP, p.MaxHP(), p.MP, p.MaxMP())
	case len(p.Inv) > invMax:
		return fmt.Errorf("pack holds %d/%d", len(p.Inv), invMax)
	case p.Gold < 0 || p.HPot < 0 || p.MPot < 0 || p.Scrolls < 0:
		return fmt.Errorf("negative gold/potions/scrolls")
	case p.Points < 0:
		return fmt.Errorf("negative stat points")
	}
	if m := l.MonsterAt(p.X, p.Y); m != nil {
		return fmt.Errorf("%s stands on the player", m.Name)
	}
	cells := map[Pos]string{}
	for _, m := range l.Monsters {
		if m.Dead {
			continue
		}
		if !l.Walkable(m.X, m.Y) {
			return fmt.Errorf("%s inside %s", m.Name, tdefs[l.At(m.X, m.Y)].Name)
		}
		if o, ok := cells[Pos{m.X, m.Y}]; ok {
			return fmt.Errorf("%s and %s share %d,%d", m.Name, o, m.X, m.Y)
		}
		cells[Pos{m.X, m.Y}] = m.Name
	}
	for slot, it := range p.Eq {
		if it != nil && (it.Kind != IKEquip || !eqSlotOK(slot, it)) {
			return fmt.Errorf("%s equipped in slot %d", it.Name, slot)
		}
	}
	if w := p.Eq[EqWeapon]; w != nil && w.Base.TwoHanded && p.Eq[EqOffhand] != nil {
		return fmt.Errorf("two-handed %s with %s in off hand", w.Name, p.Eq[EqOffhand].Name)
	}
	return nil
}

var eqSlotFor = [EqCount]Slot{SlotWeapon, SlotOffhand, SlotHelm, SlotArmor, SlotGloves, SlotBoots, SlotRing, SlotRing, SlotAmulet}

func eqSlotOK(eq int, it *Item) bool { return it.Slot() == eqSlotFor[eq] }
