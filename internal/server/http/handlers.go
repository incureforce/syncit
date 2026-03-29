package serverhttp

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	serverdb "go-syncit/internal/server/db"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

type registerRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type tagsRequest struct {
	Tags []string `json:"tags"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req registerRequest
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		http.Error(w, "id required", http.StatusBadRequest)
		return
	}
	if err := s.db.RegisterClient(r.Context(), req.ID, req.Name); err != nil {
		http.Error(w, "register failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": req.ID})
}

func (s *Server) handleClientTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cid := clientIDFrom(r)
	if cid == "" {
		http.Error(w, "missing "+syncitClientHeader, http.StatusBadRequest)
		return
	}
	var req tagsRequest
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := s.db.SetClientTags(r.Context(), cid, req.Tags); err != nil {
		if err == serverdb.ErrNotFound {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMountsList(w http.ResponseWriter, r *http.Request) {
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
	rows, err := s.db.ListGlobalMounts(r.Context())
	if err != nil {
		http.Error(w, "list mounts failed", http.StatusInternalServerError)
		return
	}
	type m struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	out := make([]m, 0, len(rows))
	for _, row := range rows {
		out = append(out, m{ID: row.ID, Name: row.Name})
	}
	writeJSON(w, http.StatusOK, map[string]any{"mounts": out})
}

type putClientMountsRequest struct {
	Mounts []string `json:"mounts"`
}

func (s *Server) handleClientMounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cid := clientIDFrom(r)
	if cid == "" {
		http.Error(w, "missing "+syncitClientHeader, http.StatusBadRequest)
		return
	}
	var req putClientMountsRequest
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := s.db.SyncClientMounts(r.Context(), cid, req.Mounts); err != nil {
		if err == serverdb.ErrNotFound {
			http.Error(w, "unknown client", http.StatusUnauthorized)
			return
		}
		http.Error(w, "sync mounts failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
