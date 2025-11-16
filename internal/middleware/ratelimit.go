package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimitMiddleware applies a simple IP-based rate limit.
// This is a best-effort in-memory limiter suitable for a single gateway instance.
func RateLimitMiddleware(rps int, burst int) func(http.Handler) http.Handler {
	if rps <= 0 {
		rps = 100
	}
	if burst <= 0 {
		burst = 100
	}

	var (
		mu       sync.Mutex
		limiters = make(map[string]*rate.Limiter)
	)

	getLimiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		if l, ok := limiters[ip]; ok {
			return l
		}
		l := rate.NewLimiter(rate.Limit(rps), burst)
		limiters[ip] = l
		return l
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			limiter := getLimiter(ip)
			if !limiter.Allow() {
				w.Header().Set("Retry-After", time.Second.String())
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}


