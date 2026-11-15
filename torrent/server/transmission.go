package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

type torrentClient struct {
	cfg  config
	http *http.Client

	// transmission hands out a session id and then rejects every request
	// that does not carry it, so it is kept between calls and only
	// re-fetched when the daemon says it went stale
	mu      sync.Mutex
	session string
}

func newTorrentClient(cfg config) *torrentClient {
	return &torrentClient{cfg: cfg, http: &http.Client{Timeout: 15 * time.Second}}
}

type torrentSummary struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	Percent    float64 `json:"percent"`
	Size       float64 `json:"size"`
	DownBps    float64 `json:"downBps"`
	UpBps      float64 `json:"upBps"`
	Ratio      float64 `json:"ratio"`
	Peers      int     `json:"peers"`
	ETASec     float64 `json:"etaSec"`
	Downloaded float64 `json:"downloaded"`
	Uploaded   float64 `json:"uploaded"`
	AddedAt    float64 `json:"addedAt"`
	Error      string  `json:"error"`
}

// transmission's numeric status codes, in rpc-spec order
var statusNames = map[int]string{
	0: "paused", 1: "check wait", 2: "checking",
	3: "queued", 4: "downloading", 5: "seed wait", 6: "seeding",
}

var torrentFields = []string{
	"id", "name", "status", "percentDone", "totalSize", "rateDownload", "rateUpload",
	"uploadRatio", "peersConnected", "eta", "downloadedEver", "uploadedEver", "addedDate", "errorString",
}

func (c *torrentClient) list(ctx context.Context) ([]torrentSummary, error) {
	var out struct {
		Arguments struct {
			Torrents []struct {
				ID         int64   `json:"id"`
				Name       string  `json:"name"`
				Status     int     `json:"status"`
				Percent    float64 `json:"percentDone"`
				Size       float64 `json:"totalSize"`
				Down       float64 `json:"rateDownload"`
				Up         float64 `json:"rateUpload"`
				Ratio      float64 `json:"uploadRatio"`
				Peers      int     `json:"peersConnected"`
				ETA        float64 `json:"eta"`
				Downloaded float64 `json:"downloadedEver"`
				Uploaded   float64 `json:"uploadedEver"`
				AddedDate  float64 `json:"addedDate"`
				Error      string  `json:"errorString"`
			} `json:"torrents"`
		} `json:"arguments"`
	}
	req := map[string]any{
		"method":    "torrent-get",
		"arguments": map[string]any{"fields": torrentFields},
	}
	if err := c.rpc(ctx, req, &out); err != nil {
		return nil, err
	}

	list := make([]torrentSummary, 0, len(out.Arguments.Torrents))
	for _, t := range out.Arguments.Torrents {
		name, ok := statusNames[t.Status]
		if !ok {
			name = "unknown"
		}
		list = append(list, torrentSummary{
			ID: t.ID, Name: t.Name, Status: name, Percent: t.Percent * 100, Size: t.Size,
			DownBps: t.Down, UpBps: t.Up, Ratio: t.Ratio, Peers: t.Peers, ETASec: t.ETA,
			Downloaded: t.Downloaded, Uploaded: t.Uploaded, AddedAt: t.AddedDate, Error: t.Error,
		})
	}
	// busiest first, then newest, so a stalled or paused torrent never buries
	// whatever is actually moving right now
	sort.SliceStable(list, func(i, j int) bool {
		ai, aj := list[i].DownBps+list[i].UpBps, list[j].DownBps+list[j].UpBps
		if ai != aj {
			return ai > aj
		}
		return list[i].AddedAt > list[j].AddedAt
	})
	return list, nil
}

func (c *torrentClient) setActive(ctx context.Context, id int64, active bool) error {
	method := "torrent-stop"
	if active {
		method = "torrent-start"
	}
	var out struct {
		Result string `json:"result"`
	}
	req := map[string]any{
		"method":    method,
		"arguments": map[string]any{"ids": []int64{id}},
	}
	if err := c.rpc(ctx, req, &out); err != nil {
		return err
	}
	return resultErr(out.Result)
}

func (c *torrentClient) remove(ctx context.Context, id int64, deleteData bool) error {
	var out struct {
		Result string `json:"result"`
	}
	req := map[string]any{
		"method": "torrent-remove",
		"arguments": map[string]any{
			"ids":               []int64{id},
			"delete-local-data": deleteData,
		},
	}
	if err := c.rpc(ctx, req, &out); err != nil {
		return err
	}
	return resultErr(out.Result)
}

// add accepts either a magnet link in magnet, or the base64-encoded contents
// of a .torrent file in metainfo; transmission only ever wants one of them.
// it is handed back downloading rather than paused, so progress only stops
// once the done hook pauses it; seeding again after that is a manual choice.
func (c *torrentClient) add(ctx context.Context, magnet, metainfo string) (*torrentSummary, error) {
	args := map[string]any{"paused": false}
	switch {
	case metainfo != "":
		args["metainfo"] = metainfo
	case magnet != "":
		args["filename"] = magnet
	default:
		return nil, errors.New("need a magnet link or a .torrent file")
	}

	var out struct {
		Result    string `json:"result"`
		Arguments struct {
			TorrentAdded *struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"torrent-added"`
			TorrentDuplicate *struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"torrent-duplicate"`
		} `json:"arguments"`
	}
	if err := c.rpc(ctx, map[string]any{"method": "torrent-add", "arguments": args}, &out); err != nil {
		return nil, err
	}
	if err := resultErr(out.Result); err != nil {
		return nil, err
	}

	added := out.Arguments.TorrentAdded
	if added == nil {
		added = out.Arguments.TorrentDuplicate
	}
	if added == nil {
		return nil, errors.New("transmission did not say what it added")
	}
	return &torrentSummary{ID: added.ID, Name: added.Name}, nil
}

func resultErr(result string) error {
	if result != "" && result != "success" {
		return fmt.Errorf("transmission: %s", result)
	}
	return nil
}

func (c *torrentClient) rpc(ctx context.Context, body any, into any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	send := func(session string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.transmissionURL, bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if session != "" {
			req.Header.Set("X-Transmission-Session-Id", session)
		}
		return c.http.Do(req)
	}

	c.mu.Lock()
	session := c.session
	c.mu.Unlock()

	res, err := send(session)
	if err != nil {
		return err
	}
	// 409 means the session id rotated; take the fresh one from the response
	// and replay once
	if res.StatusCode == http.StatusConflict {
		session = res.Header.Get("X-Transmission-Session-Id")
		_ = res.Body.Close()
		c.mu.Lock()
		c.session = session
		c.mu.Unlock()
		if res, err = send(session); err != nil {
			return err
		}
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("transmission answered %s", res.Status)
	}
	return json.NewDecoder(res.Body).Decode(into)
}
