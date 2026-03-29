package serverhttp

import (
	"encoding/json"
	"sync"
	"time"
)

const maxEventBuffer = 4096

// storedEvent is a server-issued notification with wall time for SSE replay.
type storedEvent struct {
	at  time.Time
	raw []byte
}

// EventHub holds a bounded buffer of recent sync events and routes live events to SSE subscribers per client ID.
type EventHub struct {
	mu   sync.Mutex
	buf  []storedEvent
	subs map[string]map[chan []byte]struct{}
}

// NewEventHub creates an empty hub.
func NewEventHub() *EventHub {
	return &EventHub{subs: make(map[string]map[chan []byte]struct{})}
}

type fileVersionPayload struct {
	Kind       string `json:"kind"`
	Mount      string `json:"mount"`
	Path       string `json:"path"`
	Version    int    `json:"version"`
	FileHash   string `json:"file_hash"`
	OccurredAt string `json:"occurred_at"`
}

// PublishFileVersion records an event and delivers it to live subscribers for the given client IDs.
func (h *EventHub) PublishFileVersion(affected []string, mount, path string, version int, fileHash string) {
	at := time.Now().UTC()
	p := fileVersionPayload{
		Kind:       "file_version",
		Mount:      mount,
		Path:       path,
		Version:    version,
		FileHash:   fileHash,
		OccurredAt: at.Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return
	}
	se := storedEvent{at: at, raw: raw}

	h.mu.Lock()
	h.buf = append(h.buf, se)
	if len(h.buf) > maxEventBuffer {
		h.buf = h.buf[len(h.buf)-maxEventBuffer:]
	}

	var targets []chan []byte
	for _, cid := range affected {
		for ch := range h.subs[cid] {
			targets = append(targets, ch)
		}
	}
	h.mu.Unlock()

	payload := append([]byte(nil), raw...)
	for _, ch := range targets {
		select {
		case ch <- payload:
		default:
		}
	}
}

// Register prepares SSE replay and a live channel. If syncedOk is false, replay is empty (client must run a full pull first).
// The caller must invoke unsubscribe when the HTTP request ends.
func (h *EventHub) Register(clientID string, syncedAt time.Time, syncedOk bool) (replay [][]byte, live <-chan []byte, unsubscribe func()) {
	ch := make(chan []byte, 256)
	h.mu.Lock()
	if syncedOk {
		for _, e := range h.buf {
			if e.at.After(syncedAt) {
				replay = append(replay, append([]byte(nil), e.raw...))
			}
		}
	}
	if h.subs[clientID] == nil {
		h.subs[clientID] = make(map[chan []byte]struct{})
	}
	h.subs[clientID][ch] = struct{}{}
	h.mu.Unlock()

	return replay, ch, func() {
		h.mu.Lock()
		if m, ok := h.subs[clientID]; ok {
			delete(m, ch)
			if len(m) == 0 {
				delete(h.subs, clientID)
			}
		}
		h.mu.Unlock()
		close(ch)
	}
}
