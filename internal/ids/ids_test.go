package ids

import "testing"

func TestNewValid(t *testing.T) {
	for _, n := range []int{6, 8} {
		id := New(n)
		if len(id) != n*2 {
			t.Errorf("New(%d) = %q, want %d characters", n, id, n*2)
		}
		if !Valid(id, n) {
			t.Errorf("Valid(%q, %d) = false, want true", id, n)
		}
		// a different length must not validate, or one service's handler
		// would accept another's ids
		if Valid(id, n+1) {
			t.Errorf("Valid(%q, %d) = true, want false", id, n+1)
		}
	}

	for _, bad := range []string{"", "zz", "GGGGGGGGGGGG", "abcdefabcde"} {
		if Valid(bad, 6) {
			t.Errorf("Valid(%q, 6) = true, want false", bad)
		}
	}
}

func TestNewIsRandom(t *testing.T) {
	seen := map[string]bool{}
	for range 100 {
		id := New(8)
		if seen[id] {
			t.Fatalf("New(8) repeated %q within 100 calls", id)
		}
		seen[id] = true
	}
}
