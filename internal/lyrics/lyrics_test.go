package lyrics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanTitle(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"01. Bohemian Rhapsody.mp3", "Bohemian Rhapsody"},
		{"02 - Track Title.flac", "Track Title"},
		{"05 Song.ogg", "Song"},
		{"Normal Song Title", "Normal Song Title"},
		{"  Spaces Before and After  ", "Spaces Before and After"},
		{"12. Something - Subtitle.wav", "Something - Subtitle"},
	}

	for _, tc := range tests {
		got := cleanTitle(tc.input)
		if got != tc.expected {
			t.Errorf("cleanTitle(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestLocalLRC(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lyrics_test_*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	audioPath := filepath.Join(tmpDir, "test_song.mp3")
	lrcPath := filepath.Join(tmpDir, "test_song.lrc")

	err = os.WriteFile(audioPath, []byte("fake mp3 data"), 0o644)
	if err != nil {
		t.Fatalf("WriteFile audio failed: %v", err)
	}

	sampleLRC := "[00:01.00] Line 1\n[00:05.50] Line 2\n"
	err = os.WriteFile(lrcPath, []byte(sampleLRC), 0o644)
	if err != nil {
		t.Fatalf("WriteFile lrc failed: %v", err)
	}

	client := NewClient()
	res, err := client.GetLyrics(context.Background(), "Artist", "Song", "Album", 180, audioPath, false)
	if err != nil {
		t.Fatalf("GetLyrics error: %v", err)
	}

	if !res.Found {
		t.Errorf("expected lyrics to be found from local file")
	}
	if res.Source != "local" {
		t.Errorf("expected source to be 'local', got %q", res.Source)
	}
	if res.SyncedLyrics != sampleLRC {
		t.Errorf("expected synced lyrics %q, got %q", sampleLRC, res.SyncedLyrics)
	}
}

func TestCache(t *testing.T) {
	client := NewClient()
	client.cacheMu.Lock()
	client.cache[cacheKey("Queen", "Radio Ga Ga")] = &Result{
		Found:        true,
		Artist:       "Queen",
		Title:        "Radio Ga Ga",
		SyncedLyrics: "[00:10.00] Radio",
		Source:       "lrclib",
	}
	client.cacheMu.Unlock()

	res, err := client.GetLyrics(context.Background(), "queen", "Radio Ga Ga", "", 0, "", false)
	if err != nil {
		t.Fatalf("GetLyrics cache test failed: %v", err)
	}
	if !res.Found || res.Title != "Radio Ga Ga" {
		t.Errorf("expected cached item, got %+v", res)
	}
}
