package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	metricsOnce     sync.Once
	reqCounter      *prometheus.CounterVec
	reqDurationHist *prometheus.HistogramVec
)

func initMetrics() {
	reqCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_http_requests_total",
			Help: "Total number of HTTP requests received by the gateway.",
		},
		[]string{"method", "path", "status"},
	)

	reqDurationHist = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_http_request_duration_seconds",
			Help:    "Histogram of HTTP request latencies.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	prometheus.MustRegister(reqCounter, reqDurationHist)
}

// MetricsMiddleware records basic Prometheus metrics for each HTTP request.
func MetricsMiddleware(next http.Handler) http.Handler {
	metricsOnce.Do(initMetrics)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		path := r.URL.Path
		status := ww.Status()

		reqCounter.WithLabelValues(r.Method, path, strconv.Itoa(status)).Inc()
		reqDurationHist.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
	})
}


