package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// dailyUpdateInserter is the write side of the daily update repository.
type dailyUpdateInserter interface {
	Insert(ctx context.Context, userID int64, text string) error
}

type slackHandler struct {
	signingSecret string
	pool          *pgxpool.Pool
	updates       dailyUpdateInserter
}

// NewSlackHandler constructs a slackHandler wired with the given dependencies.
func NewSlackHandler(signingSecret string, pool *pgxpool.Pool, updates dailyUpdateInserter) *slackHandler {
	return &slackHandler{signingSecret: signingSecret, pool: pool, updates: updates}
}

// handleEvent handles POST /api/slack/events.
// It verifies the Slack signing secret, then dispatches url_verification or event_callback.
func (h *slackHandler) handleEvent(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !h.verifySignature(r.Header, body) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var payload struct {
		Type      string          `json:"type"`
		Challenge string          `json:"challenge"`
		Event     json.RawMessage `json:"event"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	switch payload.Type {
	case "url_verification":
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(payload.Challenge))
	case "event_callback":
		h.dispatchEvent(w, r.Context(), payload.Event)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (h *slackHandler) dispatchEvent(w http.ResponseWriter, ctx context.Context, raw json.RawMessage) {
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

	userID, err := h.findUserBySlackID(ctx, ev.User)
	if err != nil {
		// Slack user not registered in the app — silently ignore.
		return
	}

	if err := h.updates.Insert(ctx, userID, ev.Text); err != nil {
		slog.Error("failed to save daily update", "slack_user", ev.User, "error", err)
	}
}

// findUserBySlackID looks up the internal user ID for a given Slack provider_id.
func (h *slackHandler) findUserBySlackID(ctx context.Context, slackID string) (int64, error) {
	const q = `SELECT user_id FROM user_identities WHERE provider = 'slack' AND provider_id = $1`
	var id int64
	err := h.pool.QueryRow(ctx, q, slackID).Scan(&id)
	return id, err
}

// verifySignature validates the Slack signing secret using HMAC-SHA256.
// Spec: https://api.slack.com/authentication/verifying-requests-from-slack
func (h *slackHandler) verifySignature(header http.Header, body []byte) bool {
	ts := header.Get("X-Slack-Request-Timestamp")
	sig := header.Get("X-Slack-Signature")
	if ts == "" || sig == "" {
		return false
	}
	base := "v0:" + ts + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(h.signingSecret))
	mac.Write([]byte(base))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}
