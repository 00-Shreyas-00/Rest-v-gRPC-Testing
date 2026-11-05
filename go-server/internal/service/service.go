// Package service is the single place where product business rules run.
// Both the REST and gRPC handlers call it; neither handler talks to a
// repository directly or re-implements validation.
package service

import (
	"context"

	"go-vs-ts-benchmark/go-server/internal/product"
)

// Repository is the data source contract. It is implemented by
// repository.Postgres and repository.Memory.
//
// Implementations must return product.ErrNotFound for unknown ids and a
// non-nil (possibly empty) slice from List and Search.
type Repository interface {
	Get(ctx context.Context, id int64) (product.Product, error)
	List(ctx context.Context, limit, offset int) ([]product.Product, error)
	Search(ctx context.Context, query string, limit int) ([]product.Product, error)
	Create(ctx context.Context, in product.Input) (product.Product, error)
	Update(ctx context.Context, id int64, in product.Input) (product.Product, error)
	Delete(ctx context.Context, id int64) error
}

// Service validates requests and delegates to a Repository.
//
// Errors: *product.ValidationError for invalid input (repository not called),
// product.ErrNotFound for unknown ids, anything else is an internal error.
type Service struct {
	repo Repository
}

// New returns a Service backed by repo.
func New(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Get(ctx context.Context, id int64) (product.Product, error) {
	if err := product.ValidateID(id); err != nil {
		return product.Product{}, err
	}
	return s.repo.Get(ctx, id)
}

// List returns products ordered by id. Callers apply product.DefaultListLimit
// when the client did not send a limit.
func (s *Service) List(ctx context.Context, limit, offset int) ([]product.Product, error) {
	if err := product.ValidatePage(limit, offset); err != nil {
		return nil, err
	}
	return s.repo.List(ctx, limit, offset)
}

// Search returns up to limit products whose name contains query
// (case-insensitive), ordered by id. Callers apply product.DefaultSearchLimit
// when the client did not send a limit.
func (s *Service) Search(ctx context.Context, query string, limit int) ([]product.Product, error) {
	if err := product.ValidateSearch(query, limit); err != nil {
		return nil, err
	}
	return s.repo.Search(ctx, query, limit)
}

func (s *Service) Create(ctx context.Context, in product.Input) (product.Product, error) {
	if err := in.Validate(); err != nil {
		return product.Product{}, err
	}
	return s.repo.Create(ctx, in)
}

func (s *Service) Update(ctx context.Context, id int64, in product.Input) (product.Product, error) {
	if err := joinValidation(product.ValidateID(id), in.Validate()); err != nil {
		return product.Product{}, err
	}
	return s.repo.Update(ctx, id, in)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := product.ValidateID(id); err != nil {
		return err
	}
	return s.repo.Delete(ctx, id)
}

// joinValidation merges validation errors, preserving order.
func joinValidation(errs ...error) error {
	var fields []product.FieldError
	for _, err := range errs {
		if ve, ok := err.(*product.ValidationError); ok {
			fields = append(fields, ve.Fields...)
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return &product.ValidationError{Fields: fields}
}
