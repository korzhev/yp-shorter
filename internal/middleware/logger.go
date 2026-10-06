package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"go.uber.org/zap"
)

// LoggerMiddleware wraps a handler with request and response logging.
type LoggerMiddleware func(next http.Handler) http.Handler

// NewLoggerMiddleware creates middleware that logs the request URI, method,
// response status, elapsed time, and response size using sugar.
func NewLoggerMiddleware(sugar *zap.SugaredLogger) LoggerMiddleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				rLogger := sugar.With(
					zap.String("uri", r.RequestURI),
					zap.Int("status", ww.Status()),
					zap.String("method", r.Method),
					zap.Duration("duration", time.Since(start)),
					zap.Int("size", ww.BytesWritten()),
				)
				rLogger.Infow("Req-Res")
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
