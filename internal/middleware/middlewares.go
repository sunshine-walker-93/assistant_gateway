package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

// RegisterGlobalMiddlewares installs global middlewares on the router.
// It includes request ID, recover, and structured logging.
func RegisterGlobalMiddlewares(r interface {
	Use(middlewares ...func(http.Handler) http.Handler)
}, logger *zap.Logger) {
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(MetricsMiddleware)
	r.Use(RateLimitMiddleware(100, 100))
	r.Use(APIKeyAuthMiddleware)
	r.Use(NewLoggingMiddleware(logger))
}

// NewLoggingMiddleware returns a chi-compatible logging middleware.
func NewLoggingMiddleware(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			logger.Info("http_request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Int("bytes", ww.BytesWritten()),
				zap.Duration("latency", time.Since(start)),
				zap.String("request_id", middleware.GetReqID(r.Context())),
			)
		})
	}
}


