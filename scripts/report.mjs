#!/usr/bin/env node
// Aggregates benchmark results into a Markdown report and a CSV.
//
//   node scripts/report.mjs [--out DIR] RESULTS_DIR...
//
// Reads every *.jsonl (results.jsonl) and environment.json below the given directories.
// Runs are grouped by (data source, server CPUs, workload, parameters, target,
// VUs); repeats of a group are combined into the median, with the spread
// (max - min) / median shown next to throughput. Writes report.md and
// report.csv to --out (default: the first RESULTS_DIR).
//
// No dependencies; runs on Node.js >= 18.

import fs from 'node:fs';
import path from 'node:path';

// ---------------------------------------------------------------------------
// Input

function parseArgs(argv) {
  const dirs = [];
  let out = null;
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === '--out') out = argv[++i];
    else if (argv[i] === '-h' || argv[i] === '--help') usage();
    else dirs.push(argv[i]);
  }
  if (dirs.length === 0) usage();
  return { dirs, out: out ?? dirs[0] };
}

function usage() {
  console.error('usage: node scripts/report.mjs [--out DIR] RESULTS_DIR...');
  process.exit(1);
}

function findFiles(dir, name, found = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) findFiles(p, name, found);
    else if (name instanceof RegExp ? name.test(entry.name) : entry.name === name) found.push(p);
  }
  return found;
}

function loadRuns(dirs) {
  const runs = [];
  const environments = [];
  for (const dir of dirs) {
    for (const file of findFiles(dir, /\.jsonl$/)) {
      for (const line of fs.readFileSync(file, 'utf8').split('\n')) {
        if (line.trim()) runs.push(JSON.parse(line));
      }
    }
    for (const file of findFiles(dir, 'environment.json')) {
      environments.push(JSON.parse(fs.readFileSync(file, 'utf8')));
    }
  }
  return { runs, environments };
}

// ---------------------------------------------------------------------------
// Aggregation

const TARGET_ORDER = ['go-rest', 'ts-rest', 'go-grpc'];
const LABEL = { 'go-rest': 'Go REST', 'ts-rest': 'TS REST', 'go-grpc': 'Go gRPC' };

const median = (xs) => {
  const v = xs.filter((x) => typeof x === 'number' && Number.isFinite(x)).sort((a, b) => a - b);
  if (v.length === 0) return null;
  const m = Math.floor(v.length / 2);
  return v.length % 2 ? v[m] : (v[m - 1] + v[m]) / 2;
};
const maxOf = (xs) => {
  const v = xs.filter((x) => typeof x === 'number' && Number.isFinite(x));
  return v.length ? Math.max(...v) : null;
};
const minOf = (xs) => {
  const v = xs.filter((x) => typeof x === 'number' && Number.isFinite(x));
  return v.length ? Math.min(...v) : null;
};
const get = (obj, keys) => keys.reduce((o, k) => (o == null ? undefined : o[k]), obj);

function paramKey(run) {
  const p = run.params || {};
  if (run.workload === 'list-products') return `limit=${p.limit}`;
  if (run.workload === 'search-products') {
    const terms = Array.isArray(p.terms) ? (p.terms.length > 1 ? 'all terms' : `term=${p.terms[0]}`) : '';
    return `${terms}, limit=${p.limit}`;
  }
  return '';
}

function aggregate(runs) {
  const groups = new Map();
  for (const r of runs) {
    const key = JSON.stringify([r.data_source, `${r.server_cpus} ${r.cpu_mode ?? 'quota'}`, r.workload, paramKey(r), r.target, r.vus]);
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(r);
  }
  const rows = [];
  for (const [key, rs] of groups) {
    const [dataSource, cpus, workload, params, target, vus] = JSON.parse(key);
    const col = (keys) => rs.map((r) => get(r, keys));
    const rps = col(['requests_per_second']);
    const rpsMedian = median(rps);
    rows.push({
      data_source: dataSource,
      server_cpus: cpus,
      workload,
      params,
      target,
      vus,
      runs: rs.length,
      rps_median: rpsMedian,
      rps_min: minOf(rps),
      rps_max: maxOf(rps),
      rps_spread: rpsMedian ? (maxOf(rps) - minOf(rps)) / rpsMedian : null,
      p50_ms: median(col(['latency_ms', 'med'])),
      p90_ms: median(col(['latency_ms', 'p90'])),
      p95_ms: median(col(['latency_ms', 'p95'])),
      p99_ms: median(col(['latency_ms', 'p99'])),
      avg_ms: median(col(['latency_ms', 'avg'])),
      errors: col(['errors']).reduce((a, b) => a + (b || 0), 0),
      timeouts: col(['timeouts']).reduce((a, b) => a + (b || 0), 0),
      requests: col(['requests']).reduce((a, b) => a + (b || 0), 0),
      rx_bytes_per_request: median(col(['bytes', 'received_per_request'])),
      tx_bytes_per_request: median(col(['bytes', 'sent_per_request'])),
      server_cpu_cores: median(col(['server', 'cpu_cores'])),
      server_cpu_ms_per_request: median(col(['server', 'cpu_ms_per_request'])),
      server_rss_mb_max: maxOf(col(['server', 'rss_mb', 'max'])),
      server_heap_mb_max: maxOf(col(['server', 'heap_used_mb', 'max'])),
      gc_per_1k_requests: median(col(['server', 'gc', 'per_1k_requests'])),
      gc_cpu_fraction: median(col(['server', 'gc', 'cpu_fraction'])),
      go_alloc_bytes_per_request: median(col(['server', 'go', 'allocated_bytes_per_request'])),
      go_goroutines_max: maxOf(col(['server', 'go', 'goroutines', 'max'])),
      node_event_loop_utilization: median(col(['server', 'node', 'event_loop_utilization'])),
      node_loop_delay_p99_ms_max: maxOf(col(['server', 'node', 'event_loop_delay_p99_ms', 'max'])),
      throttled_period_fraction: median(col(['server', 'cpu_throttling', 'throttled_period_fraction'])),
      k6_cpu_percent: median(col(['containers', 'k6', 'cpu_percent_avg'])),
      k6_cpu_bound: rs.some((r) => get(r, ['containers', 'k6_cpu_bound']) === true),
      postgres_cpu_percent: median(col(['containers', 'postgres', 'cpu_percent_avg'])),
    });
  }
  return rows;
}

// ---------------------------------------------------------------------------
// Formatting

const fmt = (x, digits = 0) =>
  x == null ? '–' : Number(x).toLocaleString('en-US', { minimumFractionDigits: digits, maximumFractionDigits: digits });
const pct = (x) => (x == null ? '' : ` ±${Math.round(x * 50)}%`); // half the full range
const ms = (x) => (x == null ? '–' : x < 10 ? fmt(x, 2) : x < 100 ? fmt(x, 1) : fmt(x, 0));

function flags(row) {
  let f = '';
  if (row.k6_cpu_bound) f += ' ⚠k6';
  if (row.errors > 0) f += ` ⚠${row.errors}err`;
  if (row.timeouts > 0) f += ` ⚠${row.timeouts}t/o`;
  return f;
}

function table(header, rows) {
  const line = (cells) => `| ${cells.join(' | ')} |`;
  return [line(header), line(header.map((_, i) => (i === 0 ? '---' : '---:'))), ...rows.map(line)].join('\n');
}

/** Ratio a/b with a marker when the two min..max ranges overlap (difference within run-to-run noise). */
function ratio(a, b) {
  if (!a || !b || !a.rps_median || !b.rps_median) return '–';
  const r = a.rps_median / b.rps_median;
  const overlap = a.rps_min <= b.rps_max && b.rps_min <= a.rps_max && (a.runs > 1 || b.runs > 1);
  return `${overlap ? '≈' : ''}${fmt(r, 2)}×`;
}
function latencyRatio(a, b) {
  if (!a || !b || !a.p99_ms || !b.p99_ms) return '–';
  return `${fmt(a.p99_ms / b.p99_ms, 2)}×`;
}

function sectionMarkdown(rows) {
  const targets = TARGET_ORDER.filter((t) => rows.some((r) => r.target === t));
  const vusLevels = [...new Set(rows.map((r) => r.vus))].sort((a, b) => a - b);
  const at = (t, v) => rows.find((r) => r.target === t && r.vus === v);
  const out = [];

  out.push('**Throughput** (requests/s, median of repeats, ± half the min–max range):\n');
  out.push(table(['VUs', ...targets.map((t) => LABEL[t] ?? t)],
    vusLevels.map((v) => [v, ...targets.map((t) => {
      const r = at(t, v);
      return r ? `${fmt(r.rps_median)}${pct(r.rps_spread)}${flags(r)}` : '–';
    })])));

  out.push('\n**Latency** (ms, median of repeats: p50 / p95 / p99):\n');
  out.push(table(['VUs', ...targets.map((t) => LABEL[t] ?? t)],
    vusLevels.map((v) => [v, ...targets.map((t) => {
      const r = at(t, v);
      return r ? `${ms(r.p50_ms)} / ${ms(r.p95_ms)} / ${ms(r.p99_ms)}` : '–';
    })])));

  out.push('\n**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):\n');
  out.push(table(['VUs', ...targets.map((t) => LABEL[t] ?? t)],
    vusLevels.map((v) => [v, ...targets.map((t) => {
      const r = at(t, v);
      return r ? `${fmt(r.server_cpu_cores, 2)} / ${fmt(r.server_cpu_ms_per_request, 3)} / ${fmt(r.server_rss_mb_max)}` : '–';
    })])));

  const runtimeRows = vusLevels.map((v) => {
    const cells = targets.map((t) => {
      const r = at(t, v);
      if (!r) return '–';
      const thr = r.throttled_period_fraction ? `, throttled ${fmt(r.throttled_period_fraction * 100, 0)}% of periods` : '';
      if (r.node_event_loop_utilization != null) {
        return `ELU ${fmt(r.node_event_loop_utilization, 2)}, loop delay p99 ≤ ${ms(r.node_loop_delay_p99_ms_max)} ms, GC/1k req ${fmt(r.gc_per_1k_requests, 0)}${thr}`;
      }
      if (r.gc_cpu_fraction != null) {
        return `GC ${fmt(r.gc_cpu_fraction * 100, 0)}% CPU, ${fmt((r.go_alloc_bytes_per_request ?? 0) / 1024, 1)} KiB alloc/req, ≤ ${fmt(r.go_goroutines_max)} goroutines${thr}`;
      }
      return '–';
    });
    return [v, ...cells];
  });
  out.push('\n**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):\n');
  out.push(table(['VUs', ...targets.map((t) => LABEL[t] ?? t)], runtimeRows));

  const has = (t) => targets.includes(t);
  const exps = [];
  if (has('go-rest') && has('ts-rest')) exps.push(['A: Go REST ÷ TS REST', 'go-rest', 'ts-rest']);
  if (has('go-grpc') && has('go-rest')) exps.push(['B: Go gRPC ÷ Go REST', 'go-grpc', 'go-rest']);
  if (has('go-grpc') && has('ts-rest')) exps.push(['C: Go gRPC ÷ TS REST', 'go-grpc', 'ts-rest']);
  if (exps.length) {
    out.push('\n**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):\n');
    out.push(table(['VUs', ...exps.map((e) => `${e[0]} req/s`), ...exps.map((e) => `${e[0].split(':')[0]} p99`)],
      vusLevels.map((v) => [v,
        ...exps.map(([, a, b]) => ratio(at(a, v), at(b, v))),
        ...exps.map(([, a, b]) => latencyRatio(at(a, v), at(b, v)))])));
  }

  out.push('\n**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):\n');
  out.push(table(['', ...targets.map((t) => LABEL[t] ?? t)], [
    ['received', ...targets.map((t) => fmt(median(rows.filter((r) => r.target === t).map((r) => r.rx_bytes_per_request))))],
    ['sent', ...targets.map((t) => fmt(median(rows.filter((r) => r.target === t).map((r) => r.tx_bytes_per_request))))],
  ]));
  return out.join('\n');
}

function environmentMarkdown(environments, runs) {
  const distinct = (key) => [...new Set(environments.map((e) => e[key]).filter((x) => x !== undefined && x !== ''))].join('; ');
  const fields = [
    ['Host CPU', 'host_cpu'], ['Host OS', 'host_os'], ['Docker', 'docker_version'], ['Docker VM', 'docker_os'],
    ['Docker VM kernel', 'docker_kernel'], ['Docker VM CPUs / memory', null], ['Go', 'go_version'],
    ['Node.js', 'node_version'], ['TypeScript', 'typescript_version'], ['PostgreSQL', 'postgres_version'],
    ['k6', 'k6_version'], ['Server CPU limit', 'server_cpus'], ['CPU limit mode', 'cpu_mode'], ['Server CPU set', 'server_cpuset'], ['k6 CPU set', 'k6_cpuset'], ['Server memory limit', 'server_memory'],
    ['PostgreSQL CPU limit', 'postgres_cpus'], ['k6 CPU limit', 'k6_cpus'], ['DB pool per server', 'db_max_conns'],
    ['Data source', 'data_source'], ['Products / dataset checksum', null], ['Warm-up / measured / cool-down', null],
    ['Repeats per point', 'repeat'], ['Network', 'network'], ['TLS', 'tls'],
  ];
  const rows = fields.map(([label, key]) => {
    if (key) return [label, distinct(key) || '–'];
    if (label.startsWith('Docker VM CPUs')) {
      const mem = distinct('docker_memory_bytes').split('; ').map((b) => (b ? `${(Number(b) / 2 ** 30).toFixed(1)} GiB` : '')).join('; ');
      return [label, `${distinct('docker_cpus')} / ${mem}`];
    }
    if (label.startsWith('Products')) return [label, `${distinct('dataset_products')} / ${distinct('dataset_checksum')}`];
    return [label, `${distinct('warmup')} / ${distinct('duration')} / ${distinct('cooldown_seconds')} s`];
  });
  const flagged = runs.filter((r) => get(r, ['containers', 'k6_cpu_bound'])).length;
  const errs = runs.reduce((a, r) => a + (r.errors || 0), 0);
  return [
    table(['', ''], rows),
    '',
    `Runs: ${runs.length}. Runs where k6 was CPU-bound: ${flagged}. Total errors: ${errs}.`,
  ].join('\n');
}

function csv(rows) {
  const cols = Object.keys(rows[0] ?? {});
  const esc = (v) => (v == null ? '' : /[",\n]/.test(String(v)) ? `"${String(v).replace(/"/g, '""')}"` : String(v));
  return [cols.join(','), ...rows.map((r) => cols.map((c) => esc(r[c])).join(','))].join('\n') + '\n';
}

// ---------------------------------------------------------------------------

function main() {
  const { dirs, out } = parseArgs(process.argv.slice(2));
  const { runs, environments } = loadRuns(dirs);
  if (runs.length === 0) {
    console.error(`no results.jsonl found under ${dirs.join(', ')}`);
    process.exit(1);
  }
  const rows = aggregate(runs);
  rows.sort((a, b) =>
    a.data_source.localeCompare(b.data_source) || a.server_cpus.localeCompare(b.server_cpus) ||
    a.workload.localeCompare(b.workload) || a.params.localeCompare(b.params, undefined, { numeric: true }) ||
    a.vus - b.vus || TARGET_ORDER.indexOf(a.target) - TARGET_ORDER.indexOf(b.target));

  const sections = new Map();
  for (const r of rows) {
    const key = `${r.data_source}|${r.server_cpus}|${r.workload}|${r.params}`;
    if (!sections.has(key)) sections.set(key, []);
    sections.get(key).push(r);
  }

  const md = ['# Benchmark report', '', `Generated ${new Date().toISOString()} from ${dirs.join(', ')}.`, '',
    '## Environment', '', environmentMarkdown(environments, runs), ''];
  md.push('Legend: ⚠k6 = k6 was CPU-bound in at least one repeat (client limit, not a server result); ⚠Nerr / ⚠Nt/o = errors / timeouts summed over repeats.', '');
  for (const [key, sectionRows] of sections) {
    const [ds, cpuKey, workload, params] = key.split('|');
    const [cpus, mode] = cpuKey.split(' ');
    md.push(`## ${workload}${params ? ` (${params})` : ''}: ${ds} mode, ${cpus} CPU${cpus === '1' ? '' : 's'} per server (${mode})`, '');
    md.push(sectionMarkdown(sectionRows), '');
  }

  fs.mkdirSync(out, { recursive: true });
  fs.writeFileSync(path.join(out, 'report.md'), md.join('\n'));
  fs.writeFileSync(path.join(out, 'report.csv'), csv(rows));
  console.log(md.join('\n'));
  console.error(`\nwrote ${path.join(out, 'report.md')} and report.csv (${runs.length} runs, ${rows.length} groups)`);
}

main();
