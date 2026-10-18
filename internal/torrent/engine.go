// Package torrent wraps anacrolix/torrent and exposes a simple API for
// adding torrents, selective file downloading, streaming, and seeding.
package torrent

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

// TorrentState represents the current state of a managed torrent.
type TorrentState string

const (
	StateQueued      TorrentState = "queued"
	StateDownloading TorrentState = "downloading"
	StateSeeding     TorrentState = "seeding"
	StatePaused      TorrentState = "paused"
)

// ManagedTorrent wraps a torrent.Torrent with player-specific metadata.
type ManagedTorrent struct {
	T               *torrent.Torrent
	InfoHash        string
	Name            string
	MagnetURI       string
	State           TorrentState
	SelectedPaths   []string // paths of files chosen by the user for selective download
	DownloadAll     bool     // true if user explicitly requested all files
	AddedAt         time.Time
	lastReadBytes   int64
	lastWriteBytes  int64
	lastSpeedSample time.Time
	downloadSpeed   float64
	uploadSpeed     float64
}

// DownloadProgress holds current download/upload stats.
type DownloadProgress struct {
	InfoHash      string  `json:"infoHash"`
	Name          string  `json:"name"`
	Downloaded    int64   `json:"downloaded"`
	Total         int64   `json:"total"`
	PercentDone   float64 `json:"percentDone"`
	DownloadSpeed float64 `json:"downloadSpeed"`
	UploadSpeed   float64 `json:"uploadSpeed"`
	Seeders       int     `json:"seeders"`
	Peers         int     `json:"peers"`
	Leechers      int     `json:"leechers"`
	State         string  `json:"state"`
	AddedAt       int64   `json:"addedAt"`
	SelectedCount int     `json:"selectedCount"`
	SelectedTrack string  `json:"selectedTrack,omitempty"`
}

var defaultPublicTrackers = [][]string{
	{"udp://tracker.opentrackr.org:1337/announce"},
	{"udp://open.stealth.si:80/announce"},
	{"udp://tracker.torrent.eu.org:451/announce"},
	{"udp://tracker.bittor.pw:1337/announce"},
	{"udp://public.popcorn-tracker.org:6969/announce"},
	{"udp://tracker.dler.org:6969/announce"},
	{"udp://exodus.desync.com:6969/announce"},
	{"udp://open.demonii.com:1337/announce"},
	{"udp://tracker.openbittorrent.com:6969/announce"},
	{"http://tracker.openbittorrent.com:80/announce"},
}

func injectPublicTrackers(spec *torrent.TorrentSpec) {
	if spec == nil {
		return
	}
	existing := make(map[string]bool)
	for _, tier := range spec.Trackers {
		for _, tr := range tier {
			existing[strings.ToLower(strings.TrimSpace(tr))] = true
		}
	}
	for _, tier := range defaultPublicTrackers {
		for _, tr := range tier {
			norm := strings.ToLower(strings.TrimSpace(tr))
			if !existing[norm] {
				spec.Trackers = append(spec.Trackers, []string{tr})
				existing[norm] = true
			}
		}
	}
}

// Engine manages all torrent operations.
type Engine struct {
	client          *torrent.Client
	torrents        map[string]*ManagedTorrent
	pendingTorrents map[string]*torrent.Torrent // preview torrents that are not active downloads
	mu              sync.RWMutex
	cacheDir        string // internal raw piece storage
	exportDir       string // clean user-facing folder (Music/TorrentPlayer)
	db              *sql.DB
}

// NewEngine creates a new torrent engine and restores any previously active torrents from the database.
func NewEngine(cacheDir, exportDir string, db *sql.DB) (*Engine, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return nil, fmt.Errorf("create export dir: %w", err)
	}

	cfg := torrent.NewDefaultClientConfig()
	// Storage uses internal cacheDir so exportDir NEVER gets polluted with dummy 0-byte unselected files!
	cfg.DefaultStorage = storage.NewFile(cacheDir)
	cfg.NoDHT = false
	cfg.DisablePEX = false
	cfg.Seed = true

	client, err := torrent.NewClient(cfg)
	if err != nil {
		// Fallback to dynamic port if default port is in use
		cfg.ListenPort = 0
		client, err = torrent.NewClient(cfg)
		if err != nil {
			return nil, fmt.Errorf("create torrent client: %w", err)
		}
	}

	e := &Engine{
		client:          client,
		torrents:        make(map[string]*ManagedTorrent),
		pendingTorrents: make(map[string]*torrent.Torrent),
		cacheDir:        cacheDir,
		exportDir:       exportDir,
		db:              db,
	}

	if db != nil {
		e.initDB()
		e.restoreTorrents()
	}

	return e, nil
}

func (e *Engine) initDB() {
	schema := `
	CREATE TABLE IF NOT EXISTS managed_torrents (
		info_hash TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		magnet_uri TEXT NOT NULL DEFAULT '',
		selected_paths TEXT NOT NULL DEFAULT '[]',
		state TEXT NOT NULL DEFAULT 'downloading',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	DELETE FROM managed_torrents WHERE state = 'queued';
	`
	_, _ = e.db.Exec(schema)
}

func (e *Engine) saveTorrent(infoHash, name, magnetURI string, selectedPaths []string, state TorrentState) {
	if e.db == nil {
		return
	}
	pathsJSON, _ := json.Marshal(selectedPaths)
	_, _ = e.db.Exec(`
		INSERT INTO managed_torrents (info_hash, name, magnet_uri, selected_paths, state)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(info_hash) DO UPDATE SET
			name = CASE WHEN excluded.name != '' THEN excluded.name ELSE managed_torrents.name END,
			magnet_uri = CASE WHEN excluded.magnet_uri != '' THEN excluded.magnet_uri ELSE managed_torrents.magnet_uri END,
			selected_paths = excluded.selected_paths,
			state = excluded.state,
			created_at = CURRENT_TIMESTAMP
	`, infoHash, name, magnetURI, string(pathsJSON), string(state))
}

func (e *Engine) updateTorrentSelected(infoHash string, selectedPaths []string) {
	if e.db == nil {
		return
	}
	pathsJSON, _ := json.Marshal(selectedPaths)
	_, _ = e.db.Exec(`UPDATE managed_torrents SET selected_paths = ? WHERE info_hash = ?`, string(pathsJSON), infoHash)
}

func (e *Engine) updateTorrentState(infoHash string, state TorrentState) {
	if e.db == nil {
		return
	}
	_, _ = e.db.Exec(`UPDATE managed_torrents SET state = ? WHERE info_hash = ?`, string(state), infoHash)
}

func (e *Engine) deleteTorrent(infoHash string) {
	if e.db == nil {
		return
	}
	_, _ = e.db.Exec(`DELETE FROM managed_torrents WHERE info_hash = ?`, infoHash)
}

// saveTorrentMetainfo caches the .torrent file in cacheDir so future launches load metadata instantly.
func (e *Engine) saveTorrentMetainfo(t *torrent.Torrent) {
	if t == nil || t.Info() == nil {
		return
	}
	infoHash := t.InfoHash().HexString()
	metaPath := filepath.Join(e.cacheDir, infoHash+".torrent")
	if _, err := os.Stat(metaPath); err == nil {
		return // already cached
	}
	mi := t.Metainfo()
	f, err := os.Create(metaPath)
	if err != nil {
		return
	}
	defer f.Close()
	_ = mi.Write(f)
}

// syncExportToCache ensures that any completed files that exist in exportDir are also present in cacheDir,
// using hard links (or copies) so anacrolix/torrent can verify and seed existing pieces.
func (e *Engine) syncExportToCache(t *torrent.Torrent) {
	if t == nil || t.Info() == nil {
		return
	}
	for _, f := range t.Files() {
		fLen := f.Length()
		if fLen <= 0 {
			continue
		}
		dstInCache := filepath.Join(e.cacheDir, f.Path())
		srcInExport := filepath.Join(e.exportDir, f.Path())

		cacheFi, cacheErr := os.Stat(dstInCache)
		exportFi, exportErr := os.Stat(srcInExport)

		if (os.IsNotExist(cacheErr) || (cacheErr == nil && cacheFi.Size() < fLen)) &&
			(exportErr == nil && exportFi.Size() >= fLen) {
			_ = os.MkdirAll(filepath.Dir(dstInCache), 0o755)
			if err := os.Link(srcInExport, dstInCache); err != nil {
				_ = copyFile(srcInExport, dstInCache)
			}
		}
	}
}

func (e *Engine) restoreTorrents() {
	if e.db == nil {
		return
	}
	rows, err := e.db.Query(`SELECT info_hash, name, magnet_uri, selected_paths, state, COALESCE(created_at, CURRENT_TIMESTAMP) FROM managed_torrents ORDER BY created_at DESC`)
	if err != nil {
		return
	}
	defer rows.Close()

	type savedT struct {
		infoHash     string
		name         string
		magnetURI    string
		selectedJSON string
		state        string
		createdAt    string
	}
	var list []savedT
	for rows.Next() {
		var s savedT
		if err := rows.Scan(&s.infoHash, &s.name, &s.magnetURI, &s.selectedJSON, &s.state, &s.createdAt); err == nil {
			list = append(list, s)
		}
	}
	rows.Close()

	for _, item := range list {
		if item.magnetURI == "" || item.state == string(StateQueued) {
			continue
		}
		var paths []string
		_ = json.Unmarshal([]byte(item.selectedJSON), &paths)

		var addedAt time.Time
		if t, err := time.Parse("2006-01-02 15:04:05", item.createdAt); err == nil {
			addedAt = t
		} else if t, err := time.Parse(time.RFC3339, item.createdAt); err == nil {
			addedAt = t
		} else {
			addedAt = time.Now()
		}

		metaPath := filepath.Join(e.cacheDir, item.infoHash+".torrent")
		var t *torrent.Torrent
		if _, err := os.Stat(metaPath); err == nil {
			t, _ = e.client.AddTorrentFromFile(metaPath)
		}
		if t == nil {
			spec, specErr := torrent.TorrentSpecFromMagnetUri(item.magnetURI)
			if specErr == nil {
				injectPublicTrackers(spec)
				t, _, _ = e.client.AddTorrentSpec(spec)
			}
			if t == nil {
				var addErr error
				t, addErr = e.client.AddMagnet(item.magnetURI)
				if addErr != nil {
					continue
				}
			}
		}
		t.AddTrackers(defaultPublicTrackers)

		infoHash := t.InfoHash().HexString()
		mt := &ManagedTorrent{
			T:             t,
			InfoHash:      infoHash,
			Name:          item.name,
			MagnetURI:     item.magnetURI,
			State:         TorrentState(item.state),
			SelectedPaths: paths,
			AddedAt:       addedAt,
		}

		e.mu.Lock()
		e.torrents[infoHash] = mt
		e.mu.Unlock()

		go func(t *torrent.Torrent, mt *ManagedTorrent, savedState string) {
			select {
			case <-t.GotInfo():
			case <-time.After(90 * time.Second):
				return
			}

			e.saveTorrentMetainfo(t)

			if mt.Name == "" || mt.Name == mt.InfoHash {
				mt.Name = t.Name()
				e.saveTorrent(mt.InfoHash, t.Name(), mt.MagnetURI, mt.SelectedPaths, mt.State)
			}

			if savedState == string(StatePaused) {
				t.CancelPieces(0, t.NumPieces())
				return
			}

			// Ensure existing files are verified so pieces are marked complete for seeding
			e.syncExportToCache(t)
			t.VerifyData()
			t.AllowDataUpload()

			_ = e.DownloadSelectedFiles(mt.InfoHash, mt.SelectedPaths)
		}(t, mt, item.state)
	}
}

// GetDownloadDir returns the user-facing export directory.
func (e *Engine) GetDownloadDir() string {
	return e.exportDir
}

// AddMagnetPending adds a torrent for inspection/metadata resolution ONLY.
// If the torrent is already managed, it returns its file list without touching active transfers.
// Otherwise, it explicitly disallows data transfer, sets piece priorities to None,
// and holds the torrent in pendingTorrents without saving it to DB.
func (e *Engine) AddMagnetPending(magnetURI string) (string, []FileInfo, error) {
	spec, err := torrent.TorrentSpecFromMagnetUri(magnetURI)
	if err != nil {
		return "", nil, fmt.Errorf("add magnet: %w", err)
	}

	infoHash := spec.InfoHash.HexString()

	// If already in active managed torrents, simply return its current file list!
	e.mu.RLock()
	existingMt, exists := e.torrents[infoHash]
	e.mu.RUnlock()
	if exists && existingMt.T != nil && existingMt.T.Info() != nil {
		return infoHash, e.buildFileList(existingMt.T, infoHash, false), nil
	}

	spec.DisallowDataDownload = true
	spec.DisallowDataUpload = true
	injectPublicTrackers(spec)

	t, _, err := e.client.AddTorrentSpec(spec)
	if err != nil {
		return "", nil, fmt.Errorf("add magnet: %w", err)
	}

	t.AddTrackers(defaultPublicTrackers)
	t.DisallowDataDownload()
	t.DisallowDataUpload()

	select {
	case <-t.GotInfo():
	case <-time.After(35 * time.Second):
		t.Drop()
		return "", nil, fmt.Errorf("таймаут получения списка файлов (нет активных пиров для передачи метаданных)")
	}

	t.CancelPieces(0, t.NumPieces())
	for _, f := range t.Files() {
		f.SetPriority(torrent.PiecePriorityNone)
	}
	t.AllowDataDownload()
	t.AllowDataUpload()

	infoHash = t.InfoHash().HexString()

	e.mu.Lock()
	e.pendingTorrents[infoHash] = t
	e.mu.Unlock()

	return infoHash, e.buildFileList(t, infoHash, false), nil
}

// AddTorrentFilePending adds a .torrent file for inspection/metadata resolution ONLY.
func (e *Engine) AddTorrentFilePending(filePath string) (string, []FileInfo, error) {
	mi, err := metainfo.LoadFromFile(filePath)
	if err != nil {
		return "", nil, fmt.Errorf("load torrent file: %w", err)
	}

	infoHash := mi.HashInfoBytes().HexString()

	// If already in active managed torrents, simply return its current file list!
	e.mu.RLock()
	existingMt, exists := e.torrents[infoHash]
	e.mu.RUnlock()
	if exists && existingMt.T != nil && existingMt.T.Info() != nil {
		return infoHash, e.buildFileList(existingMt.T, infoHash, false), nil
	}

	spec := torrent.TorrentSpecFromMetaInfo(mi)
	spec.DisallowDataDownload = true
	spec.DisallowDataUpload = true
	injectPublicTrackers(spec)

	t, _, err := e.client.AddTorrentSpec(spec)
	if err != nil {
		return "", nil, fmt.Errorf("add torrent spec: %w", err)
	}

	t.AddTrackers(defaultPublicTrackers)
	t.DisallowDataDownload()
	t.DisallowDataUpload()

	select {
	case <-t.GotInfo():
	case <-time.After(15 * time.Second):
		t.Drop()
		return "", nil, fmt.Errorf("таймаут чтения .torrent")
	}

	t.CancelPieces(0, t.NumPieces())
	for _, f := range t.Files() {
		f.SetPriority(torrent.PiecePriorityNone)
	}
	t.AllowDataDownload()
	t.AllowDataUpload()

	infoHash = t.InfoHash().HexString()

	e.mu.Lock()
	e.pendingTorrents[infoHash] = t
	e.mu.Unlock()

	return infoHash, e.buildFileList(t, infoHash, false), nil
}

// CancelPendingTorrent drops a pending preview torrent without saving or downloading it.
func (e *Engine) CancelPendingTorrent(infoHash string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if t, ok := e.pendingTorrents[infoHash]; ok {
		delete(e.pendingTorrents, infoHash)
		t.Drop()
	}
}

// normalizeRelPath converts forward slashes and backslashes to system separator and cleans the path.
func normalizeRelPath(p string) string {
	p = strings.ReplaceAll(p, "/", string(filepath.Separator))
	p = strings.ReplaceAll(p, "\\", string(filepath.Separator))
	return filepath.Clean(p)
}

// DownloadSelectedFiles starts downloading only the specified file paths.
// Unselected files are set to PriorityNone.
// Any cover image residing in the folder of a selected file is automatically downloaded.
func (e *Engine) DownloadSelectedFiles(infoHash string, selectedPaths []string) error {
	e.mu.Lock()
	mt, ok := e.torrents[infoHash]
	if !ok {
		// Look up in pendingTorrents (promotes from preview to active download)
		t, pendingOk := e.pendingTorrents[infoHash]
		if !pendingOk {
			e.mu.Unlock()
			return fmt.Errorf("torrent %s not found", infoHash)
		}
		delete(e.pendingTorrents, infoHash)

		ih := t.InfoHash()
		magnet := ""
		if t.Info() != nil {
			magnet = t.Metainfo().Magnet(&ih, t.Info()).String()
		}

		mt = &ManagedTorrent{
			T:             t,
			InfoHash:      infoHash,
			Name:          t.Name(),
			MagnetURI:     magnet,
			State:         StateDownloading,
			SelectedPaths: selectedPaths,
			AddedAt:       time.Now(),
		}
		e.torrents[infoHash] = mt
	} else {
		// Merge newly selected paths with existing ones so earlier songs are not lost!
		existingSet := make(map[string]bool, len(mt.SelectedPaths)*2)
		for _, p := range mt.SelectedPaths {
			existingSet[p] = true
			existingSet[normalizeRelPath(p)] = true
		}
		for _, p := range selectedPaths {
			if !existingSet[p] && !existingSet[normalizeRelPath(p)] {
				mt.SelectedPaths = append(mt.SelectedPaths, p)
			}
		}
		mt.AddedAt = time.Now()
		mt.State = StateDownloading
	}
	e.mu.Unlock()

	// Enable data transfer on the torrent now that download has been confirmed
	mt.T.AllowDataDownload()
	mt.T.AllowDataUpload()
	mt.T.AddTrackers(defaultPublicTrackers)

	// Sync any already downloaded files from export to cache and verify pieces
	e.syncExportToCache(mt.T)
	mt.T.VerifyData()

	isDownloadAll := len(selectedPaths) == 0 && len(mt.SelectedPaths) == 0
	mt.DownloadAll = isDownloadAll

	selected := make(map[string]bool, len(mt.SelectedPaths)*3)
	selectedDirs := make(map[string]bool)
	for _, p := range mt.SelectedPaths {
		normP := normalizeRelPath(p)
		selected[p] = true
		selected[normP] = true
		selected[strings.ToLower(normP)] = true

		d := filepath.Clean(filepath.Dir(normP))
		dLower := strings.ToLower(d)
		selectedDirs[d] = true
		selectedDirs[dLower] = true

		// If track is in a sub-disc folder (e.g. CD1, CD2, Disc 1), also allow parent album folder
		baseD := strings.ToLower(filepath.Base(d))
		if strings.HasPrefix(baseD, "cd") || strings.HasPrefix(baseD, "disc") {
			parent := filepath.Clean(filepath.Dir(d))
			if parent != "." && parent != "/" && parent != "\\" {
				selectedDirs[parent] = true
				selectedDirs[strings.ToLower(parent)] = true
			}
		}
	}

	// Set selected files and matching covers to Normal and request pieces
	for _, f := range mt.T.Files() {
		fPath := f.Path()
		fNorm := normalizeRelPath(fPath)
		fDir := filepath.Dir(fNorm)
		fDirClean := filepath.Clean(fDir)
		fDirLower := strings.ToLower(fDirClean)

		isImage := isCoverImageFile(fPath)
		isCoverForSelected := isImage && (isDownloadAll || selectedDirs[fDir] || selectedDirs[fDirClean] || selectedDirs[fDirLower])

		isSelected := isDownloadAll || selected[fPath] || selected[fNorm] || selected[strings.ToLower(fNorm)]

		if isSelected || isCoverForSelected {
			f.SetPriority(torrent.PiecePriorityNormal)
			mt.T.DownloadPieces(f.BeginPieceIndex(), f.EndPieceIndex())
			f.Download() // Trigger active download
		} else {
			f.SetPriority(torrent.PiecePriorityNone)
		}
	}

	e.mu.Lock()
	mt.State = StateDownloading
	e.mu.Unlock()

	e.saveTorrent(infoHash, mt.Name, mt.MagnetURI, mt.SelectedPaths, StateDownloading)

	return nil
}

// buildFileList collects all (or only audio) files from a torrent into FileInfo slice.
func (e *Engine) buildFileList(t *torrent.Torrent, infoHash string, audioOnly bool) []FileInfo {
	var files []FileInfo

	e.mu.RLock()
	mt := e.torrents[infoHash]
	e.mu.RUnlock()

	isSelective := false
	selectedMap := make(map[string]bool)
	if mt != nil && len(mt.SelectedPaths) > 0 {
		isSelective = true
		for _, p := range mt.SelectedPaths {
			normP := normalizeRelPath(p)
			selectedMap[p] = true
			selectedMap[normP] = true
			selectedMap[strings.ToLower(normP)] = true
		}
	}

	for _, f := range t.Files() {
		if audioOnly && !isAudioFile(f.Path()) {
			continue
		}

		size := f.Length()
		completed := f.BytesCompleted()

		// Check if exported file exists on disk
		exportedPath := filepath.Join(e.exportDir, f.Path())
		isExported := false
		if fi, err := os.Stat(exportedPath); err == nil && fi.Size() > 0 {
			isExported = true
		}

		isComplete := isExported
		fNorm := normalizeRelPath(f.Path())
		isSelected := true
		if isSelective {
			isSelected = selectedMap[f.Path()] || selectedMap[fNorm] || selectedMap[strings.ToLower(fNorm)]
		}

		var pct float64
		if isComplete {
			pct = 100.0
		} else if size > 0 {
			pct = float64(completed) / float64(size) * 100.0
			if pct > 100 {
				pct = 100
			}
		}

		files = append(files, FileInfo{
			Path:           f.Path(),
			Name:           filepath.Base(f.Path()),
			Size:           size,
			InfoHash:       infoHash,
			BytesCompleted: completed,
			PercentDone:    pct,
			IsSelected:     isSelected,
			IsComplete:     isComplete,
		})
	}
	return files
}

// AddMagnet adds a torrent via magnet link and starts downloading all files.
func (e *Engine) AddMagnet(magnetURI string) (string, error) {
	spec, err := torrent.TorrentSpecFromMagnetUri(magnetURI)
	if err != nil {
		return "", fmt.Errorf("add magnet: %w", err)
	}
	injectPublicTrackers(spec)

	t, _, err := e.client.AddTorrentSpec(spec)
	if err != nil {
		return "", fmt.Errorf("add magnet: %w", err)
	}
	t.AddTrackers(defaultPublicTrackers)

	<-t.GotInfo()
	infoHash := t.InfoHash().HexString()

	e.mu.Lock()
	e.torrents[infoHash] = &ManagedTorrent{
		T:         t,
		InfoHash:  infoHash,
		Name:      t.Name(),
		MagnetURI: magnetURI,
		State:     StateDownloading,
		AddedAt:   time.Now(),
	}
	e.mu.Unlock()

	e.saveTorrentMetainfo(t)
	e.saveTorrent(infoHash, t.Name(), magnetURI, nil, StateDownloading)

	t.DownloadAll()
	return infoHash, nil
}

// AddTorrentFile adds a torrent from a .torrent file path.
func (e *Engine) AddTorrentFile(filePath string) (string, error) {
	t, err := e.client.AddTorrentFromFile(filePath)
	if err != nil {
		return "", fmt.Errorf("add torrent file: %w", err)
	}
	t.AddTrackers(defaultPublicTrackers)

	<-t.GotInfo()
	infoHash := t.InfoHash().HexString()
	ih := t.InfoHash()
	magnet := ""
	if t.Info() != nil {
		magnet = t.Metainfo().Magnet(&ih, t.Info()).String()
	}

	e.mu.Lock()
	e.torrents[infoHash] = &ManagedTorrent{
		T:         t,
		InfoHash:  infoHash,
		Name:      t.Name(),
		MagnetURI: magnet,
		State:     StateDownloading,
		AddedAt:   time.Now(),
	}
	e.mu.Unlock()

	e.saveTorrentMetainfo(t)
	e.saveTorrent(infoHash, t.Name(), magnet, nil, StateDownloading)

	t.DownloadAll()
	return infoHash, nil
}

// OpenFileReader returns an io.ReadSeekCloser for audio streaming.
// It prioritizes the first pieces (headers) and the last piece (metadata/duration).
// It supports both active downloading torrents and pending preview torrents.
func (e *Engine) OpenFileReader(infoHash, fileName string) (io.ReadSeekCloser, error) {
	infoHashLower := strings.ToLower(infoHash)
	infoHashUpper := strings.ToUpper(infoHash)

	e.mu.RLock()
	var t *torrent.Torrent
	if mt, ok := e.torrents[infoHashLower]; ok && mt.T != nil {
		t = mt.T
	} else if mt, ok := e.torrents[infoHashUpper]; ok && mt.T != nil {
		t = mt.T
	} else if mt, ok := e.torrents[infoHash]; ok && mt.T != nil {
		t = mt.T
	} else if pt, ok := e.pendingTorrents[infoHashLower]; ok && pt != nil {
		t = pt
	} else if pt, ok := e.pendingTorrents[infoHashUpper]; ok && pt != nil {
		t = pt
	} else if pt, ok := e.pendingTorrents[infoHash]; ok && pt != nil {
		t = pt
	}
	e.mu.RUnlock()

	if t == nil {
		return nil, fmt.Errorf("torrent %s not found", infoHash)
	}

	// Make sure data transfer is allowed for streaming preview
	t.AllowDataDownload()
	t.AllowDataUpload()
	t.AddTrackers(defaultPublicTrackers)

	normTarget := normalizeRelPath(fileName)
	baseTarget := filepath.Base(normTarget)

	for _, f := range t.Files() {
		fNorm := normalizeRelPath(f.Path())
		fBase := filepath.Base(fNorm)

		if fNorm == normTarget || fBase == baseTarget || f.Path() == fileName || filepath.Base(f.Path()) == fileName {
			f.SetPriority(torrent.PiecePriorityNormal)
			f.Download()

			begin := f.BeginPieceIndex()
			end := f.EndPieceIndex()

			// Prioritize the initial audio chunks aggressively with PiecePriorityNow
			for i := begin; i < end && i < begin+8; i++ {
				t.Piece(i).SetPriority(torrent.PiecePriorityNow)
			}

			r := f.NewReader()
			r.SetResponsive()
			r.SetReadahead(2 * 1024 * 1024)
			return r, nil
		}
	}

	return nil, fmt.Errorf("file %q not found in torrent %s", fileName, infoHash)
}

// GetProgress returns download/upload progress accurately calculated for selected files,
// and automatically exports completed selected files to the user's Music/TorrentPlayer directory.
func (e *Engine) GetProgress() []DownloadProgress {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := make([]DownloadProgress, 0, len(e.torrents))
	now := time.Now()

	for _, mt := range e.torrents {
		name := mt.Name
		if mt.T != nil && mt.T.Info() != nil && mt.T.Name() != "" {
			name = mt.T.Name()
		}
		if name == "" {
			name = mt.InfoHash
		}

		if mt.AddedAt.IsZero() {
			mt.AddedAt = time.Now()
		}
		addedAt := mt.AddedAt.UnixMilli()

		selectedCount := len(mt.SelectedPaths)
		selectedTrack := ""
		if selectedCount == 1 {
			selectedTrack = filepath.Base(mt.SelectedPaths[0])
		}

		if mt.T == nil || mt.T.Info() == nil {
			result = append(result, DownloadProgress{
				InfoHash:      mt.InfoHash,
				Name:          name,
				Downloaded:    0,
				Total:         0,
				PercentDone:   0,
				State:         "metadata",
				Seeders:       0,
				Peers:         0,
				Leechers:      0,
				AddedAt:       addedAt,
				SelectedCount: selectedCount,
				SelectedTrack: selectedTrack,
			})
			continue
		}

		e.saveTorrentMetainfo(mt.T)

		stats := mt.T.Stats()
		files := mt.T.Files()

		// Calculate speed deltas
		if !mt.lastSpeedSample.IsZero() {
			dt := now.Sub(mt.lastSpeedSample).Seconds()
			if dt >= 0.5 {
				readBytes := stats.BytesReadData.Int64()
				writeBytes := stats.BytesWrittenData.Int64()

				deltaRead := readBytes - mt.lastReadBytes
				deltaWrite := writeBytes - mt.lastWriteBytes
				if deltaRead < 0 {
					deltaRead = 0
				}
				if deltaWrite < 0 {
					deltaWrite = 0
				}

				mt.downloadSpeed = float64(deltaRead) / dt
				mt.uploadSpeed = float64(deltaWrite) / dt

				mt.lastReadBytes = readBytes
				mt.lastWriteBytes = writeBytes
				mt.lastSpeedSample = now
			}
		} else {
			mt.lastReadBytes = stats.BytesReadData.Int64()
			mt.lastWriteBytes = stats.BytesWrittenData.Int64()
			mt.lastSpeedSample = now
			mt.downloadSpeed = 0
			mt.uploadSpeed = 0
		}

		if mt.State == StatePaused {
			mt.downloadSpeed = 0
			mt.uploadSpeed = 0
		}

		if mt.State == StateQueued {
			continue
		}

		var total int64
		var downloaded int64

		isDownloadAll := mt.DownloadAll
		if len(mt.SelectedPaths) > 0 {
			isDownloadAll = false
		}

		selectedMap := make(map[string]bool, len(mt.SelectedPaths)*3)
		selectedDirs := make(map[string]bool)
		for _, p := range mt.SelectedPaths {
			normP := normalizeRelPath(p)
			selectedMap[p] = true
			selectedMap[normP] = true
			selectedMap[strings.ToLower(normP)] = true

			d := filepath.Clean(filepath.Dir(normP))
			dLower := strings.ToLower(d)
			selectedDirs[d] = true
			selectedDirs[dLower] = true

			// If track is in a sub-disc folder (e.g. CD1, CD2, Disc 1), also allow parent album folder
			baseD := strings.ToLower(filepath.Base(d))
			if strings.HasPrefix(baseD, "cd") || strings.HasPrefix(baseD, "disc") {
				parent := filepath.Clean(filepath.Dir(d))
				if parent != "." && parent != "/" && parent != "\\" {
					selectedDirs[parent] = true
					selectedDirs[strings.ToLower(parent)] = true
				}
			}
		}

		for _, f := range files {
			fPath := f.Path()
			fNorm := normalizeRelPath(fPath)
			fDir := filepath.Dir(fNorm)
			fDirClean := filepath.Clean(fDir)
			fDirLower := strings.ToLower(fDirClean)

			isImage := isCoverImageFile(fPath)
			isCoverForSelected := isImage && (isDownloadAll || selectedDirs[fDir] || selectedDirs[fDirClean] || selectedDirs[fDirLower])

			isSelected := isDownloadAll || selectedMap[fPath] || selectedMap[fNorm] || selectedMap[strings.ToLower(fNorm)]

			if isSelected || isCoverForSelected {
				fLen := f.Length()
				total += fLen
				fComp := f.BytesCompleted()

				// If the file already exists completed in exportDir (Music folder)
				dst := filepath.Join(e.exportDir, f.Path())
				if fi, statErr := os.Stat(dst); statErr == nil && fi.Size() >= fLen && fLen > 0 {
					fComp = fLen
				}

				downloaded += fComp

				// If 100% complete in cache, export it asynchronously
				if f.BytesCompleted() == fLen && fLen > 0 {
					if fi, statErr := os.Stat(dst); os.IsNotExist(statErr) || (statErr == nil && fi.Size() < fLen) {
						src := filepath.Join(e.cacheDir, f.Path())
						if srcFi, srcErr := os.Stat(src); srcErr == nil && srcFi.Size() >= fLen {
							go func(s, d string) {
								_ = copyFile(s, d)
							}(src, dst)
						}
					}
				}
			}
		}

		if total == 0 {
			info := mt.T.Info()
			if info != nil {
				total = info.TotalLength()
			}
			downloaded = mt.T.BytesCompleted()
		}

		var pct float64
		if total > 0 {
			pct = float64(downloaded) / float64(total) * 100
			if pct > 100 {
				pct = 100
			}
		}

		state := string(mt.State)
		if mt.State == StatePaused {
			state = string(StatePaused)
		} else if downloaded >= total && total > 0 {
			state = string(StateSeeding)
			mt.State = StateSeeding
			pct = 100.0
			mt.T.AllowDataUpload()
		} else {
			state = string(StateDownloading)
			mt.State = StateDownloading
		}

		connectedSeeders := stats.ConnectedSeeders
		activePeers := stats.ActivePeers
		leechers := activePeers - connectedSeeders
		if leechers < 0 {
			leechers = 0
		}

		result = append(result, DownloadProgress{
			InfoHash:      mt.InfoHash,
			Name:          name,
			Downloaded:    downloaded,
			Total:         total,
			PercentDone:   pct,
			DownloadSpeed: mt.downloadSpeed,
			UploadSpeed:   mt.uploadSpeed,
			Seeders:       connectedSeeders,
			Peers:         activePeers,
			Leechers:      leechers,
			State:         state,
			AddedAt:       addedAt,
			SelectedCount: selectedCount,
			SelectedTrack: selectedTrack,
		})
	}

	// Stable deterministic sorting: newest first (AddedAt DESC), tie-breaker by InfoHash
	sort.Slice(result, func(i, j int) bool {
		if result[i].AddedAt != result[j].AddedAt {
			return result[i].AddedAt > result[j].AddedAt
		}
		return result[i].InfoHash < result[j].InfoHash
	})

	return result
}

// GetFileDiskPath returns the absolute path on disk for a file.
// It checks the user export directory first, then cache directory.
func (e *Engine) GetFileDiskPath(infoHash, relPath string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 1. Check completed exported file in Music/TorrentPlayer
	exported := filepath.Join(e.exportDir, relPath)
	if fi, err := os.Stat(exported); err == nil && fi.Size() > 0 {
		return exported
	}

	// 2. Check internal cache
	cached := filepath.Join(e.cacheDir, relPath)
	if fi, err := os.Stat(cached); err == nil && fi.Size() > 0 {
		return cached
	}

	return exported
}

func copyFile(src, dst string) error {
	_ = os.MkdirAll(filepath.Dir(dst), 0o755)
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// ListFiles returns all audio files within a torrent.
func (e *Engine) ListFiles(infoHash string) ([]FileInfo, error) {
	e.mu.RLock()
	mt, ok := e.torrents[infoHash]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("torrent %s not found", infoHash)
	}

	if mt.T == nil || mt.T.Info() == nil {
		return []FileInfo{}, nil
	}

	return e.buildFileList(mt.T, infoHash, true), nil
}

// PauseTorrent pauses downloading and seeding for a torrent.
func (e *Engine) PauseTorrent(infoHash string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	mt, ok := e.torrents[infoHash]
	if !ok {
		return fmt.Errorf("torrent %s not found", infoHash)
	}
	mt.T.CancelPieces(0, mt.T.NumPieces())
	mt.T.DisallowDataDownload()
	mt.T.DisallowDataUpload()
	mt.downloadSpeed = 0
	mt.uploadSpeed = 0
	mt.State = StatePaused
	e.updateTorrentState(infoHash, StatePaused)
	return nil
}

// ResumeTorrent resumes downloading and seeding for a torrent.
func (e *Engine) ResumeTorrent(infoHash string) error {
	e.mu.Lock()
	mt, ok := e.torrents[infoHash]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("torrent %s not found", infoHash)
	}
	mt.T.AllowDataDownload()
	mt.T.AllowDataUpload()
	selectedPaths := make([]string, len(mt.SelectedPaths))
	copy(selectedPaths, mt.SelectedPaths)
	e.mu.Unlock()

	return e.DownloadSelectedFiles(infoHash, selectedPaths)
}

// RemoveTorrent removes a torrent. If deleteFiles is true, downloaded files in exportDir are deleted.
// The internal cache in cacheDir is always cleaned up to prevent discarded pieces from lingering.
func (e *Engine) RemoveTorrent(infoHash string, deleteFiles bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	mt, ok := e.torrents[infoHash]
	if !ok {
		return fmt.Errorf("torrent %s not found", infoHash)
	}
	name := mt.T.Name()
	mt.T.Drop()
	delete(e.torrents, infoHash)
	e.deleteTorrent(infoHash)

	if name != "" {
		_ = os.RemoveAll(filepath.Join(e.cacheDir, name))
	}
	if deleteFiles && name != "" {
		_ = os.RemoveAll(filepath.Join(e.exportDir, name))
	}
	return nil
}

// AddFileToTorrent begins downloading an individual file in a torrent.
func (e *Engine) AddFileToTorrent(infoHash, filePath string) error {
	e.mu.Lock()
	mt, ok := e.torrents[infoHash]
	e.mu.Unlock()

	if !ok {
		return fmt.Errorf("torrent %s not found", infoHash)
	}

	normPath := normalizeRelPath(filePath)
	normLower := strings.ToLower(normPath)

	// Add to SelectedPaths if not already present
	found := false
	for _, p := range mt.SelectedPaths {
		if p == filePath || normalizeRelPath(p) == normPath || strings.ToLower(normalizeRelPath(p)) == normLower {
			found = true
			break
		}
	}
	if !found {
		mt.SelectedPaths = append(mt.SelectedPaths, filePath)
	}

	// Make sure data transfer is allowed
	mt.T.AllowDataDownload()
	mt.T.AllowDataUpload()

	// Prioritize and start downloading this file and any cover in same folder
	fDirLower := strings.ToLower(filepath.Clean(filepath.Dir(normPath)))

	e.mu.Lock()
	for _, f := range mt.T.Files() {
		fNorm := normalizeRelPath(f.Path())
		if f.Path() == filePath || fNorm == normPath || strings.ToLower(fNorm) == normLower {
			f.SetPriority(torrent.PiecePriorityNormal)
			mt.T.DownloadPieces(f.BeginPieceIndex(), f.EndPieceIndex())
			f.Download()
		} else if isCoverImageFile(f.Path()) {
			cDirLower := strings.ToLower(filepath.Clean(filepath.Dir(fNorm)))
			if cDirLower == fDirLower {
				f.SetPriority(torrent.PiecePriorityNormal)
				mt.T.DownloadPieces(f.BeginPieceIndex(), f.EndPieceIndex())
				f.Download()
			}
		}
	}

	mt.State = StateDownloading
	e.mu.Unlock()

	e.updateTorrentSelected(infoHash, mt.SelectedPaths)
	e.updateTorrentState(infoHash, StateDownloading)

	return nil
}

// RemoveFileFromTorrent cancels downloading of a file and deletes both exported and cached copies.
func (e *Engine) RemoveFileFromTorrent(infoHash, filePath string) error {
	e.mu.Lock()
	mt, ok := e.torrents[infoHash]
	e.mu.Unlock()

	if !ok {
		return fmt.Errorf("torrent %s not found", infoHash)
	}

	normPath := normalizeRelPath(filePath)
	normLower := strings.ToLower(normPath)

	// If SelectedPaths was empty (all files), populate it with all other files except this one
	if len(mt.SelectedPaths) == 0 {
		var otherPaths []string
		for _, f := range mt.T.Files() {
			fNorm := normalizeRelPath(f.Path())
			if f.Path() != filePath && fNorm != normPath && strings.ToLower(fNorm) != normLower {
				otherPaths = append(otherPaths, f.Path())
			}
		}
		mt.SelectedPaths = otherPaths
	} else {
		var updated []string
		for _, p := range mt.SelectedPaths {
			pNorm := normalizeRelPath(p)
			if p != filePath && pNorm != normPath && strings.ToLower(pNorm) != normLower {
				updated = append(updated, p)
			}
		}
		mt.SelectedPaths = updated
	}

	for _, f := range mt.T.Files() {
		fNorm := normalizeRelPath(f.Path())
		if f.Path() == filePath || fNorm == normPath || strings.ToLower(fNorm) == normLower {
			f.SetPriority(torrent.PiecePriorityNone)
			f.Cancel()
			dst := filepath.Join(e.exportDir, f.Path())
			_ = os.Remove(dst)
			cached := filepath.Join(e.cacheDir, f.Path())
			_ = os.Remove(cached)
			break
		}
	}

	e.updateTorrentSelected(infoHash, mt.SelectedPaths)

	return nil
}

// RemoveFileFromAllTorrents removes a file from any active torrents: unselects, cancels, deletes from disk and cache.
func (e *Engine) RemoveFileFromAllTorrents(filePath, fileName string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	base := filepath.Base(filePath)
	if base == "." || base == "/" || base == "\\" {
		base = fileName
	}

	for infoHash, mt := range e.torrents {
		if mt.T == nil {
			continue
		}
		found := false
		for _, f := range mt.T.Files() {
			fBase := filepath.Base(f.Path())
			if f.Path() == filePath || fBase == base || (fileName != "" && fBase == fileName) {
				found = true
				f.SetPriority(torrent.PiecePriorityNone)
				f.Cancel()

				cached := filepath.Join(e.cacheDir, f.Path())
				_ = os.Remove(cached)

				exported := filepath.Join(e.exportDir, f.Path())
				_ = os.Remove(exported)
				break
			}
		}

		if found {
			if len(mt.SelectedPaths) == 0 {
				var remaining []string
				for _, f := range mt.T.Files() {
					fBase := filepath.Base(f.Path())
					if f.Path() != filePath && fBase != base && (fileName == "" || fBase != fileName) {
						remaining = append(remaining, f.Path())
					}
				}
				mt.SelectedPaths = remaining
			} else {
				var remaining []string
				for _, p := range mt.SelectedPaths {
					pBase := filepath.Base(p)
					if p != filePath && pBase != base && (fileName == "" || pBase != fileName) {
						remaining = append(remaining, p)
					}
				}
				mt.SelectedPaths = remaining
			}

			e.updateTorrentSelected(infoHash, mt.SelectedPaths)
		}
	}
}

// Close shuts down the torrent engine gracefully.
func (e *Engine) Close() {
	e.client.Close()
}

// FileInfo describes an audio file within a torrent.
type FileInfo struct {
	Path           string  `json:"path"`
	Name           string  `json:"name"`
	Size           int64   `json:"size"`
	InfoHash       string  `json:"infoHash"`
	BytesCompleted int64   `json:"bytesCompleted"`
	PercentDone    float64 `json:"percentDone"`
	IsSelected     bool    `json:"isSelected"`
	IsComplete     bool    `json:"isComplete"`
}

// isAudioFile returns true if the file extension is a common audio format.
func isAudioFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	audioExts := map[string]bool{
		".mp3": true, ".flac": true, ".ogg": true,
		".wav": true, ".aac": true, ".m4a": true,
		".opus": true, ".ape": true, ".wv": true,
	}
	return audioExts[ext]
}

// isCoverImageFile returns true if the file is an image format that can serve as cover art.
func isCoverImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp"
}
