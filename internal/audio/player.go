// Package audio provides audio playback using gopxl/beep.
// It supports streaming from an io.ReadSeekCloser (i.e., directly from a
// torrent file reader) without requiring the full file to be downloaded.
package audio

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/effects"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
	"github.com/gopxl/beep/vorbis"
	"github.com/gopxl/beep/wav"
)

const (
	sampleRate  = beep.SampleRate(44100)
	bufferSize  = 512
	seekStepSec = 10 // seconds to skip forward/backward
)

// PlayerState represents the current state of the audio player.
type PlayerState string

const (
	StatePlaying PlayerState = "playing"
	StatePaused  PlayerState = "paused"
	StateStopped PlayerState = "stopped"
)

// TrackInfo describes the currently playing track.
type TrackInfo struct {
	InfoHash  string `json:"infoHash"`
	FilePath  string `json:"filePath"`
	FileName  string `json:"fileName"`
	Title     string `json:"title,omitempty"`
	Artist    string `json:"artist,omitempty"`
	Album     string `json:"album,omitempty"`
	CoverPath string `json:"coverPath,omitempty"`
}

// Status is a snapshot of the player's current state (for the UI).
type Status struct {
	State       PlayerState `json:"state"`
	Track       *TrackInfo  `json:"track,omitempty"`
	PositionSec float64     `json:"positionSec"`
	DurationSec float64     `json:"durationSec"`
	Volume      float64     `json:"volume"` // 0.0 – 1.0
}

// Player manages audio playback.
type Player struct {
	mu           sync.Mutex
	state        PlayerState
	ctrl         *beep.Ctrl
	vol          *effects.Volume
	streamer     beep.StreamSeekCloser
	format       beep.Format
	track        *TrackInfo
	volume       float64 // 0.0 – 1.0, mapped to beep volume
	onTrackEnded func()
}

// NewPlayer initialises the speaker and returns a Player.
func NewPlayer() (*Player, error) {
	if err := speaker.Init(sampleRate, sampleRate.N(time.Second/10)); err != nil {
		return nil, fmt.Errorf("init speaker: %w", err)
	}
	return &Player{
		state:  StateStopped,
		volume: 0.8,
	}, nil
}

// SetOnTrackEnded sets a callback invoked when a track finishes playback.
func (p *Player) SetOnTrackEnded(cb func()) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onTrackEnded = cb
}

func (p *Player) playStreamerLocked() {
	var resampled beep.Streamer = p.streamer
	if p.format.SampleRate != sampleRate {
		resampled = beep.Resample(4, p.format.SampleRate, sampleRate, p.streamer)
	}

	p.ctrl = &beep.Ctrl{Streamer: resampled, Paused: false}
	p.vol = &effects.Volume{
		Streamer: p.ctrl,
		Base:     2,
		Volume:   p.dbVolume(),
		Silent:   p.volume <= 0.001,
	}

	speaker.Clear()
	speaker.Play(beep.Seq(p.vol, beep.Callback(func() {
		p.mu.Lock()
		p.state = StateStopped
		cb := p.onTrackEnded
		p.mu.Unlock()
		if cb != nil {
			go cb()
		}
	})))
}

// Play opens an audio stream from r and starts playback.
// ext must be the file extension (e.g. ".mp3", ".flac").
func (p *Player) Play(r io.ReadSeekCloser, ext string, track TrackInfo) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Stop any existing playback
	p.stopLocked()

	streamer, format, err := decode(r, ext)
	if err != nil {
		return fmt.Errorf("decode audio: %w", err)
	}

	p.streamer = streamer
	p.format = format
	p.track = &track
	p.state = StatePlaying

	p.playStreamerLocked()

	return nil
}

// Pause toggles pause/resume.
func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer == nil {
		return
	}

	if p.state == StateStopped {
		if p.streamer.Len() > 0 && p.streamer.Position() >= p.streamer.Len() {
			_ = p.streamer.Seek(0)
		}
		p.playStreamerLocked()
		p.state = StatePlaying
		return
	}

	if p.ctrl == nil {
		return
	}

	speaker.Lock()
	p.ctrl.Paused = !p.ctrl.Paused
	speaker.Unlock()

	if p.ctrl.Paused {
		p.state = StatePaused
	} else {
		p.state = StatePlaying
	}
}

// SeekTo seeks to position seconds from the start.
func (p *Player) SeekTo(seconds float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.streamer == nil || p.format.SampleRate == 0 {
		return nil
	}

	samplePos := p.format.SampleRate.N(time.Duration(seconds * float64(time.Second)))
	if samplePos < 0 {
		samplePos = 0
	}
	if maxLen := p.streamer.Len(); maxLen > 0 && samplePos > maxLen {
		samplePos = maxLen
	}

	if p.state == StateStopped {
		err := p.streamer.Seek(samplePos)
		if err != nil {
			return err
		}
		p.playStreamerLocked()
		p.state = StatePlaying
		return nil
	}

	wasPaused := p.state == StatePaused
	if !wasPaused && p.ctrl != nil {
		speaker.Lock()
		p.ctrl.Paused = true
		speaker.Unlock()
	}

	err := p.streamer.Seek(samplePos)

	if !wasPaused && p.ctrl != nil {
		speaker.Lock()
		p.ctrl.Paused = false
		speaker.Unlock()
	}

	return err
}

// SeekForward seeks forward by seekStepSec seconds.
func (p *Player) SeekForward() error {
	return p.SeekTo(p.PositionSec() + seekStepSec)
}

// SeekBackward seeks backward by seekStepSec seconds.
func (p *Player) SeekBackward() error {
	pos := p.PositionSec() - seekStepSec
	if pos < 0 {
		pos = 0
	}
	return p.SeekTo(pos)
}

// SetVolume sets volume in range [0.0, 1.0].
func (p *Player) SetVolume(v float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p.volume = v

	if p.vol != nil {
		speaker.Lock()
		p.vol.Volume = p.dbVolume()
		p.vol.Silent = v <= 0.001
		speaker.Unlock()
	}
}

// Stop stops playback and releases resources.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
}

// Status returns the current playback status.
func (p *Player) Status() Status {
	p.mu.Lock()
	defer p.mu.Unlock()

	s := Status{
		State:  p.state,
		Track:  p.track,
		Volume: p.volume,
	}

	if p.streamer != nil && p.format.SampleRate > 0 {
		speaker.Lock()
		pos := p.streamer.Position()
		length := p.streamer.Len()
		speaker.Unlock()

		s.PositionSec = p.format.SampleRate.D(pos).Seconds()
		s.DurationSec = p.format.SampleRate.D(length).Seconds()

		// If position reached or passed the end while state was playing, mark stopped
		if p.state == StatePlaying && length > 0 && pos >= length {
			p.state = StateStopped
			s.State = StateStopped
			cb := p.onTrackEnded
			if cb != nil {
				go cb()
			}
		}
	}

	return s
}

// PositionSec returns the current playback position in seconds (thread-safe).
func (p *Player) PositionSec() float64 {
	if p.streamer == nil || p.format.SampleRate == 0 {
		return 0
	}
	speaker.Lock()
	pos := p.streamer.Position()
	speaker.Unlock()
	return p.format.SampleRate.D(pos).Seconds()
}

// stopLocked stops playback. Must be called with p.mu held.
func (p *Player) stopLocked() {
	speaker.Clear()
	if p.streamer != nil {
		_ = p.streamer.Close()
		p.streamer = nil
	}
	p.ctrl = nil
	p.vol = nil
	p.track = nil
	p.state = StateStopped
}

// dbVolume converts a linear [0,1] volume to a beep Volume value (Base=2).
// Perceptual audio scaling uses cubic curve (gain = volume^3) so volume behaves naturally.
func (p *Player) dbVolume() float64 {
	if p.volume <= 0.001 {
		return -20 // effectively silent
	}
	// gain = volume^3 => log2(gain) = 3 * log2(volume)
	return 3.0 * math.Log2(p.volume)
}

// decode returns a StreamSeekCloser for the given reader and file extension.
func decode(r io.ReadSeekCloser, ext string) (beep.StreamSeekCloser, beep.Format, error) {
	switch strings.ToLower(filepath.Ext(ext)) {
	case ".mp3":
		return mp3.Decode(r)
	case ".flac":
		return DecodeFlac(r)
	case ".ogg":
		return vorbis.Decode(r)
	case ".wav":
		return wav.Decode(r)
	default:
		return nil, beep.Format{}, fmt.Errorf("unsupported audio format: %s", ext)
	}
}

// GetFileDuration returns the duration of an audio file in seconds, or 0 if unable to decode.
func GetFileDuration(filePath string) float64 {
	f, err := os.Open(filePath)
	if err != nil {
		return 0
	}
	defer f.Close()

	ext := filepath.Ext(filePath)
	streamer, format, err := decode(f, ext)
	if err != nil {
		return 0
	}
	defer streamer.Close()

	if format.SampleRate > 0 && streamer.Len() > 0 {
		return format.SampleRate.D(streamer.Len()).Seconds()
	}
	return 0
}
