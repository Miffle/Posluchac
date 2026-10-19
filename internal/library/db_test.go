package library

import (
	"path/filepath"
	"testing"
)

func TestUpdateTrackMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_library.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	track := Track{
		FilePath: "C:\\Music\\song.mp3",
		FileName: "song.mp3",
		Title:    "Original Title",
		Artist:   "Original Artist",
		Album:    "Original Album",
		Year:     2020,
		Genre:    "Pop",
	}

	savedID, err := db.AddTrack(track)
	if err != nil {
		t.Fatalf("AddTrack failed: %v", err)
	}

	// Update metadata
	err = db.UpdateTrackMetadata(savedID, "New Title", "New Artist", "New Album", 2024, "Rock")
	if err != nil {
		t.Fatalf("UpdateTrackMetadata failed: %v", err)
	}

	updated, err := db.GetTrack(savedID)
	if err != nil {
		t.Fatalf("GetTrack failed: %v", err)
	}

	if updated.Title != "New Title" {
		t.Errorf("expected Title 'New Title', got '%s'", updated.Title)
	}
	if updated.Artist != "New Artist" {
		t.Errorf("expected Artist 'New Artist', got '%s'", updated.Artist)
	}
	if updated.Album != "New Album" {
		t.Errorf("expected Album 'New Album', got '%s'", updated.Album)
	}
	if updated.Year != 2024 {
		t.Errorf("expected Year 2024, got %d", updated.Year)
	}
	if updated.Genre != "Rock" {
		t.Errorf("expected Genre 'Rock', got '%s'", updated.Genre)
	}
}
