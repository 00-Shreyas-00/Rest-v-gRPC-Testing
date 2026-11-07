# Experiment 2: large payloads and streaming

Experiment 1 asked which implementation serves small CRUD reads most efficiently on a
constrained machine. Its answer was mostly Go REST: gRPC lost on small messages,
and Protobuf's 35% smaller responses never turned into an advantage. That was
expected for that workload, because responses were small, the network was a loopback
and the data was mostly strings.

Experiment 2 asks the opposite question:

> **Under what workloads does gRPC's communication model give a measurable advantage
> over REST?**

It tests the two areas where gRPC is supposed to be strongest: **large payloads** and
**streaming**. It uses the same three servers as experiment 1: Go REST/JSON, TypeScript
REST/JSON (Node.js + Fastify) and Go gRPC/Protobuf.

- **Go REST vs Go gRPC** isolates the protocol: same language, same data, same process
  layout.
- **Go REST vs TS REST** isolates the runtime: same protocol, identical response bytes.

## What changed from experiment 1, and why

| Experiment 1 limitation | Experiment 2 change |
|---|---|
| Container-to-container traffic has no bandwidth limit, so smaller messages could not help | **Network emulation**: `netem` delay plus `tbf` bandwidth limits on every server and on the load generator, in three profiles |
| Product rows are mostly strings, which are the same size in JSON and Protobuf | **Three payload profiles**: text, numeric and binary |
| Real APIs compress JSON, and gzip shrinks it a lot | **Compression is a factor**: off, or gzip on every server |
| k6 buffers whole HTTP responses, cannot read SSE, and its gRPC decoding was a bottleneck | **A custom Go load generator** (`go-server/cmd/loadgen`), the same client code for every server |
| PostgreSQL hid protocol differences | **No database**: `DATA_SOURCE=none`, deterministic generated data held in memory |
| Go gRPC hit the 1 GiB memory limit on large lists | `GOMEMLIMIT=900MiB` on the Go servers; peak memory is recorded on the server for every run |

## Data

All data is generated deterministically from item ids. The generator is in Go
(`internal/x2data`) with a line-by-line TypeScript port (`src/x2/data.ts`). Both use
mulberry32 with integer and IEEE-754 operations that give identical results in both
languages.

**Item:** `id`, `sku`, `name`, `price`, `stock` (about 60 bytes), plus one heavy field
per profile:

| profile | heavy field | JSON vs Protobuf, 4000 items | after gzip (level 1) | what it tests |
|---|---|---|---|---|
| **text** | `description`: 1000 characters of pseudo-words | 4.41 MB vs 4.21 MB (1.05×) | 1.69 MB vs 1.67 MB | the control: strings encode the same in both |
| **numeric** | `timestamps` + `values`: 64 unix-ms timestamps and 64 full-precision doubles | 8.68 MB vs 3.81 MB (2.28×) | 3.89 MB vs 3.07 MB | numbers: an 8-byte double vs about 18 JSON characters; varints vs digits |
| **binary** | `thumbnail`: 1000 random bytes (base64 in JSON) | 5.75 MB vs 4.21 MB (1.36×) | 4.39 MB vs 4.21 MB | base64 overhead; incompressible |
| small | none (export, import, quote, feed) | 100k items: 8.70 MB vs 5.02 MB (1.73×) | 2.13 MB vs 1.83 MB | many small objects |

Sizes are from the codec micro-benchmark (`codec-go.jsonl` in every session).
On the numeric profile, gzipped JSON is about as large as uncompressed Protobuf.

The text uses pseudo-words built from syllables, not a small word list. A 32-word
vocabulary compressed 5.8× and made gzip about 2.5× slower than it is on real prose.

`loadgen verify` runs before every session and checks 124 cases:
- Both REST servers return **byte-identical** JSON, NDJSON and SSE frames. They equal
  the Go generator's output.
- The gRPC server returns messages equal to the generated data.
- Error responses match.
- gzip is applied on all three servers.

## Workloads

### Part A: large payloads (`GET /x2/payload` / `GetPayload`)

One request returns `count` items of one profile.

| factor | levels |
|---|---|
| server | Go REST, TS REST, Go gRPC |
| profile | text, numeric, binary |
| count | 10, 250, 4000 (about 10 KB, 250 KB, 4 MB of Protobuf) |
| compression | off; gzip (REST `Accept-Encoding`, gRPC `grpc-encoding`) |
| network | lan, dc, wan (see below) |
| concurrency | 1 (latency), 16 (throughput) |

**Compression:**
- gzip runs at **level 1** (BestSpeed, nginx's default) on all three servers.
- At level 6, gzip speed depends heavily on the data. The synthetic text compressed at
  ~7 MB/s, against ~18 MB/s for real English text on this VM. At level 1 both run at
  60–75 MB/s with similar ratios, so the results transfer to real data.

**Serialization:**
- Both REST servers serialize the whole body, then gzip it into a second buffer, then
  send it with a `Content-Length`.
- gRPC uses grpc-go's own framing and compressor.
- Every server reuses prebuilt item objects, so per request only serialization and
  compression is measured.

### Part B: streaming

| # | scenario | REST (Go and TS) | gRPC | levels |
|---|---|---|---|---|
| B1 | **bulk export** of `count` small items | `format=json`: one array; `format=ndjson`: one item per line, written in 16 KB chunks while serializing | `ExportUnary`: one message; `ExportStream`: one message per item | count 1k / 10k / 100k; concurrency 1 / 8; network lan / wan |
| B2 | **bulk import** of `count` small items | POST one JSON array, or an NDJSON body parsed line by line | `ImportUnary`: one message; `ImportStream`: client stream | count 1k / 10k; concurrency 1 / 8 |
| B3 | **many small calls** (quote lookup) | one GET per call (HTTP/1.1 keep-alive) | unary per call; or **one bidirectional stream per client** with one request/response exchange per call | concurrency 1 / 16 / 128 |
| B4 | **price feed** (server push) | Server-Sent Events | server stream, one connection per subscriber; or **100 streams per connection** | subscribers 100 / 1000 / 2500 / 5000 at 20 events/s each |

**Measured in streaming runs:**
- **Time to first item** for B1. For a unary response this equals the whole response
  time, because a normal client cannot use the first item before the body is complete.
- **Delivery latency** for B4: server send time to client receive time. The server
  stamps each event, and every container shares one VM clock.
- **Delivery ratio** and sequence gaps for B4. Each subscriber has a queue of 256
  events; when it fills, events are dropped and the client sees a gap.

**Feed fan-out:** the REST servers encode each SSE frame once and write the same bytes
to every subscriber. grpc-go marshals the message once per stream, which is how its
API works.

## Network profiles

Shaping is applied with `tc` to `eth0` of all three servers and of the load generator,
so both directions are shaped:

| profile | delay per direction | RTT | bandwidth | represents |
|---|---|---|---|---|
| **lan** | none | ~0.05 ms | unlimited (several Gbit/s) | experiment 1 conditions, same host |
| **dc** | 0.25 ms | 0.5 ms | 1 Gbit/s | services in one data centre |
| **wan** | 10 ms | 20 ms | 100 Mbit/s | cross-region, or a client on a good internet link |

- **Implementation:** `netem delay` as the root qdisc, with `tbf rate … burst 256kb
  latency …` as its child.
- **Why `tbf` and not `netem rate`:** `tbf` limits the queue in bytes. With segmentation
  offload, netem's packet-count limit would let gigabytes queue up (bufferbloat).
- **Check on 2026-11-06:** WAN measured 95.6 Mbit/s of goodput and a 21.5 ms round trip
  for small requests.
- **Support:** this works in Docker Desktop's WSL2 kernel.

## Load generator

`loadgen` (Go, `go-server/cmd/loadgen`) runs in its own container on the compose
network, pinned to CPUs 3–11.

**One connection per client:**
- REST: one HTTP/1.1 keep-alive connection per worker.
- gRPC: one HTTP/2 connection per worker.
- Closed model, as in experiment 1: each worker sends its next request when the
  previous one completes.

**Response handling:**
- Responses are not decoded during load. The REST client reads and discards the body
  (NDJSON: it counts newlines); the gRPC client counts messages without unmarshalling
  them.
- This keeps client decode cost from capping the server. In experiment 1 this was the
  k6 gRPC bottleneck.
- Decode cost is measured separately in the codec micro-benchmark.
- The B4 feed client does decode each event, because it needs the timestamp and sequence
  number.

**Other client details:**
- Import bodies and messages are encoded once per run, so client encoding is not
  measured.
- **Client saturation check:** client CPU utilisation over the window is recorded, and
  `cpu_bound` is set above 75% of its cores.

**Window and counters:**
- Warm-up, then the measured window. Operations that complete inside the window are
  measured; in-flight operations are cancelled at the end.
- Bytes on the wire are counted at the TCP connection, after compression, including
  HTTP and HTTP/2 framing.
- The server's runtime endpoint (`:9100/runtime`) is scraped at the window edges for
  CPU and GC deltas, and once per second for peak RSS, heap and goroutines.

## Controls

**CPU and memory:**
- Each server runs on **one dedicated vCPU** (cpuset 2, no quota) with **1 GiB**, as in
  experiment 1.
- All three servers share the core but are never loaded at the same time.
- PostgreSQL is stopped during the session.
- Go: `GOMEMLIMIT=900MiB`. Node: default heap sizing (it reads the container limit).

**Fresh processes and ordering:**
- Every server is restarted at the start of each repeat block. In B4 the target server
  is restarted before every run.
- Peak RSS therefore reflects the current workload, not leftovers from a previous one.
- Order: network, then repeat, then point (ascending size), then server. Every server
  sees the same drift, and repeats are spread over the session.

**Repeats and failures:**
- 3 repeats per point. The report shows the median and the spread.
- A run in which the server restarted (for example OOM-killed) is kept and marked
  `server_restarted`; the server is brought back before the next run.

**Codec micro-benchmark (no network):**
- JSON (Go `encoding/json`, Node `JSON.stringify`/`parse`) vs Protobuf encode/decode,
  plus gzip sizes and speed, for each profile and size.
- Runs pinned to the same single core as the servers.

## Hypotheses (stated before running)

1. **Text:** no meaningful gRPC advantage on any network. The sizes are equal, and
   encode CPU is similar.
2. **Numeric, binary:** on the WAN link, gRPC delivers roughly the size ratio more
   responses (about 2.3× numeric, 1.36× binary). On lan the gap is set by serialization
   CPU instead.
3. **gzip:**
   - Closes most of the size gap for text and numeric, at a CPU cost that matters on lan.
   - Does nothing for binary.
   - JSON+gzip can beat uncompressed Protobuf on the WAN.
4. **Streaming export:** streaming (NDJSON or gRPC) cuts time to first item by orders of
   magnitude, and reduces peak server memory compared with one large response. gRPC
   streaming and NDJSON are close.
5. **Push:**
   - SSE and gRPC streams cost about the same per event when each subscriber has its
     own connection.
   - Multiplexing many gRPC streams over few connections cuts CPU and per-connection
     memory. HTTP/1.1 SSE cannot do this.
6. **Many small calls:** a long-lived bidirectional gRPC stream removes most per-call
   overhead and beats both unary gRPC and REST by a wide margin. This is the "small
   messages, high RPC volume" case where experiment 1's unary gRPC lost.

The outcome of each hypothesis is in the README's
[Experiment 2 results](../README.md#hypotheses-vs-results) (session 2026-11-06/07,
data in [docs/results/2026-11-07-x2-full/](results/2026-11-07-x2-full/README.md)).

## Threats to validity

- **One laptop** (hybrid P/E cores) through a WSL2 VM. Network shaping is emulated on
  veth interfaces, so it has no real packet loss or jitter.
- **One core per server:** this measures per-core efficiency. Multi-core scaling is not
  covered.
- **Synthetic data:** real payloads mix the three profiles. The profiles bound the
  effect, from no gain (text) to the largest gain (numeric).
- **Idiomatic defaults:** grpc-go flow control and buffer sizes are left as shipped, and
  so are Fastify and net/http. Tuned configurations may differ.
- **gzip level 1** is not the strongest compression. Level 6 would shrink bytes slightly
  more at much higher CPU cost.
- **Feed fan-out encoding is not equal:** REST encodes each event once, while gRPC
  marshals per stream. This is the API as each stack offers it, and it is noted with the
  results.
- **Unary vs stream semantics:** a JSON array is unusable until it is complete. Streaming
  JSON parsers exist but are not the common client.

## Running it

Setup (once):
1. Plug in the laptop.
2. Disable sleep and screen-off (see README "Running experiment 2").
3. Keep the machine otherwise idle.

Commands:

```bash
scripts/run-x2.sh --dry-run          # plan and time estimate
scripts/run-x2.sh --quick            # ~10 min smoke test of the whole pipeline
scripts/run-x2.sh --session x2-full --repeat 3 2>&1 | tee results/x2-full.log   # both parts, ~10 h
```

Parts can be run separately: `--parts a` (~7 h) and `--parts b` (~3 h).

Output in `results/<session>/`:

| file | contents |
|---|---|
| `results.jsonl` | one line per run: all labels, throughput, latency, time to first item, wire bytes, server CPU / RSS / GC, client CPU, feed delivery |
| `report.md`, `report.csv` | medians and spreads per point, side-by-side tables and ratios |
| `codec-go.jsonl`, `codec-node.jsonl` | codec micro-benchmark |
| `verify.log` | equivalence check |
| `environment.json` | hardware, Docker, CPU sets, netem commands, image ids |
| `events.log` | network changes, failed runs, server restarts |

Regenerate the report: `node scripts/report-x2.mjs results/<session>`.
