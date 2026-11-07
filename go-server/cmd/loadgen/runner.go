package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"go-vs-ts-benchmark/go-server/gen/x2v1"
	"go-vs-ts-benchmark/go-server/internal/x2data"
)

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	var f runFlags
	fs.StringVar(&f.scenario, "scenario", "payload", "payload | export | import | quote | feed")
	fs.StringVar(&f.target, "target", "go-rest", "go-rest | ts-rest | go-grpc")
	fs.StringVar(&f.profile, "profile", "text", "payload: text | numeric | binary")
	fs.StringVar(&f.mode, "mode", "unary", "export/import/quote: unary | stream")
	fs.IntVar(&f.count, "count", 10, "payload: items per response; export/import: items per call")
	fs.IntVar(&f.conc, "conc", 1, "closed-loop workers, one connection each")
	fs.IntVar(&f.subscribers, "subscribers", 100, "feed: concurrent subscriptions")
	fs.IntVar(&f.conns, "conns", 0, "feed, gRPC only: connections shared by the subscriptions [0 = one per subscription]")
	fs.IntVar(&f.rate, "rate", 20, "feed: events per second")
	fs.BoolVar(&f.gzip, "gzip", false, "payload: request gzip (REST Accept-Encoding, gRPC compressor)")
	fs.DurationVar(&f.timeout, "timeout", 60*time.Second, "per operation")
	warmup := fs.Duration("warmup", 10*time.Second, "warm-up (not measured)")
	duration := fs.Duration("duration", 20*time.Second, "measured window")
	labels := fs.String("labels", "", "extra JSON object merged into the output line, e.g. {\"network\":\"wan\",\"repeat\":1}")
	_ = fs.Parse(args)

	out := map[string]any{
		"scenario": f.scenario, "target": f.target, "warmup_s": warmup.Seconds(), "duration_s": duration.Seconds(),
		"started_at": time.Now().UTC().Format(time.RFC3339),
	}
	switch f.scenario {
	case "payload":
		out["profile"], out["count"], out["gzip"], out["conc"] = f.profile, f.count, f.gzip, f.conc
	case "export", "import":
		out["mode"], out["count"], out["conc"] = f.mode, f.count, f.conc
	case "quote":
		out["mode"], out["conc"] = f.mode, f.conc
	case "feed":
		out["subscribers"], out["rate"], out["conns"] = f.subscribers, f.rate, f.conns
	default:
		return fmt.Errorf("unknown scenario %q", f.scenario)
	}
	if *labels != "" {
		if err := json.Unmarshal([]byte(*labels), &out); err != nil {
			return fmt.Errorf("-labels: %w", err)
		}
	}

	var err error
	if f.scenario == "feed" {
		err = runFeed(f, *warmup, *duration, out)
	} else {
		var factory opFactory
		if factory, err = buildFactory(f); err == nil {
			err = runClosed(f, factory, *warmup, *duration, out)
		}
	}
	if err != nil {
		return err
	}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
	return nil
}

// ---------------------------------------------------------------- measurement window

// window samples everything that is measured over [start, end].
type window struct {
	mon        *monitor
	rx, tx     int64
	cpu        time.Duration
	start, end time.Time
}

func (w *window) begin(mon *monitor) {
	w.mon = mon
	mon.mark()
	w.rx, w.tx, w.cpu, w.start = wireRx.Load(), wireTx.Load(), cpuTime(), time.Now()
}

func (w *window) finish(out map[string]any, ops int) {
	w.end = time.Now()
	secs := w.end.Sub(w.start).Seconds()
	rx, tx := wireRx.Load()-w.rx, wireTx.Load()-w.tx
	wire := map[string]any{"rx_bytes": rx, "tx_bytes": tx, "rx_mbit_s": round(float64(rx)*8/secs/1e6, 3), "tx_mbit_s": round(float64(tx)*8/secs/1e6, 3)}
	if ops > 0 {
		wire["bytes_per_op"] = (rx + tx) / int64(ops)
	}
	out["wire"] = wire
	clientCPU := (cpuTime() - w.cpu).Seconds()
	cores := runtime.GOMAXPROCS(0)
	util := clientCPU / secs / float64(cores)
	out["client"] = map[string]any{"cpu_s": round(clientCPU, 3), "cores": cores, "cpu_util": round(util, 3), "cpu_bound": util > 0.75, "rss_peak_bytes": peakRSS()}
	out["server"] = w.mon.result(secs, ops, rx+tx)
}

func cpuTime() time.Duration {
	var ru syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

func peakRSS() int64 {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmHWM:") {
			kb, _ := strconv.ParseInt(strings.Fields(line)[1], 10, 64)
			return kb * 1024
		}
	}
	return 0
}

// ---------------------------------------------------------------- server monitor

// monitor scrapes the server's runtime endpoint (METRICS_PORT 9100) once per
// second for peaks, and at the window edges for CPU and GC deltas.
type monitor struct {
	url    string
	client *http.Client
	stop   chan struct{}
	wg     sync.WaitGroup

	mu         sync.Mutex
	first      map[string]any
	last       map[string]any
	rssStart   float64
	rssPeak    float64
	heapPeak   float64
	gorPeak    float64
	errors     int
	collecting bool
}

func newMonitor(target string) *monitor {
	m := &monitor{url: "http://" + target + ":9100/runtime", client: &http.Client{Timeout: 3 * time.Second}, stop: make(chan struct{})}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-m.stop:
				return
			case <-t.C:
				m.sample(false)
			}
		}
	}()
	return m
}

func (m *monitor) scrape() (map[string]any, error) {
	res, err := m.client.Get(m.url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var s map[string]any
	return s, json.NewDecoder(res.Body).Decode(&s)
}

func num(s map[string]any, k string) float64 {
	v, _ := s[k].(float64)
	return v
}

func (m *monitor) sample(edge bool) {
	s, err := m.scrape()
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		if m.collecting || edge {
			m.errors++
		}
		return
	}
	if edge && m.first == nil {
		m.first, m.rssStart = s, num(s, "rss_bytes")
		m.collecting = true
	}
	if !m.collecting {
		return
	}
	if edge {
		m.last = s
	}
	m.rssPeak = max(m.rssPeak, num(s, "rss_bytes"))
	m.heapPeak = max(m.heapPeak, num(s, "heap_used_bytes"))
	m.gorPeak = max(m.gorPeak, num(s, "goroutines"))
}

// mark is called at the start of the window, then again at its end.
func (m *monitor) mark() { m.sample(true) }

func (m *monitor) close() {
	close(m.stop)
	m.wg.Wait()
}

func (m *monitor) result(secs float64, ops int, wireBytes int64) map[string]any {
	m.mark()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.first == nil || m.last == nil {
		return map[string]any{"scrape_errors": m.errors}
	}
	a, b := m.first, m.last
	d := func(k string) float64 { return num(b, k) - num(a, k) }
	cpu := d("cpu_seconds")
	r := map[string]any{
		"runtime":         b["runtime"],
		"cpu_s":           round(cpu, 4),
		"cpu_util":        round(cpu/secs, 3),
		"rss_start_bytes": int64(m.rssStart),
		"rss_peak_bytes":  int64(m.rssPeak),
		"rss_end_bytes":   int64(num(b, "rss_bytes")),
		"heap_peak_bytes": int64(m.heapPeak),
		"gc_count":        int64(d("gc_count")),
		"gc_pause_s":      round(d("gc_pause_seconds"), 4),
		"scrape_errors":   m.errors,
	}
	if ops > 0 {
		r["cpu_us_per_op"] = round(cpu/float64(ops)*1e6, 2)
	}
	if wireBytes > 0 {
		r["cpu_ms_per_wire_mb"] = round(cpu*1e3/(float64(wireBytes)/1e6), 3)
	}
	if b["runtime"] == "go" {
		r["goroutines_peak"] = int64(m.gorPeak)
		r["gc_cpu_s"] = round(d("gc_cpu_seconds"), 4)
	} else {
		act, idle := d("event_loop_active_ms"), d("event_loop_idle_ms")
		if act+idle > 0 {
			r["event_loop_utilization"] = round(act/(act+idle), 3)
		}
	}
	return r
}

// ---------------------------------------------------------------- statistics

func round(v float64, digits int) float64 {
	p := math.Pow(10, float64(digits))
	return math.Round(v*p) / p
}

// dist summarises durations in milliseconds.
func dist(us []int64) map[string]any {
	if len(us) == 0 {
		return nil
	}
	slices.Sort(us)
	pct := func(q float64) float64 {
		i := int(math.Ceil(q*float64(len(us)))) - 1
		return round(float64(us[max(i, 0)])/1e3, 3)
	}
	var sum float64
	for _, v := range us {
		sum += float64(v)
	}
	return map[string]any{
		"p50": pct(0.50), "p90": pct(0.90), "p99": pct(0.99), "p999": pct(0.999),
		"max": round(float64(us[len(us)-1])/1e3, 3), "mean": round(sum/float64(len(us))/1e3, 3),
	}
}

// ---------------------------------------------------------------- closed loop

// runClosed runs f.conc workers, each repeating its operation back to back on
// its own connection (closed model, as in experiment 1). Operations that
// complete inside the window are measured; at the end of the window in-flight
// operations are cancelled.
func runClosed(f runFlags, factory opFactory, warmup, duration time.Duration, out map[string]any) error {
	ops := make([]op, f.conc)
	closers := make([]func(), f.conc)
	for w := range f.conc {
		o, c, err := factory(w)
		if err != nil {
			return err
		}
		ops[w], closers[w] = o, c
	}
	defer func() {
		for _, c := range closers {
			c()
		}
	}()

	mon := newMonitor(f.target)
	defer mon.close()
	start := time.Now()
	winStart, winEnd := start.Add(warmup), start.Add(warmup+duration)
	// Cancel (not a deadline) at the end of the window: gRPC propagates deadlines
	// to the server, which would fail in-flight calls slightly before the
	// client's own window ends and count them as errors.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopTimer := time.AfterFunc(time.Until(winEnd), cancel)
	defer stopTimer.Stop()

	type workerStats struct {
		lat, ttfi      []int64
		n, items, errs int
		warmErrs       int
		firstErr       string
	}
	stats := make([]workerStats, f.conc)
	var wg sync.WaitGroup
	for w := range f.conc {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := &stats[w]
			for ctx.Err() == nil {
				octx, ocancel := context.WithTimeout(ctx, f.timeout)
				t0 := time.Now()
				r, err := ops[w](octx)
				t1 := time.Now()
				ocancel()
				if ctx.Err() != nil {
					return // cut off by the end of the window
				}
				if err != nil {
					if t0.Before(winStart) {
						s.warmErrs++
					} else {
						s.errs++
					}
					if s.firstErr == "" {
						s.firstErr = err.Error()
					}
					time.Sleep(10 * time.Millisecond) // do not spin on a dead server
					continue
				}
				if t1.Before(winStart) {
					continue
				}
				s.n++
				s.items += r.items
				s.lat = append(s.lat, t1.Sub(t0).Microseconds())
				s.ttfi = append(s.ttfi, r.ttfi.Microseconds())
			}
		}()
	}

	var win window
	time.Sleep(time.Until(winStart))
	win.begin(mon)
	wg.Wait()
	var lat, ttfi []int64
	var n, items, errs, warmErrs int
	var firstErr string
	for _, s := range stats {
		lat, ttfi = append(lat, s.lat...), append(ttfi, s.ttfi...)
		n, items, errs, warmErrs = n+s.n, items+s.items, errs+s.errs, warmErrs+s.warmErrs
		if firstErr == "" {
			firstErr = s.firstErr
		}
	}
	win.finish(out, n)
	secs := win.end.Sub(win.start).Seconds()
	out["ops"], out["errors"], out["warmup_errors"] = n, errs, warmErrs
	if firstErr != "" {
		out["error_sample"] = firstErr
	}
	out["ops_per_s"] = round(float64(n)/secs, 2)
	out["items_per_s"] = round(float64(items)/secs, 1)
	out["latency_ms"] = dist(lat)
	if f.scenario == "export" {
		out["ttfi_ms"] = dist(ttfi)
	}
	return nil
}

// ---------------------------------------------------------------- feed

// runFeed opens f.subscribers subscriptions (ramped up in batches), waits
// until every one has received an event, then warms up and measures the
// delivery latency (server send time -> client receive time; client and
// servers share the Docker VM's clock), events received and sequence gaps
// (events the server dropped for a slow subscriber).
func runFeed(f runFlags, warmup, duration time.Duration, out map[string]any) error {
	addr, isGRPC, err := targetAddr(f.target)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var winStart, winEnd atomic.Int64
	winStart.Store(math.MaxInt64)
	winEnd.Store(math.MaxInt64)

	type subStats struct {
		lat            []int64
		received, gaps int64
		minSeq, maxSeq int64
		err            string
	}
	stats := make([]subStats, f.subscribers)
	var connected, failed atomic.Int64

	// handle is called for every event; returns false to stop.
	handle := func(s *subStats, first *bool, lastSeq *int64, seq, sentUs int64) {
		now := time.Now()
		if !*first {
			*first = true
			connected.Add(1)
		}
		ns := now.UnixNano()
		if ns >= winStart.Load() && ns <= winEnd.Load() {
			s.received++
			s.lat = append(s.lat, now.UnixMicro()-sentUs)
			if *lastSeq > 0 && seq > *lastSeq+1 {
				s.gaps += seq - *lastSeq - 1
			}
			if s.minSeq == 0 {
				s.minSeq = seq
			}
			s.maxSeq = seq
		}
		*lastSeq = seq
	}

	var subscribe func(i int)
	var cleanup func()
	if isGRPC {
		nconns := f.conns
		if nconns <= 0 || nconns > f.subscribers {
			nconns = f.subscribers
		}
		conns := make([]*grpc.ClientConn, nconns)
		for i := range conns {
			if conns[i], err = newGRPCConn(addr, false); err != nil {
				return err
			}
		}
		cleanup = func() {
			for _, c := range conns {
				c.Close()
			}
		}
		subscribe = func(i int) {
			s := &stats[i]
			st, err := conns[i%nconns].NewStream(ctx, stream(streamSubscribe), svc+"Subscribe")
			if err == nil {
				err = st.SendMsg(&x2v1.SubscribeRequest{Rate: int32(f.rate)})
			}
			if err == nil {
				err = st.CloseSend()
			}
			first, last := false, int64(0)
			for err == nil {
				var ev x2v1.PriceEvent
				if err = st.RecvMsg(&ev); err == nil {
					handle(s, &first, &last, ev.Seq, ev.SentAtUs)
				}
			}
			if !first && ctx.Err() == nil {
				failed.Add(1)
				s.err = err.Error()
			}
		}
	} else {
		tr := &http.Transport{DialContext: dial, MaxIdleConnsPerHost: -1, ReadBufferSize: 4 << 10, WriteBufferSize: 4 << 10}
		client := &http.Client{Transport: tr}
		cleanup = tr.CloseIdleConnections
		url := fmt.Sprintf("http://%s/x2/feed?rate=%d", addr, f.rate)
		subscribe = func(i int) {
			s := &stats[i]
			first, last := false, int64(0)
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			res, err := client.Do(req)
			if err == nil {
				defer res.Body.Close()
				err = checkStatus(res)
			}
			if err == nil {
				br := bufio.NewReaderSize(res.Body, 4<<10)
				prefix := []byte("data: ")
				for {
					line, rerr := br.ReadSlice('\n')
					if rerr != nil {
						err = rerr
						break
					}
					if !bytes.HasPrefix(line, prefix) {
						continue
					}
					var ev x2data.PriceEvent
					if err = json.Unmarshal(line[len(prefix):], &ev); err != nil {
						break
					}
					handle(s, &first, &last, ev.Seq, ev.SentAtUs)
				}
			}
			if !first && ctx.Err() == nil {
				failed.Add(1)
				if err != nil {
					s.err = err.Error()
				}
			}
		}
	}
	defer cleanup()

	// Ramp up in batches so the server's accept backlog is never exceeded.
	var wg sync.WaitGroup
	rampStart := time.Now()
	const batch = 250
	for i := 0; i < f.subscribers; i += batch {
		for j := i; j < min(i+batch, f.subscribers); j++ {
			wg.Add(1)
			go func() { defer wg.Done(); subscribe(j) }()
		}
		deadline := time.Now().Add(10 * time.Second)
		for connected.Load()+failed.Load() < int64(min(i+batch, f.subscribers)) && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
	}
	out["connect_s"] = round(time.Since(rampStart).Seconds(), 2)
	out["connected"], out["connect_failures"] = connected.Load(), failed.Load()

	mon := newMonitor(f.target)
	defer mon.close()
	time.Sleep(warmup)
	var win window
	win.begin(mon)
	winStart.Store(win.start.UnixNano())
	winEnd.Store(win.start.Add(duration).UnixNano())
	time.Sleep(duration)
	winEnd.Store(time.Now().UnixNano()) // exact end, read by every subscriber
	var lat []int64
	var received, gaps, minSeq, maxSeq int64
	// Let in-flight handlers finish before reading their stats.
	time.Sleep(50 * time.Millisecond)
	win.finish(out, 0)
	cancel()
	wg.Wait()
	var errSample string
	for _, s := range stats {
		lat = append(lat, s.lat...)
		received += s.received
		gaps += s.gaps
		if s.minSeq > 0 && (minSeq == 0 || s.minSeq < minSeq) {
			minSeq = s.minSeq
		}
		maxSeq = max(maxSeq, s.maxSeq)
		if errSample == "" {
			errSample = s.err
		}
	}
	secs := win.end.Sub(win.start).Seconds()
	conn := max(connected.Load(), 1)
	// Events the server produced in the window (from sequence numbers) times
	// subscribers is what should have arrived.
	produced := max(maxSeq-minSeq+1, 0)
	expected := produced * connected.Load()
	feed := map[string]any{
		"events_received":      received,
		"events_per_s":         round(float64(received)/secs, 1),
		"server_events_per_s":  round(float64(produced)/secs, 2),
		"expected":             expected,
		"seq_gaps":             gaps,
		"per_subscriber_per_s": round(float64(received)/secs/float64(conn), 2),
	}
	if expected > 0 {
		feed["delivery_ratio"] = round(float64(received)/float64(expected), 4)
	}
	out["feed"] = feed
	out["latency_ms"] = dist(lat)
	if errSample != "" {
		out["error_sample"] = errSample
	}
	if received > 0 {
		if srv, ok := out["server"].(map[string]any); ok {
			if cpu, ok := srv["cpu_s"].(float64); ok {
				srv["cpu_us_per_event"] = round(cpu/float64(received)*1e6, 3)
			}
		}
	}
	return nil
}
