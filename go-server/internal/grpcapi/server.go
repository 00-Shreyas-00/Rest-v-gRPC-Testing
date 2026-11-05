// Package grpcapi is the gRPC/Protobuf transport. Like internal/rest it only
// translates between the wire format and the shared service; it contains no
// business rules of its own.
package grpcapi

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go-vs-ts-benchmark/go-server/gen/productv1"
	"go-vs-ts-benchmark/go-server/internal/product"
	"go-vs-ts-benchmark/go-server/internal/service"
)

// Register adds the product service, the standard health service and server
// reflection (for grpcurl and k6) to s.
func Register(s *grpc.Server, svc *service.Service, health func(context.Context) error, log *slog.Logger) {
	productv1.RegisterProductServiceServer(s, &productServer{svc: svc, log: log})
	healthgrpc.RegisterHealthServer(s, &healthServer{check: health, log: log})
	reflection.Register(s)
}

// LoggingInterceptor logs every unary call. Only installed when LOG_REQUESTS=true.
func LoggingInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := next(ctx, req)
		log.Info("request", "method", info.FullMethod, "code", status.Code(err).String(), "duration", time.Since(start))
		return resp, err
	}
}

type productServer struct {
	productv1.UnimplementedProductServiceServer
	svc *service.Service
	log *slog.Logger
}

func (s *productServer) GetProduct(ctx context.Context, req *productv1.GetProductRequest) (*productv1.Product, error) {
	p, err := s.svc.Get(ctx, req.GetId())
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toPB(p), nil
}

func (s *productServer) ListProducts(ctx context.Context, req *productv1.ListProductsRequest) (*productv1.ProductList, error) {
	limit, offset := product.DefaultListLimit, 0
	if req.Limit != nil {
		limit = int(*req.Limit)
	}
	if req.Offset != nil {
		offset = int(*req.Offset)
	}
	ps, err := s.svc.List(ctx, limit, offset)
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toPBList(ps), nil
}

func (s *productServer) SearchProducts(ctx context.Context, req *productv1.SearchProductsRequest) (*productv1.ProductList, error) {
	limit := product.DefaultSearchLimit
	if req.Limit != nil {
		limit = int(*req.Limit)
	}
	ps, err := s.svc.Search(ctx, req.GetQuery(), limit)
	if err != nil {
		return nil, s.toStatus(renameField(err, "q", "query"))
	}
	return toPBList(ps), nil
}

func (s *productServer) CreateProduct(ctx context.Context, req *productv1.CreateProductRequest) (*productv1.Product, error) {
	p, err := s.svc.Create(ctx, product.Input{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Category:    req.GetCategory(),
		Price:       req.GetPrice(),
		Stock:       req.GetStock(),
		Rating:      req.GetRating(),
	})
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toPB(p), nil
}

func (s *productServer) UpdateProduct(ctx context.Context, req *productv1.UpdateProductRequest) (*productv1.Product, error) {
	p, err := s.svc.Update(ctx, req.GetId(), product.Input{
		Name:        req.GetName(),
		Description: req.GetDescription(),
		Category:    req.GetCategory(),
		Price:       req.GetPrice(),
		Stock:       req.GetStock(),
		Rating:      req.GetRating(),
	})
	if err != nil {
		return nil, s.toStatus(err)
	}
	return toPB(p), nil
}

func (s *productServer) DeleteProduct(ctx context.Context, req *productv1.DeleteProductRequest) (*productv1.DeleteProductResponse, error) {
	if err := s.svc.Delete(ctx, req.GetId()); err != nil {
		return nil, s.toStatus(err)
	}
	return &productv1.DeleteProductResponse{}, nil
}

func toPB(p product.Product) *productv1.Product {
	return &productv1.Product{
		Id:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Category:    p.Category,
		Price:       p.Price,
		Stock:       p.Stock,
		Rating:      p.Rating,
		CreatedAt:   timestamppb.New(p.CreatedAt),
		UpdatedAt:   timestamppb.New(p.UpdatedAt),
	}
}

func toPBList(ps []product.Product) *productv1.ProductList {
	out := make([]*productv1.Product, len(ps))
	for i, p := range ps {
		out[i] = toPB(p)
	}
	return &productv1.ProductList{Products: out}
}

// toStatus maps service errors to gRPC status codes:
// validation -> INVALID_ARGUMENT (+ google.rpc.BadRequest), not found ->
// NOT_FOUND, anything else -> INTERNAL. Messages match the REST error bodies.
func (s *productServer) toStatus(err error) error {
	var ve *product.ValidationError
	switch {
	case errors.As(err, &ve):
		br := &errdetails.BadRequest{FieldViolations: make([]*errdetails.BadRequest_FieldViolation, len(ve.Fields))}
		for i, f := range ve.Fields {
			br.FieldViolations[i] = &errdetails.BadRequest_FieldViolation{Field: f.Field, Description: f.Message}
		}
		st, detailErr := status.New(codes.InvalidArgument, "validation failed").WithDetails(br)
		if detailErr != nil {
			return status.Error(codes.InvalidArgument, ve.Error())
		}
		return st.Err()
	case errors.Is(err, product.ErrNotFound):
		return status.Error(codes.NotFound, "product not found")
	default:
		s.log.Error("request failed", "err", err)
		return status.Error(codes.Internal, "internal server error")
	}
}

// renameField maps a service field name to the protobuf field name
// (REST uses ?q=, the proto message uses `query`).
func renameField(err error, from, to string) error {
	var ve *product.ValidationError
	if !errors.As(err, &ve) {
		return err
	}
	fields := make([]product.FieldError, len(ve.Fields))
	for i, f := range ve.Fields {
		if f.Field == from {
			f.Field = to
		}
		fields[i] = f
	}
	return &product.ValidationError{Fields: fields}
}
