package main

import (
	"embed"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "Posluchac",
		Width:  1280,
		Height: 820,
		AssetServer: &assetserver.Options{
			Assets: assets,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/api/cover") {
					coverPath := r.URL.Query().Get("path")
					if coverPath != "" {
						if fi, err := os.Stat(coverPath); err == nil && !fi.IsDir() {
							ext := strings.ToLower(filepath.Ext(coverPath))
							switch ext {
							case ".jpg", ".jpeg":
								w.Header().Set("Content-Type", "image/jpeg")
							case ".png":
								w.Header().Set("Content-Type", "image/png")
							case ".webp":
								w.Header().Set("Content-Type", "image/webp")
							}
							w.Header().Set("Cache-Control", "public, max-age=86400")
							http.ServeFile(w, r, coverPath)
							return
						}
					}
					http.NotFound(w, r)
					return
				}
				http.NotFound(w, r)
			}),
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 18, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
