// Package monitoring exposes process and Go runtime metrics on a separate
// HTTP listener (METRICS_PORT), off the benchmarked request path.
//
//	GET /runtime  ->  JSON snapshot (see Snapshot)
//
// Nothing is computed per request: values are read only when the endpoint is
// scraped (once per second by the k6 monitor scenario). Counters are
// cumulative since process start; the scraper computes deltas over the
// measured window. The TypeScript server serves the same common fields.
package monitoring

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"runtime"
	"runtime/metrics"
	"strconv"
	"syscall"
	"time"
)

// Snapshot is one reading. Fields shared with the Node.js server come first.
type Snapshot struct {
	Runtime        string  `json:"runtime"`         // "go"
	UptimeSeconds  float64 `json:"uptime_seconds"`  // monotonic, since process start
	CPUSeconds     float64 `json:"cpu_seconds"`     // process user+system CPU time
	RSSBytes       uint64  `json:"rss_bytes"`       // resident set size
	HeapUsedBytes  uint64  `json:"heap_used_bytes"` // memory occupied by heap objects (live + not yet swept)
	HeapTotalBytes uint64  `json:"heap_total_bytes"`
	GCCount        uint64  `json:"gc_count"`         // completed GC cycles
	GCPauseSeconds float64 `json:"gc_pause_seconds"` // stop-the-world GC pauses (approximated from a histogram)

	// Container CPU-quota throttling (cgroup v2 cpu.stat; 0 when unavailable).
	CgroupPeriods          uint64  `json:"cgroup_nr_periods"`
	CgroupThrottled        uint64  `json:"cgroup_nr_throttled"`
	CgroupThrottledSeconds float64 `json:"cgroup_throttled_seconds"`

	// Go-specific.
	Goroutines     uint64  `json:"goroutines"`
	GCCPUSeconds   float64 `json:"gc_cpu_seconds"`  // CPU spent in GC (mark assist, background, pauses)
	HeapLiveBytes  uint64  `json:"heap_live_bytes"` // live heap after the last GC
	HeapGoalBytes  uint64  `json:"heap_goal_bytes"` // heap size that triggers the next GC
	AllocatedBytes uint64  `json:"allocated_bytes"` // cumulative heap allocation
	GOMAXPROCS     int     `json:"gomaxprocs"`
}

var samples = []metrics.Sample{
	{Name: "/sched/goroutines:goroutines"},
	{Name: "/memory/classes/heap/objects:bytes"},
	{Name: "/memory/classes/heap/unused:bytes"},
	{Name: "/memory/classes/heap/free:bytes"},
	{Name: "/gc/heap/live:bytes"},
	{Name: "/gc/heap/goal:bytes"},
	{Name: "/gc/heap/allocs:bytes"},
	{Name: "/gc/cycles/total:gc-cycles"},
	{Name: "/cpu/classes/gc/total:cpu-seconds"},
	{Name: "/sched/pauses/total/gc:seconds"},
}

var start = time.Now()

// Read takes a snapshot. runtime/metrics does not stop the world.
func Read() Snapshot {
	s := make([]metrics.Sample, len(samples))
	copy(s, samples)
	metrics.Read(s)
	u := func(i int) uint64 {
		if s[i].Value.Kind() != metrics.KindUint64 {
			return 0
		}
		return s[i].Value.Uint64()
	}
	f := func(i int) float64 {
		if s[i].Value.Kind() != metrics.KindFloat64 {
			return 0
		}
		return s[i].Value.Float64()
	}

	cg := cgroupCPUStat()
	return Snapshot{
		CgroupPeriods:          cg["nr_periods"],
		CgroupThrottled:        cg["nr_throttled"],
		CgroupThrottledSeconds: float64(cg["throttled_usec"]) / 1e6,
		Runtime:                "go",
		UptimeSeconds:          time.Since(start).Seconds(),
		CPUSeconds:             processCPUSeconds(),
		RSSBytes:               rssBytes(),
		HeapUsedBytes:          u(1),
		HeapTotalBytes:         u(1) + u(2) + u(3),
		GCCount:                u(7),
		GCPauseSeconds:         histogramSum(s[9].Value),
		Goroutines:             u(0),
		GCCPUSeconds:           f(8),
		HeapLiveBytes:          u(4),
		HeapGoalBytes:          u(5),
		AllocatedBytes:         u(6),
		GOMAXPROCS:             runtime.GOMAXPROCS(0),
	}
}

// histogramSum approximates the total of a duration histogram using bucket
// midpoints (open-ended buckets use their finite bound).
func histogramSum(v metrics.Value) float64 {
	if v.Kind() != metrics.KindFloat64Histogram {
		return 0
	}
	h := v.Float64Histogram()
	var sum float64
	for i, c := range h.Counts {
		if c == 0 {
			continue
		}
		lo, hi := h.Buckets[i], h.Buckets[i+1]
		var mid float64
		switch {
		case math.IsInf(lo, -1):
			mid = hi
		case math.IsInf(hi, 1):
			mid = lo
		default:
			mid = (lo + hi) / 2
		}
		sum += float64(c) * mid
	}
	return sum
}

func processCPUSeconds() float64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	tv := func(t syscall.Timeval) float64 { return float64(t.Sec) + float64(t.Usec)/1e6 }
	return tv(ru.Utime) + tv(ru.Stime)
}

// cgroupCPUStat reads the container's cgroup v2 cpu.stat counters. With a CPU
// limit, nr_throttled counts 100 ms periods in which the container used up its
// quota and was paused until the next period.
func cgroupCPUStat() map[string]uint64 {
	out := map[string]uint64{}
	b, err := os.ReadFile("/sys/fs/cgroup/cpu.stat")
	if err != nil {
		return out
	}
	for _, line := range bytes.Split(b, []byte("\n")) {
		f := bytes.Fields(line)
		if len(f) == 2 {
			if v, err := strconv.ParseUint(string(f[1]), 10, 64); err == nil {
				out[string(f[0])] = v
			}
		}
	}
	return out
}

// rssBytes reads the resident set size from /proc/self/statm (Linux).
func rssBytes() uint64 {
	b, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0
	}
	fields := bytes.Fields(b)
	if len(fields) < 2 {
		return 0
	}
	pages, err := strconv.ParseUint(string(fields[1]), 10, 64)
	if err != nil {
		return 0
	}
	return pages * uint64(os.Getpagesize())
}

// Handler serves GET /runtime.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /runtime", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Read())
	})
	return mux
}

// Serve runs the metrics listener until ctx is cancelled. port 0 disables it.
func Serve(ctx context.Context, port int, log *slog.Logger) {
	if port == 0 {
		return
	}
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("metrics listener failed", "err", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	log.Info("metrics listening", "addr", srv.Addr, "path", "/runtime")
}
