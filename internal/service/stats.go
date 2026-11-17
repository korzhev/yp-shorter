package service

import (
	"context"

	"github.com/korzhev/yp-shorter/internal/model"
)

// StatsProvider retrieves totals for stored links and their owners.
type StatsProvider interface {
	GetStats(context.Context) (model.Stats, error)
}

// StatsService provides service-wide statistics from the repository.
type StatsService struct {
	Repository StatsProvider
}

// GetStats delegates statistics retrieval to the repository.
func (s StatsService) GetStats(ctx context.Context) (model.Stats, error) {
	return s.Repository.GetStats(ctx)
}
