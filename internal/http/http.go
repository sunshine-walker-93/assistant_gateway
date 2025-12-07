package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
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

		// For routes with short timeout (< 10s), extend to 15s to accommodate first-time
		// gRPC reflection queries which can be slow. This is a compatibility measure.
		actualTimeout := timeout
		if timeout < 10*time.Second {
			actualTimeout = 15 * time.Second
		}

		ctx, cancel := context.WithTimeout(ctx, actualTimeout)
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

		// Only try streaming for routes that explicitly indicate streaming (e.g., /stream in path)
		// This avoids unnecessary reflection queries for unary methods
		isStreamingRoute := strings.Contains(route.HTTPPattern, "/stream")

		if !isStreamingRoute {
			// Direct unary invocation for non-streaming routes
			rm.handleUnaryInvocation(w, req, ctx, addr, fullMethod, body, route)
			return
		}

		// Try to invoke as streaming for streaming routes
		msgChan, errChan, err := rm.clientMgr.InvokeJSONStream(ctx, addr, fullMethod, body)
		if err != nil {
			// Check if the error is because it's not a streaming method
			// Also fall back to unary on timeout errors (reflection may be slow)
			errMsg := err.Error()
			isNotStreaming := errMsg == "method "+fullMethod+" is not a server streaming method"
			isTimeout := ctx.Err() == context.DeadlineExceeded || errMsg == "context deadline exceeded"

			if isNotStreaming || isTimeout {
				// Fall back to unary invocation
				if isTimeout {
					rm.logger.Debug("stream_setup_timeout_fallback_to_unary",
						zap.String("backend_name", route.BackendName),
						zap.String("full_method", fullMethod),
					)
					// Create a new context with the same timeout for unary call
					// (original context was cancelled due to timeout)
					var unaryCancel context.CancelFunc
					ctx, unaryCancel = context.WithTimeout(req.Context(), timeout)
					defer unaryCancel()
				}
				rm.handleUnaryInvocation(w, req, ctx, addr, fullMethod, body, route)
				return
			}

			// Other errors during stream setup
			httpStatus, errorMsg := grpcErrorToHTTPStatus(err)
			rm.logger.Error("grpc_stream_setup_failed",
				zap.String("backend_name", route.BackendName),
				zap.String("backend_addr", addr),
				zap.String("full_method", fullMethod),
				zap.Int("http_status", httpStatus),
				zap.Error(err),
			)

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

		// Handle streaming response with newline-delimited JSON
		rm.handleStreamingResponse(w, msgChan, errChan, route)
	})
}

// handleUnaryInvocation handles a unary (non-streaming) gRPC call
func (rm *RouteManager) handleUnaryInvocation(w http.ResponseWriter, req *http.Request, ctx context.Context, addr, fullMethod string, body json.RawMessage, route config.RouteConfig) {
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
}

// handleStreamingResponse handles streaming gRPC responses and writes them as newline-delimited JSON
func (rm *RouteManager) handleStreamingResponse(w http.ResponseWriter, msgChan <-chan json.RawMessage, errChan <-chan error, route config.RouteConfig) {
	// Set headers for streaming response
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Get flusher for real-time streaming
	flusher, ok := w.(http.Flusher)
	if !ok {
		rm.logger.Error("streaming_not_supported", zap.String("route", route.HTTPPattern))
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Write status code
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Stream messages to client
	for {
		select {
		case msg, ok := <-msgChan:
			if !ok {
				// Channel closed, streaming complete
				return
			}

			// Write message as a line of newline-delimited JSON
			if _, err := w.Write(msg); err != nil {
				rm.logger.Warn("write_stream_message_failed", zap.Error(err))
				return
			}
			if _, err := w.Write([]byte("\n")); err != nil {
				rm.logger.Warn("write_newline_failed", zap.Error(err))
				return
			}

			// Flush immediately for real-time streaming
			flusher.Flush()

		case err := <-errChan:
			if err != nil {
				rm.logger.Error("stream_error",
					zap.String("backend_name", route.BackendName),
					zap.String("route", route.HTTPPattern),
					zap.Error(err),
				)
				// Write error as a JSON line
				errorResp := map[string]string{
					"error": err.Error(),
				}
				if errJSON, jsonErr := json.Marshal(errorResp); jsonErr == nil {
					w.Write(errJSON)
					w.Write([]byte("\n"))
					flusher.Flush()
				}
			}
			return
		}
	}
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
