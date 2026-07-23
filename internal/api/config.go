package api

import (
	"encoding/json"
	"net/http"
)

type configResponse struct {
	SlackOIDC bool `json:"slack_oidc"`
}

// handleConfig returns public app configuration used by the frontend before login.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(configResponse{SlackOIDC: s.slackOIDC})
}
