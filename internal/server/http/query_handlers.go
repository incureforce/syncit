package serverhttp

import (
	"net/http"
	"strings"

	serverdb "go-syncit/internal/server/db"
)

func (s *Server) handleMountFilesLatest(w http.ResponseWriter, r *http.Request) {
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
	mountName := r.PathValue("mount")
	if mountName == "" {
		http.Error(w, "missing mount", http.StatusBadRequest)
		return
	}
	entries, err := s.db.LatestVisibleFiles(r.Context(), cid, mountName)
	if err != nil {
		http.Error(w, "catalog failed", http.StatusInternalServerError)
		return
	}
	type fileJSON struct {
		MountName string   `json:"mount_name"`
		Path      string   `json:"path"`
		Tags      []string `json:"tags"`
		Version   int      `json:"version"`
		FileHash  string   `json:"file_hash"`
		FileSize  int64    `json:"file_size"`
		BlobKey   string   `json:"blob_key"`
	}
	files := make([]fileJSON, 0, len(entries))
	for _, e := range entries {
		files = append(files, fileJSON{
			MountName: e.MountName,
			Path:      e.Path,
			Tags:      e.FileTags,
			Version:   e.Version,
			FileHash:  e.FileHash,
			FileSize:  e.FileSize,
			BlobKey:   e.BlobKey,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (s *Server) handleMountFileLatest(w http.ResponseWriter, r *http.Request) {
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
	mountName := r.PathValue("mount")
	relPath := strings.TrimPrefix(r.PathValue("path"), "/")
	if mountName == "" || relPath == "" {
		http.Error(w, "mount and path required", http.StatusBadRequest)
		return
	}
	info, err := s.db.LatestVisibleFile(r.Context(), cid, mountName, relPath)
	if err != nil {
		if err == serverdb.ErrNotFound {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version":    info.Version,
		"file_hash":  info.FileHash,
		"file_size":  info.FileSize,
		"blob_key":   info.BlobKey,
		"mount_name": info.MountName,
		"path":       info.Path,
		"tags":       info.FileTags,
	})
}
