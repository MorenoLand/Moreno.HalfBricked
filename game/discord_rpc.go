package game

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// PORT ADDITION (not in the original game): Discord Rich Presence over the local Discord IPC socket. It speaks the
// documented handshake (opcode 0) and SET_ACTIVITY (opcode 1) frames itself, so it needs no library. The
// application id defaults to discordDefaultAppID and can be replaced with -discord-app-id. Presence is only sent
// while the "DISCORD RPC" extra is on and Discord is running; failures are quiet and retried every 30 s.
const discordDefaultAppID = "1558851518835466373"

var discordAppID = discordDefaultAppID

const (
	discordSendGap  = 15 * time.Second // Discord rate-limits SET_ACTIVITY to about once per 15 s
	discordRetryGap = 30 * time.Second
	discordTick     = 2 * time.Second
)

type discordRPC struct {
	mu      sync.Mutex
	enabled bool
	details string
	state   string
	started time.Time
	stop    chan struct{}
	running bool
}

// Configure turns presence on or off and starts the sender on first use.
func (d *discordRPC) Configure(enabled bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.enabled = enabled && discordAppID != "" && runtime.GOOS != "js"
	if d.enabled && !d.running {
		d.running = true
		d.stop = make(chan struct{})
		if d.started.IsZero() {
			d.started = time.Now()
		}
		go d.loop(d.stop)
	}
	if !d.enabled && d.running {
		close(d.stop)
		d.running = false
	}
}

// Set stores the activity text to show; the sender picks it up on its next tick.
func (d *discordRPC) Set(details, state string) {
	d.mu.Lock()
	d.details, d.state = details, state
	d.mu.Unlock()
}

func (d *discordRPC) snapshot() (details, state string, started time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.details, d.state, d.started
}

func (d *discordRPC) loop(stop chan struct{}) {
	var conn io.ReadWriteCloser
	var sentDetails, sentState string
	var lastSend, lastFail time.Time
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()
	ticker := time.NewTicker(discordTick)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		details, state, started := d.snapshot()
		if details == "" || (details == sentDetails && state == sentState && conn != nil) {
			continue
		}
		if time.Since(lastSend) < discordSendGap || time.Since(lastFail) < discordRetryGap {
			continue
		}
		if conn == nil {
			var err error
			if conn, err = discordConnect(discordAppID); err != nil {
				conn, lastFail = nil, time.Now()
				continue
			}
		}
		if err := discordSetActivity(conn, details, state, started); err != nil {
			conn.Close()
			conn, lastFail = nil, time.Now()
			continue
		}
		sentDetails, sentState, lastSend = details, state, time.Now()
	}
}

func discordSocketDirs() []string {
	var dirs []string
	for _, key := range []string{"XDG_RUNTIME_DIR", "TMPDIR", "TMP", "TEMP"} {
		if value := os.Getenv(key); value != "" {
			dirs = append(dirs, value, filepath.Join(value, "snap.discord"), filepath.Join(value, "app", "com.discordapp.Discord"))
		}
	}
	return append(dirs, "/tmp")
}

func discordDial(index int) (io.ReadWriteCloser, error) {
	if runtime.GOOS == "windows" {
		return os.OpenFile(fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, index), os.O_RDWR, 0)
	}
	var lastErr error = fmt.Errorf("no discord socket")
	for _, dir := range discordSocketDirs() {
		conn, err := net.DialTimeout("unix", filepath.Join(dir, fmt.Sprintf("discord-ipc-%d", index)), time.Second)
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func discordWriteFrame(w io.Writer, opcode uint32, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	header := make([]byte, 8)
	binary.LittleEndian.PutUint32(header[0:], opcode)
	binary.LittleEndian.PutUint32(header[4:], uint32(len(data)))
	_, err = w.Write(append(header, data...))
	return err
}

func discordReadFrame(r io.Reader) (uint32, []byte, error) {
	header := make([]byte, 8)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}
	size := binary.LittleEndian.Uint32(header[4:])
	if size > 1<<20 {
		return 0, nil, fmt.Errorf("discord frame of %d bytes", size)
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return 0, nil, err
	}
	return binary.LittleEndian.Uint32(header[0:]), data, nil
}

func discordConnect(appID string) (io.ReadWriteCloser, error) {
	var lastErr error
	for index := 0; index < 10; index++ {
		conn, err := discordDial(index)
		if err != nil {
			lastErr = err
			continue
		}
		if err := discordWriteFrame(conn, 0, map[string]any{"v": 1, "client_id": appID}); err != nil {
			conn.Close()
			lastErr = err
			continue
		}
		if _, _, err := discordReadFrame(conn); err != nil {
			conn.Close()
			lastErr = err
			continue
		}
		return conn, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("discord not running")
	}
	return nil, lastErr
}

func discordSetActivity(conn io.ReadWriteCloser, details, state string, started time.Time) error {
	activity := map[string]any{"details": details, "timestamps": map[string]any{"start": started.Unix()}}
	if state != "" {
		activity["state"] = state
	}
	frame := map[string]any{
		"cmd":   "SET_ACTIVITY",
		"args":  map[string]any{"pid": os.Getpid(), "activity": activity},
		"nonce": fmt.Sprintf("aoz-%d", time.Now().UnixNano()),
	}
	if err := discordWriteFrame(conn, 1, frame); err != nil {
		return err
	}
	_, _, err := discordReadFrame(conn)
	return err
}
