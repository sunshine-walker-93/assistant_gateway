package config

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Config is the top-level configuration for the gateway.
type Config struct {
	Backends []BackendConfig
	Routes   []RouteConfig
}

// BackendConfig describes a logical backend service reachable via gRPC.
// Multiple routes can reference the same backend by name.
type BackendConfig struct {
	// Name is an internal identifier used by routes, e.g. "account".
	Name string

	// Addr is the host:port of the backend, e.g. "127.0.0.1:50051"
	// or "assistant-account-app:50051" inside a Docker network.
	Addr string

	// Description is optional description of the backend service.
	Description string
}

// RouteConfig describes a single dynamic HTTP → gRPC mapping rule.
type RouteConfig struct {
	// HTTPMethod is the incoming HTTP verb, e.g. GET/POST/PUT/DELETE.
	HTTPMethod string

	// HTTPPattern is the incoming HTTP path pattern, e.g. /v1/users/{id}.
	HTTPPattern string

	// BackendService is the logical gRPC service name, e.g. user.v1.UserService.
	BackendService string

	// BackendMethod is the gRPC method name, e.g. Login.
	BackendMethod string

	// BackendName is the logical backend this route targets, referring to
	// an entry in Config.Backends.
	BackendName string

	// TimeoutMS sets a per-request timeout in milliseconds.
	TimeoutMS int

	// Description is optional description of the route.
	Description string
}

// ConfigManager provides thread-safe access to configuration with automatic polling.
type ConfigManager struct {
	mu      sync.RWMutex
	cfg     *Config
	store   ConfigStore
	version int64 // Last known config version

	// Polling
	pollInterval time.Duration
	cancel       context.CancelFunc
	onChange     func(*Config) // Callback when config changes
}

// NewConfigManager creates a new ConfigManager with database storage.
func NewConfigManager(store ConfigStore, pollInterval time.Duration, onChange func(*Config)) (*ConfigManager, error) {
	mgr := &ConfigManager{
		store:        store,
		pollInterval: pollInterval,
		onChange:     onChange,
	}

	// Load initial config
	if err := mgr.loadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load initial config: %w", err)
	}

	// Start polling if interval > 0
	if pollInterval > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		mgr.cancel = cancel
		go mgr.poll(ctx)
	}

	return mgr, nil
}

// Close stops the polling goroutine.
func (m *ConfigManager) Close() {
	if m.cancel != nil {
		m.cancel()
	}
	if mysqlStore, ok := m.store.(*MySQLStore); ok {
		mysqlStore.Close()
	}
}

// loadConfig loads configuration from the store.
func (m *ConfigManager) loadConfig() error {
	backends, err := m.store.GetBackends()
	if err != nil {
		return fmt.Errorf("failed to get backends: %w", err)
	}

	routes, err := m.store.GetRoutes()
	if err != nil {
		return fmt.Errorf("failed to get routes: %w", err)
	}

	version, err := m.store.GetConfigVersion()
	if err != nil {
		return fmt.Errorf("failed to get config version: %w", err)
	}

	m.mu.Lock()
	m.cfg = &Config{
		Backends: backends,
		Routes:   routes,
	}
	m.version = version
	m.mu.Unlock()

	return nil
}

// poll periodically checks for configuration changes.
func (m *ConfigManager) poll(ctx context.Context) {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAndReload()
		}
	}
}

// checkAndReload checks if config has changed and reloads if necessary.
func (m *ConfigManager) checkAndReload() {
	version, err := m.store.GetConfigVersion()
	if err != nil {
		// Log error but don't fail
		return
	}

	m.mu.RLock()
	currentVersion := m.version
	m.mu.RUnlock()

	if version > currentVersion {
		// Config has changed, reload
		if err := m.loadConfig(); err != nil {
			// Log error but don't fail
			return
		}

		// Notify callback
		if m.onChange != nil {
			m.mu.RLock()
			cfg := m.getConfigCopy()
			m.mu.RUnlock()
			m.onChange(cfg)
		}
	}
}

// getConfigCopy returns a copy of the current configuration.
func (m *ConfigManager) getConfigCopy() *Config {
	cfgCopy := *m.cfg
	cfgCopy.Backends = make([]BackendConfig, len(m.cfg.Backends))
	copy(cfgCopy.Backends, m.cfg.Backends)
	cfgCopy.Routes = make([]RouteConfig, len(m.cfg.Routes))
	copy(cfgCopy.Routes, m.cfg.Routes)
	return &cfgCopy
}

// GetConfig returns a copy of the current configuration.
func (m *ConfigManager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getConfigCopy()
}

// GetBackendAddr returns the address for a backend by name.
func (m *ConfigManager) GetBackendAddr(name string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, b := range m.cfg.Backends {
		if b.Name == name {
			return b.Addr, true
		}
	}
	return "", false
}

// LoadConfig loads configuration from database.
// This is a convenience function for initial setup.
func LoadConfig(dsn string) (*Config, error) {
	store, err := NewMySQLStore(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create mysql store: %w", err)
	}
	defer store.Close()

	backends, err := store.GetBackends()
	if err != nil {
		return nil, fmt.Errorf("failed to get backends: %w", err)
	}

	routes, err := store.GetRoutes()
	if err != nil {
		return nil, fmt.Errorf("failed to get routes: %w", err)
	}

	return &Config{
		Backends: backends,
		Routes:   routes,
	}, nil
}
