// Server resource monitor: one extra VU, running only during the measured
// phase, scrapes the server's runtime endpoint (METRICS_PORT, /runtime) once
// per second and records deltas of its cumulative counters as k6 metrics.
// These requests are tagged scenario=monitor and never mix with the measured
// request metrics, which are all filtered on scenario=measure.
//
// MONITOR_URL overrides the endpoint; MONITOR=off disables monitoring.
// For the compose targets it defaults to http://<target>:9100/runtime.

import http from 'k6/http';
import { sleep } from 'k6';
import { Counter, Trend } from 'k6/metrics';
import { target } from './config.js';

function resolveMonitorUrl() {
  if ((__ENV.MONITOR || '').toLowerCase() === 'off') return null;
  if (__ENV.MONITOR_URL) return __ENV.MONITOR_URL;
  if (['go-rest', 'ts-rest', 'go-grpc'].includes(target.name) && !__ENV.BASE_URL && !__ENV.GRPC_ADDR) {
    return `http://${target.name}:9100/runtime`;
  }
  return null;
}

export const monitorUrl = resolveMonitorUrl();

// Summed deltas (Counters) and sampled gauges (Trends).
export const m = {
  cpuSeconds: new Counter('server_cpu_seconds'),
  wallSeconds: new Counter('server_wall_seconds'),
  gcCount: new Counter('server_gc_count'),
  gcPauseMs: new Counter('server_gc_pause_ms'),
  rssMB: new Trend('server_rss_mb'),
  heapUsedMB: new Trend('server_heap_used_mb'),
  // Go
  goroutines: new Trend('server_goroutines'),
  gcCpuSeconds: new Counter('server_gc_cpu_seconds'),
  allocBytes: new Counter('server_alloc_bytes'),
  // Node.js
  loopActiveMs: new Counter('server_event_loop_active_ms'),
  loopIdleMs: new Counter('server_event_loop_idle_ms'),
  loopDelayP99Ms: new Trend('server_event_loop_delay_p99_ms'),
  loopDelayMaxMs: new Trend('server_event_loop_delay_max_ms'),
  scrapeErrors: new Counter('server_monitor_errors'),
  // cgroup CPU-quota throttling
  cgroupPeriods: new Counter('server_cgroup_periods'),
  cgroupThrottled: new Counter('server_cgroup_throttled_periods'),
  cgroupThrottledMs: new Counter('server_cgroup_throttled_ms'),
};

const MB = 1024 * 1024;
let prev = null;

export function monitor() {
  const res = http.get(monitorUrl, { responseType: 'text', timeout: '2s', tags: { name: 'monitor' } });
  if (res.status !== 200) {
    m.scrapeErrors.add(1);
    sleep(1);
    return;
  }
  const s = res.json();
  m.rssMB.add(s.rss_bytes / MB);
  m.heapUsedMB.add(s.heap_used_bytes / MB);
  if (s.runtime === 'go') m.goroutines.add(s.goroutines);
  if (s.runtime === 'node') {
    if (s.event_loop_delay_ms.p99 !== null) m.loopDelayP99Ms.add(s.event_loop_delay_ms.p99);
    if (s.event_loop_delay_ms.max !== null) m.loopDelayMaxMs.add(s.event_loop_delay_ms.max);
  }
  if (prev !== null) {
    m.cpuSeconds.add(s.cpu_seconds - prev.cpu_seconds);
    m.wallSeconds.add(s.uptime_seconds - prev.uptime_seconds);
    m.gcCount.add(s.gc_count - prev.gc_count);
    m.gcPauseMs.add((s.gc_pause_seconds - prev.gc_pause_seconds) * 1000);
    m.cgroupPeriods.add(s.cgroup_nr_periods - prev.cgroup_nr_periods);
    m.cgroupThrottled.add(s.cgroup_nr_throttled - prev.cgroup_nr_throttled);
    m.cgroupThrottledMs.add((s.cgroup_throttled_seconds - prev.cgroup_throttled_seconds) * 1000);
    if (s.runtime === 'go') {
      m.gcCpuSeconds.add(s.gc_cpu_seconds - prev.gc_cpu_seconds);
      m.allocBytes.add(s.allocated_bytes - prev.allocated_bytes);
    }
    if (s.runtime === 'node') {
      m.loopActiveMs.add(s.event_loop_active_ms - prev.event_loop_active_ms);
      m.loopIdleMs.add(s.event_loop_idle_ms - prev.event_loop_idle_ms);
    }
  }
  prev = s;
  sleep(1);
}
