// Package conf reads the environment variables every service is configured
// with. The conf.d files are the real documentation; these are the fallbacks
// that let a service run from a bare `go run` with nothing set.
package conf

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// String returns the trimmed value of key, or fallback when it is unset or blank.
func String(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

// Int returns key parsed as an int. A value that does not parse, or is below
// min, falls back rather than failing the whole service over one typo in a
// conf.d file.
func Int(key string, fallback, min int) int {
	n, err := strconv.Atoi(String(key, ""))
	if err != nil || n < min {
		return fallback
	}
	return n
}

// Duration returns key parsed as a Go duration, falling back on anything
// unparseable or non-positive.
func Duration(key string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(String(key, ""))
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

// Bytes returns key parsed as a size with an optional unit suffix, so a
// conf.d can say 20GB instead of counting zeroes. Units are powers of 1000,
// matching how human.Bytes renders them back.
func Bytes(key string, fallback int64) int64 {
	raw := strings.ToLower(strings.ReplaceAll(String(key, ""), " ", ""))
	if raw == "" {
		return fallback
	}

	mult := int64(1)
	for _, u := range []struct {
		suffix string
		mult   int64
	}{
		{"tb", 1000 * 1000 * 1000 * 1000},
		{"gb", 1000 * 1000 * 1000},
		{"mb", 1000 * 1000},
		{"kb", 1000},
		{"b", 1},
	} {
		if strings.HasSuffix(raw, u.suffix) {
			raw, mult = strings.TrimSuffix(raw, u.suffix), u.mult
			break
		}
	}

	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || n <= 0 {
		return fallback
	}
	return int64(n * float64(mult))
}
