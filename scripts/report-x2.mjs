#!/usr/bin/env node
// Experiment-2 report: aggregates results/<session>/results.jsonl (written by
// scripts/run-x2.sh) into report.md and report.csv in the same directory.
//
//   node scripts/report-x2.mjs results/<session>
//
// Every point is the median of its repeats; "±" is the spread between
// repeats ((max - min) / median). Marks: * errors, failed run, server
// restart (OOM) or client CPU-bound in at least one repeat; ~ spread > 20%.

import fs from 'node:fs';
import path from 'node:path';

const dir = process.argv[2];
if (!dir) {
  console.error('usage: node scripts/report-x2.mjs results/<session>');
  process.exit(1);
}

const readJSONL = (f) =>
  fs.existsSync(f)
    ? fs.readFileSync(f, 'utf8').split('\n').filter((l) => l.trim()).map((l) => JSON.parse(l))
    : [];

const runs = readJSONL(path.join(dir, 'results.jsonl'));
const env = fs.existsSync(path.join(dir, 'environment.json')) ? JSON.parse(fs.readFileSync(path.join(dir, 'environment.json'), 'utf8')) : {};

// ------------------------------------------------------------------ helpers
const median = (xs) => {
  const s = xs.filter((x) => Number.isFinite(x)).sort((a, b) => a - b);
  if (s.length === 0) return NaN;
  const m = Math.floor(s.length / 2);
  return s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2;
};
const spread = (xs) => {
  const s = xs.filter((x) => Number.isFinite(x));
  const m = median(s);
  return s.length > 1 && m ? (Math.max(...s) - Math.min(...s)) / Math.abs(m) : 0;
};
const get = (o, p) => p.split('.').reduce((v, k) => (v == null ? undefined : v[k]), o);
const fmt = (v, d = 0) => {
  if (!Number.isFinite(v)) return '–';
  if (Math.abs(v) >= 1000 && d === 0) return Math.round(v).toLocaleString('en-US');
  return v.toFixed(d);
};
const VARIANT_ORDER = ['go-rest', 'ts-rest', 'go-grpc', 'go-grpc-stream', 'go-grpc-mux'];
const LABEL = { 'go-rest': 'Go REST', 'ts-rest': 'TS REST', 'go-grpc': 'Go gRPC', 'go-grpc-stream': 'gRPC bidi stream', 'go-grpc-mux': 'gRPC, 100 streams/conn' };
const label = (v) => LABEL[v] ?? v;

// Group runs of a scenario by the given keys.
function groups(scenario, keys) {
  const map = new Map();
  for (const r of runs) {
    const sc = r.scenario ?? (r.args?.match(/--scenario (\S+)/) ?? [])[1];
    if (sc !== scenario) continue;
    const k = keys.map((key) => String(r[key] ?? (r.args?.match(new RegExp(`--${key} (\\S+)`)) ?? [])[1] ?? (key === 'gzip' && r.args ? r.args.includes('--gzip') : ''))).join('|');
    if (!map.has(k)) map.set(k, { key: Object.fromEntries(keys.map((key, i) => [key, k.split('|')[i]])), runs: [] });
    map.get(k).runs.push(r);
  }
  return [...map.values()];
}

function agg(g, metric) {
  const xs = g.runs.filter((r) => !r.failed).map((r) => (typeof metric === 'function' ? metric(r) : get(r, metric)));
  return { v: median(xs), spread: spread(xs) };
}

function marks(g) {
  const bad = g.runs.some((r) => r.failed || r.server_restarted || (r.errors ?? 0) > 0 || r.client?.cpu_bound || (r.feed && r.connect_failures > 0));
  return bad ? '*' : '';
}

function cell(g, metric, d = 0) {
  if (!g) return '–';
  const a = agg(g, metric);
  return fmt(a.v, d) + (a.spread > 0.2 ? '~' : '') + marks(g);
}

function ratio(ga, gb, metric) {
  if (!ga || !gb) return '–';
  const a = agg(ga, metric).v, b = agg(gb, metric).v;
  return Number.isFinite(a) && Number.isFinite(b) && b ? `${(a / b).toFixed(2)}×` : '–';
}

function table(header, rows) {
  return [`| ${header.join(' | ')} |`, `|${header.map((h, i) => (i < header.findIndex((x) => x.startsWith('§')) || i === 0 ? '---' : '---:')).join('|')}|`, ...rows.map((r) => `| ${r.join(' | ')} |`)].join('\n').replaceAll('§', '');
}

const numSort = (a, b) => (Number.isFinite(+a) && Number.isFinite(+b) ? +a - +b : String(a).localeCompare(String(b)));
const uniq = (xs) => [...new Set(xs)];
const variants = (gs) => uniq(gs.map((g) => g.key.variant)).sort((a, b) => VARIANT_ORDER.indexOf(a) - VARIANT_ORDER.indexOf(b));
const MB = (r) => (get(r, 'server.rss_peak_bytes') ?? NaN) / 1e6;

// pivot: one row per combination of rowKeys, one column per variant (or variant + mode).
function pivot(gs, rowKeys, colOf, metric, d, extraCols = []) {
  const cols = uniq(gs.map(colOf)).sort((a, b) => {
    const [va, ma] = a.split(' '), [vb, mb] = b.split(' ');
    return VARIANT_ORDER.indexOf(va) - VARIANT_ORDER.indexOf(vb) || String(ma).localeCompare(String(mb));
  });
  const rowMap = new Map();
  for (const g of gs) {
    const rk = rowKeys.map((k) => g.key[k]).join('|');
    if (!rowMap.has(rk)) rowMap.set(rk, {});
    rowMap.get(rk)[colOf(g)] = g;
  }
  const rowsSorted = [...rowMap.keys()].sort((a, b) => {
    const pa = a.split('|'), pb = b.split('|');
    for (let i = 0; i < pa.length; i++) {
      const c = numSort(pa[i], pb[i]);
      if (c) return c;
    }
    return 0;
  });
  const header = [...rowKeys, ...cols.map((c, i) => (i === 0 ? '§' : '') + c.split(' ').map((p, j) => (j === 0 ? label(p) : p)).join(' ')), ...extraCols.map((e) => e.title)];
  const rows = rowsSorted.map((rk) => {
    const byCol = rowMap.get(rk);
    return [...rk.split('|'), ...cols.map((c) => cell(byCol[c], metric, d)), ...extraCols.map((e) => e.fn(byCol))];
  });
  return table(header, rows);
}

// ------------------------------------------------------------------ report
const out = [];
const p = (s = '') => out.push(s);

p(`# Experiment 2 report: ${env.session ?? path.basename(dir)}`);
p();
p('Large payloads and streaming: Go REST/JSON vs TypeScript REST/JSON vs Go gRPC/Protobuf. Methodology: [docs/experiment-2-design.md](../../docs/experiment-2-design.md).');
p();
p(`Runs: ${runs.length} (${runs.filter((r) => r.failed).length} failed, ${runs.filter((r) => r.server_restarted).length} with a server restart). ` +
  'Each cell is the median of the repeats. `~` spread between repeats > 20%; `*` errors, failed run, server restart or client CPU-bound in at least one repeat.');
p();
if (env.docker) {
  p('| environment | |');
  p('|---|---|');
  p(`| CPU | ${env.cpu_model ?? '?'}; Docker VM ${env.docker_vm?.cpus} vCPUs, ${(env.docker_vm?.memory_bytes / 2 ** 30).toFixed(1)} GiB |`);
  p(`| Docker | ${env.docker.server}, kernel ${env.docker.kernel} |`);
  p(`| placement | servers pinned to CPU ${env.server_cpuset} (1 core, 1 GiB, GOMEMLIMIT ${env.gomemlimit}); load generator on ${env.loadgen_cpuset} |`);
  p(`| options | ${JSON.stringify(env.options)} |`);
  p(`| gzip | level ${env.gzip_level} on every server |`);
  p();
}

// ---- codec
const codecGo = readJSONL(path.join(dir, 'codec-go.jsonl'));
const codecNode = readJSONL(path.join(dir, 'codec-node.jsonl'));
if (codecGo.length) {
  p('## Codec micro-benchmark (one core, no network)');
  p();
  p('Time per whole message, µs. Sizes in KB. gzip at level 1.');
  p();
  const nodeOf = (c) => codecNode.find((n) => n.profile === c.profile && n.count === c.count) ?? {};
  p(table(
    ['profile', 'items', '§JSON KB', 'Proto KB', 'JSON gz KB', 'Proto gz KB', 'Go JSON enc', 'Go JSON dec', 'Proto enc', 'Proto dec', 'Node stringify', 'Node parse', 'gzip JSON', 'gzip Proto'],
    codecGo.map((c) => [c.profile, c.count, fmt(c.json_bytes / 1e3, 1), fmt(c.proto_bytes / 1e3, 1), fmt(c.json_gzip_bytes / 1e3, 1), fmt(c.proto_gzip_bytes / 1e3, 1),
      fmt(c.json_encode_us, 1), fmt(c.json_decode_us, 1), fmt(c.proto_encode_us, 1), fmt(c.proto_decode_us, 1), fmt(nodeOf(c).json_encode_us, 1), fmt(nodeOf(c).json_decode_us, 1),
      fmt(c.json_gzip_encode_us, 1), fmt(c.proto_gzip_encode_us, 1)]),
  ));
  p();
}

// ---- part A
const payload = groups('payload', ['network', 'profile', 'count', 'gzip', 'conc', 'variant']);
if (payload.length) {
  p('## Part A: large payloads (one request, one response)');
  p();
  p('Throughput is responses per second; goodput is response Mbit/s on the wire. `gRPC÷REST` compares Go gRPC with Go REST (same code, different protocol); `Go÷TS` compares the two REST servers.');
  for (const net of uniq(payload.map((g) => g.key.network))) {
    const gs = payload.filter((g) => g.key.network === net);
    const by = (v) => (b) => b[v];
    p();
    p(`### Network: ${net}`);
    p();
    p('**Responses per second**');
    p();
    p(pivot(gs, ['profile', 'count', 'gzip', 'conc'], (g) => g.key.variant, 'ops_per_s', 1, [
      { title: 'gRPC÷REST', fn: (b) => ratio(by('go-grpc')(b), by('go-rest')(b), 'ops_per_s') },
      { title: 'Go÷TS', fn: (b) => ratio(by('go-rest')(b), by('ts-rest')(b), 'ops_per_s') },
    ]));
    p();
    p('**Bytes on the wire per response (KB)** and **server CPU per response (ms)**');
    p();
    p(pivot(gs, ['profile', 'count', 'gzip', 'conc'], (g) => g.key.variant, (r) => (get(r, 'wire.bytes_per_op') ?? NaN) / 1e3, 1, [
      { title: 'CPU ms Go REST', fn: (b) => cell(b['go-rest'], (r) => get(r, 'server.cpu_us_per_op') / 1e3, 2) },
      { title: 'CPU ms TS REST', fn: (b) => cell(b['ts-rest'], (r) => get(r, 'server.cpu_us_per_op') / 1e3, 2) },
      { title: 'CPU ms gRPC', fn: (b) => cell(b['go-grpc'], (r) => get(r, 'server.cpu_us_per_op') / 1e3, 2) },
    ]));
    p();
    p('**Latency p50 / p99 (ms)** and **peak server RSS (MB)**');
    p();
    p(pivot(gs, ['profile', 'count', 'gzip', 'conc'], (g) => g.key.variant, 'latency_ms.p50', 1, [
      ...['go-rest', 'ts-rest', 'go-grpc'].map((v) => ({ title: `p99 ${label(v)}`, fn: (b) => cell(b[v], 'latency_ms.p99', 1) })),
      ...['go-rest', 'ts-rest', 'go-grpc'].map((v) => ({ title: `RSS ${label(v)}`, fn: (b) => cell(b[v], MB, 0) })),
    ]));
  }
  p();
}

// ---- part B
const exportG = groups('export', ['network', 'count', 'conc', 'mode', 'variant']);
if (exportG.length) {
  p('## Part B1: bulk export (unary = one JSON array / one message; stream = NDJSON / gRPC server stream)');
  for (const net of uniq(exportG.map((g) => g.key.network))) {
    const gs = exportG.filter((g) => g.key.network === net);
    const col = (g) => `${g.key.variant} ${g.key.mode}`;
    p();
    p(`### Network: ${net}`);
    p();
    p('**Items per second**');
    p();
    p(pivot(gs, ['count', 'conc'], col, 'items_per_s', 0));
    p();
    p('**Time to first item, p50 (ms)** (unary: the whole response)');
    p();
    p(pivot(gs, ['count', 'conc'], col, 'ttfi_ms.p50', 1));
    p();
    p('**Peak server RSS (MB)**');
    p();
    p(pivot(gs, ['count', 'conc'], col, MB, 0));
    p();
    p('**Server CPU per call (ms)**');
    p();
    p(pivot(gs, ['count', 'conc'], col, (r) => get(r, 'server.cpu_us_per_op') / 1e3, 1));
  }
  p();
}

const importG = groups('import', ['count', 'conc', 'mode', 'variant']);
if (importG.length) {
  const col = (g) => `${g.key.variant} ${g.key.mode}`;
  p('## Part B2: bulk import (unary = one JSON array / one message; stream = NDJSON body / gRPC client stream)');
  p();
  p('**Items per second**');
  p();
  p(pivot(importG, ['count', 'conc'], col, 'items_per_s', 0));
  p();
  p('**Server CPU per item (µs)** and **peak server RSS (MB)**');
  p();
  p(pivot(importG, ['count', 'conc'], col, (r) => get(r, 'server.cpu_us_per_op') / r.count, 2));
  p();
  p(pivot(importG, ['count', 'conc'], col, MB, 0));
  p();
}

const quoteG = groups('quote', ['conc', 'variant']);
if (quoteG.length) {
  p('## Part B3: many small calls (quote lookups; REST requests vs gRPC unary vs one gRPC bidirectional stream per client)');
  p();
  p('**Calls per second**');
  p();
  p(pivot(quoteG, ['conc'], (g) => g.key.variant, 'ops_per_s', 0, [
    { title: 'stream÷unary', fn: (b) => ratio(b['go-grpc-stream'], b['go-grpc'], 'ops_per_s') },
    { title: 'stream÷Go REST', fn: (b) => ratio(b['go-grpc-stream'], b['go-rest'], 'ops_per_s') },
  ]));
  p();
  p('**Server CPU per call (µs)**');
  p();
  p(pivot(quoteG, ['conc'], (g) => g.key.variant, 'server.cpu_us_per_op', 1));
  p();
  p('**Latency p50 / p99 (ms)**');
  p();
  p(pivot(quoteG, ['conc'], (g) => g.key.variant, 'latency_ms.p50', 2, ['go-rest', 'ts-rest', 'go-grpc', 'go-grpc-stream'].map((v) => ({ title: `p99 ${label(v)}`, fn: (b) => cell(b[v], 'latency_ms.p99', 2) }))));
  p();
}

const feedG = groups('feed', ['subscribers', 'rate', 'variant']);
if (feedG.length) {
  p('## Part B4: price feed (server push; SSE vs gRPC server streams)');
  p();
  p('Every subscriber receives every event. Load = subscribers × rate events per second.');
  p();
  p('**Delivery ratio** (received ÷ produced × subscribers)');
  p();
  p(pivot(feedG, ['subscribers', 'rate'], (g) => g.key.variant, 'feed.delivery_ratio', 3));
  p();
  p('**Delivery latency p50 / p99 (ms)** (server send → client receive)');
  p();
  p(pivot(feedG, ['subscribers', 'rate'], (g) => g.key.variant, 'latency_ms.p50', 1, variants(feedG).map((v) => ({ title: `p99 ${label(v)}`, fn: (b) => cell(b[v], 'latency_ms.p99', 1) }))));
  p();
  p('**Server CPU per delivered event (µs)**, **CPU utilisation** and **peak RSS (MB)**');
  p();
  p(pivot(feedG, ['subscribers', 'rate'], (g) => g.key.variant, 'server.cpu_us_per_event', 2, [
    ...variants(feedG).map((v) => ({ title: `util ${label(v)}`, fn: (b) => cell(b[v], 'server.cpu_util', 2) })),
    ...variants(feedG).map((v) => ({ title: `RSS ${label(v)}`, fn: (b) => cell(b[v], MB, 0) })),
  ]));
  p();
}

fs.writeFileSync(path.join(dir, 'report.md'), out.join('\n') + '\n');

// ------------------------------------------------------------------ CSV
const CSV_METRICS = [
  ['ops_per_s', 'ops_per_s'], ['items_per_s', 'items_per_s'], ['latency_p50_ms', 'latency_ms.p50'], ['latency_p99_ms', 'latency_ms.p99'],
  ['ttfi_p50_ms', 'ttfi_ms.p50'], ['wire_bytes_per_op', 'wire.bytes_per_op'], ['rx_mbit_s', 'wire.rx_mbit_s'], ['tx_mbit_s', 'wire.tx_mbit_s'],
  ['server_cpu_util', 'server.cpu_util'], ['server_cpu_us_per_op', 'server.cpu_us_per_op'], ['server_cpu_us_per_event', 'server.cpu_us_per_event'],
  ['server_rss_peak_bytes', 'server.rss_peak_bytes'], ['client_cpu_util', 'client.cpu_util'],
  ['delivery_ratio', 'feed.delivery_ratio'], ['events_per_s', 'feed.events_per_s'],
];
const KEYS = ['scenario', 'network', 'profile', 'count', 'gzip', 'conc', 'mode', 'subscribers', 'rate', 'variant'];
const csv = [[...KEYS, 'repeats', 'flags', ...CSV_METRICS.flatMap(([n]) => [n, `${n}_spread`])].join(',')];
for (const sc of ['payload', 'export', 'import', 'quote', 'feed']) {
  for (const g of groups(sc, KEYS.slice(1))) {
    const row = [sc, ...KEYS.slice(1).map((k) => g.key[k] ?? ''), g.runs.length, marks(g)];
    for (const [, m] of CSV_METRICS) {
      const a = agg(g, m);
      row.push(Number.isFinite(a.v) ? +a.v.toFixed(4) : '', Number.isFinite(a.v) ? +a.spread.toFixed(3) : '');
    }
    csv.push(row.join(','));
  }
}
fs.writeFileSync(path.join(dir, 'report.csv'), csv.join('\n') + '\n');
console.log(`wrote ${path.join(dir, 'report.md')} and report.csv (${runs.length} runs)`);
