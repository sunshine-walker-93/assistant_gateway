package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sunshine-walker-93/assistant_gateway/internal/config"
	"github.com/sunshine-walker-93/assistant_gateway/internal/grpcclient"
)

// RouteManager manages dynamic routes and their handlers.
type RouteManager struct {
	mu        sync.RWMutex
	router    *chi.Mux
	clientMgr *grpcclient.ClientManager
	configMgr *config.ConfigManager
	logger    *zap.Logger
}

// grpcErrorToHTTPStatus maps gRPC error codes to HTTP status codes.
func grpcErrorToHTTPStatus(err error) (int, string) {
	st, ok := status.FromError(err)
	if !ok {
		// Not a gRPC error, treat as internal server error
		return http.StatusInternalServerError, "internal error"
	}

	code := st.Code()
	message := st.Message()

	switch code {
	case codes.InvalidArgument:
		return http.StatusBadRequest, message
	case codes.NotFound:
		return http.StatusNotFound, message
	case codes.AlreadyExists:
		return http.StatusConflict, message
	case codes.PermissionDenied:
		return http.StatusForbidden, message
	case codes.Unauthenticated:
		return http.StatusUnauthorized, message
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests, message
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed, message
	case codes.Aborted:
		return http.StatusConflict, message
	case codes.OutOfRange:
		return http.StatusBadRequest, message
	case codes.Unimplemented:
		return http.StatusNotImplemented, message
	case codes.Internal:
		return http.StatusInternalServerError, message
	case codes.Unavailable:
		return http.StatusServiceUnavailable, message
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout, message
	case codes.Canceled:
		return http.StatusRequestTimeout, message
	case codes.Unknown:
		return http.StatusInternalServerError, message
	default:
		return http.StatusBadGateway, message
	}
}

// NewRouteManager creates a new RouteManager.
func NewRouteManager(r *chi.Mux, clientMgr *grpcclient.ClientManager, configMgr *config.ConfigManager, logger *zap.Logger) *RouteManager {
	return &RouteManager{
		router:    r,
		clientMgr: clientMgr,
		configMgr: configMgr,
		logger:    logger,
	}
}

// createHandler creates a handler for a route.
func (rm *RouteManager) createHandler(route config.RouteConfig) http.Handler {
	fullMethod := "/" + route.BackendService + "/" + route.BackendMethod
	timeout := time.Duration(route.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
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
			if len(data) > 0 {
				body = json.RawMessage(data)
			}
		}

		addr, ok := rm.configMgr.GetBackendAddr(route.BackendName)
		if !ok {
			rm.logger.Error("backend_not_found",
				zap.String("backend_name", route.BackendName),
			)
			http.Error(w, "backend not found", http.StatusBadGateway)
			return
		}

		// Always use InvokeJSON for zero-code configuration
		// This requires backend services to accept google.protobuf.Struct as request/response
		respJSON, err := rm.clientMgr.InvokeJSON(ctx, addr, fullMethod, body)
		if err != nil {
			// Map gRPC error to appropriate HTTP status code
			httpStatus, errorMsg := grpcErrorToHTTPStatus(err)

			rm.logger.Error("grpc_invoke_failed",
				zap.String("backend_name", route.BackendName),
				zap.String("backend_addr", addr),
				zap.String("full_method", fullMethod),
				zap.Int("http_status", httpStatus),
				zap.Error(err),
			)

			// Return error as JSON for better client experience
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(httpStatus)
			errorResp := map[string]string{
				"error": errorMsg,
			}
			if err := json.NewEncoder(w).Encode(errorResp); err != nil {
				rm.logger.Warn("write_error_response_failed", zap.Error(err))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if _, err := w.Write(respJSON); err != nil {
			rm.logger.Warn("write_response_failed", zap.Error(err))
		}
	})
}

// RegisterAllRoutes registers all routes from configuration.
// This method can be called multiple times to update routes.
func (rm *RouteManager) RegisterAllRoutes() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	cfg := rm.configMgr.GetConfig()
	for _, route := range cfg.Routes {
		handler := rm.createHandler(route)
		// Chi allows re-registering routes, which will update the handler
		rm.router.Method(route.HTTPMethod, route.HTTPPattern, handler)
	}
}

// RegisterRoutes wires public gateway routes for data forwarding.
// Admin APIs are handled by a separate management service.
// Returns the RouteManager for use in config change callbacks.
func RegisterRoutes(r *chi.Mux, logger *zap.Logger, configMgr *config.ConfigManager) (*RouteManager, error) {
	clientMgr := grpcclient.NewClientManager(logger)
	routeMgr := NewRouteManager(r, clientMgr, configMgr, logger)

	// Expose Prometheus metrics endpoint.
	r.Handle("/metrics", promhttp.Handler())

	// Register initial routes from configuration
	routeMgr.RegisterAllRoutes()

	return routeMgr, nil
}
