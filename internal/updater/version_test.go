package updater_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/torrent-player/internal/updater"
)

func findRepoRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "wails.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repository root containing wails.json")
		}
		dir = parent
	}
}

// TestVersionSynchronization verifies that the application version is identical
// across internal/updater/updater.go, frontend/package.json, and wails.json.
func TestVersionSynchronization(t *testing.T) {
	root := findRepoRoot(t)

	// 1. internal/updater/updater.go
	updaterVer := strings.TrimPrefix(strings.TrimSpace(updater.CurrentVersion), "v")
	if updaterVer == "" {
		t.Fatal("updater.CurrentVersion is empty")
	}

	// 2. frontend/package.json (or root package.json if present)
	pkgPath := filepath.Join(root, "frontend", "package.json")
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		pkgPath = filepath.Join(root, "package.json")
	}
	pkgData, err := os.ReadFile(pkgPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", pkgPath, err)
	}
	var pkgJSON struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(pkgData, &pkgJSON); err != nil {
		t.Fatalf("failed to parse %s: %v", pkgPath, err)
	}
	pkgVer := strings.TrimPrefix(strings.TrimSpace(pkgJSON.Version), "v")

	// 3. wails.json
	wailsPath := filepath.Join(root, "wails.json")
	wailsData, err := os.ReadFile(wailsPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", wailsPath, err)
	}
	var wailsJSON struct {
		Version string `json:"version"`
		Info    struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(wailsData, &wailsJSON); err != nil {
		t.Fatalf("failed to parse %s: %v", wailsPath, err)
	}
	wailsVer := strings.TrimPrefix(strings.TrimSpace(wailsJSON.Version), "v")
	if wailsVer == "" {
		wailsVer = strings.TrimPrefix(strings.TrimSpace(wailsJSON.Info.ProductVersion), "v")
	}

	t.Logf("Detected versions: updater.go=%q, package.json=%q, wails.json=%q", updaterVer, pkgVer, wailsVer)

	if pkgVer != updaterVer || wailsVer != updaterVer {
		t.Fatalf("Version mismatch detected:\n"+
			"  internal/updater/updater.go: %q\n"+
			"  frontend/package.json:       %q\n"+
			"  wails.json:                  %q\n"+
			"All three files must have matching versions!",
			updaterVer, pkgVer, wailsVer)
	}
}
