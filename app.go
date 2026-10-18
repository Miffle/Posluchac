// Package main contains the Wails App struct and all backend service methods
// exposed to the frontend via JS bindings.
package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/user/torrent-player/internal/audio"
	"github.com/user/torrent-player/internal/hotkeys"
	"github.com/user/torrent-player/internal/library"
	"github.com/user/torrent-player/internal/lyrics"
	"github.com/user/torrent-player/internal/metadata"
	"github.com/user/torrent-player/internal/search"
	"github.com/user/torrent-player/internal/sources"
	torrentpkg "github.com/user/torrent-player/internal/torrent"
	"github.com/user/torrent-player/internal/updater"
)

// App holds all backend services and is bound to the Wails runtime.
// Every exported method on App is callable from the frontend via JS.
type App struct {
	ctx          context.Context
	engine       *torrentpkg.Engine
	player       *audio.Player
	library      *library.DB
	sources      *sources.Manager
	scraper      *sources.Scraper
	searcher     *search.MultiSearcher
	rutracker    *search.RutrackerSearcher
	lyrics       *lyrics.Client
	hotkeys      *hotkeys.Manager
	dataDir      string
	downloadDir  string
	readySources chan struct{}
}

func NewApp() *App {
	home, _ := os.UserHomeDir()

	dataDir := filepath.Join(home, ".posluchac")
	oldDataDir := filepath.Join(home, ".torrent-player")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if _, oerr := os.Stat(oldDataDir); oerr == nil {
			dataDir = oldDataDir
		}
	}

	dlDir := filepath.Join(home, "Music", "Posluchac")
	oldDlDir := filepath.Join(home, "Music", "TorrentPlayer")
	if _, err := os.Stat(dlDir); os.IsNotExist(err) {
		if _, oerr := os.Stat(oldDlDir); oerr == nil {
			dlDir = oldDlDir
		}
	}

	return &App{
		dataDir:      dataDir,
		downloadDir:  dlDir,
		lyrics:       lyrics.NewClient(),
		readySources: make(chan struct{}),
	}
}

// startup is called by Wails when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	dlDir := a.downloadDir
	cacheDir := filepath.Join(a.dataDir, "torrent-cache")
	_ = os.MkdirAll(dlDir, 0o755)
	_ = os.MkdirAll(cacheDir, 0o755)
	dbPath := filepath.Join(a.dataDir, "library.db")

	var err error
	a.library, err = library.Open(dbPath)
	if err != nil {
		fmt.Println("library db error:", err)
	}

	var sqlDB *sql.DB
	if a.library != nil {
		sqlDB = a.library.SQLDB()
		var sErr error
		a.sources, sErr = sources.NewManager(sqlDB)
		if sErr != nil {
			fmt.Println("sources manager error:", sErr)
		} else {
			// Start background auto-sync (refreshes stale sources every 24 h).
			a.sources.StartAutoSync()
		}
	}
	a.scraper = sources.NewScraper()
	close(a.readySources)

	a.engine, err = torrentpkg.NewEngine(cacheDir, dlDir, sqlDB)
	if err != nil {
		fmt.Println("torrent engine error:", err)
	}

	a.player, err = audio.NewPlayer()
	if err != nil {
		fmt.Println("audio player error:", err)
	}

	// Build search backends
	rt, _ := search.NewRutrackerSearcher()
	a.rutracker = rt
	a.searcher = search.NewMultiSearcher(
		search.NewPirateBaySearcher(),
		search.NewX1337Searcher(),
		// Rutracker added after login via LoginRutracker()
	)

	// Start global media hotkeys listener (Fn+F7/F8/F9 etc.)
	a.hotkeys = hotkeys.Start(a.ctx)

	// Run initial library scan and background watcher for Music/TorrentPlayer
	go func() {
		time.Sleep(1 * time.Second)
		_, _ = a.ScanLibrary()
		for {
			time.Sleep(4 * time.Second)
			_, _ = a.ScanLibrary()
		}
	}()
}

// shutdown is called by Wails when the app closes.
func (a *App) shutdown(_ context.Context) {
	trayRemoveIcon()
	if a.hotkeys != nil {
		a.hotkeys.Stop()
	}
	if a.sources != nil {
		a.sources.Stop()
	}
	a.player.Stop()
	a.engine.Close()
	_ = a.library.Close()
}

// waitForSources waits for the sources manager to be initialized.
func (a *App) waitForSources() {
	if a.readySources == nil {
		return
	}
	select {
	case <-a.readySources:
	case <-time.After(5 * time.Second):
	}
}

// parseMagnetInfo inspects a query string to determine if it is a magnet link or infohash.
func parseMagnetInfo(raw string) (infoHash, title, magnetURI string, isMagnet bool) {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)

	if strings.HasPrefix(lower, "magnet:") {
		isMagnet = true
		magnetURI = trimmed

		if m, err := metainfo.ParseMagnetURI(trimmed); err == nil {
			infoHash = m.InfoHash.HexString()
			title = m.DisplayName
		}
		if title == "" {
			if u, err := url.Parse(trimmed); err == nil {
				title = u.Query().Get("dn")
			}
		}
		if title == "" {
			re := regexp.MustCompile(`(?i)[?&]dn=([^&]+)`)
			if matches := re.FindStringSubmatch(trimmed); len(matches) > 1 {
				if unescaped, err := url.QueryUnescape(strings.ReplaceAll(matches[1], "+", " ")); err == nil {
					title = unescaped
				} else {
					title = matches[1]
				}
			}
		}
		if title == "" {
			if infoHash != "" {
				prefix := infoHash
				if len(prefix) > 8 {
					prefix = prefix[:8]
				}
				title = "Magnet: " + prefix
			} else {
				title = "Magnet Torrent"
			}
		}
		return infoHash, title, magnetURI, true
	}

	// Check if bare hex hash (40 characters)
	if len(trimmed) == 40 {
		isHex := true
		for _, c := range trimmed {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				isHex = false
				break
			}
		}
		if isHex {
			hash := strings.ToLower(trimmed)
			return hash, "Torrent " + hash[:8], "magnet:?xt=urn:btih:" + hash, true
		}
	}

	// Check if bare base32 hash (32 characters)
	if len(trimmed) == 32 {
		isBase32 := true
		for _, c := range strings.ToUpper(trimmed) {
			if !((c >= 'A' && c <= 'Z') || (c >= '2' && c <= '7')) {
				isBase32 = false
				break
			}
		}
		if isBase32 {
			hash := strings.ToUpper(trimmed)
			return hash, "Torrent " + hash[:8], "magnet:?xt=urn:btih:" + hash, true
		}
	}

	return "", "", "", false
}

// Search queries all indexed JSON sources and torrent backends with optional format filtering.
func (a *App) Search(query string, formatFilter string) []search.Result {
	a.waitForSources()
	var results []search.Result

	trimmed := strings.TrimSpace(query)
	infoHash, magnetTitle, magnetURI, isMagnet := parseMagnetInfo(trimmed)

	cleanQuery, detectedFilter := sources.ParseQueryAndFilter(trimmed, formatFilter)
	if detectedFilter != "" {
		formatFilter = detectedFilter
	}

	if isMagnet {
		// First check indexed sources for this magnet or hash
		if a.sources != nil {
			sourceRes := a.sources.Search(a.ctx, magnetURI, formatFilter, 20)
			results = append(results, sourceRes...)
		}

		// Check if we already found an exact match in indexed sources
		hasExact := false
		for _, r := range results {
			if strings.EqualFold(r.MagnetURI, magnetURI) || (infoHash != "" && strings.Contains(strings.ToLower(r.MagnetURI), strings.ToLower(infoHash))) {
				hasExact = true
				break
			}
		}

		// If no exact item in sources, add the direct magnet result
		if !hasExact {
			results = append([]search.Result{{
				Title:     magnetTitle,
				MagnetURI: magnetURI,
				Source:    "magnet",
				Category:  "Music",
				Seeders:   0,
				Leechers:  0,
				Size:      "—",
				InfoHash:  infoHash,
			}}, results...)
		}

		// Do not query web scrapers for magnet links as they do not support them
		return results
	}

	if a.sources != nil {
		results = append(results, a.sources.Search(a.ctx, cleanQuery, formatFilter, 50)...)
	}

	// Prepare scraper query with typo correction from vocabulary if available
	scraperBase := cleanQuery
	if a.sources != nil {
		if corrected, ok := a.sources.CorrectQuery(a.ctx, cleanQuery); ok {
			scraperBase = corrected
		}
	}

	scraperQuery := scraperBase
	if formatFilter == "lossless" || formatFilter == "flac" {
		scraperQuery = scraperBase + " flac"
	} else if formatFilter == "mp3_320" || formatFilter == "320" {
		scraperQuery = scraperBase + " 320"
	}

	scraperResults := a.searcher.Search(a.ctx, scraperQuery, 30)
	if formatFilter != "" && formatFilter != "all" {
		var filtered []search.Result
		for _, r := range scraperResults {
			if sources.MatchesFormat(r.Title, formatFilter) {
				filtered = append(filtered, r)
			}
		}
		scraperResults = filtered
	}
	results = append(results, scraperResults...)
	return results
}

// AddSource downloads and indexes a remote JSON source (e.g. from GitHub Raw).
func (a *App) AddSource(url string) (*sources.Source, error) {
	a.waitForSources()
	if a.sources == nil {
		return nil, fmt.Errorf("источники ещё инициализируются")
	}
	return a.sources.AddSource(url)
}

// DeleteSource removes a source and its indexed downloads from the database.
func (a *App) DeleteSource(id int64) error {
	a.waitForSources()
	if a.sources == nil {
		return fmt.Errorf("источники ещё инициализируются")
	}
	return a.sources.DeleteSource(id)
}

// ListSources returns all registered download sources.
func (a *App) ListSources() ([]sources.Source, error) {
	a.waitForSources()
	if a.sources == nil {
		return nil, fmt.Errorf("источники ещё инициализируются")
	}
	return a.sources.ListSources()
}

// SyncSources updates all sources from their remote URLs.
func (a *App) SyncSources() error {
	a.waitForSources()
	if a.sources == nil {
		return nil
	}
	return a.sources.SyncAll()
}

// ScrapeSeederResult is returned per-hash by ScrapeSeeders.
type ScrapeSeederResult struct {
	InfoHash  string `json:"infoHash"`
	MagnetURI string `json:"magnetUri"` // original magnet URI passed by caller
	Seeders   int    `json:"seeders"`
	Leechers  int    `json:"leechers"`
	Completed int    `json:"completed"`
}

// ScrapeSeeders fetches real seeder/leecher counts from public UDP trackers
// for the supplied magnet URIs or hex info-hashes.
//
// The call blocks for up to ~3 seconds while trackers are queried in parallel,
// then returns the best result found across all trackers. Already-cached
// results (fresh within 30 min) are returned immediately without re-scraping.
func (a *App) ScrapeSeeders(magnetOrHashes []string) []ScrapeSeederResult {
	if a.scraper == nil || len(magnetOrHashes) == 0 {
		return nil
	}

	publicTrackers := []string{
		"udp://tracker.opentrackr.org:1337",
		"udp://open.stealth.si:80",
		"udp://tracker.openbittorrent.com:6969",
		"udp://exodus.desync.com:6969",
		"udp://tracker.torrent.eu.org:451",
	}

	// Deduplicate and extract hex infohashes; keep original magnet for response.
	type entry struct {
		hex    string
		magnet string
	}
	var entries []entry
	seen := make(map[string]bool)
	for _, raw := range magnetOrHashes {
		h := sources.ExtractInfoHash(raw)
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		entries = append(entries, entry{hex: h, magnet: raw})
	}
	if len(entries) == 0 {
		return nil
	}

	hexList := make([]string, len(entries))
	for i, e := range entries {
		hexList[i] = e.hex
	}

	trackerSet := make(map[string]bool)
	for _, t := range publicTrackers {
		trackerSet[t] = true
	}
	for _, raw := range magnetOrHashes {
		for _, tr := range sources.ExtractTrackers(raw) {
			trackerSet[tr] = true
		}
	}
	allTrackers := make([]string, 0, len(trackerSet))
	for t := range trackerSet {
		allTrackers = append(allTrackers, t)
	}

	// Synchronous scrape — blocks until all trackers respond or 2s timeout.
	scraped := a.scraper.ScrapeSync(allTrackers, hexList)

	out := make([]ScrapeSeederResult, 0, len(entries))
	for _, e := range entries {
		r := ScrapeSeederResult{InfoHash: e.hex, MagnetURI: e.magnet}
		if sr, ok := scraped[e.hex]; ok {
			r.Seeders = sr.Seeders
			r.Leechers = sr.Leechers
			r.Completed = sr.Completed
		}
		out = append(out, r)
	}
	return out
}

// LoginRutracker authenticates with Rutracker via username/password and enables Rutracker in search.
func (a *App) LoginRutracker(username, password string) error {
	if err := a.rutracker.Login(username, password); err != nil {
		return err
	}
	a.rebuildSearcher()
	return nil
}

// SetRutrackerCookie authenticates using browser cookies (e.g. bb_data or cf_clearance)
// and enables Rutracker in search.
func (a *App) SetRutrackerCookie(cookie string) error {
	if err := a.rutracker.SetCookie(cookie); err != nil {
		return err
	}
	a.rebuildSearcher()
	return nil
}

// SetRutrackerMirror sets custom mirror (e.g. https://rutracker.net).
func (a *App) SetRutrackerMirror(mirror string) {
	a.rutracker.SetBaseURL(mirror)
}

func (a *App) rebuildSearcher() {
	a.searcher = search.NewMultiSearcher(
		search.NewPirateBaySearcher(),
		search.NewX1337Searcher(),
		a.rutracker,
	)
}

// ─── Torrent ─────────────────────────────────────────────────────────────────

// AddMagnet adds a torrent via magnet link. Returns the info hash.
func (a *App) AddMagnet(magnetURI string) (string, error) {
	return a.engine.AddMagnet(magnetURI)
}

// AddTorrentFile adds a torrent from a local .torrent file.
func (a *App) AddTorrentFile(filePath string) (string, error) {
	return a.engine.AddTorrentFile(filePath)
}

// PendingTorrent is returned by AddFromResultPending with the file list
// so the frontend can display a file picker before downloading starts.
type PendingTorrent struct {
	InfoHash string                `json:"infoHash"`
	Name     string                `json:"name"`
	Files    []torrentpkg.FileInfo `json:"files"`
}

// AddFromResultPending resolves the torrent from a search result,
// fetches metadata, and returns the file list WITHOUT starting download.
// The frontend should then call DownloadSelectedFiles with chosen paths.
func (a *App) AddFromResultPending(result search.Result) (PendingTorrent, error) {
	var infoHash string
	var files []torrentpkg.FileInfo
	var err error

	if result.MagnetURI != "" {
		infoHash, files, err = a.engine.AddMagnetPending(result.MagnetURI)
	} else {
		switch result.Source {
		case "rutracker":
			data, dlErr := a.rutracker.DownloadTorrentFile(a.ctx, result.TorrentURL)
			if dlErr != nil {
				return PendingTorrent{}, dlErr
			}
			tmp, tmpErr := os.CreateTemp("", "tp-*.torrent")
			if tmpErr != nil {
				return PendingTorrent{}, tmpErr
			}
			defer os.Remove(tmp.Name())
			if _, wErr := tmp.Write(data); wErr != nil {
				return PendingTorrent{}, wErr
			}
			tmp.Close()
			infoHash, files, err = a.engine.AddTorrentFilePending(tmp.Name())

		case "1337x":
			x := search.NewX1337Searcher()
			magnet, mErr := x.GetMagnet(a.ctx, result.TorrentURL)
			if mErr != nil {
				return PendingTorrent{}, mErr
			}
			infoHash, files, err = a.engine.AddMagnetPending(magnet)

		default:
			return PendingTorrent{}, fmt.Errorf("cannot resolve torrent for source %q", result.Source)
		}
	}

	if err != nil {
		return PendingTorrent{}, err
	}
	return PendingTorrent{InfoHash: infoHash, Name: result.Title, Files: files}, nil
}

// DownloadSelectedFiles starts downloading only the specified file paths
// within the torrent identified by infoHash.
func (a *App) DownloadSelectedFiles(infoHash string, selectedPaths []string) error {
	return a.engine.DownloadSelectedFiles(infoHash, selectedPaths)
}

// CancelPendingTorrent drops a pending preview torrent without saving or downloading it.
func (a *App) CancelPendingTorrent(infoHash string) {
	st := a.player.Status()
	if st.Track != nil && strings.EqualFold(st.Track.InfoHash, infoHash) && st.State == audio.StatePlaying {
		// Don't kill audio stream if user is actively listening to preview
		return
	}
	a.engine.CancelPendingTorrent(infoHash)
}

// RutrackerStatus returns whether the user is currently logged in to Rutracker.
func (a *App) RutrackerStatus() bool {
	return a.rutracker != nil && a.rutracker.IsLoggedIn()
}

// AddFromResult is kept for compatibility (magnet-only quick add).
func (a *App) AddFromResult(result search.Result) (string, error) {
	pt, err := a.AddFromResultPending(result)
	if err != nil {
		return "", err
	}
	// Auto-download all files
	if err := a.engine.DownloadSelectedFiles(pt.InfoHash, nil); err != nil {
		return "", err
	}
	return pt.InfoHash, nil
}

// GetProgress returns download/upload progress for all active torrents.
func (a *App) GetProgress() []torrentpkg.DownloadProgress {
	return a.engine.GetProgress()
}

// ListTorrentFiles returns audio files within a torrent.
func (a *App) ListTorrentFiles(infoHash string) ([]torrentpkg.FileInfo, error) {
	return a.engine.ListFiles(infoHash)
}

// AddTorrentFileToDownload starts downloading an individual file from an existing torrent.
func (a *App) AddTorrentFileToDownload(infoHash, filePath string) error {
	return a.engine.AddFileToTorrent(infoHash, filePath)
}

// RemoveTorrentFileFromDownload cancels downloading of an individual file, deletes its local copy, and cleans up the library.
func (a *App) RemoveTorrentFileFromDownload(infoHash, filePath string) error {
	if err := a.engine.RemoveFileFromTorrent(infoHash, filePath); err != nil {
		return err
	}
	// Clean up any deleted track from the library DB
	_, _ = a.ScanLibrary()
	return nil
}

// PauseTorrent pauses a torrent download.
func (a *App) PauseTorrent(infoHash string) error {
	return a.engine.PauseTorrent(infoHash)
}

// ResumeTorrent resumes a paused torrent.
func (a *App) ResumeTorrent(infoHash string) error {
	return a.engine.ResumeTorrent(infoHash)
}

// RemoveTorrent removes a torrent. deleteFiles=true also deletes local files.
func (a *App) RemoveTorrent(infoHash string, deleteFiles bool) error {
	return a.engine.RemoveTorrent(infoHash, deleteFiles)
}

// ─── Playback ────────────────────────────────────────────────────────────────

// PlayTrack streams and plays a file from a torrent or local disk file.
func (a *App) PlayTrack(infoHash, filePath string) error {
	var r io.ReadSeekCloser
	var err error

	// Check if this file already exists locally on disk in Music/TorrentPlayer
	diskPath := filePath
	if !filepath.IsAbs(diskPath) {
		diskPath = filepath.Join(a.downloadDir, filePath)
	}

	if fi, statErr := os.Stat(diskPath); statErr == nil && !fi.IsDir() && fi.Size() > 0 {
		f, openErr := os.Open(diskPath)
		if openErr == nil {
			r = f
		}
	}

	if r == nil {
		r, err = a.engine.OpenFileReader(infoHash, filePath)
		if err != nil {
			return err
		}
	}

	ext := filepath.Ext(filePath)

	coverPath := ""
	var title, artist, album string
	if a.library != nil {
		if t, err := a.library.GetTrackByPath(diskPath); err == nil && t != nil {
			coverPath = t.CoverPath
			title = t.Title
			artist = t.Artist
			album = t.Album
		} else if t, err := a.library.GetTrackByPath(filePath); err == nil && t != nil {
			coverPath = t.CoverPath
			title = t.Title
			artist = t.Artist
			album = t.Album
		}
	}
	if (title == "" || artist == "") && diskPath != "" {
		if tags, err := metadata.ReadFile(diskPath); err == nil && tags != nil {
			if title == "" {
				title = tags.Title
			}
			if artist == "" {
				artist = tags.Artist
			}
			if album == "" {
				album = tags.Album
			}
		}
	}
	if title == "" {
		pArtist, pAlbum, pTitle, _ := parseSmartAudioInfo("", filePath, nil)
		if pTitle != "" {
			title = pTitle
		} else {
			title = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
		}
		if artist == "" && pArtist != "" {
			artist = pArtist
		}
		if album == "" && pAlbum != "" {
			album = pAlbum
		}
	}

	if coverPath == "" {
		coverPath = findCoverInDir(filepath.Dir(diskPath))
	}

	track := audio.TrackInfo{
		InfoHash:  infoHash,
		FilePath:  filePath,
		FileName:  filepath.Base(filePath),
		Title:     title,
		Artist:    artist,
		Album:     album,
		CoverPath: coverPath,
	}

	playErr := a.player.Play(r, ext, track)
	if playErr == nil && a.library != nil {
		go func(dp string) {
			time.Sleep(500 * time.Millisecond)
			st := a.player.Status()
			if st.DurationSec > 0 {
				_ = a.library.UpdateTrackDurationByPath(dp, st.DurationSec)
			}
		}(diskPath)
	}

	return playErr
}

// Pause toggles pause/resume of playback.
func (a *App) Pause() { a.player.Pause() }

// Stop stops playback.
func (a *App) Stop() { a.player.Stop() }

// SeekTo seeks to the given position in seconds.
func (a *App) SeekTo(seconds float64) error { return a.player.SeekTo(seconds) }

// SetVolume sets volume in [0.0, 1.0].
func (a *App) SetVolume(v float64) { a.player.SetVolume(v) }

// GetStatus returns the current playback status.
func (a *App) GetStatus() audio.Status { return a.player.Status() }

// ─── Library ─────────────────────────────────────────────────────────────────

// GetLibrary returns all tracks in the library.
func (a *App) GetLibrary() ([]library.Track, error) {
	return a.library.GetAllTracks()
}

// SearchLibrary searches the local library.
func (a *App) SearchLibrary(query string) ([]library.Track, error) {
	return a.library.SearchTracks(query)
}

// GetFavourites returns all favourite tracks.
func (a *App) GetFavourites() ([]library.Track, error) {
	return a.library.GetFavourites()
}

// ToggleFavourite flips the favourite flag for a track.
func (a *App) ToggleFavourite(trackID int64) error {
	return a.library.ToggleFavourite(trackID)
}

// ImportTrackToLibrary reads metadata from a torrent file and saves it to the library.
func (a *App) ImportTrackToLibrary(infoHash, filePath string) (library.Track, error) {
	r, err := a.engine.OpenFileReader(infoHash, filePath)
	if err != nil {
		return library.Track{}, err
	}
	defer r.Close()

	ext := filepath.Ext(filePath)
	tags, err := metadata.ReadReader(r, ext)
	if err != nil {
		tags = &metadata.Tags{}
	}

	name := filepath.Base(filePath)
	artist, album, title, year := parseSmartAudioInfo(a.downloadDir, filePath, tags)

	track := library.Track{
		InfoHash: infoHash,
		FilePath: filePath,
		FileName: name,
		Title:    title,
		Artist:   artist,
		Album:    album,
		Year:     year,
		Genre:    tags.Genre,
	}

	id, err := a.library.AddTrack(track)
	if err != nil {
		return library.Track{}, err
	}
	track.ID = id
	return track, nil
}

// ─── Playlists ────────────────────────────────────────────────────────────────

// CreatePlaylist creates a new playlist.
func (a *App) CreatePlaylist(name string) (int64, error) {
	return a.library.CreatePlaylist(name)
}

// ListPlaylists returns all playlists.
func (a *App) ListPlaylists() ([]library.Playlist, error) {
	return a.library.ListPlaylists()
}

// GetPlaylistTracks returns tracks in a playlist.
func (a *App) GetPlaylistTracks(playlistID int64) ([]library.Track, error) {
	return a.library.GetPlaylistTracks(playlistID)
}

// AddToPlaylist adds a track to a playlist.
func (a *App) AddToPlaylist(playlistID, trackID int64, position int) error {
	return a.library.AddToPlaylist(playlistID, trackID, position)
}

// DeletePlaylist removes a playlist.
func (a *App) DeletePlaylist(playlistID int64) error {
	return a.library.DeletePlaylist(playlistID)
}

// RemoveFromPlaylist removes a track from a playlist.
func (a *App) RemoveFromPlaylist(playlistID, trackID int64) error {
	if a.library == nil {
		return fmt.Errorf("library not initialized")
	}
	return a.library.RemoveFromPlaylist(playlistID, trackID)
}

// GetPlaylistsForTrack returns IDs of all playlists containing this track.
func (a *App) GetPlaylistsForTrack(trackID int64) ([]int64, error) {
	if a.library == nil {
		return nil, fmt.Errorf("library not initialized")
	}
	return a.library.GetPlaylistsForTrack(trackID)
}

// ─── Files and Folders ────────────────────────────────────────────────────────

// GetDownloadDir returns the music downloads folder path.
func (a *App) GetDownloadDir() string {
	return a.downloadDir
}

// OpenDownloadFolder opens the music downloads folder in Windows Explorer.
func (a *App) OpenDownloadFolder() error {
	_ = os.MkdirAll(a.downloadDir, 0o755)
	return exec.Command("explorer", a.downloadDir).Start()
}

// OpenFileInFolder opens Explorer and highlights the downloaded file.
func (a *App) OpenFileInFolder(infoHash, filePath string) error {
	fullPath := a.engine.GetFileDiskPath(infoHash, filePath)
	if fullPath != "" {
		if _, err := os.Stat(fullPath); err == nil {
			return exec.Command("explorer", "/select,", fullPath).Start()
		}
	}
	return a.OpenDownloadFolder()
}

// DeleteLibraryTrack removes a track from the library DB, deletes its audio file from disk,
// and unselects/cancels it from any active torrents.
func (a *App) DeleteLibraryTrack(trackID int64, deleteFile bool) error {
	if a.library == nil {
		return fmt.Errorf("library not initialized")
	}

	track, err := a.library.GetTrack(trackID)
	if err != nil || track == nil {
		return fmt.Errorf("track not found")
	}

	// 1. If currently playing this track, stop playback so file handle is released
	status := a.player.Status()
	if status.Track != nil && (status.Track.FilePath == track.FilePath || status.Track.FileName == track.FileName) {
		a.player.Stop()
	}

	// 2. Remove from active torrents, cache, and exported tracking
	if a.engine != nil {
		a.engine.RemoveFileFromAllTorrents(track.FilePath, track.FileName)
	}

	// 3. Delete file from disk
	if deleteFile && track.FilePath != "" {
		diskPath := track.FilePath
		if !filepath.IsAbs(diskPath) {
			diskPath = filepath.Join(a.downloadDir, diskPath)
		}
		_ = os.Remove(diskPath)
	}

	// 4. Remove from database
	return a.library.DeleteTrack(trackID)
}

// ScanLibrary scans downloadDir for all audio files (from completed torrents or manually placed
// by the user into Music/TorrentPlayer) and automatically registers them with ID3 tags into the library.
// It also cleans up any tracks whose files were deleted from disk and backfills duration and cover art.
func (a *App) ScanLibrary() (int, error) {
	if a.library == nil {
		return 0, fmt.Errorf("library not initialized")
	}

	// 1. Remove deleted files from the database and backfill missing durations/covers
	existing, err := a.library.GetAllTracks()
	existingByPath := make(map[string]*library.Track)
	if err == nil {
		for _, t := range existing {
			if t.FilePath != "" {
				if _, statErr := os.Stat(t.FilePath); os.IsNotExist(statErr) {
					_ = a.library.DeleteTrack(t.ID)
					continue
				}

				// Backfill missing duration using the player decoder
				if t.DurationSec <= 0 {
					if dur := audio.GetFileDuration(t.FilePath); dur > 0 {
						t.DurationSec = dur
						_ = a.library.UpdateTrackDuration(t.ID, dur)
					}
				}

				// Backfill missing cover art
				needCover := t.CoverPath == ""
				if !needCover {
					if _, cErr := os.Stat(t.CoverPath); os.IsNotExist(cErr) {
						needCover = true
					}
				}
				if needCover {
					if cov := findCoverInDir(filepath.Dir(t.FilePath)); cov != "" {
						t.CoverPath = cov
						_ = a.library.UpdateTrackCover(t.ID, cov)
					}
				}

				existingByPath[t.FilePath] = &t
			}
		}
	}

	// 2. Scan for new and completed audio files
	count := 0
	err = filepath.WalkDir(a.downloadDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}

		if !isAudioExtension(path) {
			return nil
		}

		fi, err := d.Info()
		if err != nil || fi.Size() == 0 {
			return nil
		}

		// Read ID3 or FLAC metadata
		tags, _ := metadata.ReadFile(path)
		if tags == nil {
			tags = &metadata.Tags{}
		}

		fileName := filepath.Base(path)
		artist, album, title, year := parseSmartAudioInfo(a.downloadDir, path, tags)

		var durationSec float64
		if ex, exists := existingByPath[path]; exists && ex.DurationSec > 0 {
			durationSec = ex.DurationSec
		} else {
			durationSec = audio.GetFileDuration(path)
		}

		coverPath := findCoverInDir(filepath.Dir(path))
		if coverPath == "" && tags != nil && len(tags.CoverData) > 0 {
			coverPath, _ = metadata.SaveCover(tags, path)
		}

		track := library.Track{
			InfoHash:    "local",
			FilePath:    path,
			FileName:    fileName,
			Title:       title,
			Artist:      artist,
			Album:       album,
			Year:        year,
			Genre:       tags.Genre,
			SizeByes:    fi.Size(),
			DurationSec: durationSec,
			CoverPath:   coverPath,
		}

		_, addErr := a.library.AddTrack(track)
		if addErr == nil {
			count++
		}
		return nil
	})

	return count, err
}

// parseSmartAudioInfo extracts artist, album, title and year from metadata tags or from file path and naming heuristics.
func parseSmartAudioInfo(downloadDir, fullPath string, tags *metadata.Tags) (artist, album, title string, year int) {
	if tags != nil {
		artist = strings.TrimSpace(tags.Artist)
		album = strings.TrimSpace(tags.Album)
		title = strings.TrimSpace(tags.Title)
		year = tags.Year
		if tags.AlbumArtist != "" && (artist == "" || strings.Contains(artist, ";") || strings.Contains(artist, ",")) {
			artist = strings.TrimSpace(tags.AlbumArtist)
		}
	}

	fileName := filepath.Base(fullPath)
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// If artist or title is missing, try parsing from filename (e.g. "01. Artist - Title" or "Artist - Title")
	cleanName := base
	reLead := regexp.MustCompile(`^(\d{1,3}[\.\-\s]+)+`)
	cleanName = strings.TrimSpace(reLead.ReplaceAllString(cleanName, ""))

	if strings.Contains(cleanName, " - ") {
		parts := strings.SplitN(cleanName, " - ", 2)
		pArtist := strings.TrimSpace(parts[0])
		pTitle := strings.TrimSpace(parts[1])
		if artist == "" && pArtist != "" {
			artist = pArtist
		}
		if title == "" && pTitle != "" {
			title = pTitle
		}
	}

	if title == "" {
		if cleanName != "" {
			title = cleanName
		} else {
			title = base
		}
	}

	// If artist or album is missing, deduce from folder hierarchy relative to downloadDir
	if rel, err := filepath.Rel(downloadDir, fullPath); err == nil && !strings.HasPrefix(rel, "..") {
		dirs := strings.Split(filepath.Dir(rel), string(filepath.Separator))
		var validDirs []string
		for _, d := range dirs {
			d = strings.TrimSpace(d)
			if d != "" && d != "." && d != "TorrentPlayer" && d != "Posluchac" {
				validDirs = append(validDirs, d)
			}
		}

		if len(validDirs) > 0 {
			if artist == "" {
				topDir := validDirs[0]
				reClean := regexp.MustCompile(`(?i)\s*(\[.*?\]|\(.*?\)|-\s*коллекция.*|-\s*discography.*)`)
				cleaned := strings.TrimSpace(reClean.ReplaceAllString(topDir, ""))
				if cleaned != "" {
					artist = cleaned
				} else {
					artist = topDir
				}
			}

			if album == "" && len(validDirs) >= 2 {
				lastDir := validDirs[len(validDirs)-1]
				if strings.EqualFold(lastDir, "cd1") || strings.EqualFold(lastDir, "cd2") || strings.EqualFold(lastDir, "single") {
					if len(validDirs) >= 3 {
						lastDir = validDirs[len(validDirs)-2]
					}
				}
				reDate := regexp.MustCompile(`^(\[\d{4}[\.\-]\d{2}[\.\-]\d{2}\]|\d{4}[\.\-]\d{2}[\.\-]\d{2})\s*`)
				if year == 0 {
					reYear := regexp.MustCompile(`\b(19\d\d|20\d\d)\b`)
					if m := reYear.FindString(lastDir); m != "" {
						if y, err := strconv.Atoi(m); err == nil {
							year = y
						}
					}
				}
				cleanedAlbum := strings.TrimSpace(reDate.ReplaceAllString(lastDir, ""))
				if cleanedAlbum != "" {
					album = cleanedAlbum
				}
			}
		}
	}

	// Leave artist empty if unknown so frontend can localize Unknown Artist
	return
}

// GetCover returns base64 Data URL for a cover image file, or empty string.
func (a *App) GetCover(coverPath string) string {
	if coverPath == "" {
		return ""
	}
	data, err := os.ReadFile(coverPath)
	if err != nil || len(data) == 0 {
		return ""
	}
	mime := "image/jpeg"
	ext := strings.ToLower(filepath.Ext(coverPath))
	switch ext {
	case ".png":
		mime = "image/png"
	case ".webp":
		mime = "image/webp"
	}
	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
}

// findCoverInDir looks for cover.jpg/png/jpeg/folder.jpg in dir or parent dir.
func findCoverInDir(dir string) string {
	if dir == "" || dir == "." {
		return ""
	}

	candidates := []string{
		"cover.jpg", "cover.jpeg", "cover.png", "Cover.png", "cover.webp",
		"folder.jpg", "folder.jpeg", "folder.png",
		"front.jpg", "front.jpeg", "front.png",
		"albumart.jpg", "albumart.png", "albumartsmall.jpg",
		"artwork.jpg", "artwork.png",
		"обложка.jpg", "обложка.png",
	}

	entries, err := os.ReadDir(dir)
	if err == nil {
		lowerMap := make(map[string]string)
		var imageFiles []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			lower := strings.ToLower(e.Name())
			lowerMap[lower] = e.Name()
			ext := filepath.Ext(lower)
			if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
				imageFiles = append(imageFiles, e.Name())
			}
		}

		// 1. Check exact candidate names
		for _, name := range candidates {
			if origName, ok := lowerMap[name]; ok {
				return filepath.Join(dir, origName)
			}
		}

		// 2. Check if any image name contains "cover", "front", "folder", "album"
		for _, img := range imageFiles {
			low := strings.ToLower(img)
			if strings.Contains(low, "cover") || strings.Contains(low, "front") ||
				strings.Contains(low, "folder") || strings.Contains(low, "обложк") ||
				strings.Contains(low, "album") {
				return filepath.Join(dir, img)
			}
		}

		// 3. Fallback to any image file in the directory
		if len(imageFiles) > 0 {
			return filepath.Join(dir, imageFiles[0])
		}
	}

	// 4. Check parent directory if not root (e.g. discography or CD1 folder)
	parent := filepath.Dir(dir)
	if parent != "" && parent != dir && filepath.Base(parent) != "TorrentPlayer" && filepath.Base(parent) != "Posluchac" && filepath.Base(parent) != "." {
		return findCoverInDir(parent)
	}

	return ""
}

func isAudioExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".mp3" || ext == ".flac" || ext == ".ogg" || ext == ".wav" ||
		ext == ".m4a" || ext == ".aac" || ext == ".opus" || ext == ".ape" || ext == ".wv"
}

// ─── Auto-Updater ────────────────────────────────────────────────────────────

// GetAppVersion returns the current application version.
func (a *App) GetAppVersion() string {
	return updater.CurrentVersion
}

// CheckForUpdates checks GitHub Releases for new versions of Posluchac.
func (a *App) CheckForUpdates() (*updater.UpdateInfo, error) {
	return updater.CheckForUpdates()
}

// ApplyUpdate downloads the new version executable and restarts Posluchac.
func (a *App) ApplyUpdate(downloadURL string) error {
	return updater.ApplyUpdate(downloadURL)
}

// OpenExternalURL opens a URL in the user's default browser.
func (a *App) OpenExternalURL(url string) error {
	return updater.OpenURL(url)
}

// ─── Lyrics (LRC) ────────────────────────────────────────────────────────────

// GetLyrics searches for synchronized or plain lyrics from local files or LRCLIB.
func (a *App) GetLyrics(artist, title, album string, durationSec float64, filePath string) (*lyrics.Result, error) {
	if a.lyrics == nil {
		a.lyrics = lyrics.NewClient()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()
	return a.lyrics.GetLyrics(ctx, artist, title, album, durationSec, filePath)
}
