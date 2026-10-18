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
	"time"

	"github.com/user/torrent-player/internal/search"
)

// Source represents an added repository/source.
type Source struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	ItemCount int       `json:"itemCount"`
	UpdatedAt time.Time `json:"updatedAt"`
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
}

// NewManager creates a sources manager and ensures DB tables exist.
func NewManager(db *sql.DB) (*Manager, error) {
	m := &Manager{
		db: db,
		client: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
	if err := m.initSchema(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) initSchema() error {
	tables := `
	CREATE TABLE IF NOT EXISTS sources (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		url TEXT NOT NULL UNIQUE,
		item_count INTEGER DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS source_downloads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_id INTEGER NOT NULL,
		title TEXT NOT NULL,
		title_lower TEXT NOT NULL DEFAULT '',
		magnet_uri TEXT NOT NULL,
		FOREIGN KEY(source_id) REFERENCES sources(id) ON DELETE CASCADE
	);
	`
	if _, err := m.db.Exec(tables); err != nil {
		return err
	}

	// Safe column migration for existing tables created before title_lower
	_, _ = m.db.Exec(`ALTER TABLE source_downloads ADD COLUMN title_lower TEXT NOT NULL DEFAULT ''`)

	indices := `
	CREATE INDEX IF NOT EXISTS idx_source_downloads_title ON source_downloads(title);
	CREATE INDEX IF NOT EXISTS idx_source_downloads_title_lower ON source_downloads(title_lower);
	CREATE INDEX IF NOT EXISTS idx_source_downloads_source ON source_downloads(source_id);
	`
	if _, err := m.db.Exec(indices); err != nil {
		return err
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

	return &Source{
		ID:        sourceID,
		Name:      data.Name,
		URL:       url,
		ItemCount: len(data.Downloads),
		UpdatedAt: time.Now(),
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
	return err
}

// ListSources returns all registered sources with their item counts.
func (m *Manager) ListSources() ([]Source, error) {
	rows, err := m.db.Query(`SELECT id, name, url, item_count, updated_at FROM sources ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Source
	for rows.Next() {
		var s Source
		if err := rows.Scan(&s.ID, &s.Name, &s.URL, &s.ItemCount, &s.UpdatedAt); err == nil {
			result = append(result, s)
		}
	}
	return result, nil
}

// Search queries all indexed source downloads matching the given terms or magnet URI/hash.
func (m *Manager) Search(ctx context.Context, query string, limit int) []search.Result {
	if m.db == nil || limit <= 0 {
		return nil
	}
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil
	}

	// Check if search query is a magnet link or infohash
	lowerQuery := strings.ToLower(trimmed)
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
	} else if (len(trimmed) == 40 || len(trimmed) == 32) && !strings.ContainsAny(trimmed, " \t\r\n") {
		hashToFind = lowerQuery
	}

	var sqlQuery string
	var args []any

	if hashToFind != "" {
		sqlQuery = `
			SELECT sd.title, sd.magnet_uri, s.name
			FROM source_downloads sd
			JOIN sources s ON s.id = sd.source_id
			WHERE sd.magnet_uri LIKE ?
			LIMIT ?`
		args = []any{"%" + hashToFind + "%", limit}
	} else {
		words := strings.Fields(lowerQuery)
		if len(words) == 0 {
			return nil
		}

		var whereClauses []string
		for _, w := range words {
			wClean := strings.ReplaceAll(w, "%", "")
			wClean = strings.ReplaceAll(wClean, "_", "")
			if wClean == "" {
				continue
			}
			whereClauses = append(whereClauses, "(sd.title_lower LIKE ? OR sd.magnet_uri LIKE ?)")
			args = append(args, "%"+wClean+"%", "%"+wClean+"%")
		}
		if len(whereClauses) == 0 {
			return nil
		}
		args = append(args, limit)

		sqlQuery = fmt.Sprintf(`
			SELECT sd.title, sd.magnet_uri, s.name
			FROM source_downloads sd
			JOIN sources s ON s.id = sd.source_id
			WHERE %s
			LIMIT ?`, strings.Join(whereClauses, " AND "))
	}

	rows, err := m.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []search.Result
	for rows.Next() {
		var title, magnetURI, sourceName string
		if err := rows.Scan(&title, &magnetURI, &sourceName); err == nil {
			results = append(results, search.Result{
				Title:     title,
				MagnetURI: magnetURI,
				Source:    sourceName,
				Category:  "Music",
				Seeders:   10, // Default indicator for sources
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
