// Package app wires configuration, data source and service together.
// Both cmd/rest-server and cmd/grpc-server start through Build, so the two
// transports always run on an identically constructed service.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go-vs-ts-benchmark/go-server/internal/config"
	"go-vs-ts-benchmark/go-server/internal/repository"
	"go-vs-ts-benchmark/go-server/internal/service"
)

// App is a ready-to-serve service plus its lifecycle hooks.
type App struct {
	Service *service.Service
	// Health reports whether the data source can serve requests.
	Health func(ctx context.Context) error
	// Close releases resources (the database pool in postgres mode).
	Close func()
}

// Build opens the configured data source.
//
// postgres: requests query PostgreSQL through a pool of cfg.DBMaxConns.
// none:     no product API (Service is nil); experiment-2 endpoints only.
// memory:   the products table is loaded once at startup, the pool is closed,
//
//	and requests never touch the database.
func Build(ctx context.Context, cfg config.Config, log *slog.Logger) (*App, error) {
	if cfg.DataSource == config.DataSourceNone {
		log.Info("data source ready", "data_source", cfg.DataSource)
		return &App{Health: func(context.Context) error { return nil }, Close: func() {}}, nil
	}
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	pool, err := repository.OpenPool(connectCtx, cfg.DatabaseURL, cfg.DBMaxConns)
	if err != nil {
		return nil, err
	}
	pg := repository.NewPostgres(pool)

	switch cfg.DataSource {
	case config.DataSourceMemory:
		start := time.Now()
		mem, err := repository.LoadMemory(connectCtx, pg)
		pool.Close()
		if err != nil {
			return nil, fmt.Errorf("load in-memory snapshot: %w", err)
		}
		log.Info("data source ready", "data_source", cfg.DataSource, "products", mem.Len(), "load_ms", time.Since(start).Milliseconds())
		return &App{
			Service: service.New(mem),
			Health:  func(context.Context) error { return nil },
			Close:   func() {},
		}, nil

	default:
		log.Info("data source ready", "data_source", cfg.DataSource, "max_conns", cfg.DBMaxConns)
		return &App{
			Service: service.New(pg),
			Health:  pool.Ping,
			Close:   pool.Close,
		}, nil
	}
}
