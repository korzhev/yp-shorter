package service

import (
	"strconv"

	"github.com/korzhev/yp-shorter/internal/logger"
)

// AuditRepository persists or forwards audit events.
type AuditRepository interface {
	// Save records an action, decimal user ID, and original URL.
	Save(action, userID, url string) error
}

// AuditPublisher dispatches events to named audit repositories asynchronously.
// Its zero value is ready for registration. Register must not run concurrently
// with Register or Publish; repositories must support concurrent Save calls.
type AuditPublisher struct {
	subs map[string]AuditRepository
}

// Register adds ar under name, replacing any existing repository with that name.
func (p *AuditPublisher) Register(name string, ar AuditRepository) {
	if p.subs == nil {
		p.subs = make(map[string]AuditRepository)
	}
	p.subs[name] = ar
}

// Publish starts a goroutine for each registered repository to save the event.
// It does not wait for delivery; save errors are logged rather than returned.
func (p *AuditPublisher) Publish(action string, userID int, url string) {
	for _, ar := range p.subs {
		go func() {
			err := ar.Save(action, strconv.Itoa(userID), url)
			if err != nil {
				logger.Log.Errorf("Audit Error",
					"error", err,
					"action", action,
					"user_id", userID,
					"url", url,
				)
			}
		}()
	}
}
