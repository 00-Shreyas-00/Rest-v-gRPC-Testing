# Benchmark session 2026-11-05: database mode, pinned CPUs

> **Superseded by [2026-11-06-full-pinned](../2026-11-06-full-pinned/).** This partial
> session ran while the laptop repeatedly entered Windows Modern Standby. Its run-to-run
> spread was ±30–450%, and its throughput was 25–50% below the clean session's. Some of
> its conclusions do not hold. It is kept to document the power-state problem and how it
> was detected.

| file | contents |
|---|---|
| [report.md](report.md) | full report: throughput, latency, server resources, runtime detail, experiment ratios, wire bytes, per workload and concurrency level |
| [report.csv](report.csv) | the same aggregates, one row per (workload, server, VUs) |
| [raw/](raw/) | every kept run (one JSON object per line), including `server`, `containers` and `measure_window_ms` |
| [environment.json](environment.json) | host, Docker VM, versions, CPU sets, limits, dataset checksum |
| [exclusions.json](exclusions.json) | the 18 runs dropped because the laptop entered Windows Modern Standby, with the standby periods |
| [equivalence.log](equivalence.log) | the equivalence check that passed before the run |

## Scope

- **Configuration:** `scripts/run-matrix.sh --repeat 3 --warmup 10s --duration 20s
  --cooldown 3`, with the default pinned CPU layout (servers on vCPU 2, PostgreSQL on 3–5,
  k6 on 6–11).
- **Completed:** the database-mode pass for get-product and list-products at limits
  10/50/100/500.
- **Stopped:** the session was stopped at 315 of 756 planned runs. Search (8 runs) is left
  out; the in-memory pass never started.

Regenerate the report from the raw data:

```bash
node scripts/report.mjs --out /tmp/report docs/results/2026-11-05-postgres-pinned/raw
```

(The raw directory has no per-workload `environment.json`, so the regenerated report's
environment table will be empty; the values are in `environment.json` here.)

## Caveats

- Run-to-run noise on this laptop was large. List-10 and list-50 in particular have
  spreads that make every comparison inconclusive (`≈` in the report).
- One CPU per server: the results do not include Go's ability to use several cores.
