# Go gRPC/Protobuf vs Go REST/JSON vs TypeScript REST/JSON

A small, controlled benchmark comparing three implementations of the same
Product Catalog API, all backed by one PostgreSQL database:

| Implementation        | Runtime  | Transport | Encoding |
|-----------------------|----------|-----------|----------|
| Go REST               | Go       | HTTP/1.1  | JSON     |
| Go gRPC               | Go       | HTTP/2    | Protobuf |
| TypeScript REST       | Node.js  | HTTP/1.1  | JSON     |

Two separate variables are under test, and they are kept apart on purpose:

- **Go vs Node.js** (Experiment A: Go REST vs TypeScript REST): same protocol, different runtime.
- **REST/JSON vs gRPC/Protobuf** (Experiment B: Go REST vs Go gRPC): same runtime and service code, different protocol.

Experiment C (TypeScript REST vs Go gRPC) compares the complete stacks, but on its own
it cannot say which variable caused a difference.

> **Status:** All ten phases are implemented. The first benchmark session (2026-11-05)
> covers **database mode only**: get-product and all four list sizes. Search and in-memory
> mode were not measured. See [Results](#results-2026-11-05-database-mode).

## Quick start

Requirements: Docker with Compose v2, and a bash shell (Linux, macOS, WSL or Git Bash on
Windows). You do not need a local Go, Node.js or `psql`.

```bash
docker compose up -d --build --wait
```

This builds the images, starts PostgreSQL, creates and seeds the database (100k products,
first start only) and starts the three servers:

| service | address | |
|---|---|---|
| `go-rest` | http://localhost:8080 (metrics :9101) | Go REST + JSON |
| `ts-rest` | http://localhost:8081 (metrics :9102) | TypeScript REST + JSON |
| `go-grpc` | localhost:9090 (metrics :9103) | Go gRPC + Protobuf (plaintext HTTP/2) |
| `postgres` | localhost:5432 | PostgreSQL 18.1 (`bench` / `bench`, database `products`) |

```bash
curl localhost:8080/products/12345
```
```bash
curl localhost:8081/products/12345
```
```bash
grpcurl -plaintext -d '{"id":12345}' localhost:9090 product.v1.ProductService/GetProduct
```

Switch every server to in-memory mode, and back:

```bash
DATA_SOURCE=memory docker compose up -d --wait
```
```bash
docker compose up -d --wait
```

Check that all three servers behave identically before benchmarking:

```bash
scripts/equivalence.sh
```

Run a benchmark (k6 in a container, all three servers, every concurrency level):

```bash
scripts/bench.sh get-product
```

## Results (2026-11-05, database mode)

Full report, CSV and raw per-run data:
[docs/results/2026-11-05-postgres-pinned/](docs/results/2026-11-05-postgres-pinned/report.md).

### What was measured

| | |
|---|---|
| Machine | Laptop, Intel i5-12450H (4 performance + 4 efficiency cores, 12 threads), Windows 11, Docker Desktop 29.7.2 (WSL2 VM: 12 vCPUs, 7.6 GiB) |
| Versions | Go 1.26.3, Node.js 24.22.0, TypeScript 7.0.2, Fastify 5.12.7, grpc-go 1.84.0, PostgreSQL 18.1, k6 2.2.0 |
| CPU | each server **pinned to one vCPU** (no quota); PostgreSQL 3 vCPUs; k6 6 vCPUs, never CPU-bound in any run |
| Data | PostgreSQL, 100,000 products (checksum `d26bc7a6…`), pool of 20 connections per server, warm cache |
| Workloads | get one product (1000 deterministic ids); list 10 / 50 / 100 / 500 products (first 20 pages) |
| Load | closed model, 1–1000 concurrent VUs, 10 s warm-up + 20 s measured, 3 interleaved repeats |
| Network | container to container on the Docker bridge network, plaintext (no TLS), HTTP/1.1 keep-alive and HTTP/2 |
| Correctness | equivalence suite passed on all three servers before the run (1204 REST + 1188 gRPC requests per mode) |
| Runs | 297 kept; 18 excluded because the laptop entered Windows Modern Standby during them ([exclusions.json](docs/results/2026-11-05-postgres-pinned/exclusions.json)) |

**Not measured:** search, in-memory mode, more than one CPU per server, and quota-limited
CPUs. The session was stopped after 315 of 756 planned runs.

### Main numbers

Throughput is the median req/s of 3 repeats; latency is the median p99. Each server had one
dedicated CPU and used all of it at 10 VUs and above.

| workload, VUs | Go REST | TS REST | Go gRPC |
|---|---:|---:|---:|
| get-product, 50 | 9,802 req/s, p99 13 ms | 5,546, p99 17 ms | 6,264, p99 18 ms |
| get-product, 1000 | 8,379, p99 193 ms | 3,722, p99 424 ms | 5,434, p99 297 ms |
| list 100, 100 | 1,481, p99 118 ms | 1,524, p99 116 ms | 1,638, p99 96 ms |
| list 500, 250 | 373, p99 905 ms | 267, p99 1,212 ms | 404, p99 877 ms |
| list 500, 1000 | 390, p99 3.1 s | 279 (2,848 timeouts > 10 s) | 389, p99 3.6 s |

Server cost per request, in CPU ms (the most stable metric across repeats):

| workload (at 50–250 VUs) | Go REST | TS REST | Go gRPC |
|---|---:|---:|---:|
| get-product | 0.10–0.11 | 0.18–0.22 | 0.16–0.17 |
| list 100 | 0.67–0.69 | 0.63–0.89 | 0.60–0.66 |
| list 500 | 2.1–2.7 | 3.4–3.7 | 2.2–2.5 |

Memory and bytes on the wire:
- **Peak RSS:** Go REST 21–77 MB, Go gRPC 36–150 MB, TS REST 123–187 MB.
- **Bytes per request:** Protobuf responses were 35% smaller on the wire. List 500 was
  107 KB vs 165 KB, and a single product 295 B vs 453 B (Go REST) and 502 B (TS REST).

### Findings

These apply to this machine, this workload and one dedicated CPU per server. They are
not general claims about the technologies.

1. **Experiment A, Go vs Node.js (both REST/JSON).**
   - **Small responses:** Go REST delivered 1.6–2.3× the throughput of TS REST for single
     products at every concurrency, with lower p99, and used about half the CPU per request.
   - **Mid-size lists (100 items):** there was no clear difference at 10–100 VUs. Node was
     sometimes slightly ahead, and the ratios fall within noise.
   - **Large lists (500 items):** Go REST was 1.4× faster from 250 VUs up, and needed 30%
     less CPU per request.
   - **Overload:** at 500 and 1000 VUs on 500-item lists, TS REST hit the 10 s client
     timeout for thousands of requests, while Go's latency rose more evenly.
   - **Node's limit:** its event loop was 100% busy (utilization 1.0) from 10 VUs on, so
     the single JavaScript thread was the limit.
2. **Experiment B, REST/JSON vs gRPC/Protobuf (both Go, same service code).**
   - **Small responses:** gRPC reached only 0.6–0.8× of REST's throughput and used about
     60% more CPU per request, despite 35% fewer bytes. Per-call HTTP/2 and gRPC handling
     costs more than the JSON it saves on a 300-byte message.
   - **Lists of 100:** gRPC was about 1.1–1.2× faster at moderate concurrency.
   - **Lists of 500:** throughput and CPU per request were equal within noise.
   - **Bottom line:** in this setup, Protobuf's size advantage shows up on the network,
     not as server CPU savings.
3. **Experiment C, TS REST vs Go gRPC.** Go gRPC was 1.2–1.5× faster for most points, but
   this compares complete stacks. The difference mixes the runtime effect (A, in Go's
   favour) and the protocol effect (B, which here partly cancels it).
4. **Memory.** Under the same load, Go REST used 2–6× and Go gRPC 1.2–3.5× less resident
   memory than Node.js. gRPC's footprint grows with concurrency, at two goroutines per
   connection.

### Limits of this data

- **Noise.** Identical runs on this laptop varied by up to ±30%, and much more for list-10
  and list-50, whose ratios the report marks `≈` (inconclusive).
  - The likely cause is Windows moving the Docker VM's virtual CPUs between performance
    and efficiency cores, plus turbo and power-state changes.
  - The machine also went into Modern Standby several times. Affected runs were excluded,
    but the CPU state after waking differed.
  - Results from a dedicated Linux host would be considerably tighter.
- **Database mode only.** The servers each saturated their single core, so CPU per
  request is meaningful. PostgreSQL still adds a fixed share of every request's latency.
  In-memory mode, which isolates runtime and serialization cost, still has to be run.
- **One CPU per server.** Go's advantage from multiple cores (Node.js uses one JavaScript
  thread) is not part of these results. A 2-CPU run would add it; see
  [CPU limits](#cpu-limits-quota-vs-pinning).

Reproduce or extend the session (search, memory mode and a 2-CPU pass; about 10 hours):

```bash
scripts/run-matrix.sh --session my-run --repeat 3 --warmup 10s --duration 20s --cooldown 3
```

Before starting, set the screen timeout to "Never" while plugged in. On laptops with
Modern Standby, a screen turning off suspends the run.

## Architecture

```
                 PostgreSQL  (or in-memory snapshot, DATA_SOURCE=memory)
                      │
       ┌──────────────┴───────────────┐
  Go repository                 TS repository        ← same SQL text in both
       │                              │
  Go service                    TS service
    /      \                          │
 REST     gRPC                   REST (Node)
```

The two Go servers share one repository and service package. The TypeScript server
uses the same queries and the same validation rules.

## Repository layout

```
.
├── database/
│   ├── Dockerfile                 pinned PostgreSQL 18.1, benchmark settings
│   ├── manage.sh                  init / migrate / seed / reset / verify (runs in the container)
│   ├── migrations/                the only copy of the schema (Go and TS both use it)
│   │   └── 0001_create_products.sql
│   └── seed/
│       └── seed.sql               deterministic generator (pure SQL)
├── go-server/
│   ├── Dockerfile                 targets `rest` and `grpc` (static binaries on distroless)
│   ├── go.mod                     module go-vs-ts-benchmark/go-server, pgx v5, grpc v1.84
│   ├── cmd/rest-server/           REST entry point (graceful shutdown, -healthcheck)
│   ├── cmd/grpc-server/           gRPC entry point (graceful shutdown, -healthcheck)
│   ├── gen/productv1/             generated protobuf + gRPC code (committed)
│   ├── equivalence/               black-box test of all three running servers
│   └── internal/
│       ├── config/                environment variables (shared with TS)
│       ├── app/                   builds the service for the chosen DATA_SOURCE
│       ├── product/               domain model, input type, validation rules
│       ├── repository/            Postgres + in-memory implementations, canonical SQL
│       ├── service/               validation + delegation (shared by REST and gRPC)
│       ├── rest/                  net/http handlers (standard library router)
│       ├── grpcapi/               gRPC handlers, health service, reflection
│       └── monitoring/            runtime metrics endpoint (METRICS_PORT)
├── typescript-server/             Node.js 24, Fastify 5, node-postgres 8, TypeScript 7
│   ├── Dockerfile
│   ├── src/
│   │   ├── product/               model, validation, timestamp helpers
│   │   ├── repository/            Postgres (same SQL text) + in-memory store
│   │   ├── service/               validation + delegation
│   │   ├── routes/                Fastify routes, body/param decoding
│   │   ├── app.ts                 Fastify setup, data-source wiring
│   │   ├── monitoring.ts          runtime metrics endpoint (METRICS_PORT)
│   │   └── main.ts                entry point (graceful shutdown)
│   └── test/                      node:test suites (unit, HTTP, Postgres contract)
├── benchmark/                     k6 scripts (run by scripts/bench.sh)
│   ├── get-product.js
│   ├── list-products.js
│   ├── search-products.js
│   ├── scenarios.js               warm-up + measured phase + monitor, result output
│   └── lib/                       target config, REST/gRPC client, inputs, server monitor
├── proto/
│   └── product.proto              ProductService definition
├── scripts/
│   ├── run-matrix.sh              full session: equivalence, every mode/workload/level, report
│   ├── bench.sh                   runs a workload across servers and concurrency levels
│   ├── report.sh, report.mjs      aggregates results into report.md / report.csv
│   ├── db.sh                      database lifecycle (wraps database/manage.sh)
│   ├── equivalence.sh             runs the equivalence suite against the built images
│   └── gen-proto.sh               regenerates go-server/gen/productv1 in Docker
├── docs/results/                  published benchmark sessions (report, CSV, raw runs)
├── results/                       local benchmark output (git-ignored)
├── docker-compose.yml             stack with CPU quotas (default)
├── docker-compose.pinned.yml      override: CPU pinning instead of quotas
└── .env.example
```

Two deviations from the original outline:
- There is no `go-server/migrations/`, so the schema exists in one place only.
- The gRPC handler package is `internal/grpcapi` rather than `internal/grpc`, so it
  doesn't shadow the `google.golang.org/grpc` import.

## Docker environment

| service | image | base | notes |
|---|---|---|---|
| `postgres` | `go-vs-ts-benchmark/postgres:18.1` | `postgres:18.1` | `shared_buffers=256MB`, `max_connections=200`, UTC |
| `db-init` | same as `postgres` | | one-shot: create DB, migrate, seed if empty, exit |
| `go-rest` | `go-vs-ts-benchmark/go-rest:local` (22 MB) | `distroless/static-debian13:nonroot` (digest-pinned) | static binary: `CGO_ENABLED=0 -trimpath -ldflags="-s -w"` |
| `go-grpc` | `go-vs-ts-benchmark/go-grpc:local` (29 MB) | same | same build |
| `ts-rest` | `go-vs-ts-benchmark/ts-rest:local` (352 MB) | `node:24.22.0-bookworm-slim` | `tsc` output, production dependencies only, `NODE_ENV=production`, runs as `node` |
| `k6` | `grafana/k6:2.2.0` | | `bench` profile, never started by `up`; load generator, `K6_CPUS` (default 4), 65536 file descriptors, usage reporting off |
| `go` | `golang:1.26.3` | | `tools` profile, never started by `up`; used for tests and code generation |

- **Build parity.** Both stacks run release builds; neither has a debug build to mix up.
  Go binaries are always optimized; stripping symbols does not change generated code.
  TypeScript compiles to plain JavaScript run directly by `node`, with no `ts-node`, no
  type stripping and no watch mode.
- **Startup order.** Each server waits for `db-init` to finish, so the schema and data
  exist before it starts. Memory mode loads its snapshot at startup, in 0.2–0.3 s.
- **Health checks.** Each image defines a `HEALTHCHECK`. The Go binaries probe
  themselves with `-healthcheck`, since distroless has no shell or curl; the TS image
  runs a `node -e fetch(...)` one-liner. Checks run every second until the container is
  healthy, then once a minute, so they add essentially no load during a benchmark.
  k6 never calls `/health`.
- **Shutdown.** Each server runs as PID 1 and handles SIGTERM. `docker compose stop`
  takes about a second.

### Resource limits

Every API server gets the same limits from one shared block (`x-server`) in
[docker-compose.yml](docker-compose.yml):

| variable | default | applies to |
|---|---|---|
| `SERVER_CPUS` | `2` | `go-rest`, `go-grpc`, `ts-rest` (each) |
| `SERVER_MEMORY` | `1g` | `go-rest`, `go-grpc`, `ts-rest` (each) |
| `POSTGRES_CPUS` | `4` | `postgres` |
| `POSTGRES_MEMORY` | `2g` | `postgres` |

How each runtime sees these limits (verified inside the containers):
- Go sets `GOMAXPROCS` from the CPU limit (Go 1.25+), and logs `gomaxprocs: 2`.
- Node reports `availableParallelism() = 2` and sizes its heap from the memory limit
  (560 MB heap for a 1 GB container). JavaScript still runs on one thread.

Run benchmarks at more than one CPU limit, for example `SERVER_CPUS=1` and `SERVER_CPUS=4`.
That separates per-core efficiency from multi-core scaling, which matters because Go
uses every core it is given and Node.js mostly does not. Apply a new limit with:

```bash
SERVER_CPUS=1 docker compose up -d --wait
```

Load one server at a time; the idle ones use almost no CPU. On the development machine
the Docker Desktop VM has 12 CPUs and 8 GB of RAM.

### CPU limits: quota vs pinning

Docker can limit CPU in two ways, and for this comparison they give very different results.

| | quota (`cpus: N`, the default compose file) | pinning (`cpuset`, [docker-compose.pinned.yml](docker-compose.pinned.yml)) |
|---|---|---|
| mechanism | CFS bandwidth control: N × 100 ms of CPU time per 100 ms period, summed over all threads; when it is used up, the whole container is paused until the next period | the container may only run on the listed CPUs, with no time quota and no pauses |
| Go | `GOMAXPROCS` = N, but GC and runtime threads run beside it, so a saturated Go server exceeds the quota every period and is paused | `GOMAXPROCS` = number of pinned CPUs; never paused |
| Node.js | one JavaScript thread can use at most one CPU, so at 1 CPU it rarely exceeds the quota | same as quota |
| models | Kubernetes / ECS CPU limits | dedicated cores |

Measured in memory mode, search, 50 VUs, one CPU per server. Throttled means the
container hit its quota and was paused:

| | quota: req/s | quota: p99 | quota: periods throttled | pinned: req/s | pinned: p99 |
|---|---:|---:|---:|---:|---:|
| Go REST | 2,884 | 77 ms | 100% (1.24 s paused per s, summed over threads) | 5,176 | 24 ms |
| TS REST | 5,584 | 19 ms | 43% (24 ms per s) | 5,737 | 23 ms |
| Go gRPC | 3,800 | 63 ms | 99% | 7,287 | 18 ms |

Under a 1-CPU quota, the throttling roughly halves Go's throughput and quadruples its
p99. Node.js is barely affected, because its single JavaScript thread naturally stays
within one CPU. A quota-limited comparison would show "Node.js twice as fast as Go per
core", and that comes from the limit mechanism, not from the runtimes.

So `scripts/run-matrix.sh` pins CPUs by default:
- all three servers share the same CPU set, since only one is loaded at a time;
- servers run on CPU 2, PostgreSQL on 3–5, and k6 on 6–11;
- in memory mode PostgreSQL is idle, so k6 gets 3–11;
- CPUs 0–1 stay free for the Docker daemon and the sampler.

To start the stack pinned by hand, outside `run-matrix.sh`, for single `bench.sh` runs:

```bash
export COMPOSE_PATH_SEPARATOR=: COMPOSE_FILE=docker-compose.yml:docker-compose.pinned.yml
```
```bash
docker compose up -d --wait --force-recreate
```

`SERVER_CPUSET`, `POSTGRES_CPUSET` and `K6_CPUSET` override the CPU sets. Unset
`COMPOSE_FILE` and recreate to return to quota mode.

Quota mode stays available with `--cpu-mode quota`, because it is what containers get
under Kubernetes CPU limits. That is a valid experiment in its own right, but it must be
labelled as such. Every result records the mode (`cpu_mode`), and the server's runtime
endpoint reports cgroup throttling (`server.cpu_throttling`).

One caveat for pinning on Docker Desktop and WSL2: CPU numbers are virtual CPUs of the VM,
and Windows decides which physical core runs each one. On a hybrid CPU (this machine's
i5-12450H has performance and efficiency cores), the same virtual CPU can land on either
kind of core. All servers use the same CPU set, so they face the same lottery, and
repeats show the resulting spread. On a Linux host, pinning maps to real cores.

### Network topology

```
k6 container ──(compose bridge network, plaintext, no TLS)──> go-rest | ts-rest | go-grpc ──> postgres
```

k6 runs in a container on the compose network and calls `go-rest:8080`, `ts-rest:8080`
and `go-grpc:9090` directly. The published host ports are only for manual use. Running
k6 on the host would route traffic through Docker Desktop's port-forwarding proxy, which
adds latency and caps throughput, and so compresses the differences between servers.
All containers share the Docker VM's CPUs; the CPU limits keep each one within its share.

## Benchmarks (k6)

### Full session (recommended)

One command runs everything and writes a report:

```bash
scripts/run-matrix.sh --dry-run          # plan and estimated duration only
```
```bash
scripts/run-matrix.sh --session my-run   # results/my-run/report.md and report.csv
```

`run-matrix.sh` does the following:

1. Builds and starts the stack.
2. Runs the equivalence suite and stops if it fails.
3. Runs one pass per data mode (and per CPU limit, if you give several). Each pass
   resets the database and recreates the three servers fresh, then runs every workload
   (list once per page size) with `bench.sh`.
4. Restores the compose defaults and calls `scripts/report.sh`.

| option | default | |
|---|---|---|
| `--modes` | `"postgres memory"` | data sources, one pass each |
| `--cpu-mode` | `pinned` | `pinned` (CPU sets, no throttling) or `quota` (CFS quota); see [CPU limits](#cpu-limits-quota-vs-pinning) |
| `--server-cpusets` | `"2"` | pinned: server CPU set per pass; e.g. `"2 2-3"` for 1 and 2 CPUs |
| `--postgres-cpuset` / `--k6-cpuset` | `3-5` / `6-11` | pinned: CPU sets for PostgreSQL and k6 |
| `--k6-cpuset-memory` | `3-11` | pinned: k6 CPU set in memory mode, where PostgreSQL is idle |
| `--cpus` / `--k6-cpus` | `"1"` / `6` | quota: server CPU limit per pass, k6 limit |
| `--workloads` | all three | |
| `--list-limits` | `"10 50 100 500"` | |
| `--vus` | `"1 10 50 100 250 500 1000"` | |
| `--repeat` | `3` | runs per point; the report shows median and spread |
| `--warmup` / `--duration` / `--cooldown` | `15s` / `30s` / `5` | |

The defaults run 756 runs and take about 13.5 hours. With `--warmup 10s --duration 20s
--cooldown 3` they take about 10 hours, or about 6.7 hours with `--repeat 2` as well.
Keep the machine otherwise idle and plugged in while it runs: other load on the host
shows up directly as noise.

**Why one pinned CPU per server, and 6–9 CPUs for k6.** k6 spends more CPU per request
than the servers do:
- In a pilot with 2 CPUs per server and 4 for k6, k6 was the bottleneck in memory mode
  for all three servers. Go REST at 27,000 req/s used 1.7 of its 2 cores while k6 was
  flat out; k6 spent about 2.3× more CPU per request than the Go server.
- Pinned to one core, Go REST serves 25,000–45,000 req/s in memory mode. That kept even
  4 k6 CPUs busy, so the default gives k6 6 CPUs in database mode and 9 in memory mode.

With these settings, every server saturates its core first (0.98–0.99 cores used) while
k6 stays under 70% of its limit. Every result records k6's CPU and flags any run where it
was CPU-bound. `run-matrix.sh` refuses CPU sets that do not exist in the Docker VM.

**Noise on this machine.** Repeated identical runs differed by up to ±30% (for example
24,000–45,000 req/s for pinned Go REST get-product in memory mode). The likely cause is
Windows moving the Docker VM's virtual CPUs between the hybrid CPU's performance and
efficiency cores, plus turbo behaviour. Repeats are interleaved over the whole session,
and the report shows each point's spread. Treat ratios marked `≈` as inconclusive.

### Report

```bash
scripts/report.sh results/my-run
```

[scripts/report.mjs](scripts/report.mjs) reads every `results.jsonl` and
`environment.json` below the given directories. It groups runs by data mode, CPU limit,
workload, parameters, server and VUs, and writes `report.md` and `report.csv`. For each
section it gives:

- **Throughput.** Median req/s with ± half the min–max range across repeats.
- **Latency.** Median p50 / p95 / p99.
- **Server resources.** CPU cores, CPU ms per request, peak RSS.
- **Runtime detail.** Go: GC share of CPU, allocation per request, goroutines. Node:
  event-loop utilization, loop delay, GCs per 1000 requests.
- **Wire bytes per request.**
- **Experiment ratios.** A = Go REST ÷ TS REST, B = Go gRPC ÷ Go REST, C = Go gRPC ÷ TS
  REST. A ratio is prefixed `≈` when the two servers' min–max ranges overlap, meaning the
  difference is within run-to-run noise.
- **Flags.** `⚠k6` marks a point where k6 was CPU-bound in any repeat; `⚠err` and
  `⚠t/o` mark errors and timeouts.

The environment table is filled in automatically: host CPU and OS, Docker VM, all
runtime versions, limits, dataset size and checksum, timings, network and TLS.

### Interpreting results

- **Keep the two variables apart.**
  - Experiment A (Go REST vs TS REST) changes only the runtime and server stack, with the
    same protocol and encoding.
  - Experiment B (Go REST vs Go gRPC) changes only the protocol and encoding, with the
    same runtime and the same service code.
  - Experiment C mixes both. It describes the complete stacks, but it cannot attribute a
    difference to either variable.
- **Results hold for this setup only.** They depend on payload shape, concurrency, CPU
  limits, data source and hardware. A ratio measured at one point does not carry over to
  other workloads or machines, and nothing here shows that one technology is
  universally faster.
- **Compare like with like.** Database-mode and memory-mode results answer different
  questions: a realistic backend, or protocol and runtime overhead with PostgreSQL removed.
  Never mix them in one comparison.
- **Read throughput together with CPU.**
  - A server below its CPU limit at high VUs is waiting on something else: PostgreSQL,
    the pool of 20 connections, or k6.
  - For Node, an event-loop utilization near 1.0 means the single JS thread is the limit,
    even when process CPU stays below the container limit.
  - CPU ms per request compares efficiency independently of how many cores were available.
- **Discard flagged points.** A point marked `⚠k6` measures the load generator. Points
  with `≈` ratios are within noise.
- **Consider the workload's realism.** Very high VUs on large payloads (for example 1000
  VUs × 500-product lists) measure queueing more than request handling.

### Running single workloads

The servers must be up (`docker compose up -d --wait`) in the data mode you want to
measure. Then run one of these:

```bash
scripts/bench.sh get-product
```
```bash
scripts/bench.sh list-products --limit 500
```
```bash
scripts/bench.sh search-products --vus "10 100 1000"
```

| option | default | |
|---|---|---|
| `--targets` | `"go-rest ts-rest go-grpc"` | servers to run; pick two for a single experiment, e.g. `"go-rest go-grpc"` for Experiment B |
| `--vus` | `"1 10 50 100 250 500 1000"` | concurrency levels; one run per level and target |
| `--repeat` | `1` | runs per point, spread over the session (repeat → VUs → target) |
| `--warmup` / `--duration` | `15s` / `30s` | warm-up (not reported) and measured phase, per run |
| `--dir` | `results/<timestamp>-<workload>` | output directory |
| `--cooldown` | `5` | idle seconds between runs |
| `--limit` | list `50`, search server default | page size / result limit |
| `--term` | rotate all five | search: pin one term |
| `--label` | | added to the results directory name |

Before starting, `bench.sh` checks that every target is healthy and that all of them run
with the same `DATA_SOURCE`, CPU limit and memory limit. It refuses to compare servers
that differ. Runs are interleaved: at each concurrency level every target runs once,
so slow drift on the machine affects all of them alike.

Each invocation writes `results/<UTC timestamp>-<workload>[-<label>]/` (or `--dir`):

| file | contents |
|---|---|
| `environment.json` | host CPU and OS, Docker VM, Go/Node/TypeScript/PostgreSQL/k6/Docker versions, all limits, pool size, dataset size and checksum (warns if it is not the seeded dataset), network, TLS |
| `<target>-vus<N>-r<R>.json` | one result per run (see metrics below) |
| `results.jsonl` | the same results, one line per run |

A summary row per run is also printed to the terminal.

Scripts can also be run directly. `TARGET` selects a compose service, and `BASE_URL` or
`GRPC_ADDR` point at any server; nothing is hard-coded to one implementation:

```bash
docker compose run --rm k6 run -e TARGET=go-grpc -e VUS=50 /scripts/get-product.js
```
```bash
k6 run -e BASE_URL=http://localhost:8081 -e VUS=10 benchmark/get-product.js
```

A local k6 also works for REST, but it goes through the port proxy. For gRPC, run it
from the repository root so `/proto` resolves, or adjust the path.

### Workloads

| script | REST | gRPC | inputs |
|---|---|---|---|
| `get-product.js` | `GET /products/{id}` | `GetProduct` | 1000 ids from a 32-bit LCG (seed 42) spread over the table |
| `list-products.js` | `GET /products?limit=N&offset=M` | `ListProducts` | `LIMIT` per run (benchmark 10, 50, 100, 500); offsets `page × LIMIT` for pages 0–19 |
| `search-products.js` | `GET /products/search?q=…` | `SearchProducts` | keyboard, phone, laptop, monitor, mouse in rotation (or `--term`); server default limit 50 |

- **Deterministic inputs.** Each VU walks the input list from its own starting position,
  so a run with the same VUs sends the same requests every time. The equivalence suite
  checks exactly these inputs, all 1000 ids and all 80 list pages, on all three servers.
  A test pins the id sequence against the JavaScript generator.
- **Shallow list offsets.** Offsets stay within the first 20 pages on purpose: deep
  offsets mostly measure PostgreSQL skipping rows, not the API.
- **Per-term search results.** Search also reports latency and throughput per term,
  because payload sizes differ by term.

### Methodology

- **Closed model.** Constant VUs with no think time, so `VUS` = requests in flight.
  Throughput is what the server sustains at that concurrency.
- **Phases.** A warm-up phase (connections, JIT, caches) runs at the same VUs, then
  a 2 s gap, then the measured phase. Only the measured phase is reported, using k6
  sub-metrics filtered on `scenario:measure`.
- **Connections.** One connection per VU for both protocols: an HTTP/1.1 keep-alive
  connection, or one HTTP/2 connection per VU for gRPC. Neither multiplexes VUs over a
  shared connection, so the protocols are compared connection for connection.
- **Client-side cost.** k6 discards REST response bodies (`discardResponseBodies`) and gRPC
  response messages (`discardResponseMessage`) and checks only the status. Response
  correctness is established by the equivalence suite, not during load.
  - Without `discardResponseMessage`, k6 converts every gRPC response to JavaScript
    objects. For `list limit=500` that maxed out k6 at 4 CPUs while the gRPC server
    used only half of its 2 CPUs, which made gRPC look 2.5× slower than it is (361
    vs 887 req/s).
  - Even with it, k6 spends about 3× more CPU per gRPC request than per REST request.
    Every result records k6's CPU and flags runs where k6 was CPU-bound
    (see [Container sampling and client saturation](#container-sampling-and-client-saturation)).
- **Concurrency levels are not all meaningful for every workload.** For example, 1000
  VUs each fetching 500-product lists is mostly a queueing test. In database mode,
  concurrency beyond the pool size (20) queues in the server. Report each level
  separately; never average across levels.
- **Isolation.** Load one server at a time. Health checks drop to once a minute after
  startup, and request logging is off.

### Metrics recorded per run

| requirement | field in result JSON | source |
|---|---|---|
| requests/sec | `requests_per_second` | measured requests ÷ measured duration |
| average / median / p90 / p95 / p99 latency | `latency_ms.avg/med/p90/p95/p99` (+ `min`, `max`) | `http_req_duration` or `grpc_req_duration` |
| error rate | `error_rate`, `errors` | non-200 / non-OK statuses, transport errors and timeouts |
| timeouts | `timeouts` | requests exceeding `TIMEOUT` (default 10 s) |
| request / response size on the wire | `bytes.sent_per_request`, `bytes.received_per_request` | k6 `data_sent` / `data_received` ÷ requests |
| total bytes transferred | `bytes.sent_total`, `bytes.received_total` | measured phase only |

Wire bytes include protocol overhead: HTTP/1.1 headers, or HTTP/2 frames and HPACK
headers. Message-body sizes are listed under [Payload sizes](#payload-sizes-json-vs-protobuf).

One header difference is visible here. Node's HTTP server sends `Connection: keep-alive`
and `Keep-Alive: timeout=120` on every response, while Go's HTTP/1.1 server does not. So
ts-rest responses are about 49 bytes larger on the wire than go-rest's, with identical
bodies.

The terminal row printed per run shows: requests/s; latency avg, p50, p90, p95 and p99;
errors; timeouts (`t/o`); received bytes per request; server CPU in cores (`srv_cpu`);
server peak RSS in MB (`srv_MB`); and k6 CPU % (`k6_cpu%`, where 100 = one core).

## Server resource monitoring

Resource usage comes from two sources, both collected automatically by `scripts/bench.sh`:

| source | what | used for |
|---|---|---|
| each server's own runtime endpoint (`:9100/runtime`), scraped by k6 | process CPU time, RSS, heap, GC, goroutines (Go), event loop (Node) | the server figures in every result (`result.server`) |
| `docker stats` snapshots, taken by `bench.sh` | container CPU % and memory of the server, k6 and PostgreSQL | k6 saturation check, PostgreSQL load, cross-check (`result.containers`) |

### Runtime endpoint

Each server answers `GET /runtime` on its own listener, `METRICS_PORT` (default 9100,
and `0` disables it). It is separate from the benchmarked port, router and middleware.
On the host it is published as :9101 (go-rest), :9102 (ts-rest) and :9103 (go-grpc):

```bash
curl localhost:9102/runtime
```

Nothing is measured per request. Values are read only when the endpoint is scraped;
counters are cumulative since process start, and the scraper computes deltas.

| field | Go ([monitoring.go](go-server/internal/monitoring/monitoring.go)) | Node.js ([monitoring.ts](typescript-server/src/monitoring.ts)) |
|---|---|---|
| `cpu_seconds` | `getrusage(RUSAGE_SELF)` user + system | `process.cpuUsage()` user + system |
| `rss_bytes` | `/proc/self/statm` | `process.memoryUsage().rss` |
| `heap_used_bytes` / `heap_total_bytes` | heap objects / objects + unused + free (`runtime/metrics`) | V8 `heapUsed` / `heapTotal` |
| `gc_count` | completed GC cycles | GC events (`PerformanceObserver`, type `gc`) |
| `gc_pause_seconds` | stop-the-world GC pauses (from the pause histogram, approximate) | main-thread time in GC |
| Go only | `goroutines`, `gc_cpu_seconds`, `heap_live_bytes`, `heap_goal_bytes`, `allocated_bytes`, `gomaxprocs` | |
| Node only | | `event_loop_active_ms`/`idle_ms` (event-loop utilization), `event_loop_delay_ms` p50/p99/max since the last scrape, `external_bytes`, `active_resources` |

Go's `runtime/metrics` does not stop the world. Node's only continuous costs are the GC
observer and an event-loop delay histogram that samples every 20 ms. Node 24 records the
full interval between samples, so the endpoint subtracts the 20 ms and reports only the
delay beyond it; an idle server reads about 0.3 ms.

### How results get server metrics

When the target has a runtime endpoint, k6 runs a third scenario, `monitor`: one VU that
starts with the measured phase, lasts as long, and scrapes the endpoint once per second.
Its requests are tagged `scenario=monitor`, so they never mix with the measured request
metrics. The deltas between scrapes become the `server` block of each result:

| field | meaning |
|---|---|
| `server.cpu_cores` | process CPU seconds ÷ wall seconds over the measured window (2.0 = two full cores) |
| `server.cpu_percent_of_limit` | `cpu_cores` as a percentage of `SERVER_CPUS` |
| `server.cpu_ms_per_request` | CPU cost per request: `cpu_cores × 1000 ÷ requests/s` |
| `server.rss_mb`, `server.heap_used_mb` | average and maximum of the 1 s samples |
| `server.gc` | GC cycles, total pause ms, cycles per 1000 requests; Go adds `cpu_fraction` (share of CPU spent in GC) |
| `server.go` | goroutines (avg/max), heap bytes allocated per request |
| `server.node` | event-loop utilization (1.0 = the JS thread never idles), event-loop delay p99 (avg/max) and max |

`MONITOR=off` disables the scenario. `MONITOR_URL` points it at another endpoint (for
`BASE_URL`/`GRPC_ADDR` targets).

### Container sampling and client saturation

During each run, `bench.sh` takes `docker stats --no-stream` snapshots in a loop, about
one every 2 s. It keeps only those inside the measured window, which k6 reports as
`measure_window_ms`, and writes `result.containers`:

- **`server`, `k6`, `postgres`:** average and maximum CPU % (100 = one core), peak
  memory, and the number of samples.
- **`k6_cpu_bound`:** `true` when k6 averaged over 75% of its CPU limit (`K6_CPUS`).
  `bench.sh` prints a warning for such runs. Treat their numbers as a client limit, not
  a server result, and rerun with a higher `K6_CPUS`.

`docker stats` is coarse: about 5 samples per 12 s window. It also read about 15% below
the process's own CPU accounting in testing (1.63 vs 1.94 cores), so the server's
self-reported `cpu_cores` is the authoritative figure, and the saturation threshold is
deliberately low.

The sampler is a loop of short-lived `docker stats --no-stream` calls rather than one
streaming `docker stats`. On Windows, a backgrounded streaming `docker.exe` survives
`kill` from Git Bash and kept running after the benchmark.

### Overhead

- **Go.** Scraping on vs off, two rounds of `get-product` at 50 VUs: 18,766 / 18,677 vs
  18,541 / 17,800 req/s. That is no measurable difference.
- **Node.js.** Disabling the endpoint, GC observer and delay histogram entirely
  (`METRICS_PORT=0`) also showed no consistent difference. Three alternating rounds gave
  3,861 / 7,948 / 6,522 req/s off vs 7,951 / 6,108 / 6,105 on.

The Node numbers show a methodology issue in their own right: TS throughput varies by up
to ±30% between short runs, and the first run after a restart is the slowest. Phase 10
runs use longer warm-ups and repeated runs, and report the spread.

### What the first monitored runs show

One `list-products limit=500` run at 50 VUs per server, in database mode with 2 CPUs per
server:

| | Go REST | TS REST | Go gRPC |
|---|---|---|---|
| server CPU | 1.94 cores | 1.30 cores | 1.93 cores |
| peak RSS | 24 MB | 183 MB | 42 MB |
| GC | 14% of CPU, 0.61 MB allocated per request | 293 GCs, 930 ms main-thread GC time | 12% of CPU, 0.55 MB allocated per request |
| runtime-specific | 124 goroutines | event-loop utilization 1.00, delay p99 34 ms | 226 goroutines |

The Go servers use their whole 2-CPU allowance. The Node server's JavaScript thread is
fully busy (utilization 1.0), while the process as a whole uses only 1.3 of its 2 cores.
This is the single-thread limit described under [Resource limits](#resource-limits).
These are single short runs that show what the monitoring captures; they are not
benchmark results.

## Configuration

All servers read the same environment variables. Compose passes the same values to every
server.

| variable | default | meaning |
|---|---|---|
| `DATA_SOURCE` | `postgres` | `postgres`, or `memory` (table loaded once at startup) |
| `DATABASE_URL` | `postgres://bench:bench@localhost:5432/products` | also used by memory mode to load the snapshot (compose points it at the `postgres` service) |
| `DB_MAX_CONNS` | `20` | connection pool size; keep it identical across servers |
| `HTTP_PORT` | `8080` | REST port inside the container |
| `GRPC_PORT` | `9090` | gRPC port inside the container |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_REQUESTS` | `false` | per-request logging; leave off for benchmarks |
| `METRICS_PORT` | `9100` | runtime metrics listener (`GET /runtime`); `0` disables it |

Compose-only variables:
- the host ports `GO_REST_PORT`, `TS_REST_PORT` and `GO_GRPC_PORT`;
- the host metrics ports `*_METRICS_PORT` (9101–9103);
- the database credentials and `PRODUCT_COUNT`;
- the resource limits above, plus `K6_CPUS`.
Copy [.env.example](.env.example) to `.env` to change any of them.

## Database

```bash
scripts/db.sh setup     # start postgres, create DB, migrate, seed if empty
scripts/db.sh verify    # row count + dataset checksum + search hit counts
scripts/db.sh reset     # drop everything, migrate, reseed (run after write benchmarks)
scripts/db.sh migrate   # apply pending migrations
scripts/db.sh testdb    # products_test database with 1000 products, for tests
scripts/db.sh psql      # interactive shell
```

`db.sh` runs [database/manage.sh](database/manage.sh) inside the `db-init` container,
which is the same code the compose startup uses.
- `DB_NAME=other scripts/db.sh reset` targets another database. The script never changes
  `POSTGRES_DB`, so compose never recreates the running `postgres` container.
- `PRODUCT_COUNT=1000000 scripts/db.sh seed` seeds a different size.

Expected output of `verify` with the default dataset:

| metric | value |
|---|---|
| products | 100,000 (ids 1–100000) |
| checksum | `d26bc7a601e76ff91ba94bc08ebebc68` |
| `keyboard` / `laptop` / `monitor` / `mouse` matches | 5,000 each |
| `phone` matches | 15,000 (Phone, Headphones, Microphone) |
| description length | 45 / 126 / 214 bytes (min / avg / max) |
| table + indexes | ~39 MB |

If your checksum is different, your dataset is different and your results can't be
compared with other runs.

### Schema

```sql
products (
  id          BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
  name        TEXT           NOT NULL,  -- 1..200 chars
  description TEXT           NOT NULL,  -- ≤ 2000 chars
  category    TEXT           NOT NULL,  -- 1..100 chars
  price       NUMERIC(10,2)  NOT NULL,  -- ≥ 0
  stock       INTEGER        NOT NULL,  -- ≥ 0
  rating      NUMERIC(2,1)   NOT NULL,  -- 0.0..5.0
  created_at  TIMESTAMPTZ    NOT NULL,
  updated_at  TIMESTAMPTZ    NOT NULL
)
```

Indexes:

- `products_pkey`: get by id; `ORDER BY id` for list and search.
- `products_name_trgm_idx` (GIN, `pg_trgm`): case-insensitive substring search on `name`.

No other indexes. Unused indexes would slow down writes in all three servers and
show nothing about the comparison.

### Migration strategy

- Plain, forward-only SQL files in `database/migrations/`, named `NNNN_description.sql`
  and applied in lexical order.
- Each applied file is recorded in `schema_migrations`. A file and its bookkeeping row
  commit in one transaction, and running `migrate` again does nothing.
- There are no down migrations. `reset` drops the `public` schema and rebuilds it. That
  is the only rollback a benchmark needs, and it guarantees a clean physical state.
- The servers never run migrations themselves. The schema is in place before any server
  starts, so startup time and behaviour are the same for every implementation.

### Deterministic seed strategy

- The seed is pure SQL (`generate_series`). Every column is a function of the row number
  `i`, with no `random()`, no `now()`, and no dependence on locale or time zone.
- Ids are assigned explicitly (1..N), and afterwards the identity sequence is advanced so
  `POST /products` continues from N+1.
- Names look like `Nimbus Ergonomic Keyboard KE-000042`. They are unique, and 20 product
  types are spread evenly, so benchmark search terms hit a known number of rows.
- Descriptions have 1 to 4 sentences, so response payload sizes vary by row the way real
  data does.
- After seeding, `VACUUM (ANALYZE)` runs so every run starts with the same planner
  statistics and visibility map.
- Neither Go nor TypeScript contains its own seed generator, so the dataset cannot drift
  between languages.

## REST API contract

| request | success | body |
|---|---|---|
| `GET /products/{id}` | 200 | product |
| `GET /products?limit=20&offset=0` | 200 | `{"products":[...]}` |
| `GET /products/search?q=…&limit=50` | 200 | `{"products":[...]}` |
| `POST /products` | 201 | product |
| `PUT /products/{id}` | 200 | product |
| `DELETE /products/{id}` | 204 | none |
| `GET /health` | 200 / 503 | `{"status":"ok"}` / `{"status":"unavailable"}` |

Product JSON, byte for byte (product 1 of the seed):

```json
{"id":1,"name":"Orion Smart Phone PH-000001","description":"The Orion smart phone is designed for home offices. Firmware updates are delivered automatically.","category":"Mobile","price":80.18,"stock":37,"rating":2.3,"created_at":"2024-01-01T00:05:00Z","updated_at":"2024-01-01T07:05:00Z"}
```

Errors:

| status | body |
|---|---|
| 400 | `{"error":"validation failed","fields":[{"field":…,"message":…}]}` |
| 400 | `{"error":"invalid request body"}` |
| 404 | `{"error":"product not found"}` |
| 404 | `{"error":"not found"}` (unknown route, or unsupported method on a known one) |
| 413 | `{"error":"request body too large"}` (over 1 MiB) |
| 500 | `{"error":"internal server error"}` |

Rules both REST servers follow:

- **Encoding.** Compact JSON with no trailing newline and no HTML escaping, matching
  `JSON.stringify`. `Content-Type: application/json; charset=utf-8` and an explicit
  `Content-Length`, so responses are never chunked.
- **Timestamps.** RFC 3339 in UTC, with trailing zero fraction digits omitted and
  microsecond precision when present (Go's `RFC3339Nano`), e.g.
  `2026-11-05T06:25:14.165483Z`.
- **Numbers.** Shortest round-trip form (`80.18`, `4`, `10`), which is the same in Go and JS.
- **Path id.** Anything other than an integer from 1 to 2^53−1 gets the `id` validation
  error. That includes `abc` and `1.5`.
- **Query parameters.** An absent parameter takes its default. When a parameter is
  repeated, the first value wins.
  - If any parameter is not an integer (including empty), only the `must be an integer`
    errors are returned.
  - Otherwise the range checks run. Integers too large to parse count as out of range.
- **Body.**
  - The body must be a single JSON object. `null` counts as `{}`.
  - Unknown fields, wrong types, non-integer `stock` and trailing data all return
    `invalid request body`.
  - An absent field takes its zero value, as in proto3, and then validation runs.
  - An invalid path id is reported before the body is read.

Known edge differences that the benchmark never exercises:
- Go matches JSON keys case-insensitively, while TS does not.
- Go escapes U+2028/U+2029 in strings, while `JSON.stringify` does not.
- For a body over 1 MiB, both servers answer 413 and close the connection. Fastify closes
  it without draining the upload, so a client that is still sending can see a broken
  pipe instead of the 413. Go reads a small remainder first.

## gRPC API

[proto/product.proto](proto/product.proto) defines `product.v1.ProductService`. Its fields
mirror the REST JSON one to one.

| REST | gRPC |
|---|---|
| `GET /products/{id}` | `GetProduct` → `Product` |
| `GET /products` | `ListProducts` → `ProductList` |
| `GET /products/search` | `SearchProducts` → `ProductList` |
| `POST /products` | `CreateProduct` → `Product` |
| `PUT /products/{id}` | `UpdateProduct` → `Product` |
| `DELETE /products/{id}` | `DeleteProduct` → `DeleteProductResponse` |
| `GET /health` | `grpc.health.v1.Health/Check` |

- **Defaults.** `limit` and `offset` are proto3 `optional`. Unset means the default
  applies; an explicit 0 is a validation error, exactly like REST.
- **Errors.**
  - Validation: `INVALID_ARGUMENT`, message `validation failed`, with a
    `google.rpc.BadRequest` detail holding the same field/message pairs as REST. The
    search field is reported as `query`, not `q`.
  - Unknown id: `NOT_FOUND` with message `product not found`.
  - Anything else: `INTERNAL` with message `internal server error`.
- **Types.**
  - Timestamps are `google.protobuf.Timestamp`.
  - Ids are `int64`, with the same 1 to 2^53−1 range as REST.
  - Prices and ratings are `double`.
- **Health.** The health service runs the same data-source check as REST
  `/health`. It accepts `""` or `product.v1.ProductService`.
- **Reflection.** Server reflection is on, so `grpcurl` works without the `.proto` file.
- **Transport.** Plaintext HTTP/2 with no TLS, the same as REST, which also has no TLS.
  grpc-go server options are left at their defaults.

The generated code is committed. Regenerate it only after changing the `.proto`:

```bash
scripts/gen-proto.sh
```

It pins protoc-gen-go v1.36.11 and protoc-gen-go-grpc v1.6.1; protoc 3.21.12 comes from
Debian trixie.

### Payload sizes: JSON vs Protobuf

Message body bytes, measured against the seeded database. They are the same in both
data-source modes.

| request | JSON | Protobuf | Protobuf / JSON |
|---|---:|---:|---:|
| get 12345 | 296 | 177 | 60% |
| list limit 10 | 3,236 | 2,086 | 64% |
| list limit 50 | 16,331 | 10,618 | 65% |
| list limit 100 | 32,715 | 21,296 | 65% |
| list limit 500 | 163,980 | 106,922 | 65% |
| search keyboard | 13,193 | 7,447 | 56% |
| search phone | 14,389 | 8,640 | 60% |
| search laptop | 17,593 | 11,895 | 68% |
| search monitor | 20,040 | 14,340 | 72% |
| search mouse | 12,896 | 7,152 | 55% |

These are serialized message sizes only. On the wire, gRPC adds a 5-byte length prefix
plus HTTP/2 frame and HPACK header bytes, and REST adds HTTP/1.1 headers. Neither side
uses compression. The saving comes mostly from field names and numbers; free text
(names and descriptions) is the same size in both formats, which is why search for
`monitor` (longer descriptions) gains least.

Search payloads differ by term because description length follows the row number,
and so does the product type. Report search results per term rather than as one
blended number. k6 measures total bytes on the wire in Phase 8.

## TypeScript server: how it matches Go

| concern | how |
|---|---|
| SQL | Same statement text and parameter order (`src/repository/postgres.ts`) |
| Prepared statements | Named queries are prepared once per connection, like pgx's statement cache |
| `BIGINT` / `NUMERIC` | Type parsers return JS numbers (node-postgres default is strings) |
| `TIMESTAMPTZ` | PostgreSQL text kept and reformatted to `…T…Z` with microseconds; it never passes through `Date`, which only has milliseconds |
| JSON output | `JSON.stringify`, the stack's default, with no Fastify response schema |
| Request bodies | Any `Content-Type`, read raw and decoded with the Go rules (`decodeInput`) |
| `/products/` | Fastify would match an empty `:id`; it is answered as an unknown route, like Go |
| Logging | Fastify logger off; one optional `onResponse` hook when `LOG_REQUESTS=true` |
| Keep-alive | 120 s idle timeout, as in Go |

Differences that come from the technology and are kept on purpose:

- **CPU parallelism.** A Node.js process runs JavaScript on one thread, while the Go
  servers use every core they are given. The TS server runs as a single process: a
  cluster would give each worker its own copy of the in-memory store, and writes would
  diverge. See [Resource limits](#resource-limits) for how this is handled.
- **Database wire format.** node-postgres reads results in the PostgreSQL text format;
  pgx uses the binary format.
- **Serialization.** `JSON.stringify` versus Go's reflection-based `encoding/json`. Both
  are the default, idiomatic choice for their stack. Neither server uses a code-generated
  or schema-compiled serializer.

## Shared rules (Go and TypeScript)

The Go sources are the reference; the TypeScript server matches them exactly.

**SQL** ([postgres.go](go-server/internal/repository/postgres.go)): the `SQL*` constants
are the canonical statements. TS uses the same text and parameter order. Search escapes
`\`, `%` and `_` in the query, so it is always a literal substring match.

**Validation** ([validation.go](go-server/internal/product/validation.go)):

| field / param | rule |
|---|---|
| `id` | 1 to 2^53−1 |
| `name` | 1–200 characters (Unicode code points) |
| `description` | 0–2000 characters |
| `category` | 1–100 characters |
| `price` | 0–99999999.99, at most 2 decimal places |
| `stock` | ≥ 0 (32-bit integer) |
| `rating` | 0–5, at most 1 decimal place |
| `limit` | 1–1000; default 20 for list, 50 for search |
| `offset` | 0–1,000,000,000; default 0 |
| `q` | 1–100 characters |

- A value with too many decimal places is rejected, not rounded. PostgreSQL would round
  it silently and the in-memory store would not, so rounding would make the two data
  sources diverge.
- Errors list every invalid field in the fixed order above, with identical messages.
- An out-of-range `limit` is an error, not clamped.
- Update replaces all writable fields (PUT semantics).

**In-memory store** ([memory.go](go-server/internal/repository/memory.go)):
- Rows are an array sorted by id, and Get uses binary search.
- Search scans in id order against lowercased names computed once, and stops at `limit`.
- New ids continue from the highest id and are never reused.
- Timestamps are UTC, truncated to microseconds.
- One reader/writer lock. Node.js needs no lock because its JS execution is single-threaded.

List and search always return an array, never null.

## Tests

### Go

The `go` service in compose runs the tests, so you do not need a local Go install:

```bash
scripts/db.sh testdb
```
```bash
docker compose run --rm go go test -race ./...
```

With a local Go toolchain, run them from `go-server/` with
`TEST_DATABASE_URL=postgres://bench:bench@localhost:5432/products_test`.

Integration tests are skipped when `TEST_DATABASE_URL` is unset. They never touch the
benchmark database. The repository contract suite runs the same assertions against
PostgreSQL and against an in-memory store loaded from it, and a second test compares
their Get, List and Search results field by field. That is what keeps
`DATA_SOURCE=postgres` and `DATA_SOURCE=memory` equivalent.

### TypeScript

Tests use Node's built-in runner and run the `.ts` files directly through Node's type
stripping:

```bash
cd typescript-server && npm ci
```
```bash
npm run typecheck
```
```bash
TEST_DATABASE_URL=postgres://bench:bench@localhost:5432/products_test npm test
```

### Equivalence (all three servers)

Run this before every benchmark session. It checks that all three servers do the same
work:

```bash
scripts/equivalence.sh            # both data modes, about 40 s
```
```bash
scripts/equivalence.sh memory     # one mode
```

For each data mode, the script does the following:

1. Resets a separate `products_equiv` database (100k products).
2. Starts the **same images that are benchmarked** (`go-rest`, `go-grpc`, `ts-rest`) as
   separate `equiv-*` containers. The benchmark database and the running benchmark
   servers are never touched.
3. Runs [go-server/equivalence](go-server/equivalence/equivalence_test.go), then removes
   the containers. It exits non-zero on any difference.

| check | what must hold | requests |
|---|---|---|
| REST reads | Go and TS byte-identical: status, `Content-Type`, `Content-Length`, body; never chunked | 1204 |
| gRPC reads | each gRPC response or error, re-encoded with the REST rules, equals the Go REST body | 1188 |
| REST write errors | Go and TS byte-identical for malformed, invalid and oversized bodies and bad ids | ~40 |
| gRPC write validation | same field errors as REST | 6 |
| write flows | create → get → update → get → delete → get → delete give the same results on all three servers; ids and timestamps are checked separately (format, ordering, near now) | 3 × 7 |
| cleanup | the dataset is unchanged afterwards | — |

The read corpus contains every request the k6 workloads send:
- all 1000 get-product ids;
- all 80 list pages (limits 10/50/100/500 × pages 0–19);
- every search term with the default limit and at limits 1–1000.

It also covers other limits and offsets, and edge cases: wildcards, case, `+` encoding,
repeated parameters, and out-of-range or non-numeric values. In total that is 1204 REST
and 1188 gRPC read requests per mode.

The suite was verified to fail on two planted bugs: a TS default search limit of 49
instead of 50, and gRPC returning `stock + 1`. It flagged every affected request,
reporting the first differing byte.

Some differences are by design and are not compared:
- an invalid path id combined with a body error (REST rejects the id before reading the
  body, while gRPC reports both);
- the edge cases listed under the REST API contract.

For the oversized body, transport errors are retried and the first real response from
each server is compared.

## Benchmark design decisions

1. **Numeric fields.** `price` and `rating` are `NUMERIC` in the database and plain numbers
   in the API (JSON number, protobuf `double`). Seeded values have at most 2 decimals, so
   they print the same in Go and JS.
2. **Timestamps.** Stored as `TIMESTAMPTZ`. The server and container run in UTC, and the API
   returns RFC 3339 UTC strings (JSON) or `google.protobuf.Timestamp` (gRPC).
3. **Search semantics.** `WHERE name ILIKE $1 ORDER BY id LIMIT $2`, with wildcards in the
   query escaped. Search matches the name only, is case-insensitive, defaults to 50
   results and returns them in a deterministic order. For the common benchmark terms
   PostgreSQL picks a primary-key scan with a filter (about 1 ms), not the trigram index.
   The trigram index matters for rare terms. Query plans will be captured with the
   results.
4. **Pagination.** `ORDER BY id LIMIT $1 OFFSET $2`, with `limit` capped at 1000. Cost
   grows with `OFFSET` (OFFSET 50000 takes about 9 ms here), so the list workload pages
   through the first 20 pages only.
5. **In-memory mode data.** At startup, `DATA_SOURCE=memory` loads the table from PostgreSQL
   once into memory. Both languages then serve exactly the seeded dataset, and no
   generator is duplicated. The database is not touched per request.
6. **Writes change the dataset.** Write benchmarks must be followed by `scripts/db.sh reset`,
   and read benchmarks run against a freshly reset database.
7. **Connection pools.** Every server uses a pool of `DB_MAX_CONNS=20`.
   `max_connections=200` leaves room for three servers plus admin sessions.
8. **PostgreSQL settings.** Pinned image `postgres:18.1` with `shared_buffers=256MB`, so the
   whole dataset stays cached. Database-mode results therefore measure a warm cache, not
   disk I/O.
9. **Prepared statements.** pgx prepares each statement once per connection and reuses it.
   The TS repository names its queries so node-postgres does the same.
10. **gRPC optional fields.** `limit` and `offset` are proto3 `optional`, so "not sent"
    (apply the default) and "sent 0" (validation error) stay distinct, as they are in REST.
11. **CPU and memory limits.** Every server gets identical limits (`SERVER_CPUS`,
    `SERVER_MEMORY`). Results must state the limit, and are best reported at more than one.
12. **Benchmark isolation.** Health checks run once a minute after startup, request
    logging is off, and only one server is loaded at a time.
13. **Load generator placement.** k6 runs in a container on the compose network, avoiding
    Docker Desktop's port proxy.
14. **Client-side decoding.** k6 discards REST bodies and gRPC messages, so the client
    measures servers rather than its own decoding. Runs where k6 is CPU-bound are flagged
    automatically.
15. **Server metrics off the request path.** Each server measures itself on a separate
    port, only when scraped once per second. No per-request instrumentation exists in
    any server.
16. **Run-to-run variance.** Identical runs vary by up to ±30% on this machine. Results come
    from 3 interleaved repeats, with the spread reported, never from a single run.
17. **CPU pinning, not quotas.** CFS quotas throttle Go's multi-threaded runtime much more
    than Node's single JavaScript thread (Go was throttled in 100% of periods at 1 CPU),
    which would turn a limit artifact into an apparent 2× runtime difference. Servers are
    pinned to one dedicated CPU without a quota; quota mode remains available as a labelled
    experiment.
