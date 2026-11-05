package config

import (
	"log/slog"
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDefaults(t *testing.T) {
	cfg, err := load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		DataSource:  DataSourcePostgres,
		DatabaseURL: "postgres://bench:bench@localhost:5432/products",
		DBMaxConns:  20,
		HTTPPort:    8080,
		GRPCPort:    9090,
		MetricsPort: 9100,
		LogLevel:    slog.LevelInfo,
	}
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestOverrides(t *testing.T) {
	cfg, err := load(env(map[string]string{
		"DATA_SOURCE": "MEMORY", "DB_MAX_CONNS": "8", "HTTP_PORT": "3000",
		"LOG_LEVEL": "debug", "LOG_REQUESTS": "true",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DataSource != DataSourceMemory || cfg.DBMaxConns != 8 || cfg.HTTPPort != 3000 ||
		cfg.LogLevel != slog.LevelDebug || !cfg.LogRequests {
		t.Errorf("got %+v", cfg)
	}
}

func TestInvalidValuesAreReportedTogether(t *testing.T) {
	_, err := load(env(map[string]string{"DATA_SOURCE": "redis", "DB_MAX_CONNS": "0", "LOG_REQUESTS": "maybe"}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, s := range []string{"DATA_SOURCE", "DB_MAX_CONNS", "LOG_REQUESTS"} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("error does not mention %s: %v", s, err)
		}
	}
}
