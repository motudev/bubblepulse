// Package slack implements the messaging.PlatformAdapter for Slack Events API webhooks.
package slack

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/motudev/bubblepulse/internal/messaging"
)

// Adapter handles POST /api/slack/events: verifies the Slack signing secret,
// parses the event envelope, and delegates to MessageService for valid DMs.
type Adapter struct {
	signingSecret string
	svc           *messaging.MessageService
}

// NewAdapter constructs a Slack Adapter.
func NewAdapter(signingSecret string, svc *messaging.MessageService) *Adapter {
	return &Adapter{signingSecret: signingSecret, svc: svc}
}

// Route returns the mux pattern for this adapter's endpoint.
func (a *Adapter) Route() string { return "POST /api/slack/events" }

// Handler returns the http.HandlerFunc for the Slack Events API webhook.
func (a *Adapter) Handler() http.HandlerFunc { return a.handleEvent }

func (a *Adapter) handleEvent(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Warn("slack event: failed to read request body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !a.verifySignature(r.Header, body) {
		slog.Warn("slack event: signature verification failed")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var payload struct {
		Type      string          `json:"type"`
		Challenge string          `json:"challenge"`
		TeamID    string          `json:"team_id"`
		Event     json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Warn("slack event: failed to parse payload", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch payload.Type {
	case "url_verification":
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(payload.Challenge))
	case "event_callback":
		a.dispatchEvent(w, r.Context(), payload.TeamID, payload.Event)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (a *Adapter) dispatchEvent(w http.ResponseWriter, ctx context.Context, teamID string, raw json.RawMessage) {
	// Acknowledge immediately; Slack retries non-2xx responses.
	w.WriteHeader(http.StatusOK)

	var ev struct {
		Type        string `json:"type"`
		ChannelType string `json:"channel_type"`
		User        string `json:"user"`
		Text        string `json:"text"`
		BotID       string `json:"bot_id"`
		SubType     string `json:"subtype"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		slog.Warn("slack event: failed to parse inner event", "error", err)
		return
	}

	// Ignore anything that isn't a plain user DM.
	if ev.BotID != "" || ev.SubType == "bot_message" || ev.SubType == "message_changed" {
		return
	}
	if ev.Type != "message" || ev.ChannelType != "im" {
		return
	}
	if ev.User == "" || ev.Text == "" {
		return
	}

	err := a.svc.Handle(ctx, messaging.IncomingMessage{
		Provider:       "https://slack.com",
		PlatformUserID: ev.User,
		WorkspaceID:    teamID,
		Text:           ev.Text,
	})
	if err != nil {
		if errors.Is(err, messaging.ErrUserNotFound) || errors.Is(err, messaging.ErrOrgUnresolved) {
			slog.Warn("slack event: user not registered or unmapped", "slack_user", ev.User, "team_id", teamID, "error", err)
		} else {
			slog.Error("slack event: failed to handle message", "slack_user", ev.User, "error", err)
		}
	}
}

// verifySignature validates the Slack signing secret using HMAC-SHA256.
// Spec: https://api.slack.com/authentication/verifying-requests-from-slack
func (a *Adapter) verifySignature(header http.Header, body []byte) bool {
	ts := header.Get("X-Slack-Request-Timestamp")
	sig := header.Get("X-Slack-Signature")
	if ts == "" || sig == "" {
		return false
	}
	base := "v0:" + ts + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(a.signingSecret))
	mac.Write([]byte(base))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}
