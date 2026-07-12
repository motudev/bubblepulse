// Package messaging defines the platform-agnostic contracts for inbound message handling.
package messaging

import "net/http"

// PlatformAdapter is implemented by each messaging platform (Slack, Teams, Discord, …).
// The api layer calls Route and Handler to register the platform's webhook endpoint
// without knowing anything about the platform's protocol.
type PlatformAdapter interface {
	// Route returns the method+path pattern for http.ServeMux registration,
	// e.g. "POST /api/slack/events".
	Route() string
	// Handler returns the http.HandlerFunc for this platform's webhook endpoint.
	// The handler is responsible for request authentication, payload parsing,
	// any platform-specific handshake (e.g. Slack url_verification), and calling
	// MessageService.Handle for each valid inbound message.
	Handler() http.HandlerFunc
}
