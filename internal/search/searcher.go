// Package search defines the common interface and result types for
// all torrent search backends (Rutracker, 1337x, Pirate Bay, etc.)
package search

import (
	"context"
	"strconv"
	"strings"
)

// Result represents a single torrent search result.
type Result struct {
	Title      string `json:"title"`
	MagnetURI  string `json:"magnetUri"`
	TorrentURL string `json:"torrentUrl"` // direct .torrent file URL (if available)
	Seeders    int    `json:"seeders"`
	Leechers   int    `json:"leechers"`
	Size       string `json:"size"`
	Source     string `json:"source"` // "rutracker", "1337x", "piratebay"
	Category   string `json:"category"`
	InfoHash   string `json:"infoHash,omitempty"`
}

// Searcher is implemented by every search backend.
type Searcher interface {
	// Name returns a human-readable backend name.
	Name() string
	// Search queries the backend and returns up to limit results.
	Search(ctx context.Context, query string, limit int) ([]Result, error)
}

// MultiSearcher fans out a query to multiple backends concurrently.
type MultiSearcher struct {
	backends []Searcher
}

// NewMultiSearcher creates a MultiSearcher with the given backends.
func NewMultiSearcher(backends ...Searcher) *MultiSearcher {
	return &MultiSearcher{backends: backends}
}

// Search queries all backends concurrently and merges results.
// Results are sorted by seeders (highest first).
func (m *MultiSearcher) Search(ctx context.Context, query string, limit int) []Result {
	type partial struct {
		results []Result
		err     error
	}
	ch := make(chan partial, len(m.backends))

	for _, b := range m.backends {
		go func(s Searcher) {
			res, err := s.Search(ctx, query, limit)
			ch <- partial{res, err}
		}(b)
	}

	var all []Result
	for range m.backends {
		p := <-ch
		if p.err == nil {
			all = append(all, p.results...)
		}
	}

	// Sort by seeders descending
	sortBySeederDesc(all)
	if len(all) > limit*len(m.backends) {
		all = all[:limit*len(m.backends)]
	}
	return all
}

func sortBySeederDesc(results []Result) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].Seeders > results[j-1].Seeders; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
}

func parseIntFromText(s string) int {
	s = strings.TrimSpace(s)
	v, _ := strconv.Atoi(s)
	return v
}
