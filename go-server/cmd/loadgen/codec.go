package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"go-vs-ts-benchmark/go-server/gen/x2v1"
	"go-vs-ts-benchmark/go-server/internal/x2"
	"go-vs-ts-benchmark/go-server/internal/x2data"
)

// cmdCodec measures serialization alone, without network or HTTP: encoding
// and decoding each payload profile as JSON (encoding/json, what the Go REST
// server uses) and Protobuf (what gRPC uses), plus the sizes before and after
// gzip. The TypeScript counterpart is typescript-server/src/x2/codec-bench.ts.
// Run it pinned to the same single CPU as the servers.
func cmdCodec(args []string) error {
	fs := flag.NewFlagSet("codec", flag.ExitOnError)
	per := fs.Duration("time", time.Second, "measuring time per operation")
	_ = fs.Parse(args)

	cases := []struct {
		profile string
		count   int
	}{
		{x2data.Text, 10}, {x2data.Text, 250}, {x2data.Text, 4000},
		{x2data.Numeric, 10}, {x2data.Numeric, 250}, {x2data.Numeric, 4000},
		{x2data.Binary, 10}, {x2data.Binary, 250}, {x2data.Binary, 4000},
		{x2data.Small, 1000}, {x2data.Small, 100000},
	}
	for _, cs := range cases {
		items := x2data.Items(cs.profile, cs.count)
		list := x2data.ItemList{Items: items}
		pb := &x2v1.ItemList{}
		for i := range items {
			pb.Items = append(pb.Items, x2.ToPB(&items[i]))
		}
		js := jsonBytes(list)
		pbb, _ := proto.Marshal(pb)

		row := map[string]any{
			"runtime": "go", "profile": cs.profile, "count": cs.count,
			"json_bytes": len(js), "proto_bytes": len(pbb),
			"json_gzip_bytes": gzipLen(js), "proto_gzip_bytes": gzipLen(pbb),
		}
		row["json_encode_us"] = bench(*per, func() {
			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			enc.SetEscapeHTML(false)
			_ = enc.Encode(list)
		})
		row["json_decode_us"] = bench(*per, func() {
			var out x2data.ItemList
			_ = json.Unmarshal(js, &out)
		})
		row["proto_encode_us"] = bench(*per, func() { _, _ = proto.Marshal(pb) })
		row["proto_decode_us"] = bench(*per, func() {
			var out x2v1.ItemList
			_ = proto.Unmarshal(pbb, &out)
		})
		row["json_gzip_encode_us"] = bench(*per, func() { gzipLen(js) })
		row["proto_gzip_encode_us"] = bench(*per, func() { gzipLen(pbb) })
		b, _ := json.Marshal(row)
		fmt.Println(string(b))
	}
	return nil
}

func gzipLen(b []byte) int {
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, x2.GzipLevel)
	_, _ = zw.Write(b)
	_ = zw.Close()
	return buf.Len()
}

// bench returns microseconds per call (median of 5 timed batches).
func bench(total time.Duration, fn func()) float64 {
	fn() // warm
	n := 1
	for {
		t := time.Now()
		for range n {
			fn()
		}
		if time.Since(t) > total/20 {
			break
		}
		n *= 2
	}
	var res []float64
	for range 5 {
		t := time.Now()
		for range n {
			fn()
		}
		res = append(res, float64(time.Since(t).Microseconds())/float64(n))
	}
	for i := range res {
		for j := i + 1; j < len(res); j++ {
			if res[j] < res[i] {
				res[i], res[j] = res[j], res[i]
			}
		}
	}
	return round(res[2], 3)
}
