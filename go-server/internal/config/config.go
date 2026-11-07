// Package config reads server configuration from environment variables.
// The TypeScript server reads the same variables with the same defaults.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Data sources (DATA_SOURCE).
const (
	DataSourcePostgres = "postgres"
	DataSourceMemory   = "memory"
	// DataSourceNone serves only the experiment-2 endpoints (internal/x2); no database.
	DataSourceNone = "none"
)

// Config is the runtime configuration shared by the REST and gRPC servers.
type Config struct {
	DataSource  string     // DATA_SOURCE: postgres | memory | none
	DatabaseURL string     // DATABASE_URL
	DBMaxConns  int32      // DB_MAX_CONNS: pool size, identical across implementations
	HTTPPort    int        // HTTP_PORT (REST server)
	GRPCPort    int        // GRPC_PORT (gRPC server)
	MetricsPort int        // METRICS_PORT: runtime metrics listener, 0 disables
	LogLevel    slog.Level // LOG_LEVEL: debug | info | warn | error
	LogRequests bool       // LOG_REQUESTS: per-request logging; keep off when benchmarking
}

// Load reads the configuration from the environment.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}
	var errs []string
	intVar := func(key string, def, minVal, maxVal int) int {
		s := get(key, strconv.Itoa(def))
		n, err := strconv.Atoi(s)
		if err != nil || n < minVal || n > maxVal {
			errs = append(errs, fmt.Sprintf("%s must be an integer between %d and %d, got %q", key, minVal, maxVal, s))
			return def
		}
		return n
	}

	cfg := Config{
		DataSource:  strings.ToLower(get("DATA_SOURCE", DataSourcePostgres)),
		DatabaseURL: get("DATABASE_URL", "postgres://bench:bench@localhost:5432/products"),
		DBMaxConns:  int32(intVar("DB_MAX_CONNS", 20, 1, 1000)),
		HTTPPort:    intVar("HTTP_PORT", 8080, 1, 65535),
		GRPCPort:    intVar("GRPC_PORT", 9090, 1, 65535),
		MetricsPort: intVar("METRICS_PORT", 9100, 0, 65535),
	}

	if cfg.DataSource != DataSourcePostgres && cfg.DataSource != DataSourceMemory && cfg.DataSource != DataSourceNone {
		errs = append(errs, fmt.Sprintf("DATA_SOURCE must be %q, %q or %q, got %q", DataSourcePostgres, DataSourceMemory, DataSourceNone, cfg.DataSource))
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Sprintf("LOG_LEVEL: %v", err))
	}
	switch v := strings.ToLower(get("LOG_REQUESTS", "false")); v {
	case "true", "1":
		cfg.LogRequests = true
	case "false", "0":
	default:
		errs = append(errs, fmt.Sprintf("LOG_REQUESTS must be true or false, got %q", v))
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %s", strings.Join(errs, "; "))
	}
	return cfg, nil
}
