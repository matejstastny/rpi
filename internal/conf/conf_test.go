package conf

import (
	"testing"
	"time"
)

func TestString(t *testing.T) {
	t.Setenv("RPI_TEST", "  value  ")
	if got := String("RPI_TEST", "fallback"); got != "value" {
		t.Errorf("String trimmed = %q, want %q", got, "value")
	}

	t.Setenv("RPI_TEST", "   ")
	if got := String("RPI_TEST", "fallback"); got != "fallback" {
		t.Errorf("blank should fall back, got %q", got)
	}
}

func TestInt(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int
	}{
		{"4", 4},
		{"0", 2},    // below min
		{"-1", 2},   // below min
		{"nope", 2}, // unparseable
		{"", 2},     // unset
	} {
		t.Setenv("RPI_TEST", tc.raw)
		if got := Int("RPI_TEST", 2, 1); got != tc.want {
			t.Errorf("Int(%q) = %d, want %d", tc.raw, got, tc.want)
		}
	}
}

func TestDuration(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want time.Duration
	}{
		{"90s", 90 * time.Second},
		{"2m", 2 * time.Minute},
		{"0s", time.Hour}, // non-positive falls back
		{"soon", time.Hour},
		{"", time.Hour},
	} {
		t.Setenv("RPI_TEST", tc.raw)
		if got := Duration("RPI_TEST", time.Hour); got != tc.want {
			t.Errorf("Duration(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestBytes(t *testing.T) {
	const fallback = int64(5_000_000_000)
	for _, tc := range []struct {
		raw  string
		want int64
	}{
		{"900", 900},
		{"900B", 900},
		{"250kb", 250_000},
		{"250MB", 250_000_000},
		{"1.5GB", 1_500_000_000},
		{"2TB", 2_000_000_000_000},
		{"20 GB", 20_000_000_000}, // spaces are tolerated
		{"0", fallback},
		{"-4GB", fallback},
		{"lots", fallback},
		{"", fallback},
	} {
		t.Setenv("RPI_TEST", tc.raw)
		if got := Bytes("RPI_TEST", fallback); got != tc.want {
			t.Errorf("Bytes(%q) = %d, want %d", tc.raw, got, tc.want)
		}
	}
}
