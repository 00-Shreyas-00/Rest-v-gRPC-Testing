// Process and Node.js runtime metrics on a separate HTTP listener
// (METRICS_PORT), off the benchmarked request path. Counterpart of
// go-server/internal/monitoring; the common fields have the same names.
//
//   GET /runtime  ->  JSON snapshot
//
// Nothing is computed per request. Counters are cumulative since process
// start; the scraper (k6 monitor scenario, once per second) computes deltas.
// The only continuous costs are a GC PerformanceObserver and an event-loop
// delay histogram sampling every 20 ms.

import fs from 'node:fs';
import http from 'node:http';
import { monitorEventLoopDelay, performance, PerformanceObserver } from 'node:perf_hooks';
import type { Logger } from './log.ts';

export interface RuntimeSnapshot {
  runtime: 'node';
  uptime_seconds: number; // monotonic, since process start
  cpu_seconds: number; // process user+system CPU time (all threads)
  rss_bytes: number;
  heap_used_bytes: number; // V8 heap in use
  heap_total_bytes: number; // V8 heap reserved
  gc_count: number;
  gc_pause_seconds: number; // main-thread time spent in GC
  // Container CPU-quota throttling (cgroup v2 cpu.stat; 0 when unavailable).
  cgroup_nr_periods: number;
  cgroup_nr_throttled: number;
  cgroup_throttled_seconds: number;
  // Node-specific.
  external_bytes: number;
  event_loop_active_ms: number; // cumulative; utilization = Δactive / (Δactive + Δidle)
  event_loop_idle_ms: number;
  // Delay beyond the sampling interval (0 = responsive loop), since the previous scrape.
  event_loop_delay_ms: { p50: number | null; p99: number | null; max: number | null };
  active_resources: number;
}

// Node records the full interval between timer ticks (about this value when
// idle), so the reported delay subtracts it.
const LOOP_DELAY_RESOLUTION_MS = 20;

export class RuntimeMonitor {
  private gcCount = 0;
  private gcMs = 0;
  private readonly loopDelay = monitorEventLoopDelay({ resolution: LOOP_DELAY_RESOLUTION_MS });
  private readonly gcObserver: PerformanceObserver;

  constructor() {
    this.gcObserver = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        this.gcCount++;
        this.gcMs += entry.duration;
      }
    });
    this.gcObserver.observe({ entryTypes: ['gc'] });
    this.loopDelay.enable();
  }

  snapshot(): RuntimeSnapshot {
    const mem = process.memoryUsage();
    const cpu = process.cpuUsage();
    const elu = performance.eventLoopUtilization();
    const d = this.loopDelay;
    // ns -> ms beyond the sampling interval, 2 decimals
    const ms = (ns: number) => Math.max(0, Math.round((ns / 1e6 - LOOP_DELAY_RESOLUTION_MS) * 100) / 100);
    const delay = d.count > 0
      ? { p50: ms(d.percentile(50)), p99: ms(d.percentile(99)), max: ms(d.max) }
      : { p50: null, p99: null, max: null };
    d.reset();
    const cg = cgroupCpuStat();
    return {
      cgroup_nr_periods: cg.nr_periods ?? 0,
      cgroup_nr_throttled: cg.nr_throttled ?? 0,
      cgroup_throttled_seconds: (cg.throttled_usec ?? 0) / 1e6,
      runtime: 'node',
      uptime_seconds: performance.now() / 1000,
      cpu_seconds: (cpu.user + cpu.system) / 1e6,
      rss_bytes: mem.rss,
      heap_used_bytes: mem.heapUsed,
      heap_total_bytes: mem.heapTotal,
      gc_count: this.gcCount,
      gc_pause_seconds: this.gcMs / 1000,
      external_bytes: mem.external,
      event_loop_active_ms: elu.active,
      event_loop_idle_ms: elu.idle,
      event_loop_delay_ms: delay,
      active_resources: process.getActiveResourcesInfo().length,
    };
  }

  stop(): void {
    this.gcObserver.disconnect();
    this.loopDelay.disable();
  }
}

/** cgroup v2 cpu.stat counters of the container (empty when unavailable). */
function cgroupCpuStat(): Record<string, number> {
  const out: Record<string, number> = {};
  try {
    for (const line of fs.readFileSync('/sys/fs/cgroup/cpu.stat', 'utf8').split('\n')) {
      const [k, v] = line.split(' ');
      if (k && v !== undefined) out[k] = Number(v);
    }
  } catch {
    // not Linux or no cgroup v2: report zeros
  }
  return out;
}

export function createMetricsServer(monitor: RuntimeMonitor): http.Server {
  return http.createServer((req, res) => {
    if (req.method === 'GET' && req.url === '/runtime') {
      res.writeHead(200, { 'content-type': 'application/json' });
      res.end(JSON.stringify(monitor.snapshot()));
      return;
    }
    res.writeHead(404).end();
  });
}

/** Starts the metrics listener; port 0 disables it. */
export async function serveMetrics(port: number, log: Logger): Promise<{ close: () => Promise<void> }> {
  if (port === 0) return { close: async () => {} };
  const monitor = new RuntimeMonitor();
  const server = createMetricsServer(monitor);
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(port, '0.0.0.0', resolve);
  });
  log.info('metrics listening', { addr: `:${port}`, path: '/runtime' });
  return {
    close: () =>
      new Promise<void>((resolve) => {
        monitor.stop();
        server.close(() => resolve());
        server.closeAllConnections();
      }),
  };
}
