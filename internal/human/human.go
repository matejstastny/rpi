// Package human renders numbers the way the web uis want to print them.
package human

import "fmt"

// Bytes formats a byte count in powers of 1000, which is what disk vendors,
// yt-dlp and transmission all report in.
func Bytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	for _, u := range []string{"kB", "MB", "GB", "TB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, u)
		}
	}
	return fmt.Sprintf("%.1f PB", value/unit)
}
