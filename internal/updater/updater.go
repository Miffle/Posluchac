// Package updater handles checking for updates from GitHub Releases
// and applying in-place updates on Windows.
package updater

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	CurrentVersion = "1.0.0"
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

	// Score candidates: prefer direct Posluchac.exe, then any .exe, then .zip
	var bestURL string
	var bestSize int64
	bestScore := -1

	for _, a := range rel.Assets {
		low := strings.ToLower(a.Name)
		score := 0
		if strings.HasSuffix(low, ".exe") {
			score += 1000
		} else if strings.HasSuffix(low, ".zip") {
			score += 500
		} else {
			continue
		}

		if low == "posluchac.exe" {
			score += 300
		} else if strings.Contains(low, "posluchac") {
			score += 100
		}

		if strings.Contains(low, "windows") || strings.Contains(low, "amd64") {
			score += 50
		}

		if score > bestScore {
			bestScore = score
			bestURL = a.DownloadURL
			bestSize = a.Size
		}
	}

	if bestURL == "" {
		bestURL = rel.HTMLURL
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
		DownloadURL:    bestURL,
		AssetSize:      bestSize,
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

// ApplyUpdate downloads the new executable (or extracts it from zip) and restarts the application on Windows.
func ApplyUpdate(downloadURL string) error {
	low := strings.ToLower(downloadURL)
	isExe := strings.HasSuffix(low, ".exe")
	isZip := strings.HasSuffix(low, ".zip")

	if !isExe && !isZip {
		_ = OpenURL(downloadURL)
		return fmt.Errorf("автоматическое обновление недоступно для этого формата файлов; открыта страница релиза в браузере")
	}

	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("определение пути программы: %w", err)
	}
	currentExe = filepath.Clean(currentExe)

	tempDir := os.TempDir()
	tempExe := filepath.Join(tempDir, fmt.Sprintf("Posluchac-update-%d.exe", time.Now().UnixNano()))

	// Download asset
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("создание запроса: %w", err)
	}
	req.Header.Set("User-Agent", "Posluchac/"+CurrentVersion)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("скачивание обновления: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("сервер вернул статус %d при скачивании", resp.StatusCode)
	}

	if isZip {
		tempZip := filepath.Join(tempDir, fmt.Sprintf("Posluchac-update-%d.zip", time.Now().UnixNano()))
		zipFile, err := os.Create(tempZip)
		if err != nil {
			return fmt.Errorf("создание временного zip-файла: %w", err)
		}
		if _, err := io.Copy(zipFile, resp.Body); err != nil {
			zipFile.Close()
			_ = os.Remove(tempZip)
			return fmt.Errorf("сохранение архива обновления: %w", err)
		}
		zipFile.Close()

		err = extractExeFromZip(tempZip, tempExe)
		_ = os.Remove(tempZip)
		if err != nil {
			return err
		}
	} else {
		out, err := os.Create(tempExe)
		if err != nil {
			return fmt.Errorf("создание временного exe: %w", err)
		}
		if _, err := io.Copy(out, resp.Body); err != nil {
			out.Close()
			_ = os.Remove(tempExe)
			return fmt.Errorf("сохранение исполняемого файла: %w", err)
		}
		out.Close()
	}

	// Create batch updater script
	currentPID := os.Getpid()
	targetDir := filepath.Dir(currentExe)
	batPath := filepath.Join(tempDir, fmt.Sprintf("update-posluchac-%d.bat", time.Now().UnixNano()))
	logPath := filepath.Join(tempDir, "posluchac-update.log")

	replacer := strings.NewReplacer(
		"{{PID}}", strconv.Itoa(currentPID),
		"{{TEMP_EXE}}", tempExe,
		"{{TARGET_EXE}}", currentExe,
		"{{TARGET_DIR}}", targetDir,
		"{{LOG_FILE}}", logPath,
	)

	batTemplate := `@echo off
setlocal enabledelayedexpansion
echo [%date% %time%] Starting Posluchac updater for PID {{PID}} > "{{LOG_FILE}}"

set /a attempts=0
:wait_pid
tasklist /fi "PID eq {{PID}}" 2>nul | findstr /i "{{PID}}" >nul
if not errorlevel 1 (
    timeout /t 1 /nobreak >nul
    set /a attempts+=1
    if !attempts! leq 10 goto wait_pid
    taskkill /F /PID {{PID}} >nul 2>&1
)

set /a copy_attempts=0
:retry_copy
copy /y "{{TEMP_EXE}}" "{{TARGET_EXE}}" >> "{{LOG_FILE}}" 2>&1
if errorlevel 1 (
    timeout /t 1 /nobreak >nul
    set /a copy_attempts+=1
    if !copy_attempts! leq 20 goto retry_copy
    echo [%date% %time%] ERROR: Failed to replace {{TARGET_EXE}} >> "{{LOG_FILE}}"
    exit /b 1
)

echo [%date% %time%] Successfully copied new binary >> "{{LOG_FILE}}"
del /f /q "{{TEMP_EXE}}" >nul 2>&1

cd /d "{{TARGET_DIR}}"
echo [%date% %time%] Starting updated Posluchac... >> "{{LOG_FILE}}"
start "" "{{TARGET_EXE}}"
del /f /q "%~f0"
`
	batContent := replacer.Replace(batTemplate)

	if err := os.WriteFile(batPath, []byte(batContent), 0o755); err != nil {
		return fmt.Errorf("создание скрипта обновления: %w", err)
	}

	cmd := exec.Command("cmd.exe", "/c", batPath)
	cmd.Dir = tempDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000 | 0x00000200, // CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("запуск процесса обновления: %w", err)
	}

	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}()

	return nil
}

// extractExeFromZip extracts Posluchac.exe (or first .exe found) from a zip archive.
func extractExeFromZip(zipPath, destExePath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("чтение zip-архива: %w", err)
	}
	defer r.Close()

	var targetFile *zip.File
	for _, f := range r.File {
		low := strings.ToLower(f.Name)
		if strings.HasSuffix(low, ".exe") {
			if strings.EqualFold(filepath.Base(low), "posluchac.exe") {
				targetFile = f
				break
			}
			if targetFile == nil {
				targetFile = f
			}
		}
	}

	if targetFile == nil {
		return fmt.Errorf("в архиве обновления не найден файл .exe")
	}

	rc, err := targetFile.Open()
	if err != nil {
		return fmt.Errorf("открытие .exe из архива: %w", err)
	}
	defer rc.Close()

	out, err := os.Create(destExePath)
	if err != nil {
		return fmt.Errorf("создание временного exe: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("распаковка .exe: %w", err)
	}

	return nil
}

// OpenURL opens the given URL in the default browser on Windows.
func OpenURL(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
