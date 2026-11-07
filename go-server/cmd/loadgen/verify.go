package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcgzip "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go-vs-ts-benchmark/go-server/gen/x2v1"
	"go-vs-ts-benchmark/go-server/internal/x2"
	"go-vs-ts-benchmark/go-server/internal/x2data"
)

// jsonBytes encodes like the servers (and JSON.stringify): no HTML escaping,
// no trailing newline.
func jsonBytes(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

type checker struct {
	target       string
	pass, failed int
}

func (c *checker) check(name string, err error) {
	if err != nil {
		c.failed++
		fmt.Printf("FAIL %-8s %s: %v\n", c.target, name, err)
		return
	}
	c.pass++
}

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	targets := fs.String("targets", "go-rest ts-rest go-grpc", "servers to check")
	_ = fs.Parse(args)

	total := 0
	for _, t := range strings.Fields(*targets) {
		addr, isGRPC, err := targetAddr(t)
		if err != nil {
			return err
		}
		c := &checker{target: t}
		start := time.Now()
		if isGRPC {
			err = verifyGRPC(c, addr)
		} else {
			err = verifyREST(c, addr)
		}
		if err != nil {
			return fmt.Errorf("%s: %w", t, err)
		}
		fmt.Printf("%-8s %d checks passed, %d failed (%.1fs)\n", t, c.pass, c.failed, time.Since(start).Seconds())
		total += c.failed
	}
	if total > 0 {
		return fmt.Errorf("%d checks failed", total)
	}
	fmt.Println("PASS: every server returns identical experiment-2 data")
	return nil
}

var (
	payloadCounts = []int{1, 7, x2data.PoolSize}
	exportCounts  = []int{1, 1000, x2data.ExportSize}
	quoteIDs      = []int64{1, 2, 50000, x2data.ExportSize}
)

func expectedSummary(n int) x2data.ImportSummary {
	var s x2data.ImportSummary
	for _, it := range x2data.Items(x2data.Small, n) {
		s.Add(it.ID, it.Stock)
	}
	return s
}

func ndjsonBytes(items []x2data.Item) []byte {
	var buf bytes.Buffer
	for i := range items {
		buf.Write(jsonBytes(&items[i]))
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

func bodyEqual(got, want []byte) error {
	if bytes.Equal(got, want) {
		return nil
	}
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	snip := func(b []byte) string { return string(b[max(i-40, 0):min(i+40, len(b))]) }
	return fmt.Errorf("body differs at byte %d (len %d, want %d):\n  got  …%s…\n  want …%s…", i, len(got), len(want), snip(got), snip(want))
}

func verifyREST(c *checker, addr string) error {
	base := "http://" + addr
	client := &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{DisableCompression: true}}
	do := func(method, url, ctype string, body []byte, hdr map[string]string) (int, http.Header, []byte, error) {
		req, _ := http.NewRequest(method, url, bytes.NewReader(body))
		if ctype != "" {
			req.Header.Set("Content-Type", ctype)
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		res, err := client.Do(req)
		if err != nil {
			return 0, nil, nil, err
		}
		defer res.Body.Close()
		b, err := io.ReadAll(res.Body)
		return res.StatusCode, res.Header, b, err
	}
	expect := func(name, method, url, ctype string, body []byte, wantStatus int, want []byte) {
		st, _, got, err := do(method, url, ctype, body, nil)
		if err == nil && st != wantStatus {
			err = fmt.Errorf("status %d, want %d (%s)", st, wantStatus, got)
		}
		if err == nil {
			err = bodyEqual(got, want)
		}
		c.check(name, err)
	}

	for _, p := range x2data.Profiles {
		all := x2data.Items(p, x2data.PoolSize)
		for _, n := range payloadCounts {
			want := jsonBytes(x2data.ItemList{Items: all[:n]})
			url := fmt.Sprintf("%s/x2/payload?profile=%s&count=%d", base, p, n)
			expect(fmt.Sprintf("payload %s %d", p, n), "GET", url, "", nil, 200, want)
			st, hdr, z, err := do("GET", url, "", nil, map[string]string{"Accept-Encoding": "gzip"})
			if err == nil && (st != 200 || hdr.Get("Content-Encoding") != "gzip") {
				err = fmt.Errorf("status %d, Content-Encoding %q", st, hdr.Get("Content-Encoding"))
			}
			if err == nil {
				var zr *gzip.Reader
				if zr, err = gzip.NewReader(bytes.NewReader(z)); err == nil {
					var plain []byte
					if plain, err = io.ReadAll(zr); err == nil {
						err = bodyEqual(plain, want)
					}
				}
			}
			c.check(fmt.Sprintf("payload %s %d gzip", p, n), err)
		}
	}
	errBody := func(msg string) []byte { return jsonBytes(map[string]string{"error": msg}) }
	expect("payload bad profile", "GET", base+"/x2/payload?profile=xml&count=1", "", nil, 400, errBody("profile must be one of text, numeric, binary"))
	expect("payload count 0", "GET", base+"/x2/payload?profile=text&count=0", "", nil, 400, errBody("count must be between 1 and 4000"))
	expect("payload count x", "GET", base+"/x2/payload?profile=text&count=x", "", nil, 400, errBody("count must be an integer"))

	small := x2data.Items(x2data.Small, x2data.ExportSize)
	for _, n := range exportCounts {
		expect(fmt.Sprintf("export json %d", n), "GET", fmt.Sprintf("%s/x2/export?count=%d&format=json", base, n), "", nil, 200, jsonBytes(x2data.ItemList{Items: small[:n]}))
		expect(fmt.Sprintf("export ndjson %d", n), "GET", fmt.Sprintf("%s/x2/export?count=%d&format=ndjson", base, n), "", nil, 200, ndjsonBytes(small[:n]))
	}
	expect("export bad format", "GET", base+"/x2/export?count=1&format=csv", "", nil, 400, errBody("format must be json or ndjson"))
	expect("export count too big", "GET", base+"/x2/export?count=100001&format=json", "", nil, 400, errBody("count must be between 1 and 100000"))

	for _, n := range []int{1, 1000, 10000} {
		want := jsonBytes(expectedSummary(n))
		b, ct := importBodyREST(n, false)
		expect(fmt.Sprintf("import json %d", n), "POST", base+"/x2/import", ct, b, 200, want)
		b, ct = importBodyREST(n, true)
		expect(fmt.Sprintf("import ndjson %d", n), "POST", base+"/x2/import", ct, b, 200, want)
	}
	expect("import bad body", "POST", base+"/x2/import", "application/json", []byte("{"), 400, errBody("invalid request body"))

	for _, id := range quoteIDs {
		it := small[id-1]
		expect(fmt.Sprintf("quote %d", id), "GET", fmt.Sprintf("%s/x2/quote/%d", base, id), "", nil, 200, jsonBytes(x2data.Quote{ID: it.ID, Price: it.Price, Stock: it.Stock}))
	}
	expect("quote 0", "GET", base+"/x2/quote/0", "", nil, 400, errBody("id must be between 1 and 100000"))

	// Feed: three events with the exact SSE framing.
	c.check("feed", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, "GET", base+"/x2/feed?rate=50", nil)
		res, err := client.Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != 200 || res.Header.Get("Content-Type") != "text/event-stream" {
			return fmt.Errorf("status %d, content-type %q", res.StatusCode, res.Header.Get("Content-Type"))
		}
		br := bufio.NewReader(res.Body)
		for range 3 {
			line, err := br.ReadBytes('\n')
			if err != nil {
				return err
			}
			blank, err := br.ReadBytes('\n')
			if err != nil {
				return err
			}
			var ev x2data.PriceEvent
			if err := json.Unmarshal(bytes.TrimPrefix(line, []byte("data: ")), &ev); err != nil {
				return err
			}
			if err := checkEvent(ev, small); err != nil {
				return err
			}
			want := append(append([]byte("data: "), jsonBytes(ev)...), '\n')
			if !bytes.Equal(line, want) || string(blank) != "\n" {
				return fmt.Errorf("frame %q, want %q + blank line", line, want)
			}
		}
		return nil
	}())
	return nil
}

func checkEvent(ev x2data.PriceEvent, small []x2data.Item) error {
	want := x2data.EventFor(ev.Seq, small)
	want.SentAtUs = ev.SentAtUs
	if ev != want {
		return fmt.Errorf("event %+v, want %+v", ev, want)
	}
	if d := time.Since(time.UnixMicro(ev.SentAtUs)); d < -time.Second || d > 5*time.Second {
		return fmt.Errorf("sent_at_us is %v from now (clock?)", d)
	}
	return nil
}

func verifyGRPC(c *checker, addr string) error {
	cc, err := newGRPCConn(addr, false)
	if err != nil {
		return err
	}
	defer cc.Close()
	cl := x2v1.NewX2ServiceClient(cc)
	ctx := context.Background()
	pbList := func(items []x2data.Item) *x2v1.ItemList {
		l := &x2v1.ItemList{}
		for i := range items {
			l.Items = append(l.Items, x2.ToPB(&items[i]))
		}
		return l
	}
	equal := func(got, want proto.Message, err error) error {
		if err == nil && !proto.Equal(got, want) {
			err = errors.New("message differs from the generated data")
		}
		return err
	}
	invalid := func(err error) error {
		if status.Code(err) != codes.InvalidArgument {
			return fmt.Errorf("got %v, want InvalidArgument", err)
		}
		return nil
	}

	for _, p := range x2data.Profiles {
		all := x2data.Items(p, x2data.PoolSize)
		for _, n := range payloadCounts {
			want := pbList(all[:n])
			got, err := cl.GetPayload(ctx, &x2v1.PayloadRequest{Profile: p, Count: int32(n)})
			c.check(fmt.Sprintf("payload %s %d", p, n), equal(got, want, err))
			// grpc-go hides the grpc-encoding header from the application, so
			// compression is checked on the wire: text compresses well.
			rx0 := wireRx.Load()
			_, _ = cl.GetPayload(ctx, &x2v1.PayloadRequest{Profile: p, Count: int32(n)})
			plain := wireRx.Load() - rx0
			rx0 = wireRx.Load()
			got, err = cl.GetPayload(ctx, &x2v1.PayloadRequest{Profile: p, Count: int32(n)}, grpc.UseCompressor(grpcgzip.Name))
			zipped := wireRx.Load() - rx0
			if err == nil && p == x2data.Text && n == x2data.PoolSize && zipped*2 > plain {
				err = fmt.Errorf("gzip response used %d bytes on the wire, uncompressed %d: not compressed", zipped, plain)
			}
			c.check(fmt.Sprintf("payload %s %d gzip", p, n), equal(got, want, err))
		}
	}
	_, err = cl.GetPayload(ctx, &x2v1.PayloadRequest{Profile: "xml", Count: 1})
	c.check("payload bad profile", invalid(err))
	_, err = cl.GetPayload(ctx, &x2v1.PayloadRequest{Profile: "text", Count: 0})
	c.check("payload count 0", invalid(err))

	small := x2data.Items(x2data.Small, x2data.ExportSize)
	for _, n := range exportCounts {
		want := pbList(small[:n])
		got, err := cl.ExportUnary(ctx, &x2v1.ExportRequest{Count: int32(n)})
		c.check(fmt.Sprintf("export unary %d", n), equal(got, want, err))
		c.check(fmt.Sprintf("export stream %d", n), func() error {
			s, err := cl.ExportStream(ctx, &x2v1.ExportRequest{Count: int32(n)})
			if err != nil {
				return err
			}
			streamed := &x2v1.ItemList{}
			for {
				it, err := s.Recv()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					return err
				}
				streamed.Items = append(streamed.Items, it)
			}
			return equal(streamed, want, nil)
		}())
	}
	_, err = cl.ExportUnary(ctx, &x2v1.ExportRequest{Count: x2data.ExportSize + 1})
	c.check("export count too big", invalid(err))

	for _, n := range []int{1, 1000, 10000} {
		e := expectedSummary(n)
		want := &x2v1.ImportSummary{Count: e.Count, IdSum: e.IDSum, StockSum: e.StockSum}
		items := small[:n]
		got, err := cl.ImportUnary(ctx, pbList(items))
		c.check(fmt.Sprintf("import unary %d", n), equal(got, want, err))
		c.check(fmt.Sprintf("import stream %d", n), func() error {
			s, err := cl.ImportStream(ctx)
			if err != nil {
				return err
			}
			for i := range items {
				if err := s.Send(x2.ToPB(&items[i])); err != nil {
					return err
				}
			}
			got, err := s.CloseAndRecv()
			return equal(got, want, err)
		}())
	}

	for _, id := range quoteIDs {
		it := small[id-1]
		want := &x2v1.Quote{Id: it.ID, Price: it.Price, Stock: it.Stock}
		got, err := cl.GetQuote(ctx, &x2v1.QuoteRequest{Id: id})
		c.check(fmt.Sprintf("quote %d", id), equal(got, want, err))
	}
	_, err = cl.GetQuote(ctx, &x2v1.QuoteRequest{Id: 0})
	c.check("quote 0", invalid(err))
	c.check("quote stream", func() error {
		s, err := cl.QuoteStream(ctx)
		if err != nil {
			return err
		}
		for _, id := range quoteIDs {
			if err := s.Send(&x2v1.QuoteRequest{Id: id}); err != nil {
				return err
			}
			got, err := s.Recv()
			it := small[id-1]
			if err := equal(got, &x2v1.Quote{Id: it.ID, Price: it.Price, Stock: it.Stock}, err); err != nil {
				return err
			}
		}
		return s.CloseSend()
	}())

	c.check("feed", func() error {
		fctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		s, err := cl.Subscribe(fctx, &x2v1.SubscribeRequest{Rate: 50})
		if err != nil {
			return err
		}
		for range 3 {
			ev, err := s.Recv()
			if err != nil {
				return err
			}
			if err := checkEvent(x2data.PriceEvent{Seq: ev.Seq, ItemID: ev.ItemId, Price: ev.Price, SentAtUs: ev.SentAtUs}, small); err != nil {
				return err
			}
		}
		return nil
	}())
	return nil
}
