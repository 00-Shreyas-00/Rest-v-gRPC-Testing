package monitoring

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

var sink [][]byte

func TestSnapshotTracksRuntime(t *testing.T) {
	before := Read()
	for i := 0; i < 2000; i++ {
		sink = append(sink, make([]byte, 4096))
	}
	sink = nil
	runtime.GC()
	after := Read()

	if after.Runtime != "go" || after.GOMAXPROCS < 1 || after.Goroutines < 1 {
		t.Errorf("bad snapshot %+v", after)
	}
	if after.UptimeSeconds <= before.UptimeSeconds || after.CPUSeconds < before.CPUSeconds {
		t.Errorf("time did not advance: %+v -> %+v", before, after)
	}
	if after.GCCount <= before.GCCount {
		t.Errorf("gc_count did not increase: %d -> %d", before.GCCount, after.GCCount)
	}
	if after.AllocatedBytes-before.AllocatedBytes < 2000*4096 {
		t.Errorf("allocated_bytes grew by %d, want >= %d", after.AllocatedBytes-before.AllocatedBytes, 2000*4096)
	}
	if runtime.GOOS == "linux" && after.RSSBytes == 0 {
		t.Error("rss_bytes is 0 on linux")
	}
	if after.HeapTotalBytes < after.HeapUsedBytes || after.HeapGoalBytes == 0 {
		t.Errorf("heap fields inconsistent: %+v", after)
	}
}

func TestHandler(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	res, err := http.Get(srv.URL + "/runtime")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var m map[string]any
	if err := json.NewDecoder(res.Body).Decode(&m); err != nil || res.StatusCode != 200 {
		t.Fatalf("status %d, err %v", res.StatusCode, err)
	}
	// Fields shared with the Node.js server.
	for _, k := range []string{"runtime", "uptime_seconds", "cpu_seconds", "rss_bytes", "heap_used_bytes",
		"heap_total_bytes", "gc_count", "gc_pause_seconds", "goroutines"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing %q", k)
		}
	}
}
