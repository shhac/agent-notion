package v3

import "testing"

// TestBlockTypeMapsMutuallyConsistent guards the two hand-maintained
// vocabulary tables (v3→official in transforms.go, official→v3 in
// operations.go) against drift: every pair present in both directions must
// round-trip. Asymmetric entries (child_page/child_database collapse several
// v3 types; layout types have no official write path) are exempt by
// construction — the check only covers the shared domain.
func TestBlockTypeMapsMutuallyConsistent(t *testing.T) {
	for officialType, v3Type := range officialToV3Type {
		mapped, ok := blockTypeMap[v3Type]
		if !ok {
			t.Errorf("officialToV3Type[%q] = %q, but blockTypeMap has no entry for %q", officialType, v3Type, v3Type)
			continue
		}
		if mapped != officialType {
			t.Errorf("round trip broken: official %q → v3 %q → official %q", officialType, v3Type, mapped)
		}
	}
}
