package product

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func validInput() Input {
	return Input{Name: "Desk Lamp", Description: "", Category: "Office", Price: 19.99, Stock: 0, Rating: 4.5}
}

func fieldErrors(t *testing.T, err error) []FieldError {
	t.Helper()
	if err == nil {
		return nil
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	return ve.Fields
}

func TestInputValidate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Input)
		want   []FieldError
	}{
		{"valid", func(*Input) {}, nil},
		{"max lengths", func(in *Input) {
			in.Name = strings.Repeat("a", MaxNameLen)
			in.Description = strings.Repeat("é", MaxDescriptionLen) // counted in code points, not bytes
			in.Category = strings.Repeat("c", MaxCategoryLen)
		}, nil},
		{"boundary numbers", func(in *Input) { in.Price = MaxPrice; in.Rating = 5; in.Stock = math.MaxInt32 }, nil},
		{"zero numbers", func(in *Input) { in.Price = 0; in.Rating = 0 }, nil},
		{"empty name", func(in *Input) { in.Name = "" }, []FieldError{{"name", "is required"}}},
		{"long name", func(in *Input) { in.Name = strings.Repeat("a", MaxNameLen+1) },
			[]FieldError{{"name", "must be between 1 and 200 characters"}}},
		{"long description", func(in *Input) { in.Description = strings.Repeat("a", MaxDescriptionLen+1) },
			[]FieldError{{"description", "must be between 0 and 2000 characters"}}},
		{"empty category", func(in *Input) { in.Category = "" }, []FieldError{{"category", "is required"}}},
		{"negative price", func(in *Input) { in.Price = -0.01 }, []FieldError{{"price", "must be between 0 and 99999999.99"}}},
		{"price too large", func(in *Input) { in.Price = 100_000_000 }, []FieldError{{"price", "must be between 0 and 99999999.99"}}},
		{"price NaN", func(in *Input) { in.Price = math.NaN() }, []FieldError{{"price", "must be between 0 and 99999999.99"}}},
		{"price 3 decimals", func(in *Input) { in.Price = 1.005 }, []FieldError{{"price", "must have at most 2 decimal places"}}},
		{"negative stock", func(in *Input) { in.Stock = -1 }, []FieldError{{"stock", "must be greater than or equal to 0"}}},
		{"rating too large", func(in *Input) { in.Rating = 5.1 }, []FieldError{{"rating", "must be between 0 and 5"}}},
		{"rating 2 decimals", func(in *Input) { in.Rating = 4.25 }, []FieldError{{"rating", "must have at most 1 decimal places"}}},
		{"all fields, in field order", func(in *Input) { *in = Input{Price: -1, Stock: -1, Rating: -1} }, []FieldError{
			{"name", "is required"},
			{"category", "is required"},
			{"price", "must be between 0 and 99999999.99"},
			{"stock", "must be greater than or equal to 0"},
			{"rating", "must be between 0 and 5"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validInput()
			tt.mutate(&in)
			if got := fieldErrors(t, in.Validate()); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTwoDecimalPricesAreAccepted(t *testing.T) {
	// Every cent value must pass the decimal-places check (guards against
	// floating-point false negatives such as 0.29*100 = 28.999999999999996).
	for cents := 0; cents <= 100_000; cents++ {
		in := validInput()
		in.Price = float64(cents) / 100
		if err := in.Validate(); err != nil {
			t.Fatalf("price %v rejected: %v", in.Price, err)
		}
	}
}

func TestValidateID(t *testing.T) {
	for _, id := range []int64{1, MaxID} {
		if err := ValidateID(id); err != nil {
			t.Errorf("id %d: %v", id, err)
		}
	}
	for _, id := range []int64{0, -5, MaxID + 1} {
		if got := fieldErrors(t, ValidateID(id)); !reflect.DeepEqual(got, []FieldError{{"id", IDMessage}}) {
			t.Errorf("id %d: got %v", id, got)
		}
	}
}

func TestValidatePage(t *testing.T) {
	for _, ok := range [][2]int{{1, 0}, {MaxLimit, MaxOffset}} {
		if err := ValidatePage(ok[0], ok[1]); err != nil {
			t.Errorf("limit=%d offset=%d: %v", ok[0], ok[1], err)
		}
	}
	got := fieldErrors(t, ValidatePage(0, -1))
	want := []FieldError{{"limit", "must be between 1 and 1000"}, {"offset", "must be between 0 and 1000000000"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if fieldErrors(t, ValidatePage(MaxLimit+1, 0)) == nil {
		t.Error("limit above max accepted")
	}
}

func TestValidateSearch(t *testing.T) {
	if err := ValidateSearch("keyboard", DefaultSearchLimit); err != nil {
		t.Errorf("valid search: %v", err)
	}
	got := fieldErrors(t, ValidateSearch("", 0))
	want := []FieldError{{"q", "is required"}, {"limit", "must be between 1 and 1000"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if fieldErrors(t, ValidateSearch(strings.Repeat("q", MaxQueryLen+1), 10)) == nil {
		t.Error("over-long query accepted")
	}
}
