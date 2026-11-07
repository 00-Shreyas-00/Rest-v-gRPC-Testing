# Experiment 2 session 2026-11-06/07: large payloads and streaming

This is the dataset behind the README's
[Experiment 2 results](../../../README.md#experiment-2-results) section. The
methodology is in [docs/experiment-2-design.md](../../experiment-2-design.md).

| file | contents |
|---|---|
| [report.md](report.md) | full report: codec micro-benchmark, part A (payloads × network × gzip × concurrency), part B (export, import, quote calls, price feed) |
| [report.csv](report.csv) | the same aggregates, one row per point and server |
| [raw/results.jsonl](raw/results.jsonl) | every run (1344 lines): labels, throughput, latency, time to first item, wire bytes, server CPU / RSS / GC, client CPU, feed delivery |
| [codec-go.jsonl](codec-go.jsonl), [codec-node.jsonl](codec-node.jsonl) | codec micro-benchmark (JSON vs Protobuf, gzip level 1) |
| [environment.json](environment.json) | host, Docker VM, CPU sets, memory limits, the exact `tc` commands per network, image ids |
| [verify.log](verify.log) | the equivalence check (124 checks) that passed before the load started |
| [events.log](events.log) | network changes (UTC) |
| [session.log](session.log) | the progress log of the run (local time, IST) |

## Scope

- **Command:** `scripts/run-x2.sh --session x2-full --repeat 3` (warm-up 5 s, measured
  15 s, cool-down 2 s).
- **Covered:** 1344 runs.
  - Part A: 972 runs. Profiles text, numeric and binary; 10, 250 and 4000 items; gzip
    off and on; 1 and 16 concurrent clients; networks lan, dc and wan.
  - Part B: 372 runs. Export of 1k, 10k and 100k items (unary vs stream, lan and wan);
    import of 1k and 10k items; quote calls at 1, 16 and 128 clients; price feed with
    100 to 5000 subscribers at 20 events/s.
- **Networks** (`netem` delay + `tbf` rate on every container's eth0):
  - lan: no shaping
  - dc: 0.5 ms RTT, 1 Gbit/s
  - wan: 20 ms RTT, 100 Mbit/s
- **CPU layout:** each server pinned to vCPU 2 (1 GiB, `GOMEMLIMIT=900MiB` for Go);
  the load generator on vCPUs 3–11. Only one server is under load at a time.
- **Run quality:**
  - 0 failed runs, 0 runs with errors, 0 server restarts.
  - The load generator was never CPU-bound (threshold 75%).
  - The equivalence check passed on all three servers.

Regenerate the aggregates from the raw data (the script reads `results.jsonl`,
`environment.json` and the codec files from one directory):

```bash
mkdir -p /tmp/x2 && cp docs/results/2026-11-07-x2-full/raw/results.jsonl docs/results/2026-11-07-x2-full/*.json* /tmp/x2/ && node scripts/report-x2.mjs /tmp/x2
```

## Known event: overnight standby

During part A on the dc network (repeat 1), Windows logged a power-source change at
21:13 IST and entered Modern Standby until 05:21 IST. The run paused for about 8 hours
and continued when the machine woke, which is why the session took 17 h instead of
about 10 h.

The runs on either side of the pause match their other two repeats within about 1%
(for example 27.02 vs 27.11 and 27.09 responses/s), so no run was excluded.
