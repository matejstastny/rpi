package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type tmdbResponse struct {
	Results []struct {
		ID           int     `json:"id"`
		Title        string  `json:"title"`
		Name         string  `json:"name"`
		OriginalName string  `json:"original_name"`
		OriginalText string  `json:"original_title"`
		Overview     string  `json:"overview"`
		PosterPath   string  `json:"poster_path"`
		ReleaseDate  string  `json:"release_date"`
		FirstAirDate string  `json:"first_air_date"`
		VoteAverage  float64 `json:"vote_average"`
		Popularity   float64 `json:"popularity"`
		GenreIDs     []int   `json:"genre_ids"`
	} `json:"results"`
	StatusMessage string `json:"status_message"`
}

// tmdb is the best source for both live-action libraries, and the only part
// of this that needs a key. without one the keyless fallbacks below take over.
func (s *server) searchTMDB(ctx context.Context, mediaType, query string) ([]result, error) {
	if s.cfg.tmdbKey == "" {
		return nil, errors.New("no key set")
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
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&parsed); err != nil {
		return nil, errors.New("sent something unreadable")
	}
	if res.StatusCode != http.StatusOK {
		if parsed.StatusMessage != "" {
			return nil, errors.New(strings.ToLower(strings.TrimRight(parsed.StatusMessage, ".")))
		}
		return nil, fmt.Errorf("answered %d", res.StatusCode)
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

		poster := ""
		if m.PosterPath != "" {
			poster = "https://image.tmdb.org/t/p/w500" + m.PosterPath
		}

		out = append(out, result{
			Provider:   "tmdb",
			ProviderID: fmt.Sprintf("%s/%d", mediaType, m.ID),
			Title:      title,
			AltTitle:   alt,
			Year:       yearOf(date),
			Format:     kindLabel,
			Genres:     tmdbGenres(m.GenreIDs),
			Synopsis:   m.Overview,
			Score:      int(m.VoteAverage * 10),
			Poster:     poster,
			Link:       fmt.Sprintf("https://www.themoviedb.org/%s/%d", mediaType, m.ID),
			weight:     int(m.Popularity * 100),
		})
	}
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

type tvmazeResult struct {
	Show struct {
		ID        int      `json:"id"`
		Name      string   `json:"name"`
		Type      string   `json:"type"`
		URL       string   `json:"url"`
		Genres    []string `json:"genres"`
		Premiered string   `json:"premiered"`
		Summary   string   `json:"summary"`
		Rating    struct {
			Average float64 `json:"average"`
		} `json:"rating"`
		Weight  int `json:"weight"`
		Network struct {
			Name string `json:"name"`
		} `json:"network"`
		WebChannel struct {
			Name string `json:"name"`
		} `json:"webChannel"`
		Image struct {
			Original string `json:"original"`
			Medium   string `json:"medium"`
		} `json:"image"`
	} `json:"show"`
}

// tvmaze is the keyless standby for the series library
func searchTVmaze(ctx context.Context, query string) ([]result, error) {
	endpoint := "https://api.tvmaze.com/search/shows?q=" + url.QueryEscape(query)

	var parsed []tvmazeResult
	if err := fetchJSON(ctx, "tvmaze", endpoint, "", func(body io.Reader) error {
		return json.NewDecoder(io.LimitReader(body, 4<<20)).Decode(&parsed)
	}); err != nil {
		return nil, err
	}

	out := make([]result, 0, len(parsed))
	for _, m := range parsed {
		s := m.Show

		channel := s.Network.Name
		if channel == "" {
			channel = s.WebChannel.Name
		}

		poster := s.Image.Original
		if poster == "" {
			poster = s.Image.Medium
		}

		out = append(out, result{
			Provider:   "tvmaze",
			ProviderID: strconv.Itoa(s.ID),
			Title:      s.Name,
			Year:       yearOf(s.Premiered),
			Format:     "Series",
			Studio:     channel,
			Genres:     s.Genres,
			Synopsis:   stripHTML(s.Summary),
			Score:      int(s.Rating.Average * 10),
			Poster:     poster,
			Link:       s.URL,
			weight:     s.Weight,
		})
	}
	return out, nil
}

type imdbResponse struct {
	D []struct {
		ID    string `json:"id"`
		Label string `json:"l"`
		QID   string `json:"qid"`
		Rank  int    `json:"rank"`
		Stars string `json:"s"`
		Year  int    `json:"y"`
		Image struct {
			URL string `json:"imageUrl"`
		} `json:"i"`
	} `json:"d"`
}

// the imdb title suggester is the keyless standby for the movie library. it
// carries no synopsis or genre, so the cast line stands in as the thing that
// tells two films of the same name apart.
func searchIMDb(ctx context.Context, query string) ([]result, error) {
	cleaned := strings.ToLower(strings.TrimSpace(query))
	if cleaned == "" {
		return []result{}, nil
	}

	endpoint := "https://v3.sg.media-imdb.com/suggestion/x/" + url.PathEscape(cleaned) + ".json?includeVideos=0"

	var parsed imdbResponse
	if err := fetchJSON(ctx, "imdb", endpoint, "", func(body io.Reader) error {
		return json.NewDecoder(io.LimitReader(body, 4<<20)).Decode(&parsed)
	}); err != nil {
		return nil, err
	}

	out := []result{}
	for _, m := range parsed.D {
		// the suggester mixes in people and episodes; only whole titles belong
		// in a movie library
		switch m.QID {
		case "movie", "tvMovie", "video", "short":
		default:
			continue
		}

		synopsis := ""
		if m.Stars != "" {
			synopsis = "With " + m.Stars + "."
		}

		out = append(out, result{
			Provider:   "imdb",
			ProviderID: m.ID,
			Title:      m.Label,
			Year:       m.Year,
			Format:     "Movie",
			Synopsis:   synopsis,
			Poster:     m.Image.URL,
			Link:       "https://www.imdb.com/title/" + m.ID + "/",
			// the suggester ranks by popularity already, lower being better
			weight: 10_000_000 - m.Rank,
		})
	}
	return out, nil
}
