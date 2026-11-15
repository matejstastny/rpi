package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

// result is the shape every provider is flattened into, which is also exactly
// what the add dialog needs to build an entry
type result struct {
	Provider   string   `json:"provider"`
	ProviderID string   `json:"providerId"`
	Title      string   `json:"title"`
	AltTitle   string   `json:"altTitle,omitempty"`
	Year       int      `json:"year,omitempty"`
	Format     string   `json:"format,omitempty"`
	Episodes   int      `json:"episodes,omitempty"`
	Studio     string   `json:"studio,omitempty"`
	Genres     []string `json:"genres,omitempty"`
	Synopsis   string   `json:"synopsis,omitempty"`
	Score      int      `json:"score,omitempty"`
	Poster     string   `json:"poster,omitempty"`
	Link       string   `json:"link,omitempty"`

	// how widely watched it is, used only to break ties while ranking
	weight int `json:"-"`
}

var searchClient = &http.Client{Timeout: 12 * time.Second}

// anilist rejects an unidentified client outright, so every outbound request
// from here names itself
const userAgent = "watchlist/1.0 (+https://watch.elara.boo)"

type provider struct {
	name string
	find func(ctx context.Context, query string) ([]result, error)
}

// each library has a chain rather than one source: anilist has gone down for
// days at a time and tmdb needs a key that may not be set, so there is always
// a keyless second choice behind the preferred one
func (s *server) providers(kind string) []provider {
	// only set by tests, so the chain can be exercised without the network
	if s.providersOverride != nil {
		return s.providersOverride
	}

	switch kind {
	case "anime":
		return []provider{
			{"anilist", searchAniList},
			{"kitsu", searchKitsu},
		}

	case "series":
		out := []provider{}
		if s.cfg.tmdbKey != "" {
			out = append(out, provider{"tmdb", func(ctx context.Context, q string) ([]result, error) {
				return s.searchTMDB(ctx, "tv", q)
			}})
		}
		return append(out, provider{"tvmaze", searchTVmaze})

	case "movie":
		out := []provider{}
		if s.cfg.tmdbKey != "" {
			out = append(out, provider{"tmdb", func(ctx context.Context, q string) ([]result, error) {
				return s.searchTMDB(ctx, "movie", q)
			}})
		}
		return append(out, provider{"imdb", searchIMDb})

	default:
		return nil
	}
}

func (s *server) search(ctx context.Context, kind, query string) ([]result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []result{}, nil
	}

	chain := s.providers(kind)
	if len(chain) == 0 {
		return nil, fmt.Errorf("%q is not one of anime, series, movie", kind)
	}

	// an empty answer counts as a miss, so a provider having nothing on a
	// title still lets the next one try
	failures := []string{}
	for _, p := range chain {
		found, err := p.find(ctx, query)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", p.name, err))
			continue
		}
		if len(found) > 0 {
			rank(found, query)
			return found, nil
		}
		failures = append(failures, fmt.Sprintf("%s: nothing matched", p.name))
	}

	return nil, errors.New(strings.Join(failures, "; "))
}

// rank nudges the provider's own ordering: an exact or leading title match
// goes first, and inside a tie the more widely watched one wins. anilist's
// SEARCH_MATCH alone will happily put an obscure special above the show you
// obviously meant.
func rank(results []result, query string) {
	needle := normalizeTitle(query)

	score := func(r result) int {
		for _, title := range []string{normalizeTitle(r.Title), normalizeTitle(r.AltTitle)} {
			if title == "" {
				continue
			}
			switch {
			case title == needle:
				return 3
			case strings.HasPrefix(title, needle):
				return 2
			case strings.Contains(title, needle):
				return 1
			}
		}
		return 0
	}

	sort.SliceStable(results, func(i, j int) bool {
		si, sj := score(results[i]), score(results[j])
		if si != sj {
			return si > sj
		}
		return results[i].weight > results[j].weight
	})
}

var notAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func normalizeTitle(in string) string {
	return strings.TrimSpace(notAlnum.ReplaceAllString(strings.ToLower(in), " "))
}

func prettyFormat(f string) string {
	switch strings.ToUpper(f) {
	case "TV":
		return "TV"
	case "TV_SHORT", "TV SHORT":
		return "TV Short"
	case "MOVIE":
		return "Movie"
	case "SPECIAL":
		return "Special"
	case "OVA":
		return "OVA"
	case "ONA":
		return "ONA"
	case "MUSIC":
		return "Music"
	case "":
		return ""
	default:
		return strings.ToUpper(f[:1]) + strings.ToLower(f[1:])
	}
}

var htmlTag = regexp.MustCompile(`<[^>]*>`)

// synopses come back as html from anilist and tvmaze alike, and the <br> runs
// make a mess of a plain-text card
func stripHTML(in string) string {
	out := strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n", "</p>", "\n").Replace(in)
	out = htmlTag.ReplaceAllString(out, "")
	out = strings.NewReplacer("&quot;", `"`, "&amp;", "&", "&#039;", "'", "&apos;", "'",
		"&lt;", "<", "&gt;", ">", "&nbsp;", " ").Replace(out)

	lines := strings.Split(out, "\n")
	kept := make([]string, 0, len(lines))
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			kept = append(kept, l)
		}
	}
	return strings.Join(kept, "\n\n")
}

// fetchJSON is the shape every provider but anilist needs: a plain GET that
// identifies itself and reports a bad status as an error. accept is spelled
// out because kitsu is json:api and 406s anything but its own media type.
func fetchJSON(ctx context.Context, name, endpoint, accept string, into func(io.Reader) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if accept == "" {
		accept = "application/json"
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", userAgent)

	res, err := searchClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach %s", name)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("answered %d", res.StatusCode)
	}
	if err := into(res.Body); err != nil {
		return fmt.Errorf("sent something unreadable")
	}
	return nil
}

func yearOf(date string) int {
	if len(date) < 4 {
		return 0
	}
	year := 0
	if _, err := fmt.Sscanf(date[:4], "%d", &year); err != nil {
		return 0
	}
	return year
}
