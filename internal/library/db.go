// Package library manages the local music library: tracks, playlists,
// favourites, and download history, all stored in SQLite.
package library

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Track represents a single audio track in the library.
type Track struct {
	ID          int64     `json:"id"`
	InfoHash    string    `json:"infoHash"`
	FilePath    string    `json:"filePath"`
	FileName    string    `json:"fileName"`
	Title       string    `json:"title"`
	Artist      string    `json:"artist"`
	Album       string    `json:"album"`
	Year        int       `json:"year"`
	Genre       string    `json:"genre"`
	DurationSec float64   `json:"durationSec"`
	SizeByes    int64     `json:"sizeBytes"`
	CoverPath   string    `json:"coverPath,omitempty"`
	Favourite   bool      `json:"favourite"`
	AddedAt     time.Time `json:"addedAt"`
}

// Playlist is an ordered collection of tracks.
type Playlist struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	Tracks    []Track   `json:"tracks,omitempty"`
}

// DB wraps a SQLite database for library operations.
type DB struct {
	db *sql.DB
}

// SQLDB returns the underlying *sql.DB connection.
func (l *DB) SQLDB() *sql.DB {
	return l.db
}

// Open opens (or creates) the library database at the given path.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open library db: %w", err)
	}
	l := &DB{db: db}
	if err := l.migrate(); err != nil {
		return nil, err
	}
	return l, nil
}

// migrate applies the schema if it doesn't exist yet.
func (l *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tracks (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		info_hash   TEXT NOT NULL,
		file_path   TEXT NOT NULL UNIQUE,
		file_name   TEXT NOT NULL,
		title       TEXT DEFAULT '',
		artist      TEXT DEFAULT '',
		album       TEXT DEFAULT '',
		year        INTEGER DEFAULT 0,
		genre       TEXT DEFAULT '',
		duration_sec REAL DEFAULT 0,
		size_bytes  INTEGER DEFAULT 0,
		cover_path  TEXT DEFAULT '',
		favourite   INTEGER DEFAULT 0,
		added_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS playlists (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS playlist_tracks (
		playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
		track_id    INTEGER NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		PRIMARY KEY (playlist_id, track_id)
	);

	CREATE INDEX IF NOT EXISTS idx_tracks_info_hash ON tracks(info_hash);
	CREATE INDEX IF NOT EXISTS idx_tracks_artist    ON tracks(artist);
	`
	_, err := l.db.Exec(schema)
	return err
}

// AddTrack inserts or replaces a track in the library.
func (l *DB) AddTrack(t Track) (int64, error) {
	res, err := l.db.Exec(`
		INSERT INTO tracks
			(info_hash, file_path, file_name, title, artist, album, year, genre,
			 duration_sec, size_bytes, cover_path, favourite)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(file_path) DO UPDATE SET
			title=excluded.title, artist=excluded.artist, album=excluded.album,
			year=excluded.year, genre=excluded.genre,
			duration_sec=CASE WHEN excluded.duration_sec > 0 THEN excluded.duration_sec ELSE tracks.duration_sec END,
			cover_path=CASE WHEN excluded.cover_path != '' THEN excluded.cover_path ELSE tracks.cover_path END`,
		t.InfoHash, t.FilePath, t.FileName, t.Title, t.Artist, t.Album,
		t.Year, t.Genre, t.DurationSec, t.SizeByes, t.CoverPath, t.Favourite,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetAllTracks returns all tracks sorted by artist + album + title.
func (l *DB) GetAllTracks() ([]Track, error) {
	rows, err := l.db.Query(`
		SELECT id, info_hash, file_path, file_name, title, artist, album,
		       year, genre, duration_sec, size_bytes, cover_path, favourite, added_at
		FROM tracks ORDER BY artist, album, title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTracks(rows)
}

// SearchTracks returns tracks matching query in title, artist, album, or file name.
// Fully supports case-insensitive Unicode (Cyrillic, Latin, etc.) and multi-word queries.
func (l *DB) SearchTracks(query string) ([]Track, error) {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return l.GetAllTracks()
	}

	all, err := l.GetAllTracks()
	if err != nil {
		return nil, err
	}

	words := strings.Fields(query)
	var matches []Track
	for _, t := range all {
		tTitle := strings.ToLower(t.Title)
		tArtist := strings.ToLower(t.Artist)
		tAlbum := strings.ToLower(t.Album)
		tFile := strings.ToLower(t.FileName)

		allWordsMatch := true
		for _, w := range words {
			if !strings.Contains(tTitle, w) &&
				!strings.Contains(tArtist, w) &&
				!strings.Contains(tAlbum, w) &&
				!strings.Contains(tFile, w) {
				allWordsMatch = false
				break
			}
		}
		if allWordsMatch {
			matches = append(matches, t)
		}
	}
	return matches, nil
}

// GetFavourites returns all favourite tracks.
func (l *DB) GetFavourites() ([]Track, error) {
	rows, err := l.db.Query(`
		SELECT id, info_hash, file_path, file_name, title, artist, album,
		       year, genre, duration_sec, size_bytes, cover_path, favourite, added_at
		FROM tracks WHERE favourite = 1 ORDER BY artist, title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTracks(rows)
}

// ToggleFavourite flips the favourite flag for the given track.
func (l *DB) ToggleFavourite(trackID int64) error {
	_, err := l.db.Exec(`UPDATE tracks SET favourite = NOT favourite WHERE id = ?`, trackID)
	return err
}

// DeleteTrack removes a track from the library and any playlists.
func (l *DB) DeleteTrack(trackID int64) error {
	_, _ = l.db.Exec(`DELETE FROM playlist_tracks WHERE track_id = ?`, trackID)
	_, err := l.db.Exec(`DELETE FROM tracks WHERE id = ?`, trackID)
	return err
}

// GetTrack returns a track by its ID.
func (l *DB) GetTrack(trackID int64) (*Track, error) {
	row := l.db.QueryRow(`
		SELECT id, info_hash, file_path, file_name, title, artist, album,
		       year, genre, duration_sec, size_bytes, cover_path, favourite, added_at
		FROM tracks WHERE id = ?`, trackID)
	t := &Track{}
	err := row.Scan(&t.ID, &t.InfoHash, &t.FilePath, &t.FileName, &t.Title, &t.Artist, &t.Album,
		&t.Year, &t.Genre, &t.DurationSec, &t.SizeByes, &t.CoverPath, &t.Favourite, &t.AddedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// GetTrackByPath returns a track by its file path.
func (l *DB) GetTrackByPath(filePath string) (*Track, error) {
	row := l.db.QueryRow(`
		SELECT id, info_hash, file_path, file_name, title, artist, album,
		       year, genre, duration_sec, size_bytes, cover_path, favourite, added_at
		FROM tracks WHERE file_path = ?`, filePath)
	t := &Track{}
	err := row.Scan(&t.ID, &t.InfoHash, &t.FilePath, &t.FileName, &t.Title, &t.Artist, &t.Album,
		&t.Year, &t.Genre, &t.DurationSec, &t.SizeByes, &t.CoverPath, &t.Favourite, &t.AddedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// UpdateTrackDuration updates duration_sec for a track.
func (l *DB) UpdateTrackDuration(id int64, durationSec float64) error {
	_, err := l.db.Exec(`UPDATE tracks SET duration_sec = ? WHERE id = ?`, durationSec, id)
	return err
}

// UpdateTrackDurationByPath updates duration_sec for a track by file path.
func (l *DB) UpdateTrackDurationByPath(filePath string, durationSec float64) error {
	_, err := l.db.Exec(`UPDATE tracks SET duration_sec = ? WHERE file_path = ?`, durationSec, filePath)
	return err
}

// UpdateTrackCover updates cover_path for a track.
func (l *DB) UpdateTrackCover(id int64, coverPath string) error {
	_, err := l.db.Exec(`UPDATE tracks SET cover_path = ? WHERE id = ?`, coverPath, id)
	return err
}

// CreatePlaylist creates a new empty playlist.
func (l *DB) CreatePlaylist(name string) (int64, error) {
	res, err := l.db.Exec(`INSERT INTO playlists (name) VALUES (?)`, name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// AddToPlaylist appends a track to a playlist at the given position. If position <= 0, it places it at the end.
func (l *DB) AddToPlaylist(playlistID, trackID int64, position int) error {
	var count int
	_ = l.db.QueryRow(`SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = ? AND track_id = ?`, playlistID, trackID).Scan(&count)
	if count > 0 {
		return nil
	}

	if position <= 0 {
		row := l.db.QueryRow(`SELECT COALESCE(MAX(position), 0) + 1 FROM playlist_tracks WHERE playlist_id = ?`, playlistID)
		_ = row.Scan(&position)
	}
	_, err := l.db.Exec(`
		INSERT OR IGNORE INTO playlist_tracks (playlist_id, track_id, position)
		VALUES (?, ?, ?)`, playlistID, trackID, position)
	return err
}

// RemoveFromPlaylist removes a track from a playlist.
func (l *DB) RemoveFromPlaylist(playlistID, trackID int64) error {
	_, err := l.db.Exec(`DELETE FROM playlist_tracks WHERE playlist_id = ? AND track_id = ?`, playlistID, trackID)
	return err
}

// GetPlaylistsForTrack returns IDs of all playlists containing the track.
func (l *DB) GetPlaylistsForTrack(trackID int64) ([]int64, error) {
	rows, err := l.db.Query(`SELECT playlist_id FROM playlist_tracks WHERE track_id = ?`, trackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetPlaylistTracks returns all tracks in a playlist ordered by position.
func (l *DB) GetPlaylistTracks(playlistID int64) ([]Track, error) {
	rows, err := l.db.Query(`
		SELECT t.id, t.info_hash, t.file_path, t.file_name, t.title, t.artist, t.album,
		       t.year, t.genre, t.duration_sec, t.size_bytes, t.cover_path, t.favourite, t.added_at
		FROM tracks t
		JOIN playlist_tracks pt ON pt.track_id = t.id
		WHERE pt.playlist_id = ?
		ORDER BY pt.position`, playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTracks(rows)
}

// ListPlaylists returns all playlists (without tracks populated).
func (l *DB) ListPlaylists() ([]Playlist, error) {
	rows, err := l.db.Query(`SELECT id, name, created_at FROM playlists ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playlists []Playlist
	for rows.Next() {
		var p Playlist
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt); err != nil {
			return nil, err
		}
		playlists = append(playlists, p)
	}
	return playlists, nil
}

// DeletePlaylist removes a playlist and all its track associations.
func (l *DB) DeletePlaylist(playlistID int64) error {
	_, err := l.db.Exec(`DELETE FROM playlists WHERE id = ?`, playlistID)
	return err
}

// GetPlaylist returns a playlist with its tracks.
func (l *DB) GetPlaylist(playlistID int64) (*Playlist, error) {
	row := l.db.QueryRow(`SELECT id, name, created_at FROM playlists WHERE id = ?`, playlistID)
	var p Playlist
	if err := row.Scan(&p.ID, &p.Name, &p.CreatedAt); err != nil {
		return nil, err
	}
	tracks, err := l.GetPlaylistTracks(playlistID)
	if err == nil {
		p.Tracks = tracks
	}
	return &p, nil
}

// ExportPlaylistM3U formats the playlist as Extended M3U text (UTF-8).
func (l *DB) ExportPlaylistM3U(playlistID int64) (string, error) {
	tracks, err := l.GetPlaylistTracks(playlistID)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	for _, t := range tracks {
		duration := int(t.DurationSec)
		if duration <= 0 {
			duration = -1
		}
		displayName := t.Title
		if displayName == "" {
			displayName = t.FileName
		}
		if t.Artist != "" {
			displayName = fmt.Sprintf("%s - %s", t.Artist, displayName)
		}
		sb.WriteString(fmt.Sprintf("#EXTINF:%d,%s\n", duration, displayName))
		sb.WriteString(t.FilePath + "\n")
	}
	return sb.String(), nil
}

// FindTrackByTitleArtist searches for a track by artist and title in the local library.
func (l *DB) FindTrackByTitleArtist(title, artist string) (*Track, error) {
	title = strings.TrimSpace(title)
	artist = strings.TrimSpace(artist)
	if title == "" && artist == "" {
		return nil, sql.ErrNoRows
	}

	var row *sql.Row
	if title != "" && artist != "" {
		row = l.db.QueryRow(`
			SELECT id, info_hash, file_path, file_name, title, artist, album,
			       year, genre, duration_sec, size_bytes, cover_path, favourite, added_at
			FROM tracks
			WHERE LOWER(title) = LOWER(?) AND LOWER(artist) = LOWER(?)
			LIMIT 1`, title, artist)
	} else if title != "" {
		row = l.db.QueryRow(`
			SELECT id, info_hash, file_path, file_name, title, artist, album,
			       year, genre, duration_sec, size_bytes, cover_path, favourite, added_at
			FROM tracks
			WHERE LOWER(title) = LOWER(?) OR LOWER(file_name) LIKE LOWER(?)
			LIMIT 1`, title, "%"+title+"%")
	}
	if row == nil {
		return nil, sql.ErrNoRows
	}
	t := &Track{}
	err := row.Scan(&t.ID, &t.InfoHash, &t.FilePath, &t.FileName, &t.Title, &t.Artist, &t.Album,
		&t.Year, &t.Genre, &t.DurationSec, &t.SizeByes, &t.CoverPath, &t.Favourite, &t.AddedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// Close closes the database connection.
func (l *DB) Close() error { return l.db.Close() }

func scanTracks(rows *sql.Rows) ([]Track, error) {
	var tracks []Track
	for rows.Next() {
		var t Track
		err := rows.Scan(
			&t.ID, &t.InfoHash, &t.FilePath, &t.FileName,
			&t.Title, &t.Artist, &t.Album, &t.Year, &t.Genre,
			&t.DurationSec, &t.SizeByes, &t.CoverPath, &t.Favourite, &t.AddedAt,
		)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}
