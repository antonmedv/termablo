package main

import (
	"reflect"
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
		// tuner searches (BALANCE.md §4 B).
		if v := *p; (v < k.Lo || v > k.Hi) && k.Name != "BudgetMul" {
			t.Errorf("%s: default %v outside %v–%v", k.Name, v, k.Lo, k.Hi)
		}
		if k.Group != "combat" && k.Group != "economy" && k.Group != "loot" {
			t.Errorf("%s: group %q", k.Name, k.Group)
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
