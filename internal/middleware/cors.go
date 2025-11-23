package middleware

import (
	"net/http"
	"os"
	"strings"
)

// CORSMiddleware handles CORS (Cross-Origin Resource Sharing) headers.
// It processes OPTIONS preflight requests and adds CORS headers to all responses.
func CORSMiddleware(next http.Handler) http.Handler {
	// Get allowed origins from environment variable, default to allow all for development
	allowedOrigins := getEnv("CORS_ALLOWED_ORIGINS", "*")
	allowedMethods := getEnv("CORS_ALLOWED_METHODS", "GET,POST,PUT,DELETE,OPTIONS,PATCH")
	allowedHeaders := getEnv("CORS_ALLOWED_HEADERS", "Content-Type,Authorization,X-Requested-With")
	allowCredentials := getEnv("CORS_ALLOW_CREDENTIALS", "true") == "true"

	// Parse allowed origins
	origins := strings.Split(allowedOrigins, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Determine the origin to allow
		allowedOrigin := determineAllowedOrigin(origin, origins)

		// Set CORS headers
		if allowedOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		}
		w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
		w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
		if allowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Set("Access-Control-Max-Age", "3600") // Cache preflight for 1 hour

		// Handle preflight OPTIONS request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Continue with the next handler
		next.ServeHTTP(w, r)
	})
}

// determineAllowedOrigin determines which origin to allow based on the request origin and configured allowed origins.
func determineAllowedOrigin(requestOrigin string, allowedOrigins []string) string {
	// If no origin in request, don't set CORS headers
	if requestOrigin == "" {
		return ""
	}

	// If wildcard is allowed and credentials are not required, allow all
	if len(allowedOrigins) == 1 && allowedOrigins[0] == "*" {
		return "*"
	}

	// Check if the request origin is in the allowed list
	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return "*"
		}
		if allowed == requestOrigin {
			return requestOrigin
		}
	}

	// If not found and not wildcard, return empty (will not set header)
	return ""
}

// getEnv gets an environment variable or returns a default value.
func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

