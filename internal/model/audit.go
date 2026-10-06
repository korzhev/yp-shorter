package model

const (
	// AuditActionFollow identifies a redirect to an original URL.
	AuditActionFollow = "follow"
	// AuditActionShorten identifies a URL shortening operation.
	AuditActionShorten = "shorten"
)

// AuditLog is the JSON representation of a URL audit event.
type AuditLog struct {
	// TS is the event timestamp in Unix seconds.
	TS int64 `json:"ts"`
	// Action identifies the operation, such as follow or shorten.
	Action string `json:"action"`
	// UserID is the user's decimal ID represented as a string.
	UserID string `json:"user_id"`
	// URL is the original URL involved in the operation.
	URL string `json:"url"`
}
