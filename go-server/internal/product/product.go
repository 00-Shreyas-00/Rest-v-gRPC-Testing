// Package product holds the Product domain model, its input types and the
// validation rules shared by every transport (REST, gRPC) and data source.
//
// The TypeScript server implements the same rules; keep the two in sync.
package product

import (
	"errors"
	"time"
)

// Product is a catalog entry. JSON tags define the REST wire format, so the
// REST handler can encode it directly without an intermediate DTO.
type Product struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Price       float64   `json:"price"`
	Stock       int32     `json:"stock"`
	Rating      float64   `json:"rating"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Input is the writable part of a Product, used by both create and update
// (update replaces all writable fields, i.e. PUT semantics).
type Input struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Category    string  `json:"category"`
	Price       float64 `json:"price"`
	Stock       int32   `json:"stock"`
	Rating      float64 `json:"rating"`
}

// ErrNotFound is returned when no product exists with the requested id.
var ErrNotFound = errors.New("product not found")
