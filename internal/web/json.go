package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Read decodes a small json request body into into, rejecting unknown fields
// so a typo in the ui surfaces as a 400 instead of a silently ignored value.
func Read(w http.ResponseWriter, r *http.Request, into any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return errBadRequest
	}
	return nil
}

// ReadAny decodes a larger body and ignores fields it does not know, for the
// patch-style endpoints whose ui sends a whole object back rather than a diff.
func ReadAny(w http.ResponseWriter, r *http.Request, into any) error {
	defer func() { _ = r.Body.Close() }()

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(into); err != nil {
		return errBadJSON
	}
	return nil
}

// JSON writes body as the response, which is all any of these apis return.
// Nothing here is ever cacheable: every endpoint reports live state.
func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// Fail writes err in the {"error": "..."} shape every frontend reads.
func Fail(w http.ResponseWriter, status int, err error) {
	JSON(w, status, map[string]string{"error": err.Error()})
}

// Disposition ships both a plain ascii fallback and the real utf-8 name, since
// video titles and uploaded filenames are full of characters a bare
// filename= cannot carry.
func Disposition(name string) string {
	ascii := strings.Map(func(r rune) rune {
		if r < 32 || r > 126 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", ascii, url.PathEscape(name))
}
