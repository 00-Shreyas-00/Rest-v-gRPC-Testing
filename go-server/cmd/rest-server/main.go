// Command rest-server serves the Product API over HTTP/1.1 + JSON.
//
// Configuration is read from the environment (see internal/config):
// DATA_SOURCE, DATABASE_URL, DB_MAX_CONNS, HTTP_PORT, LOG_LEVEL, LOG_REQUESTS, METRICS_PORT.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"go-vs-ts-benchmark/go-server/internal/app"
	"go-vs-ts-benchmark/go-server/internal/config"
	"go-vs-ts-benchmark/go-server/internal/monitoring"
	"go-vs-ts-benchmark/go-server/internal/rest"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false,
		"probe GET /health on the local server and exit 0 if healthy (for container HEALTHCHECK)")
	flag.Parse()

	fn := run
	if *healthcheck {
		fn = probe
	}
	if err := fn(); err != nil {
		fmt.Fprintln(os.Stderr, "rest-server:", err)
		os.Exit(1)
	}
}

// probe checks a running server. The runtime image has no shell or curl, so
// the binary is its own health-check client.
func probe() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	res, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", cfg.HTTPPort))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned %s", res.Status)
	}
	return nil
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.Build(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer a.Close()

	// Runtime metrics on their own port, off the benchmarked request path.
	monitoring.Serve(ctx, cfg.MetricsPort, log)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           rest.NewHandler(a.Service, a.Health, log, cfg.LogRequests),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("rest-server listening",
		"addr", srv.Addr,
		"data_source", cfg.DataSource,
		"go_version", runtime.Version(),
		"gomaxprocs", runtime.GOMAXPROCS(0),
		"log_requests", cfg.LogRequests)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
