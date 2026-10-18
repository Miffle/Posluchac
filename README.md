# 🎵 Posluchac

<p align="center">
  <strong>Modern desktop music player with integrated torrent search, instant streaming preview, and music library management.</strong>
</p>

<p align="center">
  <a href="README.md"><strong>English</strong></a> •
  <a href="README.ru.md"><strong>Русский</strong></a>
</p>

<p align="center">
  <a href="https://github.com/Miffle/Posluchac/releases/latest"><img src="https://img.shields.io/github/v/release/Miffle/Posluchac?color=1db954&style=flat-square" alt="Latest Release"></a>
  <img src="https://img.shields.io/badge/platform-Windows%2010%20%7C%2011-0078D6?style=flat-square&logo=windows" alt="Platform: Windows">
  <img src="https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat-square&logo=go" alt="Go">
  <img src="https://img.shields.io/badge/Wails-v2-DF1A29?style=flat-square" alt="Wails">
  <img src="https://img.shields.io/badge/Vue-3%20%2B%20TypeScript-42b883?style=flat-square&logo=vue.js" alt="Vue 3">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License: MIT"></a>
</p>

---

## 📖 Overview

**Posluchac** (*послушак*) is a lightweight, full-featured desktop audio application built with **Go (Wails v2)** and **Vue 3**. It allows you to search music across torrent trackers, preview and stream audio files inside torrents sequentially before downloading, save individual tracks or complete releases to your collection, and enjoy your offline library with rich metadata and album art.

---

## ✨ Features

### 🔍 Torrent Search & Multiple Sources
* **Modular Custom Sources**: Add and manage custom JSON torrent indexes and search providers (e.g. from [rutracker-music-links](https://github.com/Miffle/rutracker-music-links/tree/main)).
* **File Inspector**: View all files, audio formats (MP3, FLAC, OGG, WAV, M4A, AAC), and file sizes in a torrent before starting the download.

### ⚡ Instant Streaming Preview
* **Sequential Torrent Streaming**: Play songs straight from the torrent network without waiting for the whole torrent or even the whole track to finish downloading.
* **Local HTTP Streamer**: Custom internal streaming proxy that prioritizes audio chunk delivery to eliminate buffering stutter.

### 📥 Download & Seeding Management
* **Selective Downloading**: Choose exactly which songs to download from an album or discography.
* **Background Seeding & Rate Display**: Real-time download/upload speeds, active peer counter, and progress percentage.
* **Disk Cleanup Control**: Delete torrent entries with or without removing local files from disk.

### 📚 Music Library & Tag Inspection
* **Automatic Scanning**: Detects and imports audio files and embedded cover art from your music directory.
* **ID3 Metadata Extraction**: Reads artist, album, title, year, genre, duration, and embedded front covers.
* **Discography Organization**: Group tracks by artist, album, and singles/loose tracks.
* **Favorites & Playlists**: Mark favorite songs (❤️) and organize your collection into custom playlists.

### 🎛️ Audio Playback & Media Integration
* **Queue & Playback Modes**: Loop track, repeat playlist, or shuffle queue using Fisher-Yates randomization.
* **MediaSession API**: Hardware media keys support (Play, Pause, Next, Prev) and native Windows media overlay.
* **Fullscreen Cover Mode**: Immersive vinyl-style cover presentation with player controls (`Esc` to exit).

### 🔕 Native Windows System Tray
* **Minimize / Close to Tray**: Closes directly to the Windows notification area to keep music playing in the background.
* **Single / Double Click Restore**: Instant window restore and focus on left click.
* **Native Win32 Context Menu**: Right-click menu with **Open Posluchac** and **Exit** for a clean process shutdown without orphaned background tasks.

### 🌐 Internationalization & Updates
* **Multilingual UI**: Fully localized in English and Russian, switchable in Settings on the fly.
* **Built-in Auto-Updater**: In-app notification for new GitHub releases with one-click background upgrade and automatic relaunch.

---

## 📥 Installation

Posluchac is distributed as a **portable, standalone Windows executable**. No installation or administrator rights required.

1. Go to the [**Latest Release**](https://github.com/Miffle/Posluchac/releases/latest) page.
2. Download `Posluchac.exe` (or the `.zip` archive).
3. Place `Posluchac.exe` into any preferred folder (e.g. `C:\Users\<You>\AppData\Local\Programs\Posluchac` or your Desktop) and run it.

> [!NOTE]
> Posluchac requires **Windows 10 (version 1803+)** or **Windows 11** with the **Microsoft Edge WebView2 Runtime** (pre-installed on modern Windows systems).

---

## 📡 Music Sources (JSON)

Posluchac relies on external JSON sources to search and discover music releases.

A ready-to-use music catalog from RuTracker is available in the [**Miffle/rutracker-music-links**](https://github.com/Miffle/rutracker-music-links/tree/main) repository.

### How to Add a Source:
1. Open the [rutracker-music-links](https://github.com/Miffle/rutracker-music-links/tree/main) repository.
2. Open the JSON file (e.g. [`rutracker-music.json`](https://github.com/Miffle/rutracker-music-links/blob/main/rutracker-music.json)) and click the **Raw** button to get the raw URL (or copy the direct raw link).
   * Direct Raw link: `https://raw.githubusercontent.com/Miffle/rutracker-music-links/main/rutracker-music.json`
3. Launch Posluchac, switch to the **Search** page, and click the **"Sources"** button in the header.
4. Paste the copied link into the input field and click **"Add"**.
5. The index will sync into your local database and the releases will be instantly searchable!

> [!TIP]
> **Expand the Catalog via Pull Requests:**
> If you'd like to suggest new releases or expand the music library, you are encouraged to add them directly to the JSON file in [rutracker-music-links](https://github.com/Miffle/rutracker-music-links/tree/main) and submit a **Pull Request (PR)**!

---

## 🛠️ Building from Source

### Prerequisites
* **Go**: 1.23 or newer ([Download Go](https://go.dev/dl/))
* **Node.js**: 20 or newer with `npm` ([Download Node.js](https://nodejs.org/))
* **Wails CLI**: Install via Go:
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```
* **C / Windows SDK**: Optional (pure Go SQLite driver `modernc.org/sqlite` is used, CGO not required).

### Clone and Run in Development Mode
```bash
# Clone the repository
git clone https://github.com/Miffle/Posluchac.git
cd Posluchac

# Install frontend dependencies
cd frontend
npm install
cd ..

# Start Wails live development mode (hot-reloading enabled)
wails dev
```

### Production Build
To create a standalone, optimized `.exe`:
```bash
wails build -s -platform windows/amd64 -tags disable_libutp
```
The compiled executable will be located at:
```text
build/bin/Posluchac.exe
```

---

## ⚙️ Configuration & Data Storage

* **Settings & Database**: Stored in your user directory under:
  ```text
  %USERPROFILE%\.posluchac\
    ├── library.db       # SQLite database (tracks, playlists, favorites)
    ├── covers\          # Cached album artworks
    └── torrents\        # Torrent engine state & resume data
  ```
* **Music Folder**: Configurable in **Settings** (defaults to `%USERPROFILE%\Music\Posluchac` or `%USERPROFILE%\Downloads\Posluchac`).

---

## 🧩 Tech Stack

| Component | Technology | Description |
|---|---|---|
| **GUI Framework** | [Wails v2](https://wails.io/) | Lightweight Go + Webview bridge |
| **Frontend UI** | [Vue 3](https://vuejs.org/) + [TypeScript](https://www.typescriptlang.org/) | Reactive interface, Pinia state stores, Vite bundler |
| **Torrent Engine** | [anacrolix/torrent](https://github.com/anacrolix/torrent) | Pure Go BitTorrent client with sequential piece priority |
| **Audio Metadata** | [bogem/id3v2](https://github.com/bogem/id3v2), [dhowden/tag](https://github.com/dhowden/tag) | ID3v1/v2, FLAC, OGG, and MP4 tag extraction |
| **Storage** | [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | CGO-free embedded SQLite database |
| **System Tray** | Native Win32 Shell API (`Shell_NotifyIconW`) | High-DPI tray icon, window message loop, context menu |
| **CI / CD** | GitHub Actions | Automated release build, changelog generator, Discord embeds |

---

## 🤝 Contributing

Contributions, issues, and feature requests are welcome!
Feel free to open an issue or submit a Pull Request on [GitHub](https://github.com/Miffle/Posluchac).

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feat/AmazingFeature`)
3. Commit your Changes (`git commit -m "feat: add some AmazingFeature"`)
4. Push to the Branch (`git push origin feat/AmazingFeature`)
5. Open a Pull Request

---

## 📄 License

This project is licensed under the [MIT License](LICENSE).

<p align="center">Made with ❤️ for music lovers.</p>
