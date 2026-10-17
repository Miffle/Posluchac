// Pirate Bay search backend using the official JSON API.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const pbAPIBase = "https://apibay.org"

// PirateBaySearcher implements Searcher using the apibay.org JSON API.
type PirateBaySearcher struct {
	client *http.Client
}

func NewPirateBaySearcher() *PirateBaySearcher {
	return &PirateBaySearcher{
		client: &http.Client{Timeout: 4 * time.Second},
	}
}

func (s *PirateBaySearcher) Name() string { return "piratebay" }

func (s *PirateBaySearcher) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	// cat=100 = Audio, cat=101 = Music, 0 = all
	apiURL := fmt.Sprintf("%s/q.php?q=%s&cat=100", pbAPIBase, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("piratebay search: %w", err)
	}
	defer resp.Body.Close()

	var items []pbItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("piratebay decode: %w", err)
	}

	var results []Result
	for i, item := range items {
		if i >= limit {
			break
		}
		// Skip "no results" placeholder
		if item.ID == "0" {
			continue
		}

		seeders, _ := strconv.Atoi(item.Seeders)
		leechers, _ := strconv.Atoi(item.Leechers)
		sizeBytes, _ := strconv.ParseInt(item.Size, 10, 64)

		results = append(results, Result{
			Title:     item.Name,
			MagnetURI: buildPBMagnet(item),
			InfoHash:  item.InfoHash,
			Seeders:   seeders,
			Leechers:  leechers,
			Size:      formatSize(sizeBytes),
			Source:    "piratebay",
			Category:  item.Category,
		})
	}

	return results, nil
}

type pbItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	InfoHash string `json:"info_hash"`
	Seeders  string `json:"seeders"`
	Leechers string `json:"leechers"`
	Size     string `json:"size"`
	Category string `json:"category"`
}

var pbTrackers = []string{
	"udp://tracker.openbittorrent.com:80",
	"udp://tracker.opentrackr.org:1337/announce",
	"udp://open.demonii.com:1337/announce",
	"udp://exodus.desync.com:6969/announce",
}

func buildPBMagnet(item pbItem) string {
	magnet := "magnet:?xt=urn:btih:" + item.InfoHash + "&dn=" + url.QueryEscape(item.Name)
	for _, t := range pbTrackers {
		magnet += "&tr=" + url.QueryEscape(t)
	}
	return magnet
}

func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.2f GB", float64(bytes)/gb)
	case bytes >= mb:
		return fmt.Sprintf("%.2f MB", float64(bytes)/mb)
	case bytes >= kb:
		return fmt.Sprintf("%.2f KB", float64(bytes)/kb)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
