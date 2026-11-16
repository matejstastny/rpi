// Package ids makes the opaque hex handles the apis hand out for jobs and
// files.
package ids

import (
	"crypto/rand"
	"encoding/hex"
)

// New returns a random lowercase hex string of n bytes, so 2n characters.
func New(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand failing means the host is broken in a way nothing here can recover from
		panic(err)
	}
	return hex.EncodeToString(buf)
}

// Valid reports whether id could have come from New(n). Handlers check this
// before an id reaches the filesystem or a store lookup.
func Valid(id string, n int) bool {
	if len(id) != n*2 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
