// Package x2 serves experiment 2 (large payloads and streaming) over REST and
// gRPC. See proto/x2.proto for the endpoint list and docs/experiment-2-design.md
// for the methodology.
//
// The data set is built lazily on first use, so experiment-1 runs never pay
// for it. Responses reuse prebuilt items: per request the servers only
// serialize (and compress), which is the cost being compared.
package x2

import (
	"sync"
	"time"

	"go-vs-ts-benchmark/go-server/internal/x2data"
)

// MaxBodyBytes limits import request bodies (same limit as the TS server).
const MaxBodyBytes = 64 << 20

// MaxMessageBytes is the gRPC message size limit on both sides (default 4 MB).
const MaxMessageBytes = 64 << 20

type dataset struct {
	small    []x2data.Item
	profiles map[string][]x2data.Item
}

var (
	dataOnce sync.Once
	data     *dataset
)

func getData() *dataset {
	dataOnce.Do(func() {
		d := &dataset{small: x2data.Items(x2data.Small, x2data.ExportSize), profiles: map[string][]x2data.Item{}}
		for _, p := range x2data.Profiles {
			d.profiles[p] = x2data.Items(p, x2data.PoolSize)
		}
		data = d
	})
	return data
}

func nowUs() int64 { return time.Now().UnixMicro() }
