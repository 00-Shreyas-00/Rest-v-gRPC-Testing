package rest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"go-vs-ts-benchmark/go-server/internal/product"
	"go-vs-ts-benchmark/go-server/internal/repository"
	"go-vs-ts-benchmark/go-server/internal/service"
)

// seedProduct1 is product 1 of the deterministic seed. Its JSON encoding
// (want1) is the byte-exact reference the TypeScript server must also produce.
var seedProduct1 = product.Product{
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

const want1 = `{"id":1,"name":"Orion Smart Phone PH-000001",` +
	`"description":"The Orion smart phone is designed for home offices. Firmware updates are delivered automatically.",` +
	`"category":"Mobile","price":80.18,"stock":37,"rating":2.3,` +
	`"created_at":"2024-01-01T00:05:00Z","updated_at":"2024-01-01T07:05:00Z"}`

func newServer(t *testing.T, health func(context.Context) error) *httptest.Server {
	t.Helper()
	mem := repository.NewMemory([]product.Product{
		seedProduct1,
		{ID: 2, Name: "Quanta Max Laptop LA-000002", Category: "Computers", Price: 159.37, Stock: 74, Rating: 3.6,
			CreatedAt: seedProduct1.CreatedAt, UpdatedAt: seedProduct1.UpdatedAt},
		{ID: 3, Name: "Nimbus Ultra Monitor MO-000003", Category: "Displays", Price: 238.56, Stock: 111, Rating: 4.9,
			CreatedAt: seedProduct1.CreatedAt, UpdatedAt: seedProduct1.UpdatedAt},
	})
	if health == nil {
		health = func(context.Context) error { return nil }
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(NewHandler(service.New(mem), health, log, false))
	t.Cleanup(srv.Close)
	return srv
}

type response struct {
	status int
	header http.Header
	body   string
}

func do(t *testing.T, srv *httptest.Server, method, path, body string) response {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return response{status: res.StatusCode, header: res.Header, body: string(b)}
}

func expect(t *testing.T, got response, status int, body string) {
	t.Helper()
	if got.status != status || got.body != body {
		t.Errorf("got %d %s\nwant %d %s", got.status, got.body, status, body)
	}
}

const (
	errNotFound = `{"error":"product not found"}`
	errBadBody  = `{"error":"invalid request body"}`
	errBadID    = `{"error":"validation failed","fields":[{"field":"id","message":"must be an integer between 1 and 9007199254740991"}]}`
)

func TestGetProduct(t *testing.T) {
	srv := newServer(t, nil)

	res := do(t, srv, "GET", "/products/1", "")
	expect(t, res, 200, want1)
	if ct := res.header.Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cl := res.header.Get("Content-Length"); cl != strconv.Itoa(len(want1)) {
		t.Errorf("Content-Length = %q, want %d", cl, len(want1))
	}

	expect(t, do(t, srv, "GET", "/products/999", ""), 404, errNotFound)
	for _, id := range []string{"0", "-1", "abc", "1.5", "9007199254740992", "99999999999999999999"} {
		expect(t, do(t, srv, "GET", "/products/"+id, ""), 400, errBadID)
	}
}

func listIDs(t *testing.T, body string) []int64 {
	t.Helper()
	var pl ProductList
	if err := json.Unmarshal([]byte(body), &pl); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	ids := make([]int64, len(pl.Products))
	for i, p := range pl.Products {
		ids[i] = p.ID
	}
	return ids
}

func TestListProducts(t *testing.T) {
	srv := newServer(t, nil)

	res := do(t, srv, "GET", "/products?limit=1", "")
	expect(t, res, 200, `{"products":[`+want1+`]}`)

	if ids := listIDs(t, do(t, srv, "GET", "/products", "").body); len(ids) != 3 {
		t.Errorf("default list ids = %v", ids)
	}
	if ids := listIDs(t, do(t, srv, "GET", "/products?limit=2&offset=1", "").body); len(ids) != 2 || ids[0] != 2 || ids[1] != 3 {
		t.Errorf("paged ids = %v", ids)
	}
	// First value wins for repeated parameters.
	if ids := listIDs(t, do(t, srv, "GET", "/products?limit=1&limit=3", "").body); len(ids) != 1 {
		t.Errorf("repeated limit ids = %v", ids)
	}
	expect(t, do(t, srv, "GET", "/products?offset=100", ""), 200, `{"products":[]}`)

	expect(t, do(t, srv, "GET", "/products?limit=0&offset=-1", ""), 400,
		`{"error":"validation failed","fields":[{"field":"limit","message":"must be between 1 and 1000"},{"field":"offset","message":"must be between 0 and 1000000000"}]}`)
	expect(t, do(t, srv, "GET", "/products?limit=99999999999999999999", ""), 400,
		`{"error":"validation failed","fields":[{"field":"limit","message":"must be between 1 and 1000"}]}`)
	// Parse errors are reported alone, before range validation.
	expect(t, do(t, srv, "GET", "/products?limit=abc&offset=-1", ""), 400,
		`{"error":"validation failed","fields":[{"field":"limit","message":"must be an integer"}]}`)
	expect(t, do(t, srv, "GET", "/products?limit=&offset=1.5", ""), 400,
		`{"error":"validation failed","fields":[{"field":"limit","message":"must be an integer"},{"field":"offset","message":"must be an integer"}]}`)
}

func TestSearchProducts(t *testing.T) {
	srv := newServer(t, nil)

	expect(t, do(t, srv, "GET", "/products/search?q=PHONE", ""), 200, `{"products":[`+want1+`]}`)
	if ids := listIDs(t, do(t, srv, "GET", "/products/search?q=o&limit=2", "").body); len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Errorf("limited search ids = %v", ids)
	}
	expect(t, do(t, srv, "GET", "/products/search?q=%25", ""), 200, `{"products":[]}`)
	expect(t, do(t, srv, "GET", "/products/search", ""), 400,
		`{"error":"validation failed","fields":[{"field":"q","message":"is required"}]}`)
	expect(t, do(t, srv, "GET", "/products/search?q=x&limit=1001", ""), 400,
		`{"error":"validation failed","fields":[{"field":"limit","message":"must be between 1 and 1000"}]}`)
}

func TestCreateUpdateDelete(t *testing.T) {
	srv := newServer(t, nil)

	res := do(t, srv, "POST", "/products",
		`{"name":"Desk <Lamp> & Co","description":"","category":"Office","price":19.99,"stock":5,"rating":4.5}`)
	if res.status != 201 {
		t.Fatalf("create: %d %s", res.status, res.body)
	}
	// No HTML escaping, same as JSON.stringify.
	if !strings.Contains(res.body, `"name":"Desk <Lamp> & Co"`) {
		t.Errorf("create body = %s", res.body)
	}
	var created product.Product
	if err := json.Unmarshal([]byte(res.body), &created); err != nil || created.ID != 4 {
		t.Fatalf("created = %+v, %v", created, err)
	}

	res = do(t, srv, "PUT", "/products/4", `{"name":"Desk Lamp","category":"Office","price":25,"stock":1,"rating":5}`)
	var updated product.Product
	if err := json.Unmarshal([]byte(res.body), &updated); res.status != 200 || err != nil ||
		updated.Name != "Desk Lamp" || updated.Price != 25 || !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("update: %d %s", res.status, res.body)
	}

	expect(t, do(t, srv, "DELETE", "/products/4", ""), 204, "")
	expect(t, do(t, srv, "DELETE", "/products/4", ""), 404, errNotFound)
	expect(t, do(t, srv, "PUT", "/products/4", `{"name":"x","category":"y"}`), 404, errNotFound)
	expect(t, do(t, srv, "DELETE", "/products/x", ""), 400, errBadID)
}

func TestRequestBodyErrors(t *testing.T) {
	srv := newServer(t, nil)

	for name, body := range map[string]string{
		"malformed":      `{"name":`,
		"array":          `[]`,
		"unknown field":  `{"name":"a","category":"b","color":"red"}`,
		"wrong type":     `{"name":"a","category":"b","price":"10"}`,
		"fractional int": `{"name":"a","category":"b","stock":1.5}`,
		"trailing data":  `{"name":"a","category":"b"} {}`,
		"empty":          ``,
	} {
		t.Run(name, func(t *testing.T) {
			expect(t, do(t, srv, "POST", "/products", body), 400, errBadBody)
		})
	}

	// Absent fields default to zero values, then validation runs.
	expect(t, do(t, srv, "POST", "/products", `{}`), 400,
		`{"error":"validation failed","fields":[{"field":"name","message":"is required"},{"field":"category","message":"is required"}]}`)
	expect(t, do(t, srv, "POST", "/products", `null`), 400,
		`{"error":"validation failed","fields":[{"field":"name","message":"is required"},{"field":"category","message":"is required"}]}`)
	expect(t, do(t, srv, "POST", "/products", `{"name":"a","category":"b","price":1.005}`), 400,
		`{"error":"validation failed","fields":[{"field":"price","message":"must have at most 2 decimal places"}]}`)
	// An invalid path id is rejected before the body is read.
	expect(t, do(t, srv, "PUT", "/products/0", `{"name":"","category":"b"}`), 400,
		`{"error":"validation failed","fields":[{"field":"id","message":"must be an integer between 1 and 9007199254740991"}]}`)

	big := `{"name":"` + strings.Repeat("a", MaxBodyBytes) + `","category":"b"}`
	expect(t, do(t, srv, "POST", "/products", big), 413, `{"error":"request body too large"}`)
}

func TestRoutingAndHealth(t *testing.T) {
	srv := newServer(t, nil)
	expect(t, do(t, srv, "GET", "/health", ""), 200, `{"status":"ok"}`)
	expect(t, do(t, srv, "GET", "/nope", ""), 404, `{"error":"not found"}`)
	expect(t, do(t, srv, "GET", "/products/", ""), 404, `{"error":"not found"}`)
	expect(t, do(t, srv, "PATCH", "/products/1", `{}`), 404, `{"error":"not found"}`)

	down := newServer(t, func(context.Context) error { return errors.New("db down") })
	expect(t, do(t, down, "GET", "/health", ""), 503, `{"status":"unavailable"}`)
}
