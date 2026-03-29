package serverhttp

import "net/http"

// Share endpoints are reserved for a future phase; v1 returns 501 Not Implemented.
func (s *Server) handleSharesNotImplemented(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error": "shares are not implemented in this build (phase 2)",
	})
}
