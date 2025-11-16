package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/sunshine-walker-93/assistant_gateway/internal/config"
	"github.com/sunshine-walker-93/assistant_gateway/internal/grpcclient"
)

// RegisterRoutes wires public gateway routes and simple admin endpoints.
func RegisterRoutes(r *chi.Mux, logger *zap.Logger, cfg *config.Config) error {
	clientMgr := grpcclient.NewClientManager(logger)

	// Expose Prometheus metrics endpoint.
	r.Handle("/metrics", promhttp.Handler())

	// Build backend name -> addr map for quick lookup.
	backendAddr := make(map[string]string, len(cfg.Backends))
	for _, b := range cfg.Backends {
		backendAddr[b.Name] = b.Addr
	}

	// Public dynamic routes based on configuration.
	for _, rt := range cfg.Routes {
		route := rt // capture

		fullMethod := "/" + route.BackendService + "/" + route.BackendMethod
		timeout := time.Duration(route.TimeoutMS) * time.Millisecond
		if timeout <= 0 {
			timeout = 5 * time.Second
		}

		r.Method(route.HTTPMethod, route.HTTPPattern, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()

			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			var body json.RawMessage
			if req.Body != nil {
				defer req.Body.Close()
				data, err := io.ReadAll(req.Body)
				if err != nil {
					http.Error(w, "failed to read body", http.StatusBadRequest)
					return
				}
				// If empty body, keep it nil.
				if len(data) > 0 {
					body = json.RawMessage(data)
				}
			}

			addr, ok := backendAddr[route.BackendName]
			if !ok {
				logger.Error("backend_not_found",
					zap.String("backend_name", route.BackendName),
				)
				http.Error(w, "backend not found", http.StatusBadGateway)
				return
			}
			var (
				respJSON []byte
				err      error
			)

			// If RequestType/ResponseType are configured, use strong-typed
			// proto-based invocation; otherwise fall back to Struct-based JSON.
			if route.RequestType != "" && route.ResponseType != "" {
				respJSON, err = clientMgr.InvokeProto(
					ctx,
					addr,
					fullMethod,
					body,
					route.RequestType,
					route.ResponseType,
				)
			} else {
				respJSON, err = clientMgr.InvokeJSON(ctx, addr, fullMethod, body)
			}
			if err != nil {
				logger.Error("grpc_invoke_failed",
					zap.String("backend_name", route.BackendName),
					zap.String("backend_addr", addr),
					zap.String("full_method", fullMethod),
					zap.Error(err),
				)
				http.Error(w, "backend error", http.StatusBadGateway)
				return
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if _, err := w.Write(respJSON); err != nil {
				logger.Warn("write_response_failed", zap.Error(err))
			}
		}))
	}

	// Simple admin API: list and modify configured routes (for panel or debugging).
	r.Get("/admin/routes", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(cfg.Routes); err != nil {
			logger.Warn("encode_admin_routes_failed", zap.Error(err))
		}
	})

	// Upsert route.
	r.Post("/admin/routes", func(w http.ResponseWriter, req *http.Request) {
		var rt config.RouteConfig
		if err := json.NewDecoder(req.Body).Decode(&rt); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		// Upsert by (method, pattern).
		replaced := false
		for i, existing := range cfg.Routes {
			if existing.HTTPMethod == rt.HTTPMethod && existing.HTTPPattern == rt.HTTPPattern {
				cfg.Routes[i] = rt
				replaced = true
				break
			}
		}
		if !replaced {
			cfg.Routes = append(cfg.Routes, rt)
		}

		if err := config.SaveConfig(cfg); err != nil {
			logger.Error("save_config_failed", zap.Error(err))
			http.Error(w, "failed to persist config", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// Delete route by (method, pattern) passed as query.
	r.Delete("/admin/routes", func(w http.ResponseWriter, req *http.Request) {
		method := req.URL.Query().Get("method")
		pattern := req.URL.Query().Get("pattern")
		if method == "" || pattern == "" {
			http.Error(w, "method and pattern query parameters are required", http.StatusBadRequest)
			return
		}

		newRoutes := make([]config.RouteConfig, 0, len(cfg.Routes))
		for _, existing := range cfg.Routes {
			if existing.HTTPMethod == method && existing.HTTPPattern == pattern {
				continue
			}
			newRoutes = append(newRoutes, existing)
		}
		cfg.Routes = newRoutes

		if err := config.SaveConfig(cfg); err != nil {
			logger.Error("save_config_failed", zap.Error(err))
			http.Error(w, "failed to persist config", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	return nil
}


