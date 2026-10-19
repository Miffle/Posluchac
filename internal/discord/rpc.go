package discord

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	npipe "gopkg.in/natefinch/npipe.v2"
)

const (
	// DefaultClientID is the Discord Application ID for Posluchac Rich Presence.
	DefaultClientID = "1341052684883394590"
	DefaultLogo     = "posluchac_logo"
	GithubURL       = "https://github.com/Miffle/Posluchac"
)

// TrackActivity describes the current playback state sent to Discord.
type TrackActivity struct {
	Title       string
	Artist      string
	Album       string
	CoverURL    string
	PositionSec float64
	DurationSec float64
}

// Client manages Discord Rich Presence communication over Windows IPC named pipes.
type Client struct {
	mu        sync.Mutex
	conn      net.Conn
	clientID  string
	enabled   bool
	connected bool
}

// NewClient initializes a new Discord RPC client with the given client ID.
func NewClient(clientID string) *Client {
	if clientID == "" {
		clientID = DefaultClientID
	}
	return &Client{
		clientID: clientID,
		enabled:  true,
	}
}

// SetEnabled toggles Discord RPC on or off. If turning off, clears presence and closes socket.
func (c *Client) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.enabled = enabled
	if !enabled && c.connected {
		_ = c.sendFrame(map[string]interface{}{
			"cmd": "SET_ACTIVITY",
			"args": map[string]interface{}{
				"pid":      os.Getpid(),
				"activity": nil,
			},
			"nonce": getNonce(),
		})
		_ = c.closeLocked()
	}
}

// IsEnabled returns true if Discord RPC is currently enabled.
func (c *Client) IsEnabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enabled
}

// Connect attempts to connect to the Discord IPC pipe.
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connectLocked()
}

func (c *Client) connectLocked() error {
	if !c.enabled {
		return nil
	}
	if c.connected && c.conn != nil {
		return nil
	}

	// Try pipes \\.\pipe\discord-ipc-0 through \\.\pipe\discord-ipc-9
	var conn net.Conn
	var err error
	for i := 0; i < 10; i++ {
		pipePath := fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i)
		conn, err = npipe.DialTimeout(pipePath, 500*time.Millisecond)
		if err == nil && conn != nil {
			break
		}
	}

	if conn == nil {
		c.connected = false
		return fmt.Errorf("discord named pipe not available")
	}

	c.conn = conn

	// Send handshake (opcode 0)
	handshake := map[string]interface{}{
		"v":         1,
		"client_id": c.clientID,
	}
	payload, err := json.Marshal(handshake)
	if err != nil {
		_ = c.closeLocked()
		return err
	}

	if err := c.writePacket(0, payload); err != nil {
		_ = c.closeLocked()
		return err
	}

	// Read handshake response
	buf := make([]byte, 1024)
	_ = c.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _ = c.conn.Read(buf)
	_ = c.conn.SetReadDeadline(time.Time{})

	c.connected = true
	return nil
}

// SetActivity updates Discord Rich Presence with current song title, artist, timeline, and button.
func (c *Client) SetActivity(act TrackActivity) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.enabled {
		return nil
	}

	if !c.connected || c.conn == nil {
		if err := c.connectLocked(); err != nil {
			return err
		}
	}

	details := act.Title
	if details == "" {
		details = "Музыкальный трек"
	}
	state := act.Artist
	if state == "" {
		state = "Posluchac"
	} else if act.Album != "" && act.Album != act.Title {
		state = fmt.Sprintf("%s — %s", act.Artist, act.Album)
	}

	// Discord requires details and state to be at least 2 characters
	if len([]rune(details)) < 2 {
		details = details + " "
	}
	if len([]rune(state)) < 2 {
		state = state + " "
	}
	// Limit max length to 127 characters
	if len([]rune(details)) > 127 {
		details = string([]rune(details)[:127])
	}
	if len([]rune(state)) > 127 {
		state = string([]rune(state)[:127])
	}

	activityObj := map[string]interface{}{
		"details": details,
		"state":   state,
		"assets": map[string]interface{}{
			"large_image": DefaultLogo,
			"large_text":  "Posluchac — Torrent Music Player",
			"small_image": "playing",
			"small_text":  "Воспроизведение",
		},
		"buttons": []map[string]interface{}{
			{
				"label": "Слушать в Posluchac",
				"url":   GithubURL,
			},
		},
	}

	if act.DurationSec > 0 {
		now := time.Now()
		start := now.Add(-time.Duration(act.PositionSec * float64(time.Second))).UnixMilli()
		end := start + int64(act.DurationSec*1000)
		activityObj["timestamps"] = map[string]interface{}{
			"start": start,
			"end":   end,
		}
	}

	frame := map[string]interface{}{
		"cmd": "SET_ACTIVITY",
		"args": map[string]interface{}{
			"pid":      os.Getpid(),
			"activity": activityObj,
		},
		"nonce": getNonce(),
	}

	if err := c.sendFrame(frame); err != nil {
		_ = c.closeLocked()
		return err
	}
	return nil
}

// ClearActivity removes Rich Presence activity when playback stops.
func (c *Client) ClearActivity() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return
	}

	frame := map[string]interface{}{
		"cmd": "SET_ACTIVITY",
		"args": map[string]interface{}{
			"pid":      os.Getpid(),
			"activity": nil,
		},
		"nonce": getNonce(),
	}

	_ = c.sendFrame(frame)
}

// Close closes the IPC connection cleanly.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.closeLocked()
}

func (c *Client) closeLocked() error {
	c.connected = false
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

func (c *Client) sendFrame(payloadObj interface{}) error {
	payload, err := json.Marshal(payloadObj)
	if err != nil {
		return err
	}
	return c.writePacket(1, payload)
}

func (c *Client) writePacket(opcode uint32, payload []byte) error {
	if c.conn == nil {
		return fmt.Errorf("connection is closed")
	}

	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, opcode); err != nil {
		return err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint32(len(payload))); err != nil {
		return err
	}
	if _, err := buf.Write(payload); err != nil {
		return err
	}

	_ = c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err := c.conn.Write(buf.Bytes())
	_ = c.conn.SetWriteDeadline(time.Time{})
	return err
}

func getNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
