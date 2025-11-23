package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/sunshine-walker-93/assistant_gateway/internal/config"
	ghttp "github.com/sunshine-walker-93/assistant_gateway/internal/http"
	"github.com/sunshine-walker-93/assistant_gateway/internal/middleware"
)

func main() {
	// Get database connection parameters from environment (same as account service)
	host := getEnv("DB_HOST", "127.0.0.1")
	port := getEnv("DB_PORT", "3306")
	name := getEnv("DB_NAME", "assistant_gateway_db")
	user := getEnv("DB_USER", "assistant")
	password := getEnv("DB_PASSWORD", "123456")

	// Build DSN string (same format as account service)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=Local",
		user, password, host, port, name,
	)

	// Get poll interval (default: 5 seconds)
	pollInterval := 5 * time.Second
	if pollIntervalStr := os.Getenv("GATEWAY_CONFIG_POLL_INTERVAL"); pollIntervalStr != "" {
		if interval, err := strconv.Atoi(pollIntervalStr); err == nil && interval > 0 {
			pollInterval = time.Duration(interval) * time.Second
		}
	}

	// Get HTTP listen address
	listenAddr := getEnv("GATEWAY_HTTP_LISTEN", ":8080")

	logger := middleware.NewLogger()

	// Create MySQL store
	store, err := config.NewMySQLStore(dsn)
	if err != nil {
		log.Fatalf("failed to create mysql store: %v", err)
	}

	// Build router
	r := chi.NewRouter()
	middleware.RegisterGlobalMiddlewares(r, logger)

	// Create config manager with polling
	var routeMgr *ghttp.RouteManager
	configMgr, err := config.NewConfigManager(store, pollInterval, func(cfg *config.Config) {
		// Callback when config changes - reload routes
		logger.Info("configuration_changed",
			zap.Int("backends", len(cfg.Backends)),
			zap.Int("routes", len(cfg.Routes)),
		)
		if routeMgr != nil {
			// Reload routes when configuration changes
			routeMgr.RegisterAllRoutes()
			logger.Info("routes_reloaded")
		}
	})
	if err != nil {
		log.Fatalf("failed to create config manager: %v", err)
	}
	defer configMgr.Close()

	// Register gateway HTTP handlers (data forwarding only, no admin API)
	routeMgr, err = ghttp.RegisterRoutes(r, logger, configMgr)
	if err != nil {
		log.Fatalf("failed to register routes: %v", err)
	}

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Printf("gateway listening on %s (config poll interval: %v)", listenAddr, pollInterval)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down gateway...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
