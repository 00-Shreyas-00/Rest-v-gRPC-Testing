package product

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Limits shared by all implementations. String lengths are counted in Unicode
// code points (not bytes, not UTF-16 units) to match PostgreSQL char_length().
const (
	MaxNameLen        = 200
	MaxDescriptionLen = 2000
	MaxCategoryLen    = 100
	MaxPrice          = 99_999_999.99 // NUMERIC(10,2)
	MaxRating         = 5.0
	MaxQueryLen       = 100
	MaxID             = 1<<53 - 1

	DefaultListLimit   = 20
	DefaultSearchLimit = 50
	MaxLimit           = 1000
	MaxOffset          = 1_000_000_000 // keeps OFFSET within int32 range everywhere
)

// FieldError describes one invalid field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError lists every invalid field, in a fixed field order so that
// all implementations report identical errors for identical input.
type ValidationError struct {
	Fields []FieldError `json:"fields"`
}

func (e *ValidationError) Error() string {
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + f.Message
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

type validator struct{ fields []FieldError }

func (v *validator) add(field, format string, args ...any) {
	v.fields = append(v.fields, FieldError{Field: field, Message: fmt.Sprintf(format, args...)})
}

func (v *validator) err() error {
	if len(v.fields) == 0 {
		return nil
	}
	return &ValidationError{Fields: v.fields}
}

func (v *validator) stringLen(field, s string, minLen, maxLen int) {
	n := utf8.RuneCountInString(s)
	switch {
	case n < minLen && minLen == 1:
		v.add(field, "is required")
	case n < minLen || n > maxLen:
		v.add(field, "must be between %d and %d characters", minLen, maxLen)
	}
}

// decimal checks 0 <= x <= maxVal with at most `places` decimal places.
// Excess precision is rejected rather than rounded, because PostgreSQL NUMERIC
// would round it silently while the in-memory store would not.
//
// round(x*10^p)/10^p == x holds exactly for any double parsed from a decimal
// literal with <= p places, and the same IEEE-754 arithmetic runs in JS.
func (v *validator) decimal(field string, x, maxVal float64, places int) {
	if math.IsNaN(x) || math.IsInf(x, 0) || x < 0 || x > maxVal {
		v.add(field, "must be between 0 and %s", strconv.FormatFloat(maxVal, 'f', -1, 64))
		return
	}
	scale := math.Pow10(places)
	if math.Round(x*scale)/scale != x {
		v.add(field, "must have at most %d decimal places", places)
	}
}

// Validate checks a create/update payload.
func (in Input) Validate() error {
	var v validator
	v.stringLen("name", in.Name, 1, MaxNameLen)
	v.stringLen("description", in.Description, 0, MaxDescriptionLen)
	v.stringLen("category", in.Category, 1, MaxCategoryLen)
	v.decimal("price", in.Price, MaxPrice, 2)
	if in.Stock < 0 {
		v.add("stock", "must be greater than or equal to 0")
	}
	v.decimal("rating", in.Rating, MaxRating, 1)
	return v.err()
}

// IDMessage is the error for any unusable id, including non-numeric path values.
const IDMessage = "must be an integer between 1 and 9007199254740991"

// ValidateID checks a product id. The upper bound is 2^53-1 (JavaScript's
// Number.MAX_SAFE_INTEGER) so that every implementation accepts the same ids.
func ValidateID(id int64) error {
	if id < 1 || id > MaxID {
		return &ValidationError{Fields: []FieldError{{Field: "id", Message: IDMessage}}}
	}
	return nil
}

// ValidatePage checks list pagination.
func ValidatePage(limit, offset int) error {
	var v validator
	if limit < 1 || limit > MaxLimit {
		v.add("limit", "must be between 1 and %d", MaxLimit)
	}
	if offset < 0 || offset > MaxOffset {
		v.add("offset", "must be between 0 and %d", MaxOffset)
	}
	return v.err()
}

// ValidateSearch checks search parameters.
func ValidateSearch(query string, limit int) error {
	var v validator
	v.stringLen("q", query, 1, MaxQueryLen)
	if limit < 1 || limit > MaxLimit {
		v.add("limit", "must be between 1 and %d", MaxLimit)
	}
	return v.err()
}
