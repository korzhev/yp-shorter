package repository

import (
	"context"

	"github.com/korzhev/yp-shorter/internal/config"
	"github.com/korzhev/yp-shorter/internal/model"
)

// GetStats counts all stored links (including soft-deleted ones) and distinct owners.
func (s *ShortLinkDB) GetStats(ctx context.Context) (model.Stats, error) {
	var stats model.Stats
	if s.storageType == config.Database {
		err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*), COUNT(DISTINCT user_id) FROM short_links").Scan(&stats.URLs, &stats.Users)
		return stats, err
	}
	s.storage.RLock()
	defer s.storage.RUnlock()
	users := make(map[int]struct{})
	for _, link := range s.storage.M {
		users[link.UserID] = struct{}{}
	}
	stats.URLs = len(s.storage.M)
	stats.Users = len(users)
	return stats, nil
}
