package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/korzhev/yp-shorter/internal/config/db"
	"github.com/korzhev/yp-shorter/internal/logger"
)

type PingHandler struct {
	Pg db.IPG
}

func (p PingHandler) PingHandlerFunc(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := p.Pg.PingContext(ctx)
	if err != nil {
		logger.Log.Errorw("DB problem", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	switch ctx.Err() {
	case context.DeadlineExceeded:
		logger.Log.Errorw("DB ping timeout")
	}
	w.WriteHeader(http.StatusOK)
}

