package web

import (
	"io/fs"
	"log"
	"net/http"
	"strings"
)

// Static serves the astro build embedded in the binary. Each service embeds
// its own web/dist as `//go:embed all:dist` and hands the result here, so a
// deployed binary needs no static root on disk.
func Static(embedded fs.FS) http.Handler {
	dist, err := fs.Sub(embedded, "dist")
	if err != nil {
		log.Fatalf("embedded ui: %v", err)
	}
	files := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// astro fingerprints everything under _astro, fonts included, so those
		// can be pinned forever while the pages themselves must always be
		// revalidated
		if strings.HasPrefix(r.URL.Path, "/_astro/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
