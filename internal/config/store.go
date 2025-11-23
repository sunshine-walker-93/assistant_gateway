package config

// ConfigStore defines the interface for configuration storage.
// This abstraction allows switching between different storage backends
// (file, database, config center, etc.).
type ConfigStore interface {
	// GetBackends returns all backend configurations.
	GetBackends() ([]BackendConfig, error)

	// GetBackendByName returns a backend configuration by name.
	GetBackendByName(name string) (*BackendConfig, error)

	// GetRoutes returns all route configurations (only enabled routes).
	GetRoutes() ([]RouteConfig, error)

	// GetConfigVersion returns the latest updated_at timestamp for detecting changes.
	// Returns the maximum updated_at from both backends and routes tables.
	GetConfigVersion() (int64, error)
}
