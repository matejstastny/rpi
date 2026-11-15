package human

import "testing"

func TestBytes(t *testing.T) {
	for _, tc := range []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{999, "999 B"},
		{1000, "1.0 kB"},
		{250_000_000, "250.0 MB"},
		{20_000_000_000, "20.0 GB"},
		{2_000_000_000_000, "2.0 TB"},
		{5_000_000_000_000_000, "5.0 PB"},
	} {
		if got := Bytes(tc.in); got != tc.want {
			t.Errorf("Bytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
