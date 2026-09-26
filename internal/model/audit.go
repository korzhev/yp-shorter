package model

const (
	AuditActionFollow  = "follow"
	AuditActionShorten = "shorten"
)

type AuditLog struct {
	TS     int64  `json:"ts"`
	Action string `json:"action"`
	UserID string `json:"user_id"`
	URL    string `json:"url"`
}
