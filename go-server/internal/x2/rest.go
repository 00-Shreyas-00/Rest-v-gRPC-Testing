package x2

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"go-vs-ts-benchmark/go-server/internal/x2data"
)

// RegisterREST adds the experiment-2 routes to mux:
//
//	GET  /x2/payload?profile=&count=     {"items":[...]}, gzip if Accept-Encoding allows
//	GET  /x2/export?count=&format=       json: {"items":[...]}; ndjson: one item per line, streamed
//	POST /x2/import                      application/json {"items":[...]} or application/x-ndjson
//	GET  /x2/feed?rate=                  text/event-stream, one "data: {...}" frame per event
//	GET  /x2/quote/{id}                  {"id","price","stock"}
func RegisterREST(mux *http.ServeMux) {
	hub := NewHub(func(pe x2data.PriceEvent) []byte {
		b, _ := json.Marshal(pe)
		return append(append([]byte("data: "), b...), '\n', '\n')
	})
	mux.HandleFunc("GET /x2/payload", payload)
	mux.HandleFunc("GET /x2/export", export)
	mux.HandleFunc("POST /x2/import", importItems)
	mux.HandleFunc("GET /x2/feed", func(w http.ResponseWriter, r *http.Request) { feed(w, r, hub) })
	mux.HandleFunc("GET /x2/quote/{id}", quote)
}

func badRequest(w http.ResponseWriter, err error) {
	writeBody(w, r400, mustJSON(map[string]string{"error": err.Error()}))
}

const r400 = http.StatusBadRequest

// intParam parses a required integer query parameter in [lo, hi].
func intParam(r *http.Request, name string, lo, hi int64) (int64, error) {
	v, err := strconv.ParseInt(r.URL.Query().Get(name), 10, 64)
	if err != nil {
		return 0, errors.New(name + " must be an integer")
	}
	return v, x2data.CheckRange(name, v, lo, hi)
}

// encodeJSON matches JSON.stringify: no HTML escaping, no trailing newline.
func encodeJSON(buf *bytes.Buffer, v any) {
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	buf.Truncate(buf.Len() - 1)
}

func mustJSON(v any) []byte {
	var buf bytes.Buffer
	encodeJSON(&buf, v)
	return buf.Bytes()
}

func writeBody(w http.ResponseWriter, status int, body []byte) {
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// GzipLevel is used by every experiment-2 server (Go REST, TS REST, gRPC):
// BestSpeed (nginx's default). Level 6 compresses the synthetic text 2-3x slower
// than real prose; level 1 runs at similar speed on both. See the design doc.
const GzipLevel = gzip.BestSpeed

var (
	bufPool  = sync.Pool{New: func() any { return new(bytes.Buffer) }}
	gzipPool = sync.Pool{New: func() any { w, _ := gzip.NewWriterLevel(nil, GzipLevel); return w }}
)

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		if strings.TrimSpace(strings.SplitN(part, ";", 2)[0]) == "gzip" {
			return true
		}
	}
	return false
}

// writeItems serializes the whole list, then (optionally) gzips it into a
// second buffer, and sends it with a Content-Length. The TS server does the same.
func writeItems(w http.ResponseWriter, r *http.Request, items []x2data.Item, compress bool) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)
	encodeJSON(buf, x2data.ItemList{Items: items})
	if !compress || !acceptsGzip(r) {
		writeBody(w, http.StatusOK, buf.Bytes())
		return
	}
	zbuf := bufPool.Get().(*bytes.Buffer)
	zbuf.Reset()
	defer bufPool.Put(zbuf)
	zw := gzipPool.Get().(*gzip.Writer)
	zw.Reset(zbuf)
	_, _ = zw.Write(buf.Bytes())
	_ = zw.Close()
	gzipPool.Put(zw)
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Vary", "Accept-Encoding")
	writeBody(w, http.StatusOK, zbuf.Bytes())
}

func payload(w http.ResponseWriter, r *http.Request) {
	profile := r.URL.Query().Get("profile")
	if err := x2data.CheckProfile(profile); err != nil {
		badRequest(w, err)
		return
	}
	n, err := intParam(r, "count", 1, x2data.PoolSize)
	if err != nil {
		badRequest(w, err)
		return
	}
	writeItems(w, r, getData().profiles[profile][:n], true)
}

// ndjsonChunk is the write size of streamed NDJSON (same in the TS server).
const ndjsonChunk = 16 << 10

func export(w http.ResponseWriter, r *http.Request) {
	n, err := intParam(r, "count", 1, x2data.ExportSize)
	if err != nil {
		badRequest(w, err)
		return
	}
	items := getData().small[:n]
	switch r.URL.Query().Get("format") {
	case "json":
		writeItems(w, r, items, false)
	case "ndjson":
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(http.StatusOK)
		bw := bufio.NewWriterSize(w, ndjsonChunk)
		enc := json.NewEncoder(bw) // Encode appends the newline
		enc.SetEscapeHTML(false)
		for i := range items {
			if err := enc.Encode(&items[i]); err != nil {
				return // client went away
			}
		}
		_ = bw.Flush()
	default:
		badRequest(w, errors.New("format must be json or ndjson"))
	}
}

func importItems(w http.ResponseWriter, r *http.Request) {
	body := http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	var sum x2data.ImportSummary
	ct := r.Header.Get("Content-Type")
	switch {
	case strings.HasPrefix(ct, "application/x-ndjson"):
		dec := json.NewDecoder(body)
		for {
			var it x2data.Item
			if err := dec.Decode(&it); err == io.EOF {
				break
			} else if err != nil {
				badRequest(w, errors.New("invalid request body"))
				return
			}
			sum.Add(it.ID, it.Stock)
		}
	case strings.HasPrefix(ct, "application/json"):
		var list x2data.ItemList
		if err := json.NewDecoder(body).Decode(&list); err != nil {
			badRequest(w, errors.New("invalid request body"))
			return
		}
		for i := range list.Items {
			sum.Add(list.Items[i].ID, list.Items[i].Stock)
		}
	default:
		badRequest(w, errors.New("content-type must be application/json or application/x-ndjson"))
		return
	}
	writeBody(w, http.StatusOK, mustJSON(sum))
}

func quote(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		id = 0
	}
	if err := x2data.CheckRange("id", id, 1, x2data.ExportSize); err != nil {
		badRequest(w, err)
		return
	}
	it := &getData().small[id-1]
	writeBody(w, http.StatusOK, mustJSON(x2data.Quote{ID: it.ID, Price: it.Price, Stock: it.Stock}))
}

func feed(w http.ResponseWriter, r *http.Request, hub *Hub) {
	rate, err := intParam(r, "rate", 1, x2data.MaxRate)
	if err != nil {
		badRequest(w, err)
		return
	}
	events, cancel, err := hub.Subscribe(int(rate))
	if err != nil {
		writeBody(w, http.StatusConflict, mustJSON(map[string]string{"error": err.Error()}))
		return
	}
	defer cancel()
	flusher := w.(http.Flusher)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-events:
			if _, err := w.Write(ev.SSE); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
