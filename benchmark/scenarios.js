// Shared scenario definition and result output for every workload.
//
// Each run has two phases with the same number of VUs:
//   warmup   WARMUP long; connection setup, JIT and caches. Not reported.
//   measure  DURATION long; the only phase in the results.
// Both use the closed model (constant-vus, no think time), so VUS is the
// number of concurrent in-flight requests.
//
//   monitor  1 VU alongside measure (when the target has a metrics endpoint):
//            scrapes server CPU, memory, GC and runtime metrics once per second.
//
// handleSummary writes one JSON result (RESULT_FILE) and prints one
// `RESULT {...}` line, both computed from the measure phase only.

import { seconds, settings, target } from './lib/config.js';
import { monitorUrl } from './lib/monitor.js';

export { monitor } from './lib/monitor.js';

/** Records the wall-clock start, so results can state the measured window. */
export function setup() {
  return { started_ms: Date.now() };
}

const GAP = 2; // seconds between phases, so warm-up requests never overlap the measured phase

let run = { workload: 'unknown', params: {}, splitBy: null };

/**
 * Builds the k6 options for a workload.
 * @param workload  name used in results, e.g. "list-products"
 * @param params    workload parameters recorded with the result
 * @param split     optional { tag, values } to also report latency per tag value
 */
export function buildOptions(workload, params = {}, split = null) {
  run = { workload, params, splitBy: split };
  const latency = latencyMetric();
  const warmupSeconds = seconds(settings.warmup);

  const scenarios = {};
  if (warmupSeconds > 0) {
    scenarios.warmup = {
      executor: 'constant-vus',
      vus: settings.vus,
      duration: settings.warmup,
      gracefulStop: `${GAP}s`,
    };
  }
  scenarios.measure = {
    executor: 'constant-vus',
    vus: settings.vus,
    duration: settings.duration,
    startTime: warmupSeconds > 0 ? `${warmupSeconds + GAP}s` : '0s',
    gracefulStop: '10s',
  };
  if (monitorUrl) {
    scenarios.monitor = {
      executor: 'constant-vus',
      exec: 'monitor',
      vus: 1,
      duration: settings.duration,
      startTime: scenarios.measure.startTime,
      gracefulStop: '3s',
    };
  }

  // Thresholds that always pass; they make k6 compute the measure-phase
  // sub-metrics that handleSummary reads.
  const thresholds = {
    [`${latency}{scenario:measure}`]: ['avg>=0'],
    'iterations{scenario:measure}': ['count>=0'],
    'data_sent{scenario:measure}': ['count>=0'],
    'data_received{scenario:measure}': ['count>=0'],
    'errors{scenario:measure}': ['rate>=0'],
    'timeouts{scenario:measure}': ['count>=0'],
  };
  if (split) {
    for (const v of split.values) {
      thresholds[`${latency}{scenario:measure,${split.tag}:${v}}`] = ['avg>=0'];
    }
  }

  return {
    scenarios,
    thresholds,
    discardResponseBodies: true,
    summaryTrendStats: ['count', 'avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
    tags: { target: target.name, protocol: target.protocol, workload },
  };
}

function latencyMetric() {
  return target.protocol === 'grpc' ? 'grpc_req_duration' : 'http_req_duration';
}

function values(data, name) {
  const m = data.metrics[name];
  return m ? m.values : {};
}

const round = (x, digits) =>
  x === undefined || x === null || !Number.isFinite(x) ? null : Math.round(x * 10 ** digits) / 10 ** digits;
const avgMax = (v, digits = 1) => ({ avg: round(v.avg, digits), max: round(v.max, digits) });

/**
 * Server resources over the measured window, from the monitor scenario.
 * CPU = process CPU time / wall time between the first and last scrape.
 */
function serverSummary(data, requestsPerSecond) {
  if (!monitorUrl) return null;
  const count = (name) => values(data, name).count || 0;
  const cpu = count('server_cpu_seconds');
  const wall = count('server_wall_seconds');
  if (wall <= 0) return { error: 'no runtime samples (is METRICS_PORT reachable?)', url: monitorUrl };

  const cores = cpu / wall;
  const limit = Number(__ENV.SERVER_CPUS_LABEL);
  const requestsInWindow = requestsPerSecond * wall;
  const out = {
    cpu_cores: round(cores, 3),
    cpu_percent_of_limit: Number.isFinite(limit) && limit > 0 ? round((100 * cores) / limit, 1) : null,
    cpu_ms_per_request: requestsPerSecond > 0 ? round((cores * 1000) / requestsPerSecond, 4) : null,
    rss_mb: avgMax(values(data, 'server_rss_mb')),
    heap_used_mb: avgMax(values(data, 'server_heap_used_mb')),
    gc: {
      count: count('server_gc_count'),
      pause_ms_total: round(count('server_gc_pause_ms'), 1),
      per_1k_requests: requestsInWindow > 0 ? round((1000 * count('server_gc_count')) / requestsInWindow, 3) : null,
    },
    // CPU-quota throttling: share of 100 ms scheduler periods in which the
    // container hit its CPU limit and was paused, and total paused time per second.
    cpu_throttling: count('server_cgroup_periods') > 0
      ? {
          throttled_period_fraction: round(count('server_cgroup_throttled_periods') / count('server_cgroup_periods'), 3),
          throttled_ms_per_second: round(count('server_cgroup_throttled_ms') / wall, 1),
        }
      : null,
    monitored_seconds: round(wall, 1),
    scrape_errors: count('server_monitor_errors'),
  };
  if (count('server_goroutines') > 0) {
    out.gc.cpu_fraction = cpu > 0 ? round(count('server_gc_cpu_seconds') / cpu, 4) : null;
    out.go = {
      goroutines: avgMax(values(data, 'server_goroutines'), 0),
      allocated_bytes_per_request:
        requestsInWindow > 0 ? Math.round(count('server_alloc_bytes') / requestsInWindow) : null,
    };
  }
  const active = count('server_event_loop_active_ms');
  const idle = count('server_event_loop_idle_ms');
  if (active + idle > 0) {
    out.node = {
      event_loop_utilization: round(active / (active + idle), 4),
      event_loop_delay_p99_ms: avgMax(values(data, 'server_event_loop_delay_p99_ms'), 2),
      event_loop_delay_max_ms: round(values(data, 'server_event_loop_delay_max_ms').max, 2),
    };
  }
  return out;
}

function latencyStats(v) {
  const r = (x) => (x === undefined ? null : Math.round(x * 1000) / 1000);
  return {
    avg: r(v.avg), min: r(v.min), med: r(v.med), max: r(v.max),
    p90: r(v['p(90)']), p95: r(v['p(95)']), p99: r(v['p(99)']),
  };
}

export function handleSummary(data) {
  const latency = latencyMetric();
  const measureSeconds = seconds(settings.duration);
  const lat = values(data, `${latency}{scenario:measure}`);
  const requests = values(data, 'iterations{scenario:measure}').count || 0;
  const errs = values(data, 'errors{scenario:measure}');
  const sent = values(data, 'data_sent{scenario:measure}').count || 0;
  const received = values(data, 'data_received{scenario:measure}').count || 0;
  const perRequest = (bytes) => (requests > 0 ? Math.round(bytes / requests) : null);

  const result = {
    workload: run.workload,
    params: run.params,
    target: target.name,
    protocol: target.protocol,
    address: target.address,
    data_source: __ENV.DATA_SOURCE_LABEL || 'unknown',
    server_cpus: __ENV.SERVER_CPUS_LABEL || 'unknown',
    cpu_mode: __ENV.CPU_MODE_LABEL || 'unknown', // quota | pinned | unlimited
    vus: settings.vus,
    repeat: Number(__ENV.REPEAT_INDEX) || 1,
    warmup: settings.warmup,
    duration: settings.duration,
    timeout: settings.timeout,
    started_at: __ENV.RUN_STARTED_AT || null,
    requests,
    requests_per_second: Math.round((requests / measureSeconds) * 10) / 10,
    latency_ms: latencyStats(lat),
    error_rate: errs.rate === undefined ? null : errs.rate,
    errors: errs.passes || 0,
    timeouts: values(data, 'timeouts{scenario:measure}').count || 0,
    bytes: {
      sent_total: sent,
      received_total: received,
      sent_per_request: perRequest(sent),
      received_per_request: perRequest(received),
    },
  };

  result.server = serverSummary(data, result.requests_per_second);

  // Wall-clock bounds of the measured phase (epoch ms), used by bench.sh to
  // keep only container samples taken under measured load.
  const t0 = data.setup_data && data.setup_data.started_ms;
  const warmupSeconds = seconds(settings.warmup);
  const measureStart = t0 ? t0 + (warmupSeconds > 0 ? (warmupSeconds + GAP) * 1000 : 0) : null;
  result.measure_window_ms = measureStart ? { start: measureStart, end: measureStart + measureSeconds * 1000 } : null;

  if (run.splitBy) {
    result.by_tag = {};
    for (const v of run.splitBy.values) {
      // The tag is on requests, not iterations, so count from the latency metric.
      const tagLat = values(data, `${latency}{scenario:measure,${run.splitBy.tag}:${v}}`);
      const n = tagLat.count || 0;
      result.by_tag[`${run.splitBy.tag}=${v}`] = {
        requests: n,
        requests_per_second: Math.round((n / measureSeconds) * 10) / 10,
        latency_ms: latencyStats(tagLat),
      };
    }
  }

  const l = result.latency_ms;
  const row = [result.target, result.vus, result.requests_per_second, l.avg, l.med, l.p90, l.p95, l.p99,
    result.errors, result.timeouts, result.bytes.received_per_request,
    result.server && result.server.cpu_cores != null ? result.server.cpu_cores : '-',
    result.server && result.server.rss_mb ? result.server.rss_mb.max : '-'].join(' ');
  const out = { stdout: `RESULT ${JSON.stringify(result)}\nROW ${row}\n` };
  if (__ENV.RESULT_FILE) out[__ENV.RESULT_FILE] = JSON.stringify(result, null, 2) + '\n';
  return out;
}
