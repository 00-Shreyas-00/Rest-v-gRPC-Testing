// Command grpc-server serves the Product API over gRPC (HTTP/2, plaintext) + Protobuf.
//
// Configuration is read from the environment (see internal/config):
// DATA_SOURCE, DATABASE_URL, DB_MAX_CONNS, GRPC_PORT, LOG_LEVEL, LOG_REQUESTS, METRICS_PORT.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"

	"go-vs-ts-benchmark/go-server/internal/app"
	"go-vs-ts-benchmark/go-server/internal/config"
	"go-vs-ts-benchmark/go-server/internal/grpcapi"
	"go-vs-ts-benchmark/go-server/internal/monitoring"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false,
		"call grpc.health.v1.Health/Check on the local server and exit 0 if SERVING (for container HEALTHCHECK)")
	flag.Parse()

	fn := run
	if *healthcheck {
		fn = probe
	}
	if err := fn(); err != nil {
		fmt.Fprintln(os.Stderr, "grpc-server:", err)
		os.Exit(1)
	}
}

// probe checks a running server. The runtime image has no shell or
// grpc_health_probe, so the binary is its own health-check client.
func probe() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	conn, err := grpc.NewClient(fmt.Sprintf("127.0.0.1:%d", cfg.GRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res, err := healthgrpc.NewHealthClient(conn).Check(ctx, &healthgrpc.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if res.GetStatus() != healthgrpc.HealthCheckResponse_SERVING {
		return fmt.Errorf("health check returned %s", res.GetStatus())
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

	var opts []grpc.ServerOption
	if cfg.LogRequests {
		opts = append(opts, grpc.UnaryInterceptor(grpcapi.LoggingInterceptor(log)))
	}
	srv := grpc.NewServer(opts...)
	grpcapi.Register(srv, a.Service, a.Health, log)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return err
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(lis) }()
	log.Info("grpc-server listening",
		"addr", lis.Addr().String(),
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
	done := make(chan struct{})
	go func() { srv.GracefulStop(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		srv.Stop()
	}
	return nil
}
