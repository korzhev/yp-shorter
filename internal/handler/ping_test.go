package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/korzhev/yp-shorter/internal/handler"
	"github.com/korzhev/yp-shorter/internal/logger"
	"github.com/korzhev/yp-shorter/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPingHandlerFunc(t *testing.T) {
	t.Run("successful ping", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pg := mocks.NewMockIPG(ctrl)
		pg.EXPECT().
			PingContext(gomock.Any()).
			DoAndReturn(func(ctx context.Context) error {
				_, hasDeadline := ctx.Deadline()
				require.True(t, hasDeadline, "PingContext must receive a context with a deadline")
				return nil
			})

		p := handler.PingHandler{Pg: pg}
		request := httptest.NewRequest(http.MethodGet, "/ping", nil)
		response := httptest.NewRecorder()

		p.PingHandlerFunc(response, request)

		assert.Equal(t, http.StatusOK, response.Code)
		assert.Empty(t, response.Body.String())
	})

	t.Run("database error", func(t *testing.T) {
		dbErr := errors.New("database is unavailable")
		ctrl := gomock.NewController(t)
		pg := mocks.NewMockIPG(ctrl)
		pg.EXPECT().
			PingContext(gomock.Any()).
			DoAndReturn(func(ctx context.Context) error {
				_, hasDeadline := ctx.Deadline()
				require.True(t, hasDeadline, "PingContext must receive a context with a deadline")
				return dbErr
			})

		p := handler.PingHandler{Pg: pg}
		request := httptest.NewRequest(http.MethodGet, "/ping", nil)
		response := httptest.NewRecorder()

		p.PingHandlerFunc(response, request)

		assert.Equal(t, http.StatusInternalServerError, response.Code)
		assert.Equal(t, dbErr.Error()+"\n", response.Body.String())
	})
}

func init() {
	// I don't want to do anything with logger singletone
	logger.InitLogger("error")
}
