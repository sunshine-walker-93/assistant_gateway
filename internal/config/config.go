package config

import (
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for the gateway.
type Config struct {
	HTTP     HTTPConfig      `yaml:"http"`
	Backends []BackendConfig `yaml:"backends"`
	Routes   []RouteConfig   `yaml:"routes"`
}

// HTTPConfig holds HTTP listener options.
type HTTPConfig struct {
	ListenAddress string `yaml:"listen_address"`
}

// BackendConfig describes a logical backend service reachable via gRPC.
// Multiple routes can reference the same backend by name.
type BackendConfig struct {
	// Name is an internal identifier used by routes, e.g. "account".
	Name string `yaml:"name"`

	// Addr is the host:port of the backend, e.g. "127.0.0.1:50051"
	// or "assistant-account-app:50051" inside a Docker network.
	Addr string `yaml:"addr"`
}

// RouteConfig describes a single dynamic HTTP → gRPC mapping rule.
//
// NOTE: This is an MVP-level model; it can be extended with auth, rate limit, etc.
type RouteConfig struct {
	// HTTPMethod is the incoming HTTP verb, e.g. GET/POST/PUT/DELETE.
	HTTPMethod string `yaml:"http_method"`

	// HTTPPattern is the incoming HTTP path pattern, e.g. /v1/users/{id}.
	HTTPPattern string `yaml:"http_pattern"`

	// BackendService is the logical gRPC service name, e.g. user.UserService.
	BackendService string `yaml:"backend_service"`

	// BackendMethod is the gRPC method name, e.g. GetUser.
	BackendMethod string `yaml:"backend_method"`

	// BackendName is the logical backend this route targets, referring to
	// an entry in Config.Backends.
	BackendName string `yaml:"backend_name"`

	// TimeoutMS sets a per-request timeout in milliseconds.
	TimeoutMS int `yaml:"timeout_ms"`

	// RequestType is the fully-qualified proto message name for the request,
	// e.g. "user.v1.LoginRequest". If empty, the route will fall back to the
	// generic Struct-based JSON forwarding.
	RequestType string `yaml:"request_type,omitempty"`

	// ResponseType is the fully-qualified proto message name for the response,
	// e.g. "user.v1.LoginResponse". If empty, the route will fall back to the
	// generic Struct-based JSON forwarding.
	ResponseType string `yaml:"response_type,omitempty"`
}

var (
	mu         sync.Mutex
	configPath string
)

// LoadConfig loads configuration from the default path.
//
// For MVP we read a single YAML file; later this can be replaced by a config center.
func LoadConfig() (*Config, error) {
	path := os.Getenv("GATEWAY_CONFIG")
	if path == "" {
		path = "configs/config.yaml"
	}

	mu.Lock()
	configPath = path
	mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveConfig persists the current configuration back to the underlying storage.
// For MVP we simply rewrite the YAML file that LoadConfig used.
func SaveConfig(cfg *Config) error {
	mu.Lock()
	path := configPath
	mu.Unlock()
	if path == "" {
		path = "configs/config.yaml"
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}


