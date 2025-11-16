package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sunshine-walker-93/assistant_gateway/internal/config"
	ghttp "github.com/sunshine-walker-93/assistant_gateway/internal/http"
	"github.com/sunshine-walker-93/assistant_gateway/internal/middleware"
)

func main() {
	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := middleware.NewLogger()

	// Build router
	r := chi.NewRouter()
	middleware.RegisterGlobalMiddlewares(r, logger)

	// Register gateway HTTP handlers (public + admin)
	if err := ghttp.RegisterRoutes(r, logger, cfg); err != nil {
		log.Fatalf("failed to register routes: %v", err)
	}

	srv := &http.Server{
		Addr:         cfg.HTTP.ListenAddress,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Printf("gateway listening on %s", cfg.HTTP.ListenAddress)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
}


