package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-vs-ts-benchmark/go-server/internal/product"
)

// Unit tests for behaviour specific to the in-memory store. Behaviour shared
// with PostgreSQL is covered by contract_test.go.

func TestMemorySortsInputAndContinuesIDs(t *testing.T) {
	m := NewMemory([]product.Product{{ID: 7, Name: "b"}, {ID: 3, Name: "a"}})
	ctx := context.Background()

	got, _ := m.List(ctx, 10, 0)
	if len(got) != 2 || got[0].ID != 3 || got[1].ID != 7 {
		t.Fatalf("list not sorted by id: %+v", got)
	}

	created, _ := m.Create(ctx, product.Input{Name: "c", Category: "x"})
	if created.ID != 8 {
		t.Errorf("created id = %d, want 8", created.ID)
	}
	// Ids are never reused, even after deleting the highest one.
	_ = m.Delete(ctx, 8)
	created, _ = m.Create(ctx, product.Input{Name: "d", Category: "x"})
	if created.ID != 9 {
		t.Errorf("created id after delete = %d, want 9", created.ID)
	}
}

func TestMemoryEmptyStore(t *testing.T) {
	m := NewMemory(nil)
	ctx := context.Background()
	if p, _ := m.Create(ctx, product.Input{Name: "a", Category: "x"}); p.ID != 1 {
		t.Errorf("first id = %d, want 1", p.ID)
	}
	if _, err := m.Get(ctx, 99); !errors.Is(err, product.ErrNotFound) {
		t.Errorf("get missing: %v", err)
	}
}

func TestMemoryTimestamps(t *testing.T) {
	m := NewMemory(nil)
	ctx := context.Background()
	clock := time.Date(2025, 1, 2, 3, 4, 5, 123456789, time.FixedZone("X", 3600))
	m.now = func() time.Time { return clock }

	p, _ := m.Create(ctx, product.Input{Name: "a", Category: "x"})
	want := time.Date(2025, 1, 2, 2, 4, 5, 123456000, time.UTC) // UTC, microsecond precision
	if !p.CreatedAt.Equal(want) || p.CreatedAt.Location() != time.UTC || p.UpdatedAt != p.CreatedAt {
		t.Errorf("timestamps = %v / %v, want %v", p.CreatedAt, p.UpdatedAt, want)
	}

	clock = clock.Add(time.Hour)
	u, _ := m.Update(ctx, p.ID, product.Input{Name: "b", Category: "x"})
	if !u.CreatedAt.Equal(p.CreatedAt) || !u.UpdatedAt.Equal(want.Add(time.Hour)) {
		t.Errorf("update timestamps = %v / %v", u.CreatedAt, u.UpdatedAt)
	}
}

func TestMemoryUpdateRefreshesSearchIndex(t *testing.T) {
	m := NewMemory([]product.Product{{ID: 1, Name: "Old Keyboard"}})
	ctx := context.Background()
	_, _ = m.Update(ctx, 1, product.Input{Name: "New Mouse", Category: "x"})
	if got, _ := m.Search(ctx, "keyboard", 10); len(got) != 0 {
		t.Errorf("stale name still searchable: %+v", got)
	}
	if got, _ := m.Search(ctx, "MOUSE", 10); len(got) != 1 {
		t.Errorf("new name not searchable: %+v", got)
	}
}

func TestLikePattern(t *testing.T) {
	tests := map[string]string{
		"keyboard": `%keyboard%`,
		"50%":      `%50\%%`,
		"a_b":      `%a\_b%`,
		`c:\x`:     `%c:\\x%`,
	}
	for in, want := range tests {
		if got := LikePattern(in); got != want {
			t.Errorf("LikePattern(%q) = %q, want %q", in, got, want)
		}
	}
}
