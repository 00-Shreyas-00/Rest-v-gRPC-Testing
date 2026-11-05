// Package rest is the HTTP/JSON transport. It only translates between HTTP and
// the service; all business rules live in internal/service and internal/product.
//
// Wire contract (the TypeScript server must produce the same responses):
//
//	GET    /products/{id}                -> 200 Product
//	GET    /products?limit=&offset=      -> 200 {"products":[Product...]}
//	GET    /products/search?q=&limit=    -> 200 {"products":[Product...]}
//	POST   /products                     -> 201 Product
//	PUT    /products/{id}                -> 200 Product
//	DELETE /products/{id}                -> 204 (no body)
//	GET    /health                       -> 200 {"status":"ok"} | 503 {"status":"unavailable"}
//
// Errors: 400 {"error":"validation failed","fields":[{"field","message"}...]}
//
//	400 {"error":"invalid request body"}
//	404 {"error":"product not found"} | {"error":"not found"} (unknown route)
//	413 {"error":"request body too large"}
//	500 {"error":"internal server error"}
package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go-vs-ts-benchmark/go-server/internal/product"
	"go-vs-ts-benchmark/go-server/internal/service"
)

// MaxBodyBytes limits request bodies (same limit as the TypeScript server).
const MaxBodyBytes = 1 << 20

// ProductList is the response body for list and search.
type ProductList struct {
	Products []product.Product `json:"products"`
}

type errorBody struct {
	Error  string               `json:"error"`
	Fields []product.FieldError `json:"fields,omitempty"`
}

type handler struct {
	svc    *service.Service
	health func(context.Context) error
	log    *slog.Logger
}

// NewHandler returns the REST API. health is called by GET /health only.
// When logRequests is false (the default) nothing is logged per request.
func NewHandler(svc *service.Service, health func(context.Context) error, log *slog.Logger, logRequests bool) http.Handler {
	h := &handler{svc: svc, health: health, log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.getHealth)
	mux.HandleFunc("GET /products", h.list)
	mux.HandleFunc("GET /products/search", h.search)
	mux.HandleFunc("GET /products/{id}", h.get)
	mux.HandleFunc("POST /products", h.create)
	mux.HandleFunc("PUT /products/{id}", h.update)
	mux.HandleFunc("DELETE /products/{id}", h.delete)
	// Unknown paths and unsupported methods both get a JSON 404, as in the TS server.
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusNotFound, errorBody{Error: "not found"})
	})

	if logRequests {
		return logMiddleware(mux, log)
	}
	return mux
}

func (h *handler) getHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.health(ctx); err != nil {
		h.log.Warn("health check failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	p, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var pe paramErrors
	limit := pe.int(q, "limit", product.DefaultListLimit)
	offset := pe.int(q, "offset", 0)
	if pe.failed(w) {
		return
	}
	ps, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ProductList{Products: ps})
}

func (h *handler) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var pe paramErrors
	limit := pe.int(q, "limit", product.DefaultSearchLimit)
	if pe.failed(w) {
		return
	}
	ps, err := h.svc.Search(r.Context(), q.Get("q"), limit)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ProductList{Products: ps})
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeInput(w, r)
	if !ok {
		return
	}
	p, err := h.svc.Create(r.Context(), in)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	in, ok := decodeInput(w, r)
	if !ok {
		return
	}
	p, err := h.svc.Update(r.Context(), id, in)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// pathID parses {id}. Anything that is not an integer in range gets the same
// validation error as an out-of-range id.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		id = 0 // fails ValidateID below
	}
	if err := product.ValidateID(id); err != nil {
		writeValidation(w, err.(*product.ValidationError))
		return 0, false
	}
	return id, true
}

// paramErrors collects query-parameter parse errors. Parsing happens before
// range validation: if any parameter is not an integer, only parse errors are
// reported (in parameter order).
type paramErrors struct{ fields []product.FieldError }

// int returns def when the parameter is absent. The first value wins when a
// parameter is repeated. Integers too large for int are mapped to the extreme
// value so that range validation reports them.
func (pe *paramErrors) int(q url.Values, name string, def int) int {
	if !q.Has(name) {
		return def
	}
	s := q.Get(name)
	n, err := strconv.Atoi(s)
	if err != nil {
		if errors.Is(err, strconv.ErrRange) {
			if strings.HasPrefix(s, "-") {
				return math.MinInt
			}
			return math.MaxInt
		}
		pe.fields = append(pe.fields, product.FieldError{Field: name, Message: "must be an integer"})
		return def
	}
	return n
}

func (pe *paramErrors) failed(w http.ResponseWriter) bool {
	if len(pe.fields) == 0 {
		return false
	}
	writeValidation(w, &product.ValidationError{Fields: pe.fields})
	return true
}

// decodeInput reads a create/update body. Rules: a single JSON object (null is
// treated as {}), no unknown fields, no trailing data; absent fields take their
// zero value (same as proto3), then the service validates.
func decodeInput(w http.ResponseWriter, r *http.Request) (product.Input, bool) {
	var in product.Input
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	dec.DisallowUnknownFields()
	err := dec.Decode(&in)
	if err == nil && dec.Decode(&struct{}{}) != io.EOF {
		err = errors.New("trailing data after JSON body")
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeJSON(w, http.StatusRequestEntityTooLarge, errorBody{Error: "request body too large"})
		} else {
			writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid request body"})
		}
		return product.Input{}, false
	}
	return in, true
}

func (h *handler) writeError(w http.ResponseWriter, err error) {
	var ve *product.ValidationError
	switch {
	case errors.As(err, &ve):
		writeValidation(w, ve)
	case errors.Is(err, product.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorBody{Error: "product not found"})
	default:
		h.log.Error("request failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "internal server error"})
	}
}

func writeValidation(w http.ResponseWriter, ve *product.ValidationError) {
	writeJSON(w, http.StatusBadRequest, errorBody{Error: "validation failed", Fields: ve.Fields})
}

// writeJSON encodes v exactly like JSON.stringify would for our types: no HTML
// escaping, no trailing newline, and an explicit Content-Length so responses
// are never chunked. This keeps response bytes comparable across servers.
func writeJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	body := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))

	hdr := w.Header()
	hdr.Set("Content-Type", "application/json; charset=utf-8")
	hdr.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func logMiddleware(next http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", time.Since(start))
	})
}
