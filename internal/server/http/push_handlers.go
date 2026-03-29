package serverhttp

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	serverdb "go-syncit/internal/server/db"
)

func clientIDFrom(r *http.Request) string {
	return r.Header.Get(syncitClientHeader)
}

const syncitClientHeader = "X-Syncit-Client-ID"

type pushFileBody struct {
	Path     string   `json:"path"`
	Tags     []string `json:"tags"`
	FileHash string   `json:"file_hash"`
	FileSize int64    `json:"file_size"`
}

func (s *Server) handleBlobUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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
	maxFile, _, _, err := s.db.Limits(r.Context())
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	limited := io.LimitReader(r.Body, maxFile+1)
	key, n, isNew, err := s.blobs.Put(limited)
	if err != nil {
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}
	if n > maxFile {
		_ = s.blobs.Remove(key)
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}
	if isNew {
		if err := s.db.TryCommitBlobUsage(r.Context(), n); err != nil {
			_ = s.blobs.Remove(key)
			if errors.Is(err, serverdb.ErrQuotaExceeded) {
				http.Error(w, "quota exceeded", http.StatusInsufficientStorage)
				return
			}
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"hash": key, "size": n})
}

func (s *Server) handleMountPushFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
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
	mountName := r.PathValue("mount")
	if mountName == "" {
		http.Error(w, "missing mount", http.StatusBadRequest)
		return
	}
	var req pushFileBody
	if err := readJSON(r, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.Path == "" || req.FileHash == "" || req.FileSize <= 0 {
		http.Error(w, "path, file_hash, file_size required", http.StatusBadRequest)
		return
	}
	if !s.blobs.Has(req.FileHash) {
		http.Error(w, "blob not found; upload blob first", http.StatusBadRequest)
		return
	}
	res, err := s.db.PushFileVersion(r.Context(), cid, mountName, req.Path, req.Tags, req.FileHash, req.FileSize, req.FileHash)
	if err != nil {
		if err == serverdb.ErrTagPolicy {
			http.Error(w, "tag policy violation", http.StatusForbidden)
			return
		}
		http.Error(w, "push failed", http.StatusInternalServerError)
		return
	}
	affected, err := s.db.ClientIDsAffectedByFilePush(r.Context(), mountName, req.Tags, cid)
	if err != nil {
		http.Error(w, "push failed", http.StatusInternalServerError)
		return
	}
	s.hub.PublishFileVersion(affected, mountName, req.Path, res.Version, req.FileHash)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "mount_file_id": res.MountFileID, "version": res.Version})
}

func (s *Server) handleBlobGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hash := r.PathValue("hash")
	if hash == "" || !s.blobs.Has(hash) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	f, err := s.blobs.Open(hash)
	if err != nil {
		http.Error(w, "open blob", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		http.Error(w, "stat blob", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(st.Size(), 10))
	_, _ = io.Copy(w, f)
}
