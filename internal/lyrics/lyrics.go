// Package lyrics fetches and parses synchronized (LRC) and plain text lyrics.
// It supports local .lrc files as well as the open LRCLIB API (https://lrclib.net/).
package lyrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Result represents the retrieved lyrics for a track.
type Result struct {
	Found        bool   `json:"found"`
	Artist       string `json:"artist"`
	Title        string `json:"title"`
	Album        string `json:"album"`
	Duration     int    `json:"duration"`
	Instrumental bool   `json:"instrumental"`
	PlainLyrics  string `json:"plainLyrics"`
	SyncedLyrics string `json:"syncedLyrics"`
	Source       string `json:"source"` // "local", "lrclib"
}

// lrclibResponse matches the JSON structure returned by lrclib.net.
type lrclibResponse struct {
	ID           int     `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

// Client manages fetching and caching lyrics.
type Client struct {
	httpClient *http.Client
	cacheMu    sync.RWMutex
	cache      map[string]*Result
}

// NewClient creates a new lyrics client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 8 * time.Second},
		cache:      make(map[string]*Result),
	}
}

// InvalidateCache removes a cache entry so the next call re-fetches from the API.
func (c *Client) InvalidateCache(artist, title string) {
	ck := cacheKey(artist, cleanTitle(title))
	c.cacheMu.Lock()
	delete(c.cache, ck)
	c.cacheMu.Unlock()
}

// cacheKey generates a normalized cache key.
func cacheKey(artist, title string) string {
	return strings.ToLower(strings.TrimSpace(artist)) + ":::" + strings.ToLower(strings.TrimSpace(title))
}

// GetLyrics searches for lyrics first in local file directory, then in memory cache,
// and finally queries the LRCLIB API.
// If force is true the cached entry is evicted so a fresh API request is made.
func (c *Client) GetLyrics(ctx context.Context, artist, title, album string, durationSec float64, filePath string, force bool) (*Result, error) {
	artist = strings.TrimSpace(artist)
	title = strings.TrimSpace(title)

	// Clean track title if it contains track numbers like "01. Song Name" or "01 - Song Name"
	cleanedTitle := cleanTitle(title)

	// 1. Check local .lrc file next to audio file
	if filePath != "" {
		if localRes := c.checkLocalLRC(filePath, artist, cleanedTitle); localRes != nil {
			return localRes, nil
		}
	}

	if cleanedTitle == "" {
		return &Result{Found: false}, nil
	}

	ck := cacheKey(artist, cleanedTitle)

	if force {
		// Evict stale cache entry so we hit the API again
		c.cacheMu.Lock()
		delete(c.cache, ck)
		c.cacheMu.Unlock()
	} else {
		// 2. Check in-memory cache
		c.cacheMu.RLock()
		if cached, ok := c.cache[ck]; ok {
			c.cacheMu.RUnlock()
			return cached, nil
		}
		c.cacheMu.RUnlock()
	}

	// 3. Query LRCLIB API
	res, _ := c.fetchFromLRCLIB(ctx, artist, cleanedTitle, album, durationSec)
	if res == nil || !res.Found {
		// Also try the search endpoint
		if artist != "" {
			if searchRes, sErr := c.searchLRCLIB(ctx, artist+" "+cleanedTitle); sErr == nil && searchRes != nil && searchRes.Found {
				res = searchRes
			}
		}
		if res == nil || !res.Found {
			if searchRes, sErr := c.searchLRCLIB(ctx, cleanedTitle); sErr == nil && searchRes != nil && searchRes.Found {
				res = searchRes
			}
		}
	}

	if res == nil {
		res = &Result{Found: false, Artist: artist, Title: cleanedTitle}
	}

	// Only cache successful finds. Transient 5xx failures are not cached so
	// the user can retry and get a fresh result.
	if res.Found {
		c.cacheMu.Lock()
		c.cache[ck] = res
		c.cacheMu.Unlock()
	}

	return res, nil
}

// cleanTitle strips leading track numbers (e.g., "01. ", "02 - ", "1 ") and file extensions.
func cleanTitle(title string) string {
	t := strings.TrimSpace(title)
	// Strip file extension if passed as filename
	if idx := strings.LastIndex(t, "."); idx > 0 {
		ext := strings.ToLower(t[idx:])
		if ext == ".mp3" || ext == ".flac" || ext == ".ogg" || ext == ".wav" || ext == ".m4a" || ext == ".aac" || ext == ".opus" {
			t = t[:idx]
		}
	}

	// Strip leading digits and subsequent separators (. - _ space)
	i := 0
	for i < len(t) && t[i] >= '0' && t[i] <= '9' {
		i++
	}
	if i > 0 && i <= 3 && i < len(t) {
		sep := t[i]
		if sep == '.' || sep == '-' || sep == '_' || sep == ' ' {
			rem := strings.TrimLeft(t[i:], " .-_")
			if rem != "" {
				t = rem
			}
		}
	}
	return strings.TrimSpace(t)
}

// checkLocalLRC checks if a .lrc file exists in the same folder as the audio file.
func (c *Client) checkLocalLRC(audioPath, artist, title string) *Result {
	ext := filepath.Ext(audioPath)
	base := strings.TrimSuffix(audioPath, ext)
	candidates := []string{
		base + ".lrc",
		audioPath + ".lrc",
		base + ".LRC",
	}

	for _, cand := range candidates {
		data, err := os.ReadFile(cand)
		if err == nil && len(data) > 0 {
			content := string(data)
			return &Result{
				Found:        true,
				Artist:       artist,
				Title:        title,
				SyncedLyrics: content,
				Source:       "local",
			}
		}
	}
	return nil
}

// doWithRetry executes an HTTP request and retries up to maxRetries times on 5xx responses,
// waiting retryDelay between attempts. Only GET requests with no body are supported.
func (c *Client) doWithRetry(req *http.Request, maxRetries int, retryDelay time.Duration) (*http.Response, error) {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(retryDelay):
			}
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Network error – retry
			continue
		}
		// Retry only on 5xx (server-side transient errors)
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("lrclib: server unavailable after %d attempts", maxRetries+1)
}

// fetchFromLRCLIB queries GET https://lrclib.net/api/get.
func (c *Client) fetchFromLRCLIB(ctx context.Context, artist, title, album string, durationSec float64) (*Result, error) {
	endpoint := "https://lrclib.net/api/get"
	params := url.Values{}
	params.Set("track_name", title)
	if artist != "" {
		params.Set("artist_name", artist)
	}
	if album != "" {
		params.Set("album_name", album)
	}
	if durationSec > 0 {
		params.Set("duration", fmt.Sprintf("%d", int(durationSec)))
	}

	reqURL := endpoint + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Posluchac/1.3.0 (https://github.com/user/torrent-player)")

	resp, err := c.doWithRetry(req, 2, 800*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &Result{Found: false}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lrclib status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var item lrclibResponse
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, err
	}

	found := item.SyncedLyrics != "" || item.PlainLyrics != "" || item.Instrumental
	return &Result{
		Found:        found,
		Artist:       item.ArtistName,
		Title:        item.TrackName,
		Album:        item.AlbumName,
		Duration:     int(item.Duration),
		Instrumental: item.Instrumental,
		PlainLyrics:  item.PlainLyrics,
		SyncedLyrics: item.SyncedLyrics,
		Source:       "lrclib",
	}, nil
}

// searchLRCLIB queries GET https://lrclib.net/api/search.
func (c *Client) searchLRCLIB(ctx context.Context, query string) (*Result, error) {
	endpoint := "https://lrclib.net/api/search"
	params := url.Values{}
	params.Set("q", query)

	reqURL := endpoint + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Posluchac/1.3.0 (https://github.com/user/torrent-player)")

	resp, err := c.doWithRetry(req, 2, 800*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lrclib search status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var list []lrclibResponse
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, err
	}

	if len(list) == 0 {
		return &Result{Found: false}, nil
	}

	// Pick first item that has synced lyrics, or first item
	chosen := list[0]
	for _, item := range list {
		if item.SyncedLyrics != "" {
			chosen = item
			break
		}
	}

	found := chosen.SyncedLyrics != "" || chosen.PlainLyrics != "" || chosen.Instrumental
	return &Result{
		Found:        found,
		Artist:       chosen.ArtistName,
		Title:        chosen.TrackName,
		Album:        chosen.AlbumName,
		Duration:     int(chosen.Duration),
		Instrumental: chosen.Instrumental,
		PlainLyrics:  chosen.PlainLyrics,
		SyncedLyrics: chosen.SyncedLyrics,
		Source:       "lrclib",
	}, nil
}
