package handler

import (
	"encoding/json"
	"net/http"

	"github.com/korzhev/yp-shorter/internal/service"
)

// StatsHandler serves service-wide statistics.
type StatsHandler struct {
	Service service.StatsProvider
}

// StatsHandlerFunc returns statistics as JSON, or 500 on a storage error.
func (s StatsHandler) StatsHandlerFunc(w http.ResponseWriter, r *http.Request) {
	stats, err := s.Service.GetStats(r.Context())
	if err != nil {
		http.Error(w, "Failed to retrieve statistics", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}
