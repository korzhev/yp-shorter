package service

import (
	"strconv"

	"github.com/korzhev/yp-shorter/internal/logger"
)

type AuditRepository interface {
	Save(action, userID, url string) error
}

type AuditPublisher struct {
	subs map[string]AuditRepository
}

func (p *AuditPublisher) Register(name string, ar AuditRepository) {
	if p.subs == nil {
		p.subs = make(map[string]AuditRepository)
	}
	p.subs[name] = ar
}

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
