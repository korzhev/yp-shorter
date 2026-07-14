package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestNewLoggerMiddleware(t *testing.T) {
	t.Run("calls next handler and writes response", func(t *testing.T) {
		core, logs := observer.New(zapcore.InfoLevel)
		sugar := zap.New(core).Sugar()

		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusCreated)
			_, err := w.Write([]byte("created"))
			require.NoError(t, err)
		})

		req := httptest.NewRequest(http.MethodPost, "/shorten?foo=bar", nil)
		rr := httptest.NewRecorder()

		NewLoggerMiddleware(sugar)(next).ServeHTTP(rr, req)

		assert.True(t, nextCalled)
		assert.Equal(t, http.StatusCreated, rr.Code)
		assert.Equal(t, "created", rr.Body.String())

		entries := logs.All()
		require.Len(t, entries, 1)
		assert.Equal(t, "Req-Res", entries[0].Message)
	})

	t.Run("logs request and response fields", func(t *testing.T) {
		core, logs := observer.New(zapcore.InfoLevel)
		sugar := zap.New(core).Sugar()

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			_, err := w.Write([]byte("accepted"))
			require.NoError(t, err)
		})

		req := httptest.NewRequest(http.MethodPut, "/api/links/abc?verbose=true", nil)
		rr := httptest.NewRecorder()

		NewLoggerMiddleware(sugar)(next).ServeHTTP(rr, req)

		entries := logs.All()
		require.Len(t, entries, 1)

		context := entries[0].ContextMap()
		assert.Equal(t, "/api/links/abc?verbose=true", context["uri"])
		assert.Equal(t, int64(http.StatusAccepted), context["status"])
		assert.Equal(t, http.MethodPut, context["method"])
		assert.Equal(t, int64(len("accepted")), context["size"])
		assert.Contains(t, context, "duration")
	})

	t.Run("logs zero status and zero size when handler does not write", func(t *testing.T) {
		core, logs := observer.New(zapcore.InfoLevel)
		sugar := zap.New(core).Sugar()

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		NewLoggerMiddleware(sugar)(next).ServeHTTP(rr, req)

		entries := logs.All()
		require.Len(t, entries, 1)

		context := entries[0].ContextMap()
		assert.Equal(t, int64(0), context["status"])
		assert.Equal(t, int64(0), context["size"])
	})
}
