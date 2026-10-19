package metadata

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bogem/id3v2/v2"
)

// WriteFileTags writes audio metadata (title, artist, album, year, genre) directly
// to the audio file on disk when physical disk writing is requested.
func WriteFileTags(filePath, title, artist, album string, year int, genre string) error {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".mp3" {
		tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
		if err != nil {
			return fmt.Errorf("open mp3: %w", err)
		}
		defer tag.Close()

		tag.SetTitle(title)
		tag.SetArtist(artist)
		tag.SetAlbum(album)
		if year > 0 {
			tag.SetYear(strconv.Itoa(year))
		}
		tag.SetGenre(genre)

		if err := tag.Save(); err != nil {
			return fmt.Errorf("save id3 tags: %w", err)
		}
		return nil
	}

	return nil
}
