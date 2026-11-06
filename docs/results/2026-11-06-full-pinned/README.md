# Benchmark session 2026-11-06: full matrix, pinned CPUs

This is the dataset behind the README's [Results](../../../README.md#results) section.

| file | contents |
|---|---|
| [report.md](report.md) | full report: throughput, latency, server resources, runtime detail, experiment ratios and wire bytes, for every workload, data mode and concurrency level |
| [report.csv](report.csv) | the same aggregates, one row per (data mode, workload, server, VUs) |
| [raw/postgres/](raw/postgres/), [raw/memory/](raw/memory/) | every run (one JSON object per line), including `server`, `containers` and `measure_window_ms` |
| [environment-postgres.json](environment-postgres.json), [environment-memory.json](environment-memory.json) | host, Docker VM, versions, CPU sets, limits, dataset checksum for each pass |
| [equivalence.log](equivalence.log) | the equivalence check (both data modes) that passed before the load started |

## Scope

- **Command:** `scripts/run-matrix.sh --session full-20261106 --repeat 3 --warmup 10s
  --duration 20s --cooldown 3`.
- **Covered:** both data modes; get-product, list-products at limits 10/50/100/500, and
  search; VUs 1–1000; 3 interleaved repeats; 756 runs in 7 h 58 min.
- **CPU layout:** each server pinned to vCPU 2 with no quota. PostgreSQL on 3–5. k6 on
  6–11 in database mode and 3–11 in memory mode.
- **Run quality:**
  - No run was excluded.
  - Windows logged no standby events during the session (screen, sleep and hibernate
    timeouts were set to Never while plugged in).
  - k6 was never CPU-bound.
  - The dataset checksum matched in every pass.

Regenerate the aggregates from the raw data:

```bash
node scripts/report.mjs --out /tmp/report docs/results/2026-11-06-full-pinned/raw
```

## Known anomaly

Memory mode, list-products limit=500, 1000 VUs, Go gRPC:
- **What happened:** throughput fell to about 280 req/s in all three repeats (about
  1,700 at 500 VUs), and server CPU dropped to about 0.3 cores.
- **Memory:** the process's RSS reached about 1 GB, the container's memory limit; the
  runtime endpoint also timed out (`scrape_errors`).
- **Knock-on effect:** the next Go gRPC run at 1 VU (repeat 2) still had 927 MB RSS and
  lower throughput.
- **Likely cause:** memory pressure from buffering about 1000 large in-flight responses.

The runs are kept in the data as measured. The README discusses the anomaly.
