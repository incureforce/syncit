package serverhttp

import (
	"net/http"
	"os"
	"path/filepath"

	serverdb "go-syncit/internal/server/db"
	"go-syncit/internal/server/store/blob"
)

// Config holds server runtime options.
type Config struct {
	Addr    string
	DataDir string
}

// Server is the HTTP API and backing services.
type Server struct {
	db    *serverdb.ServerDB
	blobs *blob.Store
	hub   *EventHub
	mux   *http.ServeMux
	cfg   Config
}

// New creates a server with SQLite and blob storage under DataDir.
func New(cfg Config) (*Server, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(cfg.DataDir, "server.db")
	sdb, err := serverdb.OpenServer(dbPath)
	if err != nil {
		return nil, err
	}
	blobs, err := blob.New(filepath.Join(cfg.DataDir, "blobs"))
	if err != nil {
		_ = sdb.Close()
		return nil, err
	}
	s := &Server{
		db:    sdb,
		blobs: blobs,
		hub:   NewEventHub(),
		mux:   http.NewServeMux(),
		cfg:   cfg,
	}
	s.routes()
	return s, nil
}

// Close releases database resources.
func (s *Server) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	s.mux.HandleFunc("GET /v1/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	s.mux.HandleFunc("POST /v1/clients/register", s.handleRegister)
	s.mux.HandleFunc("PUT /v1/client/tags", s.handleClientTags)
	s.mux.HandleFunc("PUT /v1/client/mounts", s.handleClientMounts)
	s.mux.HandleFunc("PUT /v1/client/sync-state", s.handleClientSyncState)
	s.mux.HandleFunc("GET /v1/client/events", s.handleClientEvents)
	s.mux.HandleFunc("POST /v1/blobs", s.handleBlobUpload)
	s.mux.HandleFunc("GET /v1/blobs/{hash}", s.handleBlobGet)
	s.mux.HandleFunc("GET /v1/mounts", s.handleMountsList)
	s.mux.HandleFunc("POST /v1/mounts/{mount}/files", s.handleMountPushFile)
	s.mux.HandleFunc("GET /v1/mounts/{mount}/files", s.handleMountFilesLatest)
	s.mux.HandleFunc("GET /v1/mounts/{mount}/files/{path...}", s.handleMountFileLatest)
	s.mux.HandleFunc("GET /v1/shares", s.handleSharesNotImplemented)
	s.mux.HandleFunc("POST /v1/shares", s.handleSharesNotImplemented)
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Blobs returns the blob store (upload handlers in later milestones).
func (s *Server) Blobs() *blob.Store {
	return s.blobs
}

// DB returns the server database.
func (s *Server) DB() *serverdb.ServerDB {
	return s.db
}
