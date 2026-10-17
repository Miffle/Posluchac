// Package updater handles checking for updates from GitHub Releases
// and applying in-place updates on Windows.
package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	CurrentVersion = "1.0.1"
	RepoOwner      = "Miffle"
	RepoName       = "Posluchac"
)

type ReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

type GitHubRelease struct {
	TagName     string         `json:"tag_name"`
	Name        string         `json:"name"`
	Body        string         `json:"body"`
	HTMLURL     string         `json:"html_url"`
	PublishedAt time.Time      `json:"published_at"`
	Assets      []ReleaseAsset `json:"assets"`
}

type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
	ReleaseName    string `json:"releaseName"`
	ReleaseNotes   string `json:"releaseNotes"`
	ReleaseURL     string `json:"releaseUrl"`
	DownloadURL    string `json:"downloadUrl"`
	AssetSize      int64  `json:"assetSize"`
	PublishedAt    string `json:"publishedAt"`
}

// CheckForUpdates queries the GitHub API for the latest release.
func CheckForUpdates() (*UpdateInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", RepoOwner, RepoName)
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Posluchac/"+CurrentVersion)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса к GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No releases published yet
		return &UpdateInfo{
			CurrentVersion: CurrentVersion,
			LatestVersion:  CurrentVersion,
			HasUpdate:      false,
			ReleaseURL:     fmt.Sprintf("https://github.com/%s/%s/releases", RepoOwner, RepoName),
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API вернул статус %d", resp.StatusCode)
	}

	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("ошибка парсинга релиза: %w", err)
	}

	latestClean := strings.TrimPrefix(strings.TrimPrefix(rel.TagName, "v"), "V")
	hasUpdate := IsNewerVersion(latestClean, CurrentVersion)

	var downloadURL string
	var assetSize int64
	for _, a := range rel.Assets {
		low := strings.ToLower(a.Name)
		if strings.HasSuffix(low, ".exe") || strings.HasSuffix(low, ".zip") {
			downloadURL = a.DownloadURL
			assetSize = a.Size
			if strings.Contains(low, "posluchac") {
				break
			}
		}
	}
	if downloadURL == "" {
		downloadURL = rel.HTMLURL
	}

	publishedStr := ""
	if !rel.PublishedAt.IsZero() {
		publishedStr = rel.PublishedAt.Format("02.01.2006")
	}

	return &UpdateInfo{
		CurrentVersion: CurrentVersion,
		LatestVersion:  latestClean,
		HasUpdate:      hasUpdate,
		ReleaseName:    rel.Name,
		ReleaseNotes:   rel.Body,
		ReleaseURL:     rel.HTMLURL,
		DownloadURL:    downloadURL,
		AssetSize:      assetSize,
		PublishedAt:    publishedStr,
	}, nil
}

// IsNewerVersion returns true if latest is strictly newer than current.
func IsNewerVersion(latest, current string) bool {
	lParts := parseVersion(latest)
	cParts := parseVersion(current)

	for i := 0; i < len(lParts) || i < len(cParts); i++ {
		l := 0
		if i < len(lParts) {
			l = lParts[i]
		}
		c := 0
		if i < len(cParts) {
			c = cParts[i]
		}
		if l > c {
			return true
		}
		if l < c {
			return false
		}
	}
	return false
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(v), "v"), "V")
	if idx := strings.Index(v, "-"); idx != -1 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	var res []int
	for _, p := range parts {
		if n, err := strconv.Atoi(p); err == nil {
			res = append(res, n)
		} else {
			res = append(res, 0)
		}
	}
	return res
}

// ApplyUpdate downloads the new executable and restarts the application on Windows.
func ApplyUpdate(downloadURL string) error {
	if !strings.HasSuffix(strings.ToLower(downloadURL), ".exe") {
		return OpenURL(downloadURL)
	}

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("определение пути программы: %w", err)
	}

	tempDir := os.TempDir()
	tempExe := filepath.Join(tempDir, fmt.Sprintf("Posluchac-update-%d.exe", time.Now().UnixNano()))

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("скачивание обновления: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("сервер вернул статус %d при скачивании", resp.StatusCode)
	}

	out, err := os.Create(tempExe)
	if err != nil {
		return fmt.Errorf("создание временного файла: %w", err)
	}
	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		return fmt.Errorf("запись файла обновления: %w", err)
	}

	batPath := filepath.Join(tempDir, fmt.Sprintf("update-posluchac-%d.bat", time.Now().UnixNano()))
	batContent := fmt.Sprintf(`@echo off
timeout /t 1 /nobreak >nul
:retry
move /y "%s" "%s" >nul 2>&1
if errorlevel 1 (
    timeout /t 1 /nobreak >nul
    goto retry
)
start "" "%s"
del "%%~f0"
`, tempExe, currentExe, currentExe)

	if err := os.WriteFile(batPath, []byte(batContent), 0o755); err != nil {
		return fmt.Errorf("создание скрипта обновления: %w", err)
	}

	cmd := exec.Command("cmd.exe", "/c", batPath)
	cmd.Dir = tempDir
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("запуск обновления: %w", err)
	}

	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}()

	return nil
}

// OpenURL opens the given URL in the default browser on Windows.
func OpenURL(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
