package repository

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"go-vs-ts-benchmark/go-server/internal/product"
)

// Integration tests. They run against the products_test database
// (scripts/db.sh testdb: 1000 seeded products) and are skipped when
// TEST_DATABASE_URL is not set.
//
// The same contract runs against PostgreSQL and against an in-memory store
// loaded from it, which is what guarantees the two DATA_SOURCE modes behave
// identically.

const seededCount = 1000

type repo interface {
	Get(ctx context.Context, id int64) (product.Product, error)
	List(ctx context.Context, limit, offset int) ([]product.Product, error)
	Search(ctx context.Context, query string, limit int) ([]product.Product, error)
	Create(ctx context.Context, in product.Input) (product.Product, error)
	Update(ctx context.Context, id int64, in product.Input) (product.Product, error)
	Delete(ctx context.Context, id int64) error
}

func openTestPostgres(t *testing.T) *Postgres {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; run scripts/db.sh testdb and set it to run integration tests")
	}
	ctx := context.Background()
	pool, err := OpenPool(ctx, url, 4)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM products").Scan(&n); err != nil || n != seededCount {
		t.Fatalf("expected %d seeded products (run scripts/db.sh testdb), got %d (err=%v)", seededCount, n, err)
	}
	return NewPostgres(pool)
}

func TestContract(t *testing.T) {
	pg := openTestPostgres(t)
	mem, err := LoadMemory(context.Background(), pg)
	if err != nil {
		t.Fatalf("load memory: %v", err)
	}
	t.Run("postgres", func(t *testing.T) { runContract(t, pg) })
	t.Run("memory", func(t *testing.T) { runContract(t, mem) })
}

// TestPostgresAndMemoryReturnIdenticalData compares read results field by field.
func TestPostgresAndMemoryReturnIdenticalData(t *testing.T) {
	pg := openTestPostgres(t)
	ctx := context.Background()
	mem, err := LoadMemory(ctx, pg)
	if err != nil {
		t.Fatalf("load memory: %v", err)
	}

	for _, id := range []int64{1, 2, 3, 500, seededCount} {
		a, errA := pg.Get(ctx, id)
		b, errB := mem.Get(ctx, id)
		if errA != nil || errB != nil || !reflect.DeepEqual(a, b) {
			t.Errorf("Get(%d): postgres=%+v (%v) memory=%+v (%v)", id, a, errA, b, errB)
		}
	}
	for _, pg2 := range [][2]int{{10, 0}, {50, 0}, {100, 250}, {500, 0}, {1000, 0}, {10, 995}, {10, 5000}} {
		a, _ := pg.List(ctx, pg2[0], pg2[1])
		b, _ := mem.List(ctx, pg2[0], pg2[1])
		if !reflect.DeepEqual(a, b) {
			t.Errorf("List(%d,%d) differs: %d vs %d rows", pg2[0], pg2[1], len(a), len(b))
		}
	}
	for _, q := range []string{"keyboard", "phone", "laptop", "monitor", "mouse", "KeYbOaRd", "ke-0000", "%", "_", "zzz"} {
		a, _ := pg.Search(ctx, q, product.DefaultSearchLimit)
		b, _ := mem.Search(ctx, q, product.DefaultSearchLimit)
		if !reflect.DeepEqual(a, b) {
			t.Errorf("Search(%q) differs: %d vs %d rows", q, len(a), len(b))
		}
	}
}

func ids(ps []product.Product) []int64 {
	out := make([]int64, len(ps))
	for i, p := range ps {
		out[i] = p.ID
	}
	return out
}

func runContract(t *testing.T, r repo) {
	ctx := context.Background()

	t.Run("get seeded product", func(t *testing.T) {
		got, err := r.Get(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		want := product.Product{
			ID:          1,
			Name:        "Orion Smart Phone PH-000001",
			Description: "The Orion smart phone is designed for home offices. Firmware updates are delivered automatically.",
			Category:    "Mobile",
			Price:       80.18,
			Stock:       37,
			Rating:      2.3,
			CreatedAt:   time.Date(2024, 1, 1, 0, 5, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2024, 1, 1, 7, 5, 0, 0, time.UTC),
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got  %+v\nwant %+v", got, want)
		}
	})

	t.Run("get missing", func(t *testing.T) {
		if _, err := r.Get(ctx, 999_999_999); !errors.Is(err, product.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("list pages in id order", func(t *testing.T) {
		got, err := r.List(ctx, 5, 0)
		if err != nil || !reflect.DeepEqual(ids(got), []int64{1, 2, 3, 4, 5}) {
			t.Errorf("first page = %v (%v)", ids(got), err)
		}
		got, _ = r.List(ctx, 5, seededCount-2)
		if !reflect.DeepEqual(ids(got), []int64{seededCount - 1, seededCount}) {
			t.Errorf("last page = %v", ids(got))
		}
		got, _ = r.List(ctx, 5, seededCount+10)
		if got == nil || len(got) != 0 {
			t.Errorf("past the end = %#v, want empty non-nil slice", got)
		}
	})

	t.Run("search", func(t *testing.T) {
		got, err := r.Search(ctx, "keyboard", 3)
		if err != nil || !reflect.DeepEqual(ids(got), []int64{20, 40, 60}) {
			t.Errorf("keyboard = %v (%v)", ids(got), err)
		}
		// Case-insensitive; "phone" also matches Headphones (i%20=5) and Microphone (i%20=12).
		got, _ = r.Search(ctx, "PHONE", 4)
		if !reflect.DeepEqual(ids(got), []int64{1, 5, 12, 21}) {
			t.Errorf("PHONE = %v", ids(got))
		}
		got, _ = r.Search(ctx, "mouse", product.DefaultSearchLimit)
		if len(got) != product.DefaultSearchLimit {
			t.Errorf("mouse returned %d rows, want %d", len(got), product.DefaultSearchLimit)
		}
		for _, p := range got {
			if !strings.Contains(strings.ToLower(p.Name), "mouse") {
				t.Errorf("non-matching result %q", p.Name)
			}
		}
		// LIKE wildcards are literal characters, not patterns.
		for _, q := range []string{"%", "_", "no-such-product"} {
			got, _ = r.Search(ctx, q, 10)
			if got == nil || len(got) != 0 {
				t.Errorf("Search(%q) = %#v, want empty non-nil slice", q, got)
			}
		}
	})

	t.Run("create, update, delete", func(t *testing.T) {
		in := product.Input{Name: "Contract Test Lamp", Description: "temp", Category: "Office", Price: 12.5, Stock: 3, Rating: 4.5}
		created, err := r.Create(ctx, in)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = r.Delete(ctx, created.ID) })

		if created.ID <= seededCount || created.Name != in.Name || created.Price != in.Price ||
			created.Stock != in.Stock || created.Rating != in.Rating || created.Description != in.Description {
			t.Errorf("created = %+v", created)
		}
		if created.CreatedAt.IsZero() || !created.CreatedAt.Equal(created.UpdatedAt) || created.CreatedAt.Location() != time.UTC {
			t.Errorf("created timestamps = %v / %v", created.CreatedAt, created.UpdatedAt)
		}
		if got, _ := r.Get(ctx, created.ID); !reflect.DeepEqual(got, created) {
			t.Errorf("get after create = %+v, want %+v", got, created)
		}

		upd := product.Input{Name: "Contract Test Lamp v2", Description: "", Category: "Home", Price: 0, Stock: 0, Rating: 0}
		updated, err := r.Update(ctx, created.ID, upd)
		if err != nil {
			t.Fatal(err)
		}
		if updated.ID != created.ID || updated.Name != upd.Name || updated.Category != upd.Category ||
			updated.Price != 0 || updated.Stock != 0 || updated.Rating != 0 || updated.Description != "" {
			t.Errorf("updated = %+v", updated)
		}
		if !updated.CreatedAt.Equal(created.CreatedAt) || updated.UpdatedAt.Before(created.UpdatedAt) {
			t.Errorf("update timestamps = %v / %v", updated.CreatedAt, updated.UpdatedAt)
		}

		if err := r.Delete(ctx, created.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := r.Get(ctx, created.ID); !errors.Is(err, product.ErrNotFound) {
			t.Errorf("get after delete: %v", err)
		}
		if _, err := r.Update(ctx, created.ID, upd); !errors.Is(err, product.ErrNotFound) {
			t.Errorf("update after delete: %v", err)
		}
		if err := r.Delete(ctx, created.ID); !errors.Is(err, product.ErrNotFound) {
			t.Errorf("delete twice: %v", err)
		}
	})
}
