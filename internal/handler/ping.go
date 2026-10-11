package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/korzhev/yp-shorter/internal/config/db"
	"github.com/korzhev/yp-shorter/internal/logger"
)

// PingHandler handles database health checks.
type PingHandler struct {
	// Pg provides the database connectivity check.
	Pg db.IPG
}

// PingHandlerFunc checks database connectivity with a three-second timeout.
// It responds with 200 on success or 500 when the check returns an error.
func (p PingHandler) PingHandlerFunc(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	err := p.Pg.PingContext(ctx)
	if err != nil {
		logger.Log.Errorw("DB problem", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ctx.Err() != nil {
		logger.Log.Errorw("DB ping timeout")
	}
	w.WriteHeader(http.StatusOK)
}
