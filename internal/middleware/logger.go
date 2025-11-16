package middleware

import "go.uber.org/zap"

// NewLogger creates a production-ready zap logger.
// In real deployments you may want to make this configurable.
func NewLogger() *zap.Logger {
	logger, err := zap.NewProduction()
	if err != nil {
		// Fallback to a no-op logger to avoid panics in middleware.
		return zap.NewNop()
	}
	return logger
}


