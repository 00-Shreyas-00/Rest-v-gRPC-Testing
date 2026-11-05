package repository

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"go-vs-ts-benchmark/go-server/internal/product"
)

// Memory is an in-memory repository that mirrors PostgreSQL semantics:
//   - rows are kept sorted by id (same order as ORDER BY id)
//   - lookups by id use binary search
//   - search is a case-insensitive literal substring match on name
//     (equivalent to ILIKE for the ASCII seed data)
//   - new ids continue from the highest loaded id and are never reused,
//     like the identity sequence
//   - timestamps have microsecond precision, like TIMESTAMPTZ
//
// It is safe for concurrent use. The TypeScript server uses the same data
// structure and algorithms.
type Memory struct {
	mu     sync.RWMutex
	items  []memItem // sorted by product ID
	nextID int64
	now    func() time.Time
}

type memItem struct {
	p         product.Product
	lowerName string // precomputed once so search does not allocate per row
}

// NewMemory builds a store from products, which need not be sorted.
func NewMemory(products []product.Product) *Memory {
	m := &Memory{
		items:  make([]memItem, len(products)),
		nextID: 1,
		now:    time.Now,
	}
	for i, p := range products {
		m.items[i] = memItem{p: p, lowerName: strings.ToLower(p.Name)}
	}
	slices.SortFunc(m.items, func(a, b memItem) int { return cmp.Compare(a.p.ID, b.p.ID) })
	if n := len(m.items); n > 0 {
		m.nextID = m.items[n-1].p.ID + 1
	}
	return m
}

// LoadMemory snapshots the products table into a new in-memory store.
func LoadMemory(ctx context.Context, pg *Postgres) (*Memory, error) {
	products, err := pg.All(ctx)
	if err != nil {
		return nil, err
	}
	return NewMemory(products), nil
}

// Len returns the number of stored products.
func (m *Memory) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

func (m *Memory) find(id int64) (int, bool) {
	return slices.BinarySearchFunc(m.items, id, func(it memItem, id int64) int {
		return cmp.Compare(it.p.ID, id)
	})
}

func (m *Memory) timestamp() time.Time {
	return m.now().UTC().Truncate(time.Microsecond)
}

func (m *Memory) Get(_ context.Context, id int64) (product.Product, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	i, ok := m.find(id)
	if !ok {
		return product.Product{}, product.ErrNotFound
	}
	return m.items[i].p, nil
}

func (m *Memory) List(_ context.Context, limit, offset int) ([]product.Product, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	start := min(offset, len(m.items))
	end := min(start+limit, len(m.items))
	out := make([]product.Product, 0, end-start)
	for _, it := range m.items[start:end] {
		out = append(out, it.p)
	}
	return out, nil
}

func (m *Memory) Search(_ context.Context, query string, limit int) ([]product.Product, error) {
	q := strings.ToLower(query)
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]product.Product, 0)
	for _, it := range m.items {
		if strings.Contains(it.lowerName, q) {
			out = append(out, it.p)
			if len(out) == limit {
				break
			}
		}
	}
	return out, nil
}

func (m *Memory) Create(_ context.Context, in product.Input) (product.Product, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.timestamp()
	p := product.Product{
		ID:          m.nextID,
		Name:        in.Name,
		Description: in.Description,
		Category:    in.Category,
		Price:       in.Price,
		Stock:       in.Stock,
		Rating:      in.Rating,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.nextID++
	m.items = append(m.items, memItem{p: p, lowerName: strings.ToLower(p.Name)}) // ids only grow
	return p, nil
}

func (m *Memory) Update(_ context.Context, id int64, in product.Input) (product.Product, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	i, ok := m.find(id)
	if !ok {
		return product.Product{}, product.ErrNotFound
	}
	p := m.items[i].p
	p.Name = in.Name
	p.Description = in.Description
	p.Category = in.Category
	p.Price = in.Price
	p.Stock = in.Stock
	p.Rating = in.Rating
	p.UpdatedAt = m.timestamp()
	m.items[i] = memItem{p: p, lowerName: strings.ToLower(p.Name)}
	return p, nil
}

func (m *Memory) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	i, ok := m.find(id)
	if !ok {
		return product.ErrNotFound
	}
	m.items = slices.Delete(m.items, i, i+1)
	return nil
}
