package main

import (
	"context"
	"embed"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	// quit is closed by the tray "Выйти" menu item to trigger a real shutdown.
	quit := make(chan struct{})

	err := wails.Run(&options.App{
		Title:  "Posluchac",
		Width:  1280,
		Height: 820,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "posluchac-music-player-single-instance-lock",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				app.showMainWindow()
			},
		},
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
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			// Start tray in its own goroutine (systray needs its own OS thread internally).
			go app.runTray(quit)
			// When "Выйти" is clicked in tray, perform a real shutdown.
			go func() {
				<-quit
				runtime.Quit(ctx)
			}()
		},
		OnShutdown: func(ctx context.Context) {
			setQuitting()
			app.shutdown(ctx)
		},
		// Hide to tray on window close instead of quitting; allow closing when user explicitly quits.
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			if getQuitting() {
				return false // Allow Wails to close and exit
			}
			runtime.WindowHide(ctx)
			return true // Prevent close, hide to tray instead
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
	os.Exit(0)
}
