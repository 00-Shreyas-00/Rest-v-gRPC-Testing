// Package equivalence is a black-box test of the three running servers.
// It establishes, before any benchmark, that they behave identically:
//
//  1. Go REST and TypeScript REST return byte-identical responses (status,
//     Content-Type, Content-Length, body) for every request in the corpus.
//  2. Every gRPC response, rendered with the REST encoding rules, equals the
//     Go REST response body for the equivalent request (same logical data,
//     same errors).
//  3. Create/update/delete flows behave the same on all three servers.
//
// The corpus includes every request the k6 benchmarks send, so equal results
// here mean the benchmarks compare equal work.
//
// Requires a freshly seeded 100k-product database and these variables
// (scripts/equivalence.sh sets them); skipped otherwise:
//
//	EQUIV_GO_REST_URL  e.g. http://localhost:8080
//	EQUIV_TS_REST_URL  e.g. http://localhost:8081
//	EQUIV_GRPC_ADDR    e.g. localhost:9090
package equivalence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go-vs-ts-benchmark/go-server/gen/productv1"
	"go-vs-ts-benchmark/go-server/internal/product"
)

const seededCount = 100_000

type env struct {
	goREST string
	tsREST string
	grpc   productv1.ProductServiceClient
	health healthgrpc.HealthClient
	http   *http.Client
}

func setup(t *testing.T) *env {
	t.Helper()
	goURL, tsURL, grpcAddr := os.Getenv("EQUIV_GO_REST_URL"), os.Getenv("EQUIV_TS_REST_URL"), os.Getenv("EQUIV_GRPC_ADDR")
	if goURL == "" || tsURL == "" || grpcAddr == "" {
		t.Skip("EQUIV_GO_REST_URL, EQUIV_TS_REST_URL and EQUIV_GRPC_ADDR not set; run scripts/equivalence.sh")
	}
	conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return &env{
		goREST: strings.TrimSuffix(goURL, "/"),
		tsREST: strings.TrimSuffix(tsURL, "/"),
		grpc:   productv1.NewProductServiceClient(conn),
		health: healthgrpc.NewHealthClient(conn),
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

// ---------------------------------------------------------------------------
// HTTP helpers

type httpResult struct {
	Status        int
	ContentType   string
	ContentLength string
	Chunked       bool
	Body          []byte
}

func (e *env) do(t *testing.T, method, base, path, body string) httpResult {
	t.Helper()
	r, err := e.try(method, base, path, body)
	if err != nil {
		t.Fatalf("%s %s%s: %v", method, base, path, err)
	}
	return r
}

// doOversized sends a body above the 1 MiB limit. Both servers answer 413 and
// close the connection, but Fastify closes it without draining the upload, so
// the client sometimes sees a broken pipe/reset instead of the response
// (Go drains a small remainder first). Retry transport errors and compare the
// first real response.
func (e *env) doOversized(t *testing.T, base, body string) httpResult {
	t.Helper()
	var lastErr error
	for range 20 {
		r, err := e.try("POST", base, "/products", body)
		if err == nil {
			return r
		}
		lastErr = err
	}
	t.Fatalf("POST oversized body to %s: no response in 20 attempts: %v", base, lastErr)
	return httpResult{}
}

func (e *env) try(method, base, path, body string) (httpResult, error) {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, base+path, rd)
	if err != nil {
		return httpResult{}, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := e.http.Do(req)
	if err != nil {
		return httpResult{}, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return httpResult{}, err
	}
	return httpResult{
		Status:        res.StatusCode,
		ContentType:   res.Header.Get("Content-Type"),
		ContentLength: res.Header.Get("Content-Length"),
		Chunked:       len(res.TransferEncoding) > 0,
		Body:          b,
	}, nil
}

// diff describes the first difference between two byte slices.
func diff(a, b []byte) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	snip := func(s []byte) string {
		lo, hi := max(0, i-40), min(len(s), i+40)
		return strconv.Quote(string(s[lo:hi]))
	}
	return fmt.Sprintf("first difference at byte %d (lengths %d vs %d)\n    go: %s\n    ts: %s", i, len(a), len(b), snip(a), snip(b))
}

func compareREST(t *testing.T, label string, g, s httpResult) {
	t.Helper()
	if g.Status != s.Status || g.ContentType != s.ContentType {
		t.Errorf("%s: status/content-type differ: go %d %q, ts %d %q", label, g.Status, g.ContentType, s.Status, s.ContentType)
	}
	if !bytes.Equal(g.Body, s.Body) {
		t.Errorf("%s: bodies differ: %s", label, diff(g.Body, s.Body))
	}
	if len(g.Body) > 0 && (g.ContentLength != strconv.Itoa(len(g.Body)) || s.ContentLength != strconv.Itoa(len(s.Body)) || g.Chunked || s.Chunked) {
		t.Errorf("%s: framing differs: go Content-Length=%q chunked=%v, ts Content-Length=%q chunked=%v",
			label, g.ContentLength, g.Chunked, s.ContentLength, s.Chunked)
	}
}

// ---------------------------------------------------------------------------
// Rendering gRPC results the way the REST server would

// encodeREST encodes like rest.writeJSON: compact, no HTML escaping, no newline.
func encodeREST(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		panic(err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

func fromPB(p *productv1.Product) product.Product {
	return product.Product{
		ID: p.GetId(), Name: p.GetName(), Description: p.GetDescription(), Category: p.GetCategory(),
		Price: p.GetPrice(), Stock: p.GetStock(), Rating: p.GetRating(),
		CreatedAt: p.GetCreatedAt().AsTime().UTC(), UpdatedAt: p.GetUpdatedAt().AsTime().UTC(),
	}
}

type productList struct {
	Products []product.Product `json:"products"`
}

type errorBody struct {
	Error  string               `json:"error"`
	Fields []product.FieldError `json:"fields,omitempty"`
}

// renderGRPC maps a gRPC response or error to the REST status and body that
// represent the same outcome.
func renderGRPC(resp proto.Message, err error) (int, []byte) {
	if err != nil {
		st := status.Convert(err)
		switch st.Code() {
		case codes.InvalidArgument:
			var fields []product.FieldError
			for _, d := range st.Details() {
				if br, ok := d.(*errdetails.BadRequest); ok {
					for _, v := range br.GetFieldViolations() {
						field := v.GetField()
						if field == "query" { // proto field name for REST ?q=
							field = "q"
						}
						fields = append(fields, product.FieldError{Field: field, Message: v.GetDescription()})
					}
				}
			}
			return 400, encodeREST(errorBody{Error: st.Message(), Fields: fields})
		case codes.NotFound:
			return 404, encodeREST(errorBody{Error: st.Message()})
		default:
			return 500, encodeREST(errorBody{Error: fmt.Sprintf("unexpected gRPC status %s: %s", st.Code(), st.Message())})
		}
	}
	switch r := resp.(type) {
	case *productv1.Product:
		return 200, encodeREST(fromPB(r))
	case *productv1.ProductList:
		out := productList{Products: make([]product.Product, 0, len(r.GetProducts()))}
		for _, p := range r.GetProducts() {
			out.Products = append(out.Products, fromPB(p))
		}
		return 200, encodeREST(out)
	default:
		panic(fmt.Sprintf("unexpected response type %T", resp))
	}
}

// ---------------------------------------------------------------------------
// Request corpus

// readCase is one read request. rpc is nil when the REST request has no gRPC
// equivalent (e.g. a non-numeric path id, which protobuf cannot express).
type readCase struct {
	path string
	rpc  func(ctx context.Context, c productv1.ProductServiceClient) (proto.Message, error)
}

func i32(v int32) *int32 { return &v }

func getCase(path string, id int64) readCase {
	return readCase{path, func(ctx context.Context, c productv1.ProductServiceClient) (proto.Message, error) {
		return c.GetProduct(ctx, &productv1.GetProductRequest{Id: id})
	}}
}

func listCase(limit, offset *int32) readCase {
	q := url.Values{}
	if limit != nil {
		q.Set("limit", strconv.Itoa(int(*limit)))
	}
	if offset != nil {
		q.Set("offset", strconv.Itoa(int(*offset)))
	}
	path := "/products"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	return readCase{path, func(ctx context.Context, c productv1.ProductServiceClient) (proto.Message, error) {
		return c.ListProducts(ctx, &productv1.ListProductsRequest{Limit: limit, Offset: offset})
	}}
}

func searchCase(query string, limit *int32) readCase {
	q := url.Values{"q": {query}}
	if limit != nil {
		q.Set("limit", strconv.Itoa(int(*limit)))
	}
	return readCase{"/products/search?" + q.Encode(), func(ctx context.Context, c productv1.ProductServiceClient) (proto.Message, error) {
		return c.SearchProducts(ctx, &productv1.SearchProductsRequest{Query: query, Limit: limit})
	}}
}

func restOnly(path string) readCase { return readCase{path: path} }

// Inputs shared with the k6 workloads (benchmark/lib/data.js).
const (
	benchmarkIDCount   = 1000 // PRODUCT_IDS length
	benchmarkListPages = 20   // LIST_PAGES
)

// benchmarkProductIDs returns the deterministic id sequence used by the
// get-product workload (benchmark/lib/data.js lcgProductIds: 32-bit LCG,
// seed 42), so equivalence covers exactly the ids k6 requests.
func benchmarkProductIDs(n int) []int64 {
	ids := make([]int64, n)
	state := uint32(42)
	for i := range ids {
		state = state*1664525 + 1013904223
		ids[i] = int64(state%seededCount) + 1
	}
	return ids
}

func readCorpus() []readCase {
	var cs []readCase

	// Single product: edges, fixed points and a deterministic spread.
	for _, id := range []int64{1, 2, 3, 20, 12345, 50000, 99999, seededCount} {
		cs = append(cs, getCase(fmt.Sprintf("/products/%d", id), id))
	}
	for _, id := range benchmarkProductIDs(benchmarkIDCount) {
		cs = append(cs, getCase(fmt.Sprintf("/products/%d", id), id))
	}
	cs = append(cs,
		getCase("/products/100001", seededCount+1),
		getCase("/products/999999999", 999999999),
		getCase("/products/0", 0),
		getCase("/products/-1", -1),
		getCase("/products/9007199254740991", product.MaxID),
		getCase("/products/9007199254740992", product.MaxID+1),
		getCase("/products/+42", 42),
		restOnly("/products/abc"),
		restOnly("/products/1.5"),
		restOnly("/products/99999999999999999999"),
		restOnly("/products/"),
	)

	// Lists: every benchmark limit at several offsets, defaults and errors.
	cs = append(cs, listCase(nil, nil))
	for _, limit := range []int32{1, 10, 50, 100, 500, 1000} {
		for _, offset := range []int32{0, 1, 50000, 99990, seededCount, 200000} {
			cs = append(cs, listCase(i32(limit), i32(offset)))
		}
	}
	// Exactly the list requests k6 sends: benchmark limits x first 20 pages.
	for _, limit := range []int32{10, 50, 100, 500} {
		for page := int32(0); page < benchmarkListPages; page++ {
			cs = append(cs, listCase(i32(limit), i32(page*limit)))
		}
	}
	cs = append(cs,
		listCase(nil, i32(99995)),
		listCase(i32(0), nil),
		listCase(i32(1001), i32(-1)),
		listCase(i32(-5), i32(1_000_000_001)),
		restOnly("/products?limit=abc"),
		restOnly("/products?limit=&offset=1.5"),
		restOnly("/products?limit=5&limit=1"),
		restOnly("/products?limit=99999999999999999999"),
		restOnly("/products?limit=%2B10"),
	)

	// Search: every benchmark term at every benchmark limit, plus edge cases.
	for _, term := range []string{"keyboard", "phone", "laptop", "monitor", "mouse"} {
		cs = append(cs, searchCase(term, nil))
		for _, limit := range []int32{1, 10, 50, 100, 500, 1000} {
			cs = append(cs, searchCase(term, i32(limit)))
		}
	}
	for _, term := range []string{"KEYBOARD", "MoUsE", "smart phone", "ke-0000", "KE-099999", "Acme Wireless", "%", "_", `\`, "100%", "zzz", "é", strings.Repeat("a", 100)} {
		cs = append(cs, searchCase(term, nil))
	}
	cs = append(cs,
		searchCase("", nil),
		searchCase(strings.Repeat("a", 101), nil),
		searchCase("x", i32(0)),
		searchCase("", i32(1001)),
		restOnly("/products/search"),
		restOnly("/products/search?q=smart+phone"),
		restOnly("/products/search?q=a&q=zzz"),
		restOnly("/products/search?q=x&limit=1e2"),
	)

	// Routing.
	cs = append(cs, restOnly("/health"), restOnly("/nope"), restOnly("/products/1/extra"))
	return cs
}

// ---------------------------------------------------------------------------
// Tests

// TestBenchmarkIDsMatchK6 pins the sequence: these are the first ids produced
// by lcgProductIds in benchmark/lib/data.js (JavaScript Math.imul arithmetic).
// Runs without servers.
func TestBenchmarkIDsMatchK6(t *testing.T) {
	want := []int64{14274, 94189, 3868, 63295, 48262}
	got := benchmarkProductIDs(len(want))
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("benchmarkProductIDs = %v, want %v (must match benchmark/lib/data.js)", got, want)
		}
	}
}

func TestEquivalence(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	t.Run("dataset is freshly seeded", func(t *testing.T) {
		r := e.do(t, "GET", e.goREST, fmt.Sprintf("/products?limit=2&offset=%d", seededCount-1), "")
		var pl productList
		if err := json.Unmarshal(r.Body, &pl); err != nil || len(pl.Products) != 1 || pl.Products[0].ID != seededCount {
			t.Fatalf("expected exactly %d seeded products (reset the database): %s", seededCount, r.Body)
		}
	})

	corpus := readCorpus()

	// Run reads before writes so they see the pristine seed.
	t.Run("REST reads: Go and TypeScript byte-identical", func(t *testing.T) {
		for _, c := range corpus {
			compareREST(t, "GET "+c.path, e.do(t, "GET", e.goREST, c.path, ""), e.do(t, "GET", e.tsREST, c.path, ""))
		}
		t.Logf("%d requests compared", len(corpus))
	})

	t.Run("gRPC reads: same data and errors as REST", func(t *testing.T) {
		n := 0
		for _, c := range corpus {
			if c.rpc == nil {
				continue
			}
			n++
			r := e.do(t, "GET", e.goREST, c.path, "")
			gotStatus, gotBody := renderGRPC(c.rpc(ctx, e.grpc))
			if gotStatus != r.Status || !bytes.Equal(gotBody, r.Body) {
				t.Errorf("%s: gRPC rendered as %d, REST %d: %s", c.path, gotStatus, r.Status, diff(r.Body, gotBody))
			}
		}
		t.Logf("%d requests compared", n)
	})

	t.Run("health", func(t *testing.T) {
		compareREST(t, "GET /health", e.do(t, "GET", e.goREST, "/health", ""), e.do(t, "GET", e.tsREST, "/health", ""))
		res, err := e.health.Check(ctx, &healthgrpc.HealthCheckRequest{})
		if err != nil || res.GetStatus() != healthgrpc.HealthCheckResponse_SERVING {
			t.Errorf("gRPC health = %v, %v", res, err)
		}
	})

	t.Run("REST write errors byte-identical", func(t *testing.T) {
		bodies := []string{
			`{}`, `null`, `[]`, `"x"`, `42`, `{"name":`, `{"name":"a","category":"b"} {}`,
			`{"name":"a","category":"b","color":"red"}`,
			`{"name":"a","category":"b","price":"10"}`,
			`{"name":"a","category":"b","stock":1.5}`,
			`{"name":"a","category":"b","stock":3000000000}`,
			`{"name":"a","category":"b","price":1e400}`,
			`{"name":"a","category":"b","price":1.005,"rating":4.25,"stock":-1}`,
			`{"name":"","category":"","price":-1,"rating":6}`,
			`{"name":"` + strings.Repeat("a", 201) + `","category":"b"}`,
			`{"name":"` + strings.Repeat("é", 200) + `","category":"b","description":"` + strings.Repeat("😀", 2001) + `"}`,
			`{"name":"a","category":"b","price":100000000}`,
			`{"name":null,"category":"b"}`,
		}
		for _, b := range bodies {
			label := "POST " + b
			if len(label) > 80 {
				label = label[:80] + "..."
			}
			compareREST(t, label, e.do(t, "POST", e.goREST, "/products", b), e.do(t, "POST", e.tsREST, "/products", b))
			compareREST(t, "PUT 1 "+label, e.do(t, "PUT", e.goREST, "/products/1", b), e.do(t, "PUT", e.tsREST, "/products/1", b))
		}
		for _, p := range []string{"/products/0", "/products/abc", "/products/999999999"} {
			compareREST(t, "PUT "+p, e.do(t, "PUT", e.goREST, p, `{"name":"a","category":"b"}`), e.do(t, "PUT", e.tsREST, p, `{"name":"a","category":"b"}`))
			compareREST(t, "DELETE "+p, e.do(t, "DELETE", e.goREST, p, ""), e.do(t, "DELETE", e.tsREST, p, ""))
		}
		big := `{"name":"` + strings.Repeat("a", 1<<20) + `","category":"b"}`
		compareREST(t, "POST oversized body", e.doOversized(t, e.goREST, big), e.doOversized(t, e.tsREST, big))
	})

	t.Run("gRPC write validation matches REST", func(t *testing.T) {
		inputs := []*productv1.CreateProductRequest{
			{},
			{Name: "a", Category: "b", Price: 1.005, Rating: 4.25, Stock: -1},
			{Name: strings.Repeat("a", 201), Category: "b", Price: 100000000},
		}
		for _, in := range inputs {
			body, _ := json.Marshal(product.Input{Name: in.Name, Description: in.Description, Category: in.Category,
				Price: in.Price, Stock: in.Stock, Rating: in.Rating})
			r := e.do(t, "POST", e.goREST, "/products", string(body))
			st, b := renderGRPC(e.grpc.CreateProduct(ctx, in))
			if st != r.Status || !bytes.Equal(b, r.Body) {
				t.Errorf("create %v: %s", in, diff(r.Body, b))
			}
			up := &productv1.UpdateProductRequest{Id: 0, Name: in.Name, Category: in.Category, Price: in.Price, Stock: in.Stock, Rating: in.Rating}
			_, b = renderGRPC(e.grpc.UpdateProduct(ctx, up))
			// REST rejects an invalid path id before reading the body; gRPC has no
			// such ordering and reports id plus body errors. Only the id error must match.
			r = e.do(t, "PUT", e.goREST, "/products/0", string(body))
			var restErr, grpcErr errorBody
			_ = json.Unmarshal(r.Body, &restErr)
			_ = json.Unmarshal(b, &grpcErr)
			if len(grpcErr.Fields) == 0 || grpcErr.Fields[0] != restErr.Fields[0] {
				t.Errorf("update id 0: REST %s, gRPC %s", r.Body, b)
			}
		}
	})

	t.Run("write flows equivalent on all three servers", func(t *testing.T) {
		type outcome struct {
			Create, Get, Update, GetAfterUpdate, Delete, GetAfterDelete, DeleteAgain string
		}
		created := `{"name":"Equivalence Lamp <é>","description":"temp","category":"Office","price":12.5,"stock":3,"rating":4.5}`
		updated := `{"name":"Equivalence Lamp v2","category":"Home","price":0,"stock":0,"rating":0}`

		restFlow := func(base string) outcome {
			var o outcome
			r := e.do(t, "POST", base, "/products", created)
			o.Create = normalize(t, r.Status, r.Body, true)
			id := idOf(t, r.Body)
			path := fmt.Sprintf("/products/%d", id)
			r = e.do(t, "GET", base, path, "")
			o.Get = normalize(t, r.Status, r.Body, true)
			r = e.do(t, "PUT", base, path, updated)
			o.Update = normalize(t, r.Status, r.Body, false)
			r = e.do(t, "GET", base, path, "")
			o.GetAfterUpdate = normalize(t, r.Status, r.Body, false)
			r = e.do(t, "DELETE", base, path, "")
			o.Delete = fmt.Sprintf("%d %s", r.Status, r.Body)
			r = e.do(t, "GET", base, path, "")
			o.GetAfterDelete = fmt.Sprintf("%d %s", r.Status, r.Body)
			r = e.do(t, "DELETE", base, path, "")
			o.DeleteAgain = fmt.Sprintf("%d %s", r.Status, r.Body)
			return o
		}
		grpcFlow := func() outcome {
			var o outcome
			p, err := e.grpc.CreateProduct(ctx, &productv1.CreateProductRequest{
				Name: "Equivalence Lamp <é>", Description: "temp", Category: "Office", Price: 12.5, Stock: 3, Rating: 4.5})
			st, b := renderGRPC(p, err)
			o.Create = normalize(t, 201, b, true) // REST create is 201
			if st != 200 {
				t.Fatalf("gRPC create failed: %s", b)
			}
			id := p.GetId()
			st, b = renderGRPC(e.grpc.GetProduct(ctx, &productv1.GetProductRequest{Id: id}))
			o.Get = normalize(t, st, b, true)
			st, b = renderGRPC(e.grpc.UpdateProduct(ctx, &productv1.UpdateProductRequest{
				Id: id, Name: "Equivalence Lamp v2", Category: "Home"}))
			o.Update = normalize(t, st, b, false)
			st, b = renderGRPC(e.grpc.GetProduct(ctx, &productv1.GetProductRequest{Id: id}))
			o.GetAfterUpdate = normalize(t, st, b, false)
			if _, err := e.grpc.DeleteProduct(ctx, &productv1.DeleteProductRequest{Id: id}); err != nil {
				t.Errorf("gRPC delete: %v", err)
			}
			o.Delete = "204 " // REST delete is 204 with no body
			st, b = renderGRPC(e.grpc.GetProduct(ctx, &productv1.GetProductRequest{Id: id}))
			o.GetAfterDelete = fmt.Sprintf("%d %s", st, b)
			_, err = e.grpc.DeleteProduct(ctx, &productv1.DeleteProductRequest{Id: id})
			st, b = renderGRPC(nil, err)
			o.DeleteAgain = fmt.Sprintf("%d %s", st, b)
			return o
		}

		goOut, tsOut, grpcOut := restFlow(e.goREST), restFlow(e.tsREST), grpcFlow()
		if goOut != tsOut {
			t.Errorf("Go REST and TS REST write flows differ:\n go: %+v\n ts: %+v", goOut, tsOut)
		}
		if goOut != grpcOut {
			t.Errorf("Go REST and gRPC write flows differ:\n rest: %+v\n grpc: %+v", goOut, grpcOut)
		}
	})

	t.Run("dataset unchanged after writes", func(t *testing.T) {
		for _, path := range []string{fmt.Sprintf("/products?limit=5&offset=%d", seededCount-3), "/products/search?q=equivalence"} {
			compareREST(t, path, e.do(t, "GET", e.goREST, path, ""), e.do(t, "GET", e.tsREST, path, ""))
			r := e.do(t, "GET", e.goREST, path, "")
			if bytes.Contains(r.Body, []byte("Equivalence")) {
				t.Errorf("%s still contains test products: %s", path, r.Body)
			}
		}
	})
}

var timestampRE = regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d{1,6})?Z$`)

// normalize checks the server-assigned fields of a product response (id,
// created_at, updated_at) and replaces them with placeholders so responses
// from different servers can be compared.
func normalize(t *testing.T, status int, body []byte, fresh bool) string {
	t.Helper()
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	if _, isProduct := m["id"]; !isProduct {
		return fmt.Sprintf("%d %s", status, body)
	}
	created, _ := m["created_at"].(string)
	updatedAt, _ := m["updated_at"].(string)
	for _, ts := range []string{created, updatedAt} {
		if !timestampRE.MatchString(ts) {
			t.Errorf("bad timestamp %q in %s", ts, body)
		}
	}
	c, _ := time.Parse(time.RFC3339Nano, created)
	u, _ := time.Parse(time.RFC3339Nano, updatedAt)
	switch {
	case fresh && !c.Equal(u):
		t.Errorf("new product has created_at %s != updated_at %s", created, updatedAt)
	case u.Before(c):
		t.Errorf("updated_at %s before created_at %s", updatedAt, created)
	case time.Since(c) > time.Hour || time.Since(c) < -time.Hour:
		t.Errorf("created_at %s is not close to now", created)
	}
	if id, err := m["id"].(json.Number).Int64(); err != nil || id <= seededCount {
		t.Errorf("unexpected id %v", m["id"])
	}
	m["id"], m["created_at"], m["updated_at"] = "ID", "T", "T"
	return fmt.Sprintf("%d %s", status, encodeREST(m))
}

func idOf(t *testing.T, body []byte) int64 {
	t.Helper()
	var p struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(body, &p); err != nil || p.ID == 0 {
		t.Fatalf("no id in %s: %v", body, err)
	}
	return p.ID
}
