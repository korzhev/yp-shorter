package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/korzhev/yp-shorter/internal/model"
	"github.com/korzhev/yp-shorter/internal/service"
	"github.com/stretchr/testify/require"
)

type statsProviderFunc func(context.Context) (model.Stats, error)

func (f statsProviderFunc) GetStats(ctx context.Context) (model.Stats, error) { return f(ctx) }

func TestStatsHandler(t *testing.T) {
	for _, failed := range []bool{false, true} {
		r := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
		provider := statsProviderFunc(func(ctx context.Context) (model.Stats, error) {
			require.Equal(t, r.Context(), ctx)
			if failed {
				return model.Stats{}, errors.New("private database error")
			}
			return model.Stats{}, nil
		})
		h := StatsHandler{Service: service.StatsService{Repository: provider}}
		w := httptest.NewRecorder()
		h.StatsHandlerFunc(w, r)
		if failed {
			require.Equal(t, http.StatusInternalServerError, w.Code)
			require.NotContains(t, w.Body.String(), "private database error")
		} else {
			require.Equal(t, http.StatusOK, w.Code)
			require.JSONEq(t, `{"urls":0,"users":0}`, w.Body.String())
		}
	}
}
