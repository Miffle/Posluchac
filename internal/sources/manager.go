// Package sources manages external JSON download sources (Hydra Launcher style).
// Sources can be hosted on GitHub Raw, paste services, or any HTTP URL.
package sources

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/user/torrent-player/internal/search"
)

// AutoSyncInterval is how often sources are refreshed in the background.
const AutoSyncInterval = 24 * time.Hour

// Source represents an added repository/source.
type Source struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	ItemCount int       `json:"itemCount"`
	UpdatedAt time.Time `json:"updatedAt"`
	SyncedAt  time.Time `json:"syncedAt"`
}

// DownloadItem represents a single release in the source JSON.
type DownloadItem struct {
	Title    string   `json:"title"`
	URIs     []string `json:"uris"`
	FileSize string   `json:"fileSize,omitempty"`
}

// SourceJSON defines the schema of the remote JSON source.
type SourceJSON struct {
	Name      string         `json:"name"`
	Downloads []DownloadItem `json:"downloads"`
}

// Manager manages sources stored in SQLite.
type Manager struct {
	db     *sql.DB
	client *http.Client
	stopCh chan struct{}

	vocabMu        sync.RWMutex
	vocabMap       map[string]int
	vocabExpiresAt time.Time
}

// NewManager creates a sources manager and ensures DB tables exist.
func NewManager(db *sql.DB) (*Manager, error) {
	m := &Manager{
		db:     db,
		client: &http.Client{Timeout: 45 * time.Second},
		stopCh: make(chan struct{}),
	}
	if err := m.initSchema(); err != nil {
		return nil, err
	}
	return m, nil
}

// Stop signals the background auto-sync goroutine to exit.
func (m *Manager) Stop() {
	select {
	case <-m.stopCh: // already closed
	default:
		close(m.stopCh)
	}
}

// StartAutoSync launches a background goroutine that keeps all sources fresh.
// On startup it immediately syncs any source whose synced_at is older than
// AutoSyncInterval (or has never been synced). After that it repeats every
// AutoSyncInterval. The goroutine exits when Stop() is called.
func (m *Manager) StartAutoSync() {
	go func() {
		// Initial sync: check each source and refresh stale ones right away.
		m.syncStale()

		ticker := time.NewTicker(AutoSyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.syncStale()
			case <-m.stopCh:
				return
			}
		}
	}()
}

// syncStale refreshes every source that has not been synced within AutoSyncInterval.
func (m *Manager) syncStale() {
	sources, err := m.ListSources()
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-AutoSyncInterval)
	for _, s := range sources {
		if s.SyncedAt.Before(cutoff) {
			_, _ = m.AddSource(s.URL)
		}
	}
}

func (m *Manager) initSchema() error {
	tables := `
	CREATE TABLE IF NOT EXISTS sources (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		url TEXT NOT NULL UNIQUE,
		item_count INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		synced_at  DATETIME DEFAULT '1970-01-01 00:00:00'
	);

	CREATE TABLE IF NOT EXISTS source_downloads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		title_lower TEXT NOT NULL DEFAULT '',
		magnet_uri TEXT NOT NULL,
		FOREIGN KEY(source_id) REFERENCES sources(id) ON DELETE CASCADE
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS source_downloads_fts USING fts5(
		title,
		content='source_downloads',
		content_rowid='id',
		tokenize='unicode61'
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS source_downloads_vocab USING fts5vocab(
		source_downloads_fts,
		'row'
	);

	CREATE TRIGGER IF NOT EXISTS source_downloads_ai AFTER INSERT ON source_downloads BEGIN
		INSERT INTO source_downloads_fts(rowid, title) VALUES (new.id, new.title);
	END;

	CREATE TRIGGER IF NOT EXISTS source_downloads_ad AFTER DELETE ON source_downloads BEGIN
		INSERT INTO source_downloads_fts(source_downloads_fts, rowid, title) VALUES('delete', old.id, old.title);
	END;

	CREATE TRIGGER IF NOT EXISTS source_downloads_au AFTER UPDATE ON source_downloads BEGIN
		INSERT INTO source_downloads_fts(source_downloads_fts, rowid, title) VALUES('delete', old.id, old.title);
		INSERT INTO source_downloads_fts(rowid, title) VALUES (new.id, new.title);
	END;
	`
	if _, err := m.db.Exec(tables); err != nil {
		return err
	}

	// Safe column migrations for tables created before these columns existed.
	_, _ = m.db.Exec(`ALTER TABLE source_downloads ADD COLUMN title_lower TEXT NOT NULL DEFAULT ''`)
	_, _ = m.db.Exec(`ALTER TABLE sources ADD COLUMN synced_at DATETIME DEFAULT '1970-01-01 00:00:00'`)

	indices := `
	CREATE INDEX IF NOT EXISTS idx_source_downloads_title ON source_downloads(title);
	CREATE INDEX IF NOT EXISTS idx_source_downloads_title_lower ON source_downloads(title_lower);
	CREATE INDEX IF NOT EXISTS idx_source_downloads_source ON source_downloads(source_id);
	`
	if _, err := m.db.Exec(indices); err != nil {
		return err
	}

	// Ensure FTS5 index is populated if source_downloads has rows but FTS5 doesn't
	var mainCount, ftsCount int
	_ = m.db.QueryRow(`SELECT count(*) FROM source_downloads`).Scan(&mainCount)
	_ = m.db.QueryRow(`SELECT count(*) FROM source_downloads_fts`).Scan(&ftsCount)
	if mainCount > 0 && ftsCount < mainCount {
		_, _ = m.db.Exec(`INSERT INTO source_downloads_fts(source_downloads_fts) VALUES('rebuild')`)
	}

	// Backfill any empty title_lower in existing rows
	rows, err := m.db.Query(`SELECT id, title FROM source_downloads WHERE title_lower = '' OR title_lower IS NULL`)
	if err == nil {
		type item struct {
			id    int64
			lower string
		}
		var items []item
		for rows.Next() {
			var id int64
			var title string
			if err := rows.Scan(&id, &title); err == nil {
				items = append(items, item{id: id, lower: strings.ToLower(title)})
			}
		}
		rows.Close()

		if len(items) > 0 {
			tx, err := m.db.Begin()
			if err == nil {
				stmt, err := tx.Prepare(`UPDATE source_downloads SET title_lower = ? WHERE id = ?`)
				if err == nil {
					for _, it := range items {
						_, _ = stmt.Exec(it.lower, it.id)
					}
					stmt.Close()
				}
				_ = tx.Commit()
			}
		}
	}

	return nil
}

// AddSource fetches a remote JSON source, stores it in SQLite, and indexes all downloads.
func (m *Manager) AddSource(url string) (*Source, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, fmt.Errorf("URL источника не может быть пустым")
	}

	data, err := m.fetchRemoteJSON(url)
	if err != nil {
		return nil, err
	}

	tx, err := m.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Insert or update sources table
	var sourceID int64
	row := tx.QueryRow(`SELECT id FROM sources WHERE url = ?`, url)
	if err := row.Scan(&sourceID); err == nil {
		// Update existing
		_, err = tx.Exec(`UPDATE sources SET name = ?, item_count = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			data.Name, len(data.Downloads), sourceID)
		if err != nil {
			return nil, err
		}
		// Clear existing downloads for re-import
		_, _ = tx.Exec(`DELETE FROM source_downloads WHERE source_id = ?`, sourceID)
	} else {
		// Insert new
		res, err := tx.Exec(`INSERT INTO sources (name, url, item_count, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
			data.Name, url, len(data.Downloads))
		if err != nil {
			return nil, err
		}
		sourceID, _ = res.LastInsertId()
	}

	// Bulk insert downloads
	stmt, err := tx.Prepare(`INSERT INTO source_downloads (source_id, title, title_lower, magnet_uri) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	for _, item := range data.Downloads {
		if strings.TrimSpace(item.Title) == "" || len(item.URIs) == 0 {
			continue
		}
		magnet := item.URIs[0]
		titleLower := strings.ToLower(item.Title)
		_, _ = stmt.Exec(sourceID, item.Title, titleLower, magnet)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	m.invalidateVocab()

	// Record successful sync time.
	now := time.Now()
	_, _ = m.db.Exec(`UPDATE sources SET synced_at = ? WHERE id = ?`, now, sourceID)

	return &Source{
		ID:        sourceID,
		Name:      data.Name,
		URL:       url,
		ItemCount: len(data.Downloads),
		UpdatedAt: now,
		SyncedAt:  now,
	}, nil
}

// SyncAll refreshes all sources from their remote URLs.
func (m *Manager) SyncAll() error {
	sources, err := m.ListSources()
	if err != nil {
		return err
	}
	for _, s := range sources {
		_, _ = m.AddSource(s.URL)
	}
	return nil
}

// DeleteSource removes a source and its indexed downloads from the database.
func (m *Manager) DeleteSource(id int64) error {
	_, _ = m.db.Exec(`DELETE FROM source_downloads WHERE source_id = ?`, id)
	_, err := m.db.Exec(`DELETE FROM sources WHERE id = ?`, id)
	m.invalidateVocab()
	return err
}

// ListSources returns all registered sources with their item counts.
func (m *Manager) ListSources() ([]Source, error) {
	rows, err := m.db.Query(`SELECT id, name, url, item_count, updated_at, synced_at FROM sources ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Source
	for rows.Next() {
		var s Source
		if err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.ItemCount, &s.UpdatedAt, &s.SyncedAt); err == nil {
			result = append(result, s)
		}
	}
	return result, nil
}

// ExtractTokens splits text into alphanumeric/Unicode words (letters and digits).
func ExtractTokens(s string) []string {
	var tokens []string
	var cur []rune
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur = append(cur, r)
		} else {
			if len(cur) > 0 {
				tokens = append(tokens, string(cur))
				cur = nil
			}
		}
	}
	if len(cur) > 0 {
		tokens = append(tokens, string(cur))
	}
	return tokens
}

// MatchesFormat checks if a release title matches a format filter ("lossless" or "mp3_320").
func MatchesFormat(title string, format string) bool {
	f := strings.ToLower(strings.TrimSpace(format))
	if f == "" || f == "all" {
		return true
	}
	tLower := strings.ToLower(title)
	switch f {
	case "lossless", "flac":
		keywords := []string{"flac", "lossless", "ape", "alac", "wav", "24bit", "24-bit", "hi-res", "hires", "dsd", "sacd"}
		for _, kw := range keywords {
			if strings.Contains(tLower, kw) {
				return true
			}
		}
		return false
	case "mp3_320", "320":
		return strings.Contains(tLower, "320")
	default:
		return true
	}
}

// ParseQueryAndFilter extracts inline directives like format:flac or format:320
// and returns the clean search terms and the resolved format filter.
func ParseQueryAndFilter(rawQuery string, explicitFilter string) (string, string) {
	trimmed := strings.TrimSpace(rawQuery)
	filter := strings.ToLower(strings.TrimSpace(explicitFilter))
	if filter == "all" {
		filter = ""
	}

	words := strings.Fields(trimmed)
	var cleanWords []string
	for _, w := range words {
		lower := strings.ToLower(w)
		if strings.HasPrefix(lower, "format:") {
			val := strings.TrimPrefix(lower, "format:")
			if val == "flac" || val == "lossless" {
				if filter == "" {
					filter = "lossless"
				}
			} else if val == "320" || val == "mp3_320" || val == "mp3" {
				if filter == "" {
					filter = "mp3_320"
				}
			}
			continue
		}
		cleanWords = append(cleanWords, w)
	}
	return strings.Join(cleanWords, " "), filter
}

// BuildFTS5Query constructs an SQLite FTS5 MATCH expression from query terms and format filter.
func BuildFTS5Query(query string, formatFilter string) string {
	tokens := ExtractTokens(query)
	var tokenExprs []string
	for _, t := range tokens {
		tClean := strings.ReplaceAll(t, `"`, "")
		if tClean != "" {
			tokenExprs = append(tokenExprs, fmt.Sprintf(`"%s"*`, tClean))
		}
	}

	var baseExpr string
	if len(tokenExprs) > 0 {
		baseExpr = strings.Join(tokenExprs, " AND ")
	}

	var formatExpr string
	f := strings.ToLower(strings.TrimSpace(formatFilter))
	switch f {
	case "lossless", "flac":
		formatExpr = `(flac* OR lossless* OR ape* OR alac* OR wav* OR "24bit"*)`
	case "mp3_320", "320":
		formatExpr = `("320"*)`
	}

	if baseExpr != "" && formatExpr != "" {
		return fmt.Sprintf("(%s) AND %s", baseExpr, formatExpr)
	} else if formatExpr != "" {
		return formatExpr
	}
	return baseExpr
}

func (m *Manager) invalidateVocab() {
	m.vocabMu.Lock()
	m.vocabMap = nil
	m.vocabMu.Unlock()
}

func (m *Manager) getVocab(ctx context.Context) map[string]int {
	m.vocabMu.RLock()
	if m.vocabMap != nil && time.Now().Before(m.vocabExpiresAt) {
		vm := m.vocabMap
		m.vocabMu.RUnlock()
		return vm
	}
	m.vocabMu.RUnlock()

	m.vocabMu.Lock()
	defer m.vocabMu.Unlock()

	if m.vocabMap != nil && time.Now().Before(m.vocabExpiresAt) {
		return m.vocabMap
	}

	rows, err := m.db.QueryContext(ctx, `SELECT term, cnt FROM source_downloads_vocab WHERE length(term) >= 3`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	vm := make(map[string]int)
	for rows.Next() {
		var term string
		var cnt int
		if err := rows.Scan(&term, &cnt); err == nil {
			vm[term] = cnt
		}
	}

	m.vocabMap = vm
	m.vocabExpiresAt = time.Now().Add(5 * time.Minute)
	return vm
}

// DamerauLevenshtein computes the edit distance between two rune slices,
// supporting insertions, deletions, substitutions, and adjacent transpositions.
func DamerauLevenshtein(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	d := make([][]int, la+1)
	for i := range d {
		d[i] = make([]int, lb+1)
		d[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		d[0][j] = j
	}

	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			d[i][j] = d[i-1][j] + 1 // deletion
			if d[i][j-1]+1 < d[i][j] {
				d[i][j] = d[i][j-1] + 1 // insertion
			}
			if d[i-1][j-1]+cost < d[i][j] {
				d[i][j] = d[i-1][j-1] + cost // substitution
			}
			// transposition of adjacent characters
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				if d[i-2][j-2]+1 < d[i][j] {
					d[i][j] = d[i-2][j-2] + 1
				}
			}
		}
	}
	return d[la][lb]
}

var formatStopwords = map[string]bool{
	"cue":        true,
	"cues":       true,
	"flac":       true,
	"mp3":        true,
	"lossless":   true,
	"tracks":     true,
	"track":      true,
	"image":      true,
	"album":      true,
	"albums":     true,
	"single":     true,
	"singles":    true,
	"soundtrack": true,
	"ost":        true,
	"web":        true,
	"rip":        true,
	"cd":         true,
	"cds":        true,
	"dvd":        true,
	"vinyl":      true,
	"remaster":   true,
	"remastered": true,
	"deluxe":     true,
	"edition":    true,
	"bonus":      true,
	"disc":       true,
	"discs":      true,
	"disk":       true,
	"disks":      true,
	"kbps":       true,
	"audio":      true,
	"music":      true,
	"genre":      true,
	"year":       true,
	"aac":        true,
	"ogg":        true,
	"wav":        true,
	"alac":       true,
	"ape":        true,
	"m4a":        true,
	"hires":      true,
}

// IsFormatStopword returns true if the word is an audio format or metadata descriptor.
func IsFormatStopword(term string) bool {
	return formatStopwords[strings.ToLower(strings.TrimSpace(term))]
}

// FindFuzzyCandidate finds the best matching term in vocab within edit distance 1 (or 2 for long words).
// Words with length < 4 are never fuzzy-matched. Technical and format keywords are blacklisted from
// being returned as typo candidates to prevent terms like "cute" matching "cue" or "rap" matching "rip".
func FindFuzzyCandidate(token string, vocab map[string]int) (string, bool) {
	rToken := []rune(strings.ToLower(token))
	lenToken := len(rToken)
	if lenToken < 4 {
		return "", false
	}

	// Exact match check
	if _, ok := vocab[string(rToken)]; ok {
		return string(rToken), true
	}

	maxDist := 1
	if lenToken >= 8 {
		maxDist = 2
	}

	// For 4-letter words, only allow candidates of at least length 4 (do not reduce to 3-letter terms)
	minLen := 4
	if lenToken >= 5 {
		minLen = lenToken - 1
	}

	bestMatch := ""
	bestDist := maxDist + 1
	bestCount := -1

	for term, cnt := range vocab {
		tLower := strings.ToLower(term)
		// Blacklist format stopwords from candidate replacements
		if formatStopwords[tLower] {
			continue
		}

		rTerm := []rune(term)
		lenTerm := len(rTerm)
		if lenTerm < minLen {
			continue
		}

		diff := lenToken - lenTerm
		if diff < 0 {
			diff = -diff
		}
		if diff > maxDist {
			continue
		}

		dist := DamerauLevenshtein(rToken, rTerm)
		if dist <= maxDist {
			if dist < bestDist || (dist == bestDist && cnt > bestCount) {
				bestDist = dist
				bestMatch = term
				bestCount = cnt
			}
		}
	}

	if bestMatch != "" {
		return bestMatch, true
	}
	return "", false
}

// BuildFuzzyFTS5Query expands misspelled tokens with their closest vocabulary candidates.
func BuildFuzzyFTS5Query(tokens []string, vocab map[string]int, formatFilter string) (string, string, bool) {
	var tokenClauses []string
	var correctedWords []string
	hadCorrection := false

	for _, t := range tokens {
		tLower := strings.ToLower(t)
		cand, found := FindFuzzyCandidate(tLower, vocab)
		if found && cand != tLower {
			tokenClauses = append(tokenClauses, fmt.Sprintf(`("%s"* OR "%s"*)`, tLower, cand))
			correctedWords = append(correctedWords, cand)
			hadCorrection = true
		} else {
			tokenClauses = append(tokenClauses, fmt.Sprintf(`"%s"*`, tLower))
			correctedWords = append(correctedWords, tLower)
		}
	}

	if !hadCorrection {
		return "", strings.Join(tokens, " "), false
	}

	var baseExpr string
	if len(tokenClauses) > 0 {
		baseExpr = strings.Join(tokenClauses, " AND ")
	}

	var formatExpr string
	f := strings.ToLower(strings.TrimSpace(formatFilter))
	switch f {
	case "lossless", "flac":
		formatExpr = `(flac* OR lossless* OR ape* OR alac* OR wav* OR "24bit"*)`
	case "mp3_320", "320":
		formatExpr = `("320"*)`
	}

	var finalFts string
	if baseExpr != "" && formatExpr != "" {
		finalFts = fmt.Sprintf("(%s) AND %s", baseExpr, formatExpr)
	} else if formatExpr != "" {
		finalFts = formatExpr
	} else {
		finalFts = baseExpr
	}

	return finalFts, strings.Join(correctedWords, " "), true
}

// BuildRelaxedFTS5Query builds a relaxed OR query across terms, useful when multi-word queries (3+ words)
// contain minor differences, ranked by bm25. It ignores format stopwords to prevent broad false-positives.
func BuildRelaxedFTS5Query(tokens []string, vocab map[string]int, formatFilter string) string {
	if len(tokens) < 3 {
		return ""
	}

	var orClauses []string
	for _, t := range tokens {
		tLower := strings.ToLower(t)
		if formatStopwords[tLower] {
			continue
		}
		cand, found := FindFuzzyCandidate(tLower, vocab)
		if found && cand != tLower && !formatStopwords[strings.ToLower(cand)] {
			orClauses = append(orClauses, fmt.Sprintf(`"%s"*`, cand))
		}
		orClauses = append(orClauses, fmt.Sprintf(`"%s"*`, tLower))
	}
	if len(orClauses) < 2 {
		return ""
	}
	baseExpr := fmt.Sprintf("(%s)", strings.Join(orClauses, " OR "))

	var formatExpr string
	f := strings.ToLower(strings.TrimSpace(formatFilter))
	switch f {
	case "lossless", "flac":
		formatExpr = `(flac* OR lossless* OR ape* OR alac* OR wav* OR "24bit"*)`
	case "mp3_320", "320":
		formatExpr = `("320"*)`
	}

	if formatExpr != "" {
		return fmt.Sprintf("%s AND %s", baseExpr, formatExpr)
	}
	return baseExpr
}

// CorrectQuery uses the vocabulary to correct misspelled words in query.
// It returns the corrected query string and true if any corrections were made.
func (m *Manager) CorrectQuery(ctx context.Context, query string) (string, bool) {
	tokens := ExtractTokens(query)
	if len(tokens) == 0 {
		return query, false
	}
	vocab := m.getVocab(ctx)
	if len(vocab) == 0 {
		return query, false
	}

	var correctedTokens []string
	hadCorrection := false

	for _, t := range tokens {
		tLower := strings.ToLower(t)
		cand, found := FindFuzzyCandidate(tLower, vocab)
		if found && cand != tLower {
			correctedTokens = append(correctedTokens, cand)
			hadCorrection = true
		} else {
			correctedTokens = append(correctedTokens, t)
		}
	}

	if hadCorrection {
		return strings.Join(correctedTokens, " "), true
	}
	return query, false
}

func (m *Manager) queryFTS5(ctx context.Context, ftsExpr string, formatFilter string, limit int) []search.Result {
	ftsSQL := `
		SELECT sd.title, sd.magnet_uri, s.name
		FROM source_downloads_fts fts
		JOIN source_downloads sd ON sd.id = fts.rowid
		JOIN sources s ON s.id = sd.source_id
		WHERE source_downloads_fts MATCH ?
		ORDER BY bm25(source_downloads_fts) ASC
		LIMIT ?`
	rows, err := m.db.QueryContext(ctx, ftsSQL, ftsExpr, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return m.scanResults(rows, formatFilter)
}

// Search queries all indexed source downloads matching the given terms or magnet URI/hash,
// using SQLite FTS5 full-text indexing with bm25 relevance ranking, typo tolerance, and format filtering.
func (m *Manager) Search(ctx context.Context, query string, formatFilter string, limit int) []search.Result {
	if m.db == nil || limit <= 0 {
		return nil
	}
	trimmed := strings.TrimSpace(query)
	if trimmed == "" && formatFilter == "" {
		return nil
	}

	cleanQuery, detectedFilter := ParseQueryAndFilter(trimmed, formatFilter)
	if detectedFilter != "" {
		formatFilter = detectedFilter
	}

	// Check if search query is a magnet link or infohash
	lowerQuery := strings.ToLower(cleanQuery)
	var hashToFind string
	if strings.HasPrefix(lowerQuery, "magnet:") {
		// Look for xt=urn:btih:
		if idx := strings.Index(lowerQuery, "urn:btih:"); idx != -1 {
			rem := lowerQuery[idx+len("urn:btih:"):]
			if end := strings.IndexAny(rem, ";&"); end != -1 {
				hashToFind = rem[:end]
			} else {
				hashToFind = rem
			}
		}
	} else if (len(cleanQuery) == 40 || len(cleanQuery) == 32) && !strings.ContainsAny(cleanQuery, " \t\r\n") {
		hashToFind = lowerQuery
	}

	if hashToFind != "" {
		sqlQuery := `
			SELECT sd.title, sd.magnet_uri, s.name
			FROM source_downloads sd
			JOIN sources s ON s.id = sd.source_id
			WHERE sd.magnet_uri LIKE ?
			LIMIT ?`
		rows, err := m.db.QueryContext(ctx, sqlQuery, "%"+hashToFind+"%", limit)
		if err != nil {
			return nil
		}
		defer rows.Close()
		return m.scanResults(rows, formatFilter)
	}

	// 1. Text query: Try direct SQLite FTS5 first
	ftsExpr := BuildFTS5Query(cleanQuery, formatFilter)
	if ftsExpr != "" {
		results := m.queryFTS5(ctx, ftsExpr, formatFilter, limit)
		if len(results) > 0 {
			return results
		}
	}

	// 2. Fuzzy / Typo tolerant matching via FTS5 vocabulary
	tokens := ExtractTokens(cleanQuery)
	if len(tokens) > 0 {
		vocab := m.getVocab(ctx)
		if len(vocab) > 0 {
			fuzzyExpr, _, hadCorrection := BuildFuzzyFTS5Query(tokens, vocab, formatFilter)
			if hadCorrection && fuzzyExpr != "" {
				results := m.queryFTS5(ctx, fuzzyExpr, formatFilter, limit)
				if len(results) > 0 {
					return results
				}
			}

			// 2b. If strict AND produced 0, try relaxed OR ranking only for queries with >= 3 words
			if len(tokens) >= 3 {
				relaxedExpr := BuildRelaxedFTS5Query(tokens, vocab, formatFilter)
				if relaxedExpr != "" {
					results := m.queryFTS5(ctx, relaxedExpr, formatFilter, limit)
					if len(results) > 0 {
						return results
					}
				}
			}
		}
	}

	// 3. Fallback to LIKE if FTS produced 0 results or encountered an error
	return m.searchLikeFallback(ctx, cleanQuery, formatFilter, limit)
}

func (m *Manager) searchLikeFallback(ctx context.Context, query string, formatFilter string, limit int) []search.Result {
	words := strings.Fields(strings.ToLower(query))
	var whereClauses []string
	var args []any

	for _, w := range words {
		wClean := strings.ReplaceAll(w, "%", "")
		wClean = strings.ReplaceAll(wClean, "_", "")
		if wClean == "" {
			continue
		}
		whereClauses = append(whereClauses, "(sd.title_lower LIKE ? OR sd.magnet_uri LIKE ?)")
		args = append(args, "%"+wClean+"%", "%"+wClean+"%")
	}

	f := strings.ToLower(formatFilter)
	if f == "lossless" || f == "flac" {
		whereClauses = append(whereClauses, "(sd.title_lower LIKE '%flac%' OR sd.title_lower LIKE '%lossless%' OR sd.title_lower LIKE '%ape%' OR sd.title_lower LIKE '%alac%' OR sd.title_lower LIKE '%wav%' OR sd.title_lower LIKE '%24bit%')")
	} else if f == "mp3_320" || f == "320" {
		whereClauses = append(whereClauses, "sd.title_lower LIKE '%320%'")
	}

	if len(whereClauses) == 0 {
		return nil
	}
	args = append(args, limit)

	sqlQuery := fmt.Sprintf(`
		SELECT sd.title, sd.magnet_uri, s.name
		FROM source_downloads sd
		JOIN sources s ON s.id = sd.source_id
		WHERE %s
		LIMIT ?`, strings.Join(whereClauses, " AND "))

	rows, err := m.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return m.scanResults(rows, "")
}

func (m *Manager) scanResults(rows *sql.Rows, formatFilter string) []search.Result {
	var results []search.Result
	for rows.Next() {
		var title, magnetURI, sourceName string
		if err := rows.Scan(&title, &magnetURI, &sourceName); err == nil {
			if formatFilter != "" && !MatchesFormat(title, formatFilter) {
				continue
			}
			results = append(results, search.Result{
				Title:     title,
				MagnetURI: magnetURI,
				Source:    sourceName,
				Category:  "Music",
				Seeders:   0, // Real count fetched async via ScrapeSeeders
				Size:      "Раздача",
			})
		}
	}
	return results
}

func (m *Manager) fetchRemoteJSON(url string) (*SourceJSON, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("создание запроса: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 Posluchac/1.0")

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка загрузки источника: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("сервер вернул статус %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	var data SourceJSON
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON источника: %w", err)
	}

	if strings.TrimSpace(data.Name) == "" {
		data.Name = "Источник"
	}

	return &data, nil
}
