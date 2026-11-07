package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/protobuf/proto"

	"go-vs-ts-benchmark/go-server/gen/x2v1"
	"go-vs-ts-benchmark/go-server/internal/x2"
	"go-vs-ts-benchmark/go-server/internal/x2data"
)

// result of one operation (one request, or one whole stream).
type result struct {
	items int           // items received (export) or sent (import); 1 otherwise
	ttfi  time.Duration // time to first item; equals the latency when items arrive all at once
}

type op func(ctx context.Context) (result, error)

// worker owns one connection and returns the operation it repeats.
type opFactory func(worker int) (op, func(), error)

const svc = "/x2.v1.X2Service/"

func stream(i int) *grpc.StreamDesc { return &x2v1.X2Service_ServiceDesc.Streams[i] }

// Stream indices in X2Service_ServiceDesc.Streams (declaration order of streaming RPCs).
const (
	streamExport = iota
	streamImport
	streamSubscribe
	streamQuote
)

func checkStatus(res *http.Response) error {
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("HTTP %d: %s", res.StatusCode, b)
	}
	return nil
}

// restGet reads the whole body without parsing it. firstLine: report the
// time the first newline arrived (NDJSON) and count lines.
func restGet(ctx context.Context, c *http.Client, url string, firstLine bool) (result, error) {
	t0 := time.Now()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	res, err := c.Do(req)
	if err != nil {
		return result{}, err
	}
	defer res.Body.Close()
	if err := checkStatus(res); err != nil {
		return result{}, err
	}
	if !firstLine {
		if _, err := io.Copy(io.Discard, res.Body); err != nil {
			return result{}, err
		}
		return result{items: 1, ttfi: time.Since(t0)}, nil
	}
	var r result
	br := bufio.NewReaderSize(res.Body, 64<<10)
	buf := make([]byte, 64<<10)
	for {
		n, err := br.Read(buf)
		if c := bytes.Count(buf[:n], []byte{'\n'}); c > 0 {
			if r.items == 0 {
				r.ttfi = time.Since(t0)
			}
			r.items += c
		}
		if err == io.EOF {
			return r, nil
		}
		if err != nil {
			return r, err
		}
	}
}

func restPost(ctx context.Context, c *http.Client, url, contentType string, body []byte) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if err := checkStatus(res); err != nil {
		return err
	}
	_, err = io.Copy(io.Discard, res.Body)
	return err
}

// runFlags is what an operation depends on.
type runFlags struct {
	scenario, target, profile, mode string
	count, conc, subscribers, conns int
	rate                            int
	gzip                            bool
	timeout                         time.Duration
}

func expectItems(r result, err error, want int) (result, error) {
	if err == nil && r.items != want {
		err = fmt.Errorf("received %d items, want %d", r.items, want)
	}
	return r, err
}

func buildFactory(f runFlags) (opFactory, error) {
	factory, err := buildFactory0(f)
	if err != nil || f.scenario != "export" || f.mode != "unary" {
		return factory, err
	}
	// A unary export delivers all items at once.
	return func(w int) (op, func(), error) {
		o, done, err := factory(w)
		if err != nil {
			return nil, nil, err
		}
		return func(ctx context.Context) (result, error) {
			r, err := o(ctx)
			r.items = f.count
			return r, err
		}, done, nil
	}, nil
}

func buildFactory0(f runFlags) (opFactory, error) {
	addr, isGRPC, err := targetAddr(f.target)
	if err != nil {
		return nil, err
	}
	base := "http://" + addr

	if !isGRPC {
		var url, ctype string
		var body []byte
		ndjson := false
		switch f.scenario {
		case "payload":
			url = fmt.Sprintf("%s/x2/payload?profile=%s&count=%d", base, f.profile, f.count)
		case "export":
			format := map[string]string{"unary": "json", "stream": "ndjson"}[f.mode]
			url, ndjson = fmt.Sprintf("%s/x2/export?count=%d&format=%s", base, f.count, format), f.mode == "stream"
		case "import":
			url = base + "/x2/import"
			body, ctype = importBodyREST(f.count, f.mode == "stream")
		case "quote":
		default:
			return nil, fmt.Errorf("scenario %q has no closed-loop operation", f.scenario)
		}
		return func(w int) (op, func(), error) {
			c := newHTTPClient(f.gzip, f.timeout)
			done := func() { c.CloseIdleConnections() }
			switch f.scenario {
			case "import":
				return func(ctx context.Context) (result, error) {
					t0 := time.Now()
					err := restPost(ctx, c, url, ctype, body)
					return result{items: f.count, ttfi: time.Since(t0)}, err
				}, done, nil
			case "quote":
				i := 0
				return func(ctx context.Context) (result, error) {
					i++
					return restGet(ctx, c, fmt.Sprintf("%s/x2/quote/%d", base, quoteID(w, i)), false)
				}, done, nil
			case "export":
				if ndjson {
					return func(ctx context.Context) (result, error) {
						r, err := restGet(ctx, c, url, true)
						return expectItems(r, err, f.count)
					}, done, nil
				}
			}
			return func(ctx context.Context) (result, error) { return restGet(ctx, c, url, false) }, done, nil
		}, nil
	}

	var pre []byte     // pre-encoded unary request
	var items [][]byte // pre-encoded stream items (import)
	if f.scenario == "import" {
		pre, items = importBodyGRPC(f.count, f.mode == "stream")
	}
	return func(w int) (op, func(), error) {
		cc, err := newGRPCConn(addr, true)
		if err != nil {
			return nil, nil, err
		}
		done := func() { cc.Close() }
		var opts []grpc.CallOption
		if f.gzip {
			opts = append(opts, grpc.UseCompressor(gzip.Name))
		}
		unary := func(method string, req any) op {
			return func(ctx context.Context) (result, error) {
				t0 := time.Now()
				var out rawMsg
				err := cc.Invoke(ctx, svc+method, req, &out, opts...)
				return result{items: 1, ttfi: time.Since(t0)}, err
			}
		}
		switch f.scenario {
		case "payload":
			return unary("GetPayload", &x2v1.PayloadRequest{Profile: f.profile, Count: int32(f.count)}), done, nil
		case "export":
			req := &x2v1.ExportRequest{Count: int32(f.count)}
			if f.mode == "unary" {
				return unary("ExportUnary", req), done, nil
			}
			return func(ctx context.Context) (result, error) {
				r, err := grpcServerStream(ctx, cc, req, opts)
				return expectItems(r, err, f.count)
			}, done, nil
		case "import":
			if f.mode == "unary" {
				o := unary("ImportUnary", &rawMsg{b: pre})
				return func(ctx context.Context) (result, error) {
					r, err := o(ctx)
					r.items = f.count
					return r, err
				}, done, nil
			}
			return func(ctx context.Context) (result, error) {
				t0 := time.Now()
				s, err := cc.NewStream(ctx, stream(streamImport), svc+"ImportStream", opts...)
				if err != nil {
					return result{}, err
				}
				for _, b := range items {
					if err := s.SendMsg(&rawMsg{b: b}); err != nil {
						return result{}, err
					}
				}
				if err := s.CloseSend(); err != nil {
					return result{}, err
				}
				var out rawMsg
				err = s.RecvMsg(&out)
				return result{items: f.count, ttfi: time.Since(t0)}, err
			}, done, nil
		case "quote":
			if f.mode == "unary" {
				i := 0
				return func(ctx context.Context) (result, error) {
					i++
					return unary("GetQuote", &x2v1.QuoteRequest{Id: quoteID(w, i)})(ctx)
				}, done, nil
			}
			// One bidirectional stream per worker for the whole run; each
			// operation is one request/response exchange on it.
			s, err := cc.NewStream(context.Background(), stream(streamQuote), svc+"QuoteStream", opts...)
			if err != nil {
				cc.Close()
				return nil, nil, err
			}
			i := 0
			return func(ctx context.Context) (result, error) {
				i++
				t0 := time.Now()
				if err := s.SendMsg(&x2v1.QuoteRequest{Id: quoteID(w, i)}); err != nil {
					return result{}, err
				}
				var out rawMsg
				err := s.RecvMsg(&out)
				return result{items: 1, ttfi: time.Since(t0)}, err
			}, func() { _ = s.CloseSend(); cc.Close() }, nil
		}
		cc.Close()
		return nil, nil, fmt.Errorf("scenario %q has no closed-loop operation", f.scenario)
	}, nil
}

func grpcServerStream(ctx context.Context, cc *grpc.ClientConn, req *x2v1.ExportRequest, opts []grpc.CallOption) (result, error) {
	t0 := time.Now()
	s, err := cc.NewStream(ctx, stream(streamExport), svc+"ExportStream", opts...)
	if err != nil {
		return result{}, err
	}
	if err := s.SendMsg(req); err != nil {
		return result{}, err
	}
	if err := s.CloseSend(); err != nil {
		return result{}, err
	}
	var r result
	for {
		var m rawMsg
		err := s.RecvMsg(&m)
		if errors.Is(err, io.EOF) {
			return r, nil
		}
		if err != nil {
			return r, err
		}
		if r.items == 0 {
			r.ttfi = time.Since(t0)
		}
		r.items++
	}
}

func quoteID(worker, i int) int64 {
	return int64((worker*7919+i*31)%x2data.ExportSize + 1)
}

// Import request bodies are encoded once per run: the client's encoding cost
// is not part of the measurement.
func importBodyREST(n int, ndjson bool) ([]byte, string) {
	items := x2data.Items(x2data.Small, n)
	if ndjson {
		var buf bytes.Buffer
		for i := range items {
			buf.Write(jsonBytes(&items[i]))
			buf.WriteByte('\n')
		}
		return buf.Bytes(), "application/x-ndjson"
	}
	return jsonBytes(x2data.ItemList{Items: items}), "application/json"
}

func importBodyGRPC(n int, perItem bool) ([]byte, [][]byte) {
	items := x2data.Items(x2data.Small, n)
	if perItem {
		out := make([][]byte, n)
		for i := range items {
			out[i], _ = proto.Marshal(x2.ToPB(&items[i]))
		}
		return nil, out
	}
	list := &x2v1.ItemList{}
	for i := range items {
		list.Items = append(list.Items, x2.ToPB(&items[i]))
	}
	b, _ := proto.Marshal(list)
	return b, nil
}
