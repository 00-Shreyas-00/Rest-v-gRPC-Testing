// Package repository provides the two product data sources used in the
// benchmark: PostgreSQL (DATA_SOURCE=postgres) and an in-memory snapshot of
// the same table (DATA_SOURCE=memory). Both have identical observable behaviour.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"go-vs-ts-benchmark/go-server/internal/product"
)

// Canonical SQL. The TypeScript server must use these statements verbatim
// (same text, same parameter order) so both servers produce the same plans.
const (
	columns = `id, name, description, category, price, stock, rating, created_at, updated_at`

	SQLGet    = `SELECT ` + columns + ` FROM products WHERE id = $1`
	SQLList   = `SELECT ` + columns + ` FROM products ORDER BY id LIMIT $1 OFFSET $2`
	SQLSearch = `SELECT ` + columns + ` FROM products WHERE name ILIKE $1 ORDER BY id LIMIT $2`
	SQLCreate = `INSERT INTO products (name, description, category, price, stock, rating)
VALUES ($1, $2, $3, $4, $5, $6) RETURNING ` + columns
	SQLUpdate = `UPDATE products
SET name = $2, description = $3, category = $4, price = $5, stock = $6, rating = $7, updated_at = now()
WHERE id = $1 RETURNING ` + columns
	SQLDelete = `DELETE FROM products WHERE id = $1`
	SQLAll    = `SELECT ` + columns + ` FROM products ORDER BY id`
)

// OpenPool connects to PostgreSQL with a fixed maximum pool size and verifies
// the connection. All server implementations use the same pool size.
func OpenPool(ctx context.Context, url string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = maxConns
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Postgres is the PostgreSQL-backed repository.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres returns a repository using pool. The caller owns the pool.
func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func scanProduct(row pgx.Row, p *product.Product) error {
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Category,
		&p.Price, &p.Stock, &p.Rating, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return err
	}
	// Normalise to UTC regardless of the process time zone.
	p.CreatedAt = p.CreatedAt.UTC()
	p.UpdatedAt = p.UpdatedAt.UTC()
	return nil
}

func (r *Postgres) queryOne(ctx context.Context, sql string, args ...any) (product.Product, error) {
	var p product.Product
	err := scanProduct(r.pool.QueryRow(ctx, sql, args...), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return product.Product{}, product.ErrNotFound
	}
	return p, err
}

func (r *Postgres) queryMany(ctx context.Context, capacity int, sql string, args ...any) ([]product.Product, error) {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]product.Product, 0, capacity) // never nil: encodes as [] not null
	for rows.Next() {
		var p product.Product
		if err := scanProduct(rows, &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Postgres) Get(ctx context.Context, id int64) (product.Product, error) {
	return r.queryOne(ctx, SQLGet, id)
}

func (r *Postgres) List(ctx context.Context, limit, offset int) ([]product.Product, error) {
	return r.queryMany(ctx, limit, SQLList, limit, offset)
}

func (r *Postgres) Search(ctx context.Context, query string, limit int) ([]product.Product, error) {
	return r.queryMany(ctx, 0, SQLSearch, LikePattern(query), limit)
}

func (r *Postgres) Create(ctx context.Context, in product.Input) (product.Product, error) {
	return r.queryOne(ctx, SQLCreate, in.Name, in.Description, in.Category, in.Price, in.Stock, in.Rating)
}

func (r *Postgres) Update(ctx context.Context, id int64, in product.Input) (product.Product, error) {
	return r.queryOne(ctx, SQLUpdate, id, in.Name, in.Description, in.Category, in.Price, in.Stock, in.Rating)
}

func (r *Postgres) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, SQLDelete, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return product.ErrNotFound
	}
	return nil
}

// All returns every product ordered by id. Used once at startup to build the
// in-memory snapshot; never called on the request path.
func (r *Postgres) All(ctx context.Context) ([]product.Product, error) {
	return r.queryMany(ctx, 0, SQLAll)
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// LikePattern turns a user query into an ILIKE pattern that matches it as a
// literal substring: LIKE wildcards in the query are escaped.
func LikePattern(query string) string {
	return "%" + likeEscaper.Replace(query) + "%"
}
