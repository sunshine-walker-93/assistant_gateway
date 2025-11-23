package config

import (
	"database/sql"
	"errors"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLStore implements ConfigStore using MySQL database.
type MySQLStore struct {
	db *sql.DB
}

// NewMySQLStore creates a new MySQLStore instance.
func NewMySQLStore(dsn string) (*MySQLStore, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &MySQLStore{db: db}, nil
}

// Close closes the database connection.
func (s *MySQLStore) Close() error {
	return s.db.Close()
}

// GetBackends returns all enabled backend configurations.
func (s *MySQLStore) GetBackends() ([]BackendConfig, error) {
	query := `
		SELECT name, addr, description, enabled, created_at, updated_at
		FROM backends
		WHERE enabled = 1
		ORDER BY name
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var backends []BackendConfig
	for rows.Next() {
		var b BackendConfig
		var enabled int
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&b.Name, &b.Addr, &b.Description, &enabled, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		backends = append(backends, b)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return backends, nil
}

// GetBackendByName returns a backend configuration by name.
func (s *MySQLStore) GetBackendByName(name string) (*BackendConfig, error) {
	query := `
		SELECT name, addr, description, enabled, created_at, updated_at
		FROM backends
		WHERE name = ? AND enabled = 1
		LIMIT 1
	`

	var b BackendConfig
	var enabled int
	var createdAt, updatedAt time.Time

	err := s.db.QueryRow(query, name).Scan(&b.Name, &b.Addr, &b.Description, &enabled, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &b, nil
}

// GetRoutes returns all enabled route configurations.
func (s *MySQLStore) GetRoutes() ([]RouteConfig, error) {
	query := `
		SELECT id, http_method, http_pattern, backend_name, backend_service, 
		       backend_method, timeout_ms, description, enabled, created_at, updated_at
		FROM routes
		WHERE enabled = 1
		ORDER BY http_method, http_pattern
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []RouteConfig
	for rows.Next() {
		var r RouteConfig
		var id int
		var enabled int
		var createdAt, updatedAt time.Time

		if err := rows.Scan(
			&id, &r.HTTPMethod, &r.HTTPPattern, &r.BackendName, &r.BackendService,
			&r.BackendMethod, &r.TimeoutMS, &r.Description, &enabled, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}

		routes = append(routes, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return routes, nil
}

// GetConfigVersion returns the maximum updated_at timestamp from both tables.
// This is used to detect configuration changes.
func (s *MySQLStore) GetConfigVersion() (int64, error) {
	query := `
		SELECT GREATEST(
			COALESCE((SELECT UNIX_TIMESTAMP(MAX(updated_at)) FROM backends), 0),
			COALESCE((SELECT UNIX_TIMESTAMP(MAX(updated_at)) FROM routes), 0)
		) AS version
	`

	var version int64
	err := s.db.QueryRow(query).Scan(&version)
	if err != nil {
		return 0, err
	}

	return version, nil
}
