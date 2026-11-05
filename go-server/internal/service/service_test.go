package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"go-vs-ts-benchmark/go-server/internal/product"
	"go-vs-ts-benchmark/go-server/internal/repository"
)

// spyRepo records whether the repository was reached.
type spyRepo struct {
	Repository
	calls int
}

func (s *spyRepo) Get(ctx context.Context, id int64) (product.Product, error) {
	s.calls++
	return s.Repository.Get(ctx, id)
}
func (s *spyRepo) List(ctx context.Context, l, o int) ([]product.Product, error) {
	s.calls++
	return s.Repository.List(ctx, l, o)
}
func (s *spyRepo) Search(ctx context.Context, q string, l int) ([]product.Product, error) {
	s.calls++
	return s.Repository.Search(ctx, q, l)
}
func (s *spyRepo) Create(ctx context.Context, in product.Input) (product.Product, error) {
	s.calls++
	return s.Repository.Create(ctx, in)
}
func (s *spyRepo) Update(ctx context.Context, id int64, in product.Input) (product.Product, error) {
	s.calls++
	return s.Repository.Update(ctx, id, in)
}
func (s *spyRepo) Delete(ctx context.Context, id int64) error {
	s.calls++
	return s.Repository.Delete(ctx, id)
}

func newService() (*Service, *spyRepo) {
	spy := &spyRepo{Repository: repository.NewMemory([]product.Product{
		{ID: 1, Name: "Wireless Keyboard", Category: "Peripherals", Price: 10},
		{ID: 2, Name: "Gaming Mouse", Category: "Peripherals", Price: 20},
	})}
	return New(spy), spy
}

func validationFields(t *testing.T, err error) []string {
	t.Helper()
	var ve *product.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	fields := make([]string, len(ve.Fields))
	for i, f := range ve.Fields {
		fields[i] = f.Field
	}
	return fields
}

func TestInvalidInputNeverReachesRepository(t *testing.T) {
	svc, spy := newService()
	ctx := context.Background()
	bad := product.Input{Price: -1}

	cases := map[string]struct {
		call func() error
		want []string
	}{
		"get":    {func() error { _, err := svc.Get(ctx, 0); return err }, []string{"id"}},
		"list":   {func() error { _, err := svc.List(ctx, 0, 0); return err }, []string{"limit"}},
		"search": {func() error { _, err := svc.Search(ctx, "", 10); return err }, []string{"q"}},
		"create": {func() error { _, err := svc.Create(ctx, bad); return err }, []string{"name", "category", "price"}},
		"update": {func() error { _, err := svc.Update(ctx, -1, bad); return err }, []string{"id", "name", "category", "price"}},
		"delete": {func() error { return svc.Delete(ctx, 0) }, []string{"id"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := validationFields(t, tc.call()); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("fields = %v, want %v", got, tc.want)
			}
		})
	}
	if spy.calls != 0 {
		t.Errorf("repository called %d times for invalid input", spy.calls)
	}
}

func TestValidRequestsDelegate(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	if p, err := svc.Get(ctx, 2); err != nil || p.Name != "Gaming Mouse" {
		t.Errorf("get = %+v, %v", p, err)
	}
	if _, err := svc.Get(ctx, 3); !errors.Is(err, product.ErrNotFound) {
		t.Errorf("get missing = %v, want ErrNotFound", err)
	}
	if ps, err := svc.List(ctx, 10, 1); err != nil || len(ps) != 1 || ps[0].ID != 2 {
		t.Errorf("list = %+v, %v", ps, err)
	}
	if ps, err := svc.Search(ctx, "KEYBOARD", 10); err != nil || len(ps) != 1 || ps[0].ID != 1 {
		t.Errorf("search = %+v, %v", ps, err)
	}

	in := product.Input{Name: "Monitor", Category: "Displays", Price: 199.99, Stock: 5, Rating: 4.0}
	created, err := svc.Create(ctx, in)
	if err != nil || created.ID != 3 {
		t.Fatalf("create = %+v, %v", created, err)
	}
	in.Stock = 4
	if updated, err := svc.Update(ctx, created.ID, in); err != nil || updated.Stock != 4 {
		t.Errorf("update = %+v, %v", updated, err)
	}
	if err := svc.Delete(ctx, created.ID); err != nil {
		t.Errorf("delete = %v", err)
	}
	if err := svc.Delete(ctx, created.ID); !errors.Is(err, product.ErrNotFound) {
		t.Errorf("delete missing = %v, want ErrNotFound", err)
	}
}
