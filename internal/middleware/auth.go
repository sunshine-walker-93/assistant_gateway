package middleware

import (
	"net/http"
	"os"
)

// APIKeyAuthMiddleware checks X-API-Key header against an environment variable.
// If GATEWAY_API_KEY is empty, this middleware is effectively disabled.
func APIKeyAuthMiddleware(next http.Handler) http.Handler {
	requiredKey := os.Getenv("GATEWAY_API_KEY")
	if requiredKey == "" {
		// Auth disabled, just pass through.
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for OPTIONS preflight requests
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get("X-API-Key")
		if key == "" || key != requiredKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}


