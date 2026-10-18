// Package sources — scraper.go
// Synchronous UDP & HTTP Tracker Scrape/Announce for real seeder and leecher counts.
package sources

import (
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent/bencode"
)

const (
	scrapeTimeout = 1500 * time.Millisecond
	cacheExpiry   = 30 * time.Minute
	connectMagic  = 0x41727101980 // UDP tracker protocol magic
	actionConnect = 0
	actionScrape  = 2
)

// ScrapeResult holds real-time data fetched from trackers.
type ScrapeResult struct {
	Seeders   int
	Leechers  int
	Completed int
}

// cachedEntry is a time-stamped scrape result.
type cachedEntry struct {
	result  ScrapeResult
	fetchAt time.Time
}

// Scraper performs UDP & HTTP tracker scrapes with an in-memory result cache.
type Scraper struct {
	mu    sync.RWMutex
	cache map[string]cachedEntry // key = infohash (lowercase hex, 40 chars)
}

// NewScraper creates a ready-to-use Scraper.
func NewScraper() *Scraper {
	return &Scraper{cache: make(map[string]cachedEntry)}
}

// getCached returns a cached result (if still fresh).
func (sc *Scraper) getCached(hexHash string) (ScrapeResult, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	e, ok := sc.cache[hexHash]
	if !ok || time.Since(e.fetchAt) > cacheExpiry {
		return ScrapeResult{}, false
	}
	return e.result, true
}

// setCached writes a result to the cache.
func (sc *Scraper) setCached(hexHash string, r ScrapeResult) {
	sc.mu.Lock()
	sc.cache[hexHash] = cachedEntry{result: r, fetchAt: time.Now()}
	sc.mu.Unlock()
}

// ScrapeSync scrapes all provided info-hashes against every tracker URL
// (supporting both udp:// and http(s):// protocols) concurrently,
// waiting for responses with a hard deadline of 2.5 seconds.
// Results are merged across trackers by taking the maximum seeders and leechers found.
func (sc *Scraper) ScrapeSync(trackerURLs []string, hexHashes []string) map[string]ScrapeResult {
	if len(hexHashes) == 0 || len(trackerURLs) == 0 {
		return nil
	}

	// Split into cached vs uncached
	results := make(map[string]ScrapeResult, len(hexHashes))
	var uncached []string
	for _, h := range hexHashes {
		if r, ok := sc.getCached(h); ok {
			results[h] = r
		} else {
			uncached = append(uncached, h)
		}
	}
	if len(uncached) == 0 {
		return results
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	mergeResult := func(hash string, r ScrapeResult) {
		mu.Lock()
		defer mu.Unlock()
		existing, ok := results[hash]
		if !ok {
			results[hash] = r
			return
		}
		if r.Seeders > existing.Seeders {
			existing.Seeders = r.Seeders
		}
		if r.Leechers > existing.Leechers {
			existing.Leechers = r.Leechers
		}
		if r.Completed > existing.Completed {
			existing.Completed = r.Completed
		}
		results[hash] = existing
	}

	for _, tURL := range trackerURLs {
		lowerURL := strings.ToLower(tURL)
		if strings.HasPrefix(lowerURL, "udp://") {
			wg.Add(1)
			go func(url string) {
				defer wg.Done()
				data := udpScrape(url, uncached)
				for hash, r := range data {
					mergeResult(hash, r)
				}
			}(tURL)
		} else if strings.HasPrefix(lowerURL, "http://") || strings.HasPrefix(lowerURL, "https://") {
			for _, h := range uncached {
				wg.Add(1)
				go func(url string, hash string) {
					defer wg.Done()
					r, ok := httpAnnounceOrScrape(url, hash)
					if ok {
						mergeResult(hash, r)
					}
				}(tURL, h)
			}
		}
	}

	// Wait for all trackers or max 2.5s total timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2500 * time.Millisecond):
	}

	// Persist new results into cache
	now := time.Now()
	sc.mu.Lock()
	for _, h := range uncached {
		if r, ok := results[h]; ok {
			sc.cache[h] = cachedEntry{result: r, fetchAt: now}
		}
	}
	sc.mu.Unlock()

	return results
}

// ExtractTrackers extracts all tracker URLs (udp://, http://, https://) from a magnet URI.
func ExtractTrackers(magnetURI string) []string {
	if !strings.HasPrefix(strings.ToLower(magnetURI), "magnet:") {
		return nil
	}
	u, err := url.Parse(magnetURI)
	if err != nil {
		return nil
	}
	var trackers []string
	for _, tr := range u.Query()["tr"] {
		tr = strings.TrimSpace(tr)
		lower := strings.ToLower(tr)
		if strings.HasPrefix(lower, "udp://") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			trackers = append(trackers, tr)
		}
	}
	return trackers
}

// ExtractInfoHash extracts a lowercase hex infohash (40 chars) from a magnet URI
// or a raw hash string. Handles both hex (40 chars) and base32 (32 chars) formats.
// Returns "" if no valid hash can be extracted.
func ExtractInfoHash(magnetOrHash string) string {
	s := strings.TrimSpace(magnetOrHash)
	lower := strings.ToLower(s)

	var raw string
	if strings.HasPrefix(lower, "magnet:") {
		if idx := strings.Index(lower, "urn:btih:"); idx != -1 {
			rem := s[idx+len("urn:btih:"):]
			if end := strings.IndexAny(rem, ";&"); end != -1 {
				rem = rem[:end]
			}
			raw = rem
		}
	} else {
		raw = s
	}

	raw = strings.TrimSpace(raw)
	switch len(raw) {
	case 40:
		// Standard hex infohash
		return strings.ToLower(raw)
	case 32:
		// Base32 infohash — decode and re-encode as hex
		b, err := base32.StdEncoding.DecodeString(strings.ToUpper(raw))
		if err == nil && len(b) == 20 {
			return hex.EncodeToString(b)
		}
	}
	return ""
}

// ─── HTTP announce / scrape implementation ────────────────────────────────────

var httpClient = &http.Client{Timeout: scrapeTimeout}

func httpAnnounceOrScrape(rawURL string, hexHash string) (ScrapeResult, bool) {
	binHash, err := hex.DecodeString(hexHash)
	if err != nil || len(binHash) != 20 {
		return ScrapeResult{}, false
	}

	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	reqURL := fmt.Sprintf("%s%sinfo_hash=%s&peer_id=-UT3550-123456789012&port=6881&uploaded=0&downloaded=0&left=1000&compact=1",
		rawURL, sep, url.QueryEscape(string(binHash)))

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return ScrapeResult{}, false
	}
	req.Header.Set("User-Agent", "uTorrent/3550(10000)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return ScrapeResult{}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ScrapeResult{}, false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ScrapeResult{}, false
	}

	var d map[string]interface{}
	if err := bencode.Unmarshal(body, &d); err != nil {
		return ScrapeResult{}, false
	}

	var res ScrapeResult
	hasData := false

	if c, ok := d["complete"].(int64); ok {
		res.Seeders = int(c)
		hasData = true
	}
	if inc, ok := d["incomplete"].(int64); ok {
		res.Leechers = int(inc)
		hasData = true
	}
	if dwn, ok := d["downloaded"].(int64); ok {
		res.Completed = int(dwn)
	}

	// For trackers that don't return complete/incomplete keys (like bt*.t-ru.org),
	// parse compact peers string (6 bytes per peer).
	if peersStr, ok := d["peers"].(string); ok && len(peersStr) >= 6 {
		peerCount := len(peersStr) / 6
		hasData = true
		if res.Seeders == 0 {
			res.Seeders = peerCount
		}
	}

	return res, hasData
}

// ─── UDP scrape implementation ────────────────────────────────────────────────

// udpScrape performs a full connect+scrape exchange against one UDP tracker.
// Returns a map of infohash (lowercase hex) → ScrapeResult.
func udpScrape(rawURL string, hexHashes []string) map[string]ScrapeResult {
	// Parse host:port from udp://host:port[/path]
	host := strings.TrimPrefix(rawURL, "udp://")
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}
	if host == "" {
		return nil
	}

	addr, err := net.ResolveUDPAddr("udp4", host)
	if err != nil {
		return nil
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return nil
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(scrapeTimeout))

	// ── Step 1: Connect ──────────────────────────────────────────────────────
	txID := rand.Uint32()
	req := make([]byte, 16)
	binary.BigEndian.PutUint64(req[0:], connectMagic)
	binary.BigEndian.PutUint32(req[8:], actionConnect)
	binary.BigEndian.PutUint32(req[12:], txID)

	if _, err := conn.Write(req); err != nil {
		return nil
	}
	resp := make([]byte, 16)
	n, err := conn.Read(resp)
	if err != nil || n < 16 {
		return nil
	}
	if binary.BigEndian.Uint32(resp[0:]) != actionConnect ||
		binary.BigEndian.Uint32(resp[4:]) != txID {
		return nil
	}
	connID := binary.BigEndian.Uint64(resp[8:])

	// ── Step 2: Scrape (up to 74 hashes per request per BEP 15) ─────────────
	results := make(map[string]ScrapeResult, len(hexHashes))

	for i := 0; i < len(hexHashes); i += 74 {
		end := i + 74
		if end > len(hexHashes) {
			end = len(hexHashes)
		}
		batch := hexHashes[i:end]

		txID2 := rand.Uint32()
		scrapeReq := make([]byte, 16+20*len(batch))
		binary.BigEndian.PutUint64(scrapeReq[0:], connID)
		binary.BigEndian.PutUint32(scrapeReq[8:], actionScrape)
		binary.BigEndian.PutUint32(scrapeReq[12:], txID2)
		for j, h := range batch {
			b, decErr := hex.DecodeString(h)
			if decErr != nil || len(b) != 20 {
				continue
			}
			copy(scrapeReq[16+j*20:], b)
		}

		_ = conn.SetDeadline(time.Now().Add(scrapeTimeout))
		if _, err := conn.Write(scrapeReq); err != nil {
			break
		}

		maxResp := 8 + 12*len(batch)
		scrapeResp := make([]byte, maxResp)
		n, err := conn.Read(scrapeResp)
		if err != nil || n < 8 {
			break
		}
		if binary.BigEndian.Uint32(scrapeResp[0:]) != actionScrape ||
			binary.BigEndian.Uint32(scrapeResp[4:]) != txID2 {
			break
		}

		entries := (n - 8) / 12
		for j := 0; j < entries && j < len(batch); j++ {
			off := 8 + j*12
			results[batch[j]] = ScrapeResult{
				Seeders:   int(binary.BigEndian.Uint32(scrapeResp[off:])),
				Completed: int(binary.BigEndian.Uint32(scrapeResp[off+4:])),
				Leechers:  int(binary.BigEndian.Uint32(scrapeResp[off+8:])),
			}
		}
	}
	return results
}
