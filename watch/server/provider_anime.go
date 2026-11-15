package main

import (
	"bytes"
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

// anilist is the first choice for anime: no key, and the richest metadata of
// anything keyless. it does go down for days at a time, which is what the
// kitsu fallback below is for.
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

	var parsed aniListResponse
	// an error body carries the only useful explanation there is ("the api
	// has been temporarily disabled"), so parse before judging the status
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&parsed); err != nil {
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("answered %d", res.StatusCode)
		}
		return nil, errors.New("sent something unreadable")
	}
	if len(parsed.Errors) > 0 {
		return nil, errors.New(strings.ToLower(strings.TrimRight(parsed.Errors[0].Message, ".")))
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("answered %d", res.StatusCode)
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
			ProviderID: strconv.Itoa(m.ID),
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
	return out, nil
}

type kitsuResponse struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			Slug           string            `json:"slug"`
			CanonicalTitle string            `json:"canonicalTitle"`
			Titles         map[string]string `json:"titles"`
			Synopsis       string            `json:"synopsis"`
			StartDate      string            `json:"startDate"`
			EpisodeCount   int               `json:"episodeCount"`
			Subtype        string            `json:"subtype"`
			AverageRating  string            `json:"averageRating"`
			UserCount      int               `json:"userCount"`
			PosterImage    struct {
				Original string `json:"original"`
				Large    string `json:"large"`
				Medium   string `json:"medium"`
			} `json:"posterImage"`
		} `json:"attributes"`
	} `json:"data"`
}

// kitsu covers the same catalogue as anilist with no key either, but carries
// neither studio nor genre on a plain search, so it is the standby rather
// than the default
func searchKitsu(ctx context.Context, query string) ([]result, error) {
	endpoint := "https://kitsu.app/api/edge/anime?filter%5Btext%5D=" +
		url.QueryEscape(query) + "&page%5Blimit%5D=12"

	var parsed kitsuResponse
	if err := fetchJSON(ctx, "kitsu", endpoint, "application/vnd.api+json", func(body io.Reader) error {
		return json.NewDecoder(io.LimitReader(body, 4<<20)).Decode(&parsed)
	}); err != nil {
		return nil, err
	}

	out := make([]result, 0, len(parsed.Data))
	for _, m := range parsed.Data {
		a := m.Attributes

		title := a.Titles["en"]
		if title == "" {
			title = a.CanonicalTitle
		}
		alt := a.Titles["en_jp"]
		if alt == "" {
			alt = a.Titles["ja_jp"]
		}
		if alt == title {
			alt = ""
		}

		poster := a.PosterImage.Original
		if poster == "" {
			poster = a.PosterImage.Large
		}

		// kitsu's rating is a 0-100 string with decimals
		score := 0
		if parsedScore, err := strconv.ParseFloat(a.AverageRating, 64); err == nil {
			score = int(parsedScore)
		}

		slug := a.Slug
		if slug == "" {
			slug = m.ID
		}

		out = append(out, result{
			Provider:   "kitsu",
			ProviderID: m.ID,
			Title:      title,
			AltTitle:   alt,
			Year:       yearOf(a.StartDate),
			Format:     prettyFormat(a.Subtype),
			Episodes:   a.EpisodeCount,
			Synopsis:   stripHTML(a.Synopsis),
			Score:      score,
			Poster:     poster,
			Link:       "https://kitsu.app/anime/" + slug,
			weight:     a.UserCount,
		})
	}
	return out, nil
}
