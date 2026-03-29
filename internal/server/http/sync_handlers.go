package serverhttp

import (
	"io"
	"net/http"
	"time"

	serverdb "go-syncit/internal/server/db"
)

type putSyncStateBody struct {
	SyncedAt *string `json:"synced_at"`
}

func (s *Server) handleClientSyncState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cid := clientIDFrom(r)
	if cid == "" {
		http.Error(w, "missing "+syncitClientHeader, http.StatusBadRequest)
		return
	}
	if _, err := s.db.GetClientTags(r.Context(), cid); err != nil {
		if err == serverdb.ErrNotFound {
			http.Error(w, "unknown client", http.StatusUnauthorized)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	var req putSyncStateBody
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	var at *time.Time
	if req.SyncedAt != nil && *req.SyncedAt != "" {
		t, err := time.Parse(time.RFC3339Nano, *req.SyncedAt)
		if err != nil {
			t, err = time.Parse(time.RFC3339, *req.SyncedAt)
			if err != nil {
				http.Error(w, "invalid synced_at", http.StatusBadRequest)
				return
			}
		}
		tu := t.UTC()
		at = &tu
	}
	if err := s.db.SetClientSyncedAt(r.Context(), cid, at); err != nil {
		if err == serverdb.ErrNotFound {
			http.Error(w, "unknown client", http.StatusNotFound)
			return
		}
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleClientEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cid := clientIDFrom(r)
	if cid == "" {
		http.Error(w, "missing "+syncitClientHeader, http.StatusBadRequest)
		return
	}
	if _, err := s.db.GetClientTags(r.Context(), cid); err != nil {
		if err == serverdb.ErrNotFound {
			http.Error(w, "unknown client", http.StatusUnauthorized)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	syncedAt, syncedOk, err := s.db.GetClientSyncedAt(r.Context(), cid)
	if err != nil {
		if err == serverdb.ErrNotFound {
			http.Error(w, "unknown client", http.StatusNotFound)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	replay, live, unsub := s.hub.Register(cid, syncedAt, syncedOk)
	defer unsub()

	writeEvent := func(data []byte) bool {
		if _, err := io.WriteString(w, "event: sync\ndata: "); err != nil {
			return false
		}
		if _, err := w.Write(data); err != nil {
			return false
		}
		if _, err := io.WriteString(w, "\n\n"); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	for _, data := range replay {
		if !writeEvent(data) {
			return
		}
	}

	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case data, ok := <-live:
			if !ok {
				return
			}
			if !writeEvent(data) {
				return
			}
		}
	}
}
