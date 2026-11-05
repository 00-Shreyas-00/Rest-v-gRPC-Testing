package grpcapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go-vs-ts-benchmark/go-server/gen/productv1"
	"go-vs-ts-benchmark/go-server/internal/product"
	"go-vs-ts-benchmark/go-server/internal/repository"
	"go-vs-ts-benchmark/go-server/internal/rest"
	"go-vs-ts-benchmark/go-server/internal/service"
)

var (
	t0 = time.Date(2024, 1, 1, 0, 5, 0, 0, time.UTC)
	t1 = time.Date(2024, 1, 1, 7, 5, 0, 0, time.UTC)

	seed = []product.Product{
		{ID: 1, Name: "Orion Smart Phone PH-000001",
			Description: "The Orion smart phone is designed for home offices. Firmware updates are delivered automatically.",
			Category:    "Mobile", Price: 80.18, Stock: 37, Rating: 2.3, CreatedAt: t0, UpdatedAt: t1},
		{ID: 2, Name: "Quanta Max Laptop LA-000002", Category: "Computers", Price: 159.37, Stock: 74, Rating: 3.6, CreatedAt: t0, UpdatedAt: t1},
		{ID: 3, Name: "Nimbus Ultra Monitor MO-000003", Category: "Displays", Price: 238.56, Stock: 111, Rating: 4.9, CreatedAt: t0, UpdatedAt: t1},
	}
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// newClient starts an in-process gRPC server (bufconn) on svc.
func newClient(t *testing.T, svc *service.Service, health func(context.Context) error) (productv1.ProductServiceClient, healthgrpc.HealthClient) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	Register(srv, svc, health, quiet)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return productv1.NewProductServiceClient(conn), healthgrpc.NewHealthClient(conn)
}

func newMemService() *service.Service {
	return service.New(repository.NewMemory(seed))
}

func ptr[T any](v T) *T { return &v }

// fieldViolations extracts the BadRequest details of an INVALID_ARGUMENT error.
func fieldViolations(t *testing.T, err error) []product.FieldError {
	t.Helper()
	st := status.Convert(err)
	if st.Code() != codes.InvalidArgument || st.Message() != "validation failed" {
		t.Fatalf("status = %v %q, want InvalidArgument \"validation failed\"", st.Code(), st.Message())
	}
	var out []product.FieldError
	for _, d := range st.Details() {
		if br, ok := d.(*errdetails.BadRequest); ok {
			for _, v := range br.GetFieldViolations() {
				out = append(out, product.FieldError{Field: v.GetField(), Message: v.GetDescription()})
			}
		}
	}
	return out
}

func TestGetProduct(t *testing.T) {
	client, _ := newClient(t, newMemService(), nil)
	ctx := context.Background()

	got, err := client.GetProduct(ctx, &productv1.GetProductRequest{Id: 1})
	if err != nil {
		t.Fatal(err)
	}
	want := &productv1.Product{
		Id: 1, Name: seed[0].Name, Description: seed[0].Description, Category: "Mobile",
		Price: 80.18, Stock: 37, Rating: 2.3,
		CreatedAt: timestamppb.New(t0), UpdatedAt: timestamppb.New(t1),
	}
	if !proto.Equal(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}

	if _, err := client.GetProduct(ctx, &productv1.GetProductRequest{Id: 99}); status.Code(err) != codes.NotFound ||
		status.Convert(err).Message() != "product not found" {
		t.Errorf("missing product: %v", err)
	}
	for _, id := range []int64{0, -1, product.MaxID + 1} {
		_, err := client.GetProduct(ctx, &productv1.GetProductRequest{Id: id})
		if got := fieldViolations(t, err); !reflect.DeepEqual(got, []product.FieldError{{Field: "id", Message: product.IDMessage}}) {
			t.Errorf("id %d: %v", id, got)
		}
	}
}

func TestListAndSearch(t *testing.T) {
	client, _ := newClient(t, newMemService(), nil)
	ctx := context.Background()

	ids := func(l *productv1.ProductList) []int64 {
		var out []int64
		for _, p := range l.GetProducts() {
			out = append(out, p.GetId())
		}
		return out
	}

	// Unset optional fields take the defaults.
	l, err := client.ListProducts(ctx, &productv1.ListProductsRequest{})
	if err != nil || !reflect.DeepEqual(ids(l), []int64{1, 2, 3}) {
		t.Errorf("default list = %v, %v", ids(l), err)
	}
	l, _ = client.ListProducts(ctx, &productv1.ListProductsRequest{Limit: ptr[int32](1), Offset: ptr[int32](1)})
	if !reflect.DeepEqual(ids(l), []int64{2}) {
		t.Errorf("paged list = %v", ids(l))
	}
	// Explicit 0 is a validation error, like ?limit=0.
	_, err = client.ListProducts(ctx, &productv1.ListProductsRequest{Limit: ptr[int32](0), Offset: ptr[int32](-1)})
	want := []product.FieldError{{Field: "limit", Message: "must be between 1 and 1000"}, {Field: "offset", Message: "must be between 0 and 1000000000"}}
	if got := fieldViolations(t, err); !reflect.DeepEqual(got, want) {
		t.Errorf("invalid page: %v", got)
	}

	l, err = client.SearchProducts(ctx, &productv1.SearchProductsRequest{Query: "MONITOR"})
	if err != nil || !reflect.DeepEqual(ids(l), []int64{3}) {
		t.Errorf("search = %v, %v", ids(l), err)
	}
	l, _ = client.SearchProducts(ctx, &productv1.SearchProductsRequest{Query: "%"})
	if len(l.GetProducts()) != 0 {
		t.Errorf("wildcard search = %v", ids(l))
	}
	_, err = client.SearchProducts(ctx, &productv1.SearchProductsRequest{Limit: ptr[int32](0)})
	want = []product.FieldError{{Field: "query", Message: "is required"}, {Field: "limit", Message: "must be between 1 and 1000"}}
	if got := fieldViolations(t, err); !reflect.DeepEqual(got, want) {
		t.Errorf("invalid search: %v", got)
	}
}

func TestCreateUpdateDelete(t *testing.T) {
	client, _ := newClient(t, newMemService(), nil)
	ctx := context.Background()

	created, err := client.CreateProduct(ctx, &productv1.CreateProductRequest{
		Name: "Desk Lamp", Category: "Office", Price: 19.99, Stock: 5, Rating: 4.5})
	if err != nil || created.GetId() != 4 || created.GetName() != "Desk Lamp" || !created.GetCreatedAt().AsTime().Equal(created.GetUpdatedAt().AsTime()) {
		t.Fatalf("create = %v, %v", created, err)
	}

	updated, err := client.UpdateProduct(ctx, &productv1.UpdateProductRequest{
		Id: 4, Name: "Desk Lamp 2", Category: "Office", Price: 25, Stock: 1, Rating: 5})
	if err != nil || updated.GetName() != "Desk Lamp 2" || !proto.Equal(updated.GetCreatedAt(), created.GetCreatedAt()) {
		t.Errorf("update = %v, %v", updated, err)
	}

	if _, err := client.DeleteProduct(ctx, &productv1.DeleteProductRequest{Id: 4}); err != nil {
		t.Errorf("delete: %v", err)
	}
	if _, err := client.DeleteProduct(ctx, &productv1.DeleteProductRequest{Id: 4}); status.Code(err) != codes.NotFound {
		t.Errorf("delete twice: %v", err)
	}
	if _, err := client.UpdateProduct(ctx, &productv1.UpdateProductRequest{Id: 4, Name: "x", Category: "y"}); status.Code(err) != codes.NotFound {
		t.Errorf("update deleted: %v", err)
	}

	_, err = client.CreateProduct(ctx, &productv1.CreateProductRequest{Price: 1.005})
	want := []product.FieldError{
		{Field: "name", Message: "is required"},
		{Field: "category", Message: "is required"},
		{Field: "price", Message: "must have at most 2 decimal places"},
	}
	if got := fieldViolations(t, err); !reflect.DeepEqual(got, want) {
		t.Errorf("invalid create: %v", got)
	}
}

func TestInternalErrorsAreHidden(t *testing.T) {
	client, _ := newClient(t, service.New(failingRepo{}), nil)
	_, err := client.GetProduct(context.Background(), &productv1.GetProductRequest{Id: 1})
	if st := status.Convert(err); st.Code() != codes.Internal || st.Message() != "internal server error" {
		t.Errorf("got %v %q", st.Code(), st.Message())
	}
}

type failingRepo struct{ service.Repository }

func (failingRepo) Get(context.Context, int64) (product.Product, error) {
	return product.Product{}, errors.New("connection refused")
}

func TestHealth(t *testing.T) {
	ctx := context.Background()
	_, health := newClient(t, newMemService(), func(context.Context) error { return nil })
	for _, svc := range []string{"", "product.v1.ProductService"} {
		res, err := health.Check(ctx, &healthgrpc.HealthCheckRequest{Service: svc})
		if err != nil || res.GetStatus() != healthgrpc.HealthCheckResponse_SERVING {
			t.Errorf("check %q = %v, %v", svc, res, err)
		}
	}
	if _, err := health.Check(ctx, &healthgrpc.HealthCheckRequest{Service: "other"}); status.Code(err) != codes.NotFound {
		t.Errorf("unknown service: %v", err)
	}

	_, down := newClient(t, newMemService(), func(context.Context) error { return errors.New("db down") })
	if res, err := down.Check(ctx, &healthgrpc.HealthCheckRequest{}); err != nil || res.GetStatus() != healthgrpc.HealthCheckResponse_NOT_SERVING {
		t.Errorf("down check = %v, %v", res, err)
	}
}

// TestRESTAndGRPCReturnSameData runs both transports on one service and checks
// that every read returns the same logical products.
func TestRESTAndGRPCReturnSameData(t *testing.T) {
	svc := newMemService()
	client, _ := newClient(t, svc, nil)
	httpSrv := httptest.NewServer(rest.NewHandler(svc, func(context.Context) error { return nil }, quiet, false))
	t.Cleanup(httpSrv.Close)
	ctx := context.Background()

	fromPB := func(p *productv1.Product) product.Product {
		return product.Product{
			ID: p.GetId(), Name: p.GetName(), Description: p.GetDescription(), Category: p.GetCategory(),
			Price: p.GetPrice(), Stock: p.GetStock(), Rating: p.GetRating(),
			CreatedAt: p.GetCreatedAt().AsTime(), UpdatedAt: p.GetUpdatedAt().AsTime(),
		}
	}
	fromPBList := func(l *productv1.ProductList) []product.Product {
		out := []product.Product{}
		for _, p := range l.GetProducts() {
			out = append(out, fromPB(p))
		}
		return out
	}
	getJSON := func(path string, v any) {
		t.Helper()
		res, err := http.Get(httpSrv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if err := json.NewDecoder(res.Body).Decode(v); err != nil {
			t.Fatal(err)
		}
	}

	var restP product.Product
	getJSON("/products/1", &restP)
	grpcP, _ := client.GetProduct(ctx, &productv1.GetProductRequest{Id: 1})
	if !reflect.DeepEqual(restP, fromPB(grpcP)) {
		t.Errorf("get differs:\nREST %+v\ngRPC %+v", restP, fromPB(grpcP))
	}

	var restL rest.ProductList
	getJSON("/products?limit=2&offset=1", &restL)
	grpcL, _ := client.ListProducts(ctx, &productv1.ListProductsRequest{Limit: ptr[int32](2), Offset: ptr[int32](1)})
	if !reflect.DeepEqual(restL.Products, fromPBList(grpcL)) {
		t.Errorf("list differs")
	}

	restL = rest.ProductList{}
	getJSON("/products/search?q=o", &restL)
	grpcL, _ = client.SearchProducts(ctx, &productv1.SearchProductsRequest{Query: "o"})
	if !reflect.DeepEqual(restL.Products, fromPBList(grpcL)) || len(restL.Products) != 3 {
		t.Errorf("search differs")
	}
}
