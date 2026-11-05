package grpcapi

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"go-vs-ts-benchmark/go-server/gen/productv1"
)

// healthServer implements grpc.health.v1.Health/Check with the same data-source
// check as REST GET /health. Watch and List are not implemented.
type healthServer struct {
	healthgrpc.UnimplementedHealthServer
	check func(context.Context) error
	log   *slog.Logger
}

func (h *healthServer) Check(ctx context.Context, req *healthgrpc.HealthCheckRequest) (*healthgrpc.HealthCheckResponse, error) {
	// "" means the whole server; the product service name is also accepted.
	if svc := req.GetService(); svc != "" && svc != productv1.ProductService_ServiceDesc.ServiceName {
		return nil, status.Error(codes.NotFound, "unknown service")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := h.check(ctx); err != nil {
		h.log.Warn("health check failed", "err", err)
		return &healthgrpc.HealthCheckResponse{Status: healthgrpc.HealthCheckResponse_NOT_SERVING}, nil
	}
	return &healthgrpc.HealthCheckResponse{Status: healthgrpc.HealthCheckResponse_SERVING}, nil
}
