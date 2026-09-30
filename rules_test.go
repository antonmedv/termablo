package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The knob registry covers every number in Rules once, each pointing at
// its own field, with today's value inside the range the tuner searches.
func TestKnobRegistry(t *testing.T) {
	r := DefaultRules()
	seen := map[*float64]string{}
	for _, k := range knobs {
		if knobByName(k.Name) == nil || knobByName(k.Name).Name != k.Name {
			t.Errorf("%s: not found by name", k.Name)
		}
		p := k.Ptr(r)
		if other, ok := seen[p]; ok {
			t.Errorf("%s and %s point at the same field", k.Name, other)
		}
		seen[p] = k.Name
		// BudgetMul's default is today's no-clip value, above the range the
		// tuner searches.
		if v := *p; (v < k.Lo || v > k.Hi) && k.Name != "BudgetMul" {
			t.Errorf("%s: default %v outside %v–%v", k.Name, v, k.Lo, k.Hi)
		}
		if k.Group != "combat" && k.Group != "economy" && k.Group != "loot" {
			t.Errorf("%s: group %q", k.Name, k.Group)
		}
		if k.Desc == "" || k.Step <= 0 || k.Step > (k.Hi-k.Lo)/2 {
			t.Errorf("%s: wants a description and a step inside the range, got %q, %v", k.Name, k.Desc, k.Step)
		}
		if k.Int != (k.Name == "DropChance" || k.Name == "GoldChance") {
			t.Errorf("%s: Int %v; dropLoot reads DropChance and GoldChance as whole numbers", k.Name, k.Int)
		}
	}
	rt := reflect.TypeOf(*r)
	for i := range rt.NumField() {
		f := rt.Field(i)
		if f.Type.Kind() != reflect.Float64 {
			continue
		}
		if k := knobByName(f.Name); k == nil {
			t.Errorf("Rules.%s is not in the registry", f.Name)
		} else if k.Ptr(r) != reflect.ValueOf(r).Elem().Field(i).Addr().Interface().(*float64) {
			t.Errorf("%s points at another field", f.Name)
		}
	}
}

// A rules file lays knobs over the defaults; unknown names and fractional
// whole-number knobs are errors, values outside the range are warnings.
func TestRulesFromFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	r, warnings, err := rulesFromFile(write("a.json", `{"HpLin": 0.3, "DropChance": 25}`))
	if err != nil || r.HpLin != 0.3 || r.DropChance != 25 || r.HpQuad != DefaultRules().HpQuad {
		t.Fatalf("overlay: %v, HpLin %v, DropChance %v", err, r.HpLin, r.DropChance)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "DropChance") {
		t.Errorf("warnings %q, want one about DropChance outside 8–20", warnings)
	}
	if diff := knobDiff(DefaultRules(), r); len(diff) != 2 || diff[0] != "HpLin: 0.32 → 0.3" {
		t.Errorf("diff %q", diff)
	}
	if _, _, err := rulesFromFile(write("b.json", `{"Nope": 1}`)); err == nil {
		t.Error("unknown knob accepted")
	}
	if _, _, err := rulesFromFile(write("c.json", `{"GoldChance": 30.5}`)); err == nil {
		t.Error("fractional GoldChance accepted")
	}
	if r, _, err := rulesFromFile(""); err != nil || *r != *DefaultRules() {
		t.Errorf("empty path: %v, %+v", err, r)
	}
}
