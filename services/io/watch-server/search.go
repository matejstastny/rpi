package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// result is the shape both providers are flattened into, which is also exactly
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

func (s *server) search(ctx context.Context, kind, query string) ([]result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []result{}, nil
	}

	switch kind {
	case "anime":
		return searchAniList(ctx, query)
	case "series":
		return s.searchTMDB(ctx, "tv", query)
	case "movie":
		return s.searchTMDB(ctx, "movie", query)
	default:
		return nil, fmt.Errorf("%q is not one of anime, series, movie", kind)
	}
}

const aniListQuery = `query ($q: String) {
  Page(perPage: 12) {
    media(search: $q, type: ANIME, sort: SEARCH_MATCH) {
      id
      title { romaji english }
      format
      episodes
      averageScore
      genres
      popularity
      siteUrl
      description(asHtml: false)
      startDate { year }
      coverImage { extraLarge large }
      studios(isMain: true) { nodes { name } }
    }
  }
}`

type aniListResponse struct {
	Data struct {
		Page struct {
			Media []struct {
				ID    int `json:"id"`
				Title struct {
					Romaji  string `json:"romaji"`
					English string `json:"english"`
				} `json:"title"`
				Format       string   `json:"format"`
				Episodes     int      `json:"episodes"`
				AverageScore int      `json:"averageScore"`
				Genres       []string `json:"genres"`
				Popularity   int      `json:"popularity"`
				SiteURL      string   `json:"siteUrl"`
				Description  string   `json:"description"`
				StartDate    struct {
					Year int `json:"year"`
				} `json:"startDate"`
				CoverImage struct {
					ExtraLarge string `json:"extraLarge"`
					Large      string `json:"large"`
				} `json:"coverImage"`
				Studios struct {
					Nodes []struct {
						Name string `json:"name"`
					} `json:"nodes"`
				} `json:"studios"`
			} `json:"media"`
		} `json:"Page"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// anilist needs no key, which is why it is the anime provider
func searchAniList(ctx context.Context, query string) ([]result, error) {
	body, err := json.Marshal(map[string]any{
		"query":     aniListQuery,
		"variables": map[string]string{"q": query}})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://graphql.anilist.co", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	res, err := searchClient.Do(req)
	if err != nil {
		return nil, errors.New("could not reach anilist")
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anilist answered %d", res.StatusCode)
	}

	var parsed aniListResponse
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, errors.New("anilist sent something unreadable")
	}
	if len(parsed.Errors) > 0 {
		return nil, errors.New(parsed.Errors[0].Message)
	}

	out := make([]result, 0, len(parsed.Data.Page.Media))
	for _, m := range parsed.Data.Page.Media {
		title := m.Title.English
		alt := m.Title.Romaji
		if title == "" {
			title, alt = m.Title.Romaji, ""
		}
		if title == alt {
			alt = ""
		}

		studio := ""
		if len(m.Studios.Nodes) > 0 {
			studio = m.Studios.Nodes[0].Name
		}
		poster := m.CoverImage.ExtraLarge
		if poster == "" {
			poster = m.CoverImage.Large
		}

		out = append(out, result{
			Provider:   "anilist",
			ProviderID: fmt.Sprint(m.ID),
			Title:      title,
			AltTitle:   alt,
			Year:       m.StartDate.Year,
			Format:     prettyFormat(m.Format),
			Episodes:   m.Episodes,
			Studio:     studio,
			Genres:     m.Genres,
			Synopsis:   stripHTML(m.Description),
			Score:      m.AverageScore,
			Poster:     poster,
			Link:       m.SiteURL,
			weight:     m.Popularity,
		})
	}

	rank(out, query)
	return out, nil
}

type tmdbResponse struct {
	Results []struct {
		ID           int      `json:"id"`
		Title        string   `json:"title"`
		Name         string   `json:"name"`
		OriginalName string   `json:"original_name"`
		OriginalText string   `json:"original_title"`
		Overview     string   `json:"overview"`
		PosterPath   string   `json:"poster_path"`
		ReleaseDate  string   `json:"release_date"`
		FirstAirDate string   `json:"first_air_date"`
		VoteAverage  float64  `json:"vote_average"`
		Popularity   float64  `json:"popularity"`
		GenreIDs     []int    `json:"genre_ids"`
		MediaType    string   `json:"media_type"`
		Origin       []string `json:"origin_country"`
	} `json:"results"`
	StatusMessage string `json:"status_message"`
}

// tmdb covers both live-action libraries; it is the only part of this that
// needs a key, so the anime side still works without one
func (s *server) searchTMDB(ctx context.Context, mediaType, query string) ([]result, error) {
	if s.cfg.tmdbKey == "" {
		return nil, errors.New("no tmdb key set, so series and movie lookup is off; add entries by hand or set WATCH_TMDB_KEY")
	}

	endpoint := fmt.Sprintf("https://api.themoviedb.org/3/search/%s?query=%s&include_adult=false&language=en-US&page=1",
		mediaType, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	// v4 bearer tokens and v3 keys look different; support whichever is pasted
	if strings.HasPrefix(s.cfg.tmdbKey, "ey") {
		req.Header.Set("Authorization", "Bearer "+s.cfg.tmdbKey)
	} else {
		q := req.URL.Query()
		q.Set("api_key", s.cfg.tmdbKey)
		req.URL.RawQuery = q.Encode()
	}

	res, err := searchClient.Do(req)
	if err != nil {
		return nil, errors.New("could not reach tmdb")
	}
	defer func() { _ = res.Body.Close() }()

	var parsed tmdbResponse
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, errors.New("tmdb sent something unreadable")
	}
	if res.StatusCode != http.StatusOK {
		if parsed.StatusMessage != "" {
			return nil, errors.New(strings.ToLower(parsed.StatusMessage))
		}
		return nil, fmt.Errorf("tmdb answered %d", res.StatusCode)
	}

	kindLabel := "Series"
	if mediaType == "movie" {
		kindLabel = "Movie"
	}

	out := make([]result, 0, len(parsed.Results))
	for _, m := range parsed.Results {
		title := m.Title
		if title == "" {
			title = m.Name
		}
		alt := m.OriginalText
		if alt == "" {
			alt = m.OriginalName
		}
		if alt == title {
			alt = ""
		}

		date := m.ReleaseDate
		if date == "" {
			date = m.FirstAirDate
		}
		year := 0
		if len(date) >= 4 {
			if _, err := fmt.Sscanf(date[:4], "%d", &year); err != nil {
				year = 0
			}
		}

		poster := ""
		if m.PosterPath != "" {
			poster = "https://image.tmdb.org/t/p/w500" + m.PosterPath
		}

		out = append(out, result{
			Provider:   "tmdb",
			ProviderID: fmt.Sprintf("%s/%d", mediaType, m.ID),
			Title:      title,
			AltTitle:   alt,
			Year:       year,
			Format:     kindLabel,
			Genres:     tmdbGenres(m.GenreIDs),
			Synopsis:   m.Overview,
			Score:      int(m.VoteAverage * 10),
			Poster:     poster,
			Link:       fmt.Sprintf("https://www.themoviedb.org/%s/%d", mediaType, m.ID),
			weight:     int(m.Popularity * 100),
		})
	}

	rank(out, query)
	return out, nil
}

// tmdb's search endpoints only return genre ids, and a second round trip per
// result just to name them is not worth it for a picker label
var tmdbGenreNames = map[int]string{
	28: "Action", 12: "Adventure", 16: "Animation", 35: "Comedy", 80: "Crime",
	99: "Documentary", 18: "Drama", 10751: "Family", 14: "Fantasy", 36: "History",
	27: "Horror", 10402: "Music", 9648: "Mystery", 10749: "Romance",
	878: "Science Fiction", 10770: "TV Movie", 53: "Thriller", 10752: "War",
	37: "Western", 10759: "Action & Adventure", 10762: "Kids", 10763: "News",
	10764: "Reality", 10765: "Sci-Fi & Fantasy", 10766: "Soap", 10767: "Talk",
	10768: "War & Politics",
}

func tmdbGenres(ids []int) []string {
	out := []string{}
	for _, id := range ids {
		if name, ok := tmdbGenreNames[id]; ok {
			out = append(out, name)
		}
	}
	return out
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
	switch f {
	case "TV":
		return "TV"
	case "TV_SHORT":
		return "TV Short"
	case "MOVIE":
		return "Movie"
	case "SPECIAL":
		return "Special"
	case "OVA", "ONA":
		return f
	case "MUSIC":
		return "Music"
	case "":
		return ""
	default:
		return strings.ToTitle(f)
	}
}

var htmlTag = regexp.MustCompile(`<[^>]*>`)

// anilist synopses are html even with asHtml false, and the <br> runs make a
// mess of a plain-text card
func stripHTML(in string) string {
	out := strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n").Replace(in)
	out = htmlTag.ReplaceAllString(out, "")
	out = strings.NewReplacer("&quot;", `"`, "&amp;", "&", "&#039;", "'", "&lt;", "<", "&gt;", ">", "&nbsp;", " ").Replace(out)

	lines := strings.Split(out, "\n")
	kept := make([]string, 0, len(lines))
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			kept = append(kept, l)
		}
	}
	return strings.Join(kept, "\n\n")
}
