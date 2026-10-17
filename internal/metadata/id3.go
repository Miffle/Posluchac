// Package metadata reads ID3 tags and cover art from audio files.
package metadata

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bogem/id3v2/v2"
	"github.com/mewkiz/flac/meta"
)

// Tags holds extracted metadata from an audio file.
type Tags struct {
	Title       string
	Artist      string
	AlbumArtist string
	Album       string
	Year        int
	Genre       string
	// CoverData is the raw cover image bytes (JPEG or PNG), or nil if not found.
	CoverData []byte
	CoverMime string
}

// readFlacStream parses VorbisComment and Picture blocks from a FLAC stream.
func readFlacStream(r io.Reader) (*Tags, error) {
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, err
	}
	if string(magic[:]) != "fLaC" {
		return nil, errors.New("not a flac stream")
	}

	tags := &Tags{}
	for {
		block, err := meta.Parse(r)
		if err != nil {
			break
		}
		if comment, ok := block.Body.(*meta.VorbisComment); ok {
			for _, tag := range comment.Tags {
				k := strings.ToUpper(strings.TrimSpace(tag[0]))
				v := strings.TrimSpace(tag[1])
				switch k {
				case "TITLE":
					if tags.Title == "" {
						tags.Title = v
					}
				case "ARTIST":
					if tags.Artist == "" {
						tags.Artist = v
					}
				case "ALBUMARTIST", "ALBUM ARTIST", "BAND":
					tags.AlbumArtist = v
					if tags.Artist == "" {
						tags.Artist = v
					}
				case "ALBUM":
					if tags.Album == "" {
						tags.Album = v
					}
				case "GENRE":
					if tags.Genre == "" {
						tags.Genre = v
					}
				case "DATE", "YEAR", "ORIGINALDATE", "ORIGINALYEAR":
					if tags.Year == 0 {
						tags.Year = parseYear(v)
					}
				}
			}
		} else if pic, ok := block.Body.(*meta.Picture); ok {
			if pic.Type == 3 || len(tags.CoverData) == 0 {
				tags.CoverData = pic.Data
				tags.CoverMime = pic.MIME
			}
		}
		if block.IsLast {
			break
		}
	}
	return tags, nil
}

func parseYear(s string) int {
	for i := 0; i+4 <= len(s); i++ {
		sub := s[i : i+4]
		if (sub[0] == '1' || sub[0] == '2') && isDigit(sub[1]) && isDigit(sub[2]) && isDigit(sub[3]) {
			if y, err := strconv.Atoi(sub); err == nil && y >= 1900 && y <= 2100 {
				return y
			}
		}
	}
	return 0
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// ReadFile opens filePath and extracts ID3 or FLAC tags.
func ReadFile(filePath string) (*Tags, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == ".flac" {
		f, err := os.Open(filePath)
		if err == nil {
			defer f.Close()
			if t, err := readFlacStream(f); err == nil && t != nil {
				return t, nil
			}
		}
	}

	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err != nil {
		// Non-fatal: return empty tags rather than failing playback
		return &Tags{}, nil
	}
	defer tag.Close()

	return extractTags(tag), nil
}

// ReadReader reads ID3 or FLAC tags from an io.ReadSeeker.
func ReadReader(r io.ReadSeeker, ext string) (*Tags, error) {
	if strings.EqualFold(ext, ".flac") {
		_, _ = r.Seek(0, io.SeekStart)
		if t, err := readFlacStream(r); err == nil && t != nil {
			return t, nil
		}
		_, _ = r.Seek(0, io.SeekStart)
	}
	// id3v2 only works with *os.File directly, so write to temp file
	tmp, err := os.CreateTemp("", "tp-meta-*"+ext)
	if err != nil {
		return &Tags{}, fmt.Errorf("create temp file for metadata: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := io.Copy(tmp, r); err != nil {
		return &Tags{}, nil
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return &Tags{}, nil
	}

	return ReadFile(tmp.Name())
}

// SaveCover writes cover art to a .jpg file alongside the audio file.
// Returns the path to the saved cover, or "" if no cover was found.
func SaveCover(tags *Tags, audioFilePath string) (string, error) {
	if len(tags.CoverData) == 0 {
		return "", nil
	}

	ext := ".jpg"
	if strings.Contains(tags.CoverMime, "png") {
		ext = ".png"
	}

	coverPath := strings.TrimSuffix(audioFilePath, filepath.Ext(audioFilePath)) + "-cover" + ext
	if err := os.WriteFile(coverPath, tags.CoverData, 0o644); err != nil {
		return "", fmt.Errorf("save cover: %w", err)
	}
	return coverPath, nil
}

// extractTags pulls tag values out of an id3v2.Tag.
func extractTags(tag *id3v2.Tag) *Tags {
	t := &Tags{
		Title:  strings.TrimSpace(tag.Title()),
		Artist: strings.TrimSpace(tag.Artist()),
		Album:  strings.TrimSpace(tag.Album()),
		Genre:  strings.TrimSpace(tag.Genre()),
	}

	yearStr := strings.TrimSpace(tag.Year())
	if y, err := strconv.Atoi(yearStr); err == nil {
		t.Year = y
	}

	// Extract embedded cover (APIC frame)
	pics := tag.GetFrames(tag.CommonID("Attached picture"))
	for _, f := range pics {
		pic, ok := f.(id3v2.PictureFrame)
		if !ok {
			continue
		}
		// Prefer front cover (type 3), fall back to any picture
		if pic.PictureType == id3v2.PTFrontCover || t.CoverData == nil {
			t.CoverData = pic.Picture
			t.CoverMime = pic.MimeType
		}
	}

	return t
}
