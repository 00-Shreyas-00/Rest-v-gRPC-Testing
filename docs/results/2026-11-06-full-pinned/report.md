# Benchmark report

Generated 2026-11-05T23:51:26.224Z from results/full-20261106.

## Environment

|  |  |
| --- | ---: |
| Host CPU | 12th Gen Intel(R) Core(TM) i5-12450H |
| Host OS | Microsoft Windows 11 Home Single Language 10.0.26200 |
| Docker | 29.7.2 |
| Docker VM | Docker Desktop (containerized) |
| Docker VM kernel | 6.18.54.1-microsoft-standard-WSL2 |
| Docker VM CPUs / memory | 12 / 7.6 GiB |
| Go | go1.26.3 |
| Node.js | v24.22.0 |
| TypeScript | 7.0.2 |
| PostgreSQL | 18.1 (Debian 18.1-1.pgdg13+2) |
| k6 | k6 v2.2.0 (commit/00a9a1b7f5, go1.26.5, linux/amd64) |
| Server CPU limit | 1 |
| CPU limit mode | pinned |
| Server CPU set | 2 |
| k6 CPU set | 3-11; 6-11 |
| Server memory limit | 1024MiB |
| PostgreSQL CPU limit | 3 pinned 3-5 |
| k6 CPU limit | 9; 6 |
| DB pool per server | 20 |
| Data source | memory; postgres |
| Products / dataset checksum | 100000 / d26bc7a601e76ff91ba94bc08ebebc68 |
| Warm-up / measured / cool-down | 10s / 20s / 3 s |
| Repeats per point | 3 |
| Network | k6 container -> server container on the compose bridge network (no published ports) |
| TLS | false |

Runs: 756. Runs where k6 was CPU-bound: 0. Total errors: 7409.

Legend: ⚠k6 = k6 was CPU-bound in at least one repeat (client limit, not a server result); ⚠Nerr / ⚠Nt/o = errors / timeouts summed over repeats.

## get-product: memory mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 6,502 ±1% | 5,833 ±2% | 4,924 ±1% |
| 10 | 41,056 ±1% | 32,689 ±1% | 19,651 ±0% |
| 50 | 44,217 ±3% | 28,890 ±6% | 25,743 ±2% |
| 100 | 39,402 ±2% | 27,263 ±1% | 24,084 ±1% |
| 250 | 35,145 ±1% | 26,018 ±2% | 21,896 ±1% |
| 500 | 33,429 ±1% | 24,364 ±6% | 20,681 ±1% |
| 1000 | 31,934 ±2% | 23,438 ±2% | 18,784 ±2% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.10 / 0.17 / 0.28 | 0.11 / 0.21 / 0.36 | 0.14 / 0.23 / 0.41 |
| 10 | 0.16 / 0.41 / 0.71 | 0.22 / 0.55 / 0.90 | 0.41 / 0.79 / 1.32 |
| 50 | 0.97 / 2.34 / 3.66 | 1.41 / 3.08 / 4.64 | 1.64 / 3.70 / 5.66 |
| 100 | 2.30 / 5.25 / 7.67 | 3.14 / 6.31 / 9.39 | 3.64 / 7.73 / 11.3 |
| 250 | 6.55 / 12.9 / 19.7 | 8.45 / 16.3 / 21.5 | 10.2 / 21.0 / 29.2 |
| 500 | 13.4 / 25.6 / 40.0 | 17.7 / 34.3 / 47.2 | 21.2 / 43.3 / 56.6 |
| 1000 | 27.2 / 54.7 / 82.0 | 37.0 / 72.6 / 92.9 | 47.7 / 88.6 / 112 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.36 / 0.055 / 93 | 0.42 / 0.072 / 199 | 0.49 / 0.100 / 103 |
| 10 | 0.93 / 0.023 / 93 | 0.99 / 0.030 / 199 | 0.84 / 0.043 / 103 |
| 50 | 0.99 / 0.023 / 93 | 1.00 / 0.035 / 199 | 0.98 / 0.038 / 107 |
| 100 | 1.00 / 0.025 / 93 | 0.99 / 0.036 / 199 | 0.99 / 0.041 / 112 |
| 250 | 1.00 / 0.028 / 95 | 1.00 / 0.038 / 199 | 0.99 / 0.045 / 128 |
| 500 | 0.99 / 0.030 / 102 | 0.99 / 0.041 / 199 | 1.00 / 0.048 / 157 |
| 1000 | 0.99 / 0.031 / 115 | 1.00 / 0.043 / 200 | 1.00 / 0.053 / 205 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 1% CPU, 2.8 KiB alloc/req, ≤ 18 goroutines | ELU 0.39, loop delay p99 ≤ 15.7 ms, GC/1k req 2 | GC 1% CPU, 5.5 KiB alloc/req, ≤ 22 goroutines |
| 10 | GC 2% CPU, 2.8 KiB alloc/req, ≤ 27 goroutines | ELU 0.98, loop delay p99 ≤ 22.8 ms, GC/1k req 1 | GC 2% CPU, 5.5 KiB alloc/req, ≤ 49 goroutines |
| 50 | GC 2% CPU, 2.8 KiB alloc/req, ≤ 68 goroutines | ELU 1.00, loop delay p99 ≤ 20.8 ms, GC/1k req 1 | GC 3% CPU, 5.5 KiB alloc/req, ≤ 170 goroutines |
| 100 | GC 2% CPU, 2.8 KiB alloc/req, ≤ 118 goroutines | ELU 1.00, loop delay p99 ≤ 22.6 ms, GC/1k req 1 | GC 3% CPU, 5.6 KiB alloc/req, ≤ 320 goroutines |
| 250 | GC 2% CPU, 2.8 KiB alloc/req, ≤ 268 goroutines | ELU 1.00, loop delay p99 ≤ 63.8 ms, GC/1k req 1 | GC 3% CPU, 5.6 KiB alloc/req, ≤ 768 goroutines |
| 500 | GC 2% CPU, 2.8 KiB alloc/req, ≤ 519 goroutines | ELU 1.00, loop delay p99 ≤ 80.6 ms, GC/1k req 0 | GC 3% CPU, 5.7 KiB alloc/req, ≤ 1,520 goroutines |
| 1000 | GC 2% CPU, 2.8 KiB alloc/req, ≤ 1,018 goroutines | ELU 1.00, loop delay p99 ≤ 111 ms, GC/1k req 0 | GC 4% CPU, 5.7 KiB alloc/req, ≤ 3,021 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1.11× | 0.76× | 0.84× | 0.79× | 1.44× | 1.14× |
| 10 | 1.26× | 0.48× | 0.60× | 0.79× | 1.86× | 1.47× |
| 50 | 1.53× | 0.58× | 0.89× | 0.79× | 1.55× | 1.22× |
| 100 | 1.45× | 0.61× | 0.88× | 0.82× | 1.48× | 1.21× |
| 250 | 1.35× | 0.62× | 0.84× | 0.92× | 1.48× | 1.36× |
| 500 | 1.37× | 0.62× | 0.85× | 0.85× | 1.41× | 1.20× |
| 1000 | 1.36× | 0.59× | 0.80× | 0.88× | 1.37× | 1.21× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 453 | 502 | 295 |
| sent | 82 | 82 | 84 |

## list-products (limit=10): memory mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 5,985 ±2% | 5,386 ±1% | 4,632 ±1% |
| 10 | 30,532 ±0% | 26,305 ±1% | 18,613 ±0% |
| 50 | 28,178 ±0% | 24,145 ±0% | 20,634 ±1% |
| 100 | 26,322 ±0% | 22,449 ±22% | 19,026 ±1% |
| 250 | 24,108 ±1% | 20,816 ±6% | 17,198 ±1% |
| 500 | 23,411 ±2% | 20,557 ±1% | 16,080 ±1% |
| 1000 | 22,520 ±1% | 20,001 ±1% | 15,532 ±1% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.11 / 0.19 / 0.32 | 0.13 / 0.22 / 0.40 | 0.15 / 0.25 / 0.47 |
| 10 | 0.25 / 0.60 / 1.19 | 0.28 / 0.69 / 1.11 | 0.41 / 0.91 / 1.57 |
| 50 | 1.60 / 3.77 / 5.33 | 1.74 / 3.63 / 5.48 | 2.03 / 4.75 / 6.64 |
| 100 | 3.51 / 7.94 / 10.9 | 3.88 / 7.69 / 11.1 | 4.53 / 10.1 / 13.7 |
| 250 | 9.61 / 19.4 / 26.2 | 10.7 / 19.3 / 25.9 | 13.1 / 25.7 / 33.8 |
| 500 | 19.5 / 37.0 / 49.2 | 21.9 / 38.0 / 49.2 | 27.5 / 52.6 / 69.2 |
| 1000 | 40.6 / 71.1 / 91.6 | 44.3 / 76.3 / 99.0 | 54.1 / 120 / 156 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.42 / 0.070 / 94 | 0.47 / 0.088 / 201 | 0.53 / 0.114 / 102 |
| 10 | 0.99 / 0.032 / 95 | 0.99 / 0.038 / 201 | 0.91 / 0.049 / 104 |
| 50 | 1.00 / 0.035 / 95 | 1.00 / 0.041 / 201 | 0.99 / 0.048 / 107 |
| 100 | 1.00 / 0.038 / 95 | 0.99 / 0.044 / 201 | 0.99 / 0.052 / 112 |
| 250 | 0.99 / 0.041 / 98 | 1.00 / 0.048 / 201 | 1.00 / 0.058 / 124 |
| 500 | 0.99 / 0.042 / 105 | 1.00 / 0.049 / 201 | 1.00 / 0.062 / 137 |
| 1000 | 0.99 / 0.044 / 119 | 1.00 / 0.050 / 201 | 1.00 / 0.064 / 189 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 2% CPU, 8.4 KiB alloc/req, ≤ 18 goroutines | ELU 0.45, loop delay p99 ≤ 8.18 ms, GC/1k req 4 | GC 1% CPU, 9.3 KiB alloc/req, ≤ 22 goroutines |
| 10 | GC 4% CPU, 8.4 KiB alloc/req, ≤ 27 goroutines | ELU 0.99, loop delay p99 ≤ 19.5 ms, GC/1k req 2 | GC 3% CPU, 9.2 KiB alloc/req, ≤ 49 goroutines |
| 50 | GC 3% CPU, 8.4 KiB alloc/req, ≤ 68 goroutines | ELU 1.00, loop delay p99 ≤ 17.3 ms, GC/1k req 1 | GC 4% CPU, 9.2 KiB alloc/req, ≤ 169 goroutines |
| 100 | GC 3% CPU, 8.4 KiB alloc/req, ≤ 119 goroutines | ELU 1.00, loop delay p99 ≤ 48.4 ms, GC/1k req 1 | GC 4% CPU, 9.2 KiB alloc/req, ≤ 320 goroutines |
| 250 | GC 3% CPU, 8.4 KiB alloc/req, ≤ 268 goroutines | ELU 1.00, loop delay p99 ≤ 69.5 ms, GC/1k req 1 | GC 5% CPU, 9.3 KiB alloc/req, ≤ 771 goroutines |
| 500 | GC 3% CPU, 8.4 KiB alloc/req, ≤ 520 goroutines | ELU 1.00, loop delay p99 ≤ 79.0 ms, GC/1k req 1 | GC 5% CPU, 9.3 KiB alloc/req, ≤ 1,521 goroutines |
| 1000 | GC 4% CPU, 8.4 KiB alloc/req, ≤ 1,020 goroutines | ELU 1.00, loop delay p99 ≤ 102 ms, GC/1k req 1 | GC 9% CPU, 9.3 KiB alloc/req, ≤ 3,021 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1.11× | 0.77× | 0.86× | 0.81× | 1.44× | 1.17× |
| 10 | 1.16× | 0.61× | 0.71× | 1.07× | 1.31× | 1.41× |
| 50 | 1.17× | 0.73× | 0.85× | 0.97× | 1.25× | 1.21× |
| 100 | 1.17× | 0.72× | ≈0.85× | 0.98× | 1.26× | 1.23× |
| 250 | 1.16× | 0.71× | 0.83× | 1.01× | 1.29× | 1.30× |
| 500 | 1.14× | 0.69× | 0.78× | 1.00× | 1.41× | 1.41× |
| 1000 | 1.13× | 0.69× | 0.78× | 0.93× | 1.70× | 1.57× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 3,415 | 3,464 | 2,217 |
| sent | 95 | 95 | 85 |

## list-products (limit=50): memory mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 4,377 ±1% | 4,305 ±4% | 3,595 ±1% |
| 10 | 11,514 ±0% | 13,873 ±1% | 12,108 ±3% |
| 50 | 10,954 ±1% | 13,379 ±1% | 12,108 ±1% |
| 100 | 10,821 ±1% | 12,972 ±1% | 11,504 ±1% |
| 250 | 10,601 ±0% | 12,459 ±1% | 10,695 ±0% |
| 500 | 10,325 ±0% | 12,153 ±1% | 10,043 ±1% |
| 1000 | 10,255 ±1% | 11,799 ±1% ⚠962err ⚠962t/o | 9,526 ±0% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.16 / 0.26 / 0.48 | 0.17 / 0.27 / 0.47 | 0.19 / 0.35 / 0.63 |
| 10 | 0.77 / 1.79 / 2.70 | 0.54 / 1.22 / 1.87 | 0.62 / 1.50 / 2.58 |
| 50 | 4.31 / 9.27 / 12.7 | 3.33 / 6.33 / 8.27 | 3.49 / 7.52 / 10.1 |
| 100 | 8.81 / 17.8 / 22.1 | 7.12 / 12.0 / 14.9 | 7.69 / 15.7 / 20.4 |
| 250 | 23.1 / 40.3 / 49.2 | 18.8 / 28.8 / 34.5 | 21.3 / 39.5 / 48.7 |
| 500 | 48.8 / 73.6 / 85.0 | 38.5 / 55.7 / 69.8 | 45.8 / 87.1 / 107 |
| 1000 | 103 / 135 / 150 | 65.8 / 96.2 / 130 | 99.5 / 174 / 212 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.54 / 0.123 / 94 | 0.51 / 0.123 / 208 | 0.52 / 0.147 / 103 |
| 10 | 0.99 / 0.086 / 99 | 1.00 / 0.072 / 206 | 0.94 / 0.078 / 110 |
| 50 | 0.99 / 0.091 / 99 | 1.00 / 0.075 / 206 | 0.99 / 0.082 / 111 |
| 100 | 0.99 / 0.092 / 99 | 1.00 / 0.077 / 206 | 0.99 / 0.086 / 116 |
| 250 | 1.00 / 0.094 / 107 | 1.00 / 0.080 / 206 | 0.99 / 0.093 / 129 |
| 500 | 0.99 / 0.097 / 113 | 1.00 / 0.082 / 206 | 0.99 / 0.099 / 137 |
| 1000 | 1.00 / 0.097 / 125 | 0.99 / 0.084 / 208 | 0.99 / 0.104 / 174 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 4% CPU, 31.7 KiB alloc/req, ≤ 18 goroutines | ELU 0.49, loop delay p99 ≤ 8.07 ms, GC/1k req 6 | GC 3% CPU, 25.2 KiB alloc/req, ≤ 22 goroutines |
| 10 | GC 5% CPU, 31.7 KiB alloc/req, ≤ 27 goroutines | ELU 1.00, loop delay p99 ≤ 16.1 ms, GC/1k req 3 | GC 6% CPU, 25.1 KiB alloc/req, ≤ 49 goroutines |
| 50 | GC 5% CPU, 31.7 KiB alloc/req, ≤ 68 goroutines | ELU 1.00, loop delay p99 ≤ 12.2 ms, GC/1k req 3 | GC 6% CPU, 25.2 KiB alloc/req, ≤ 167 goroutines |
| 100 | GC 5% CPU, 31.7 KiB alloc/req, ≤ 119 goroutines | ELU 1.00, loop delay p99 ≤ 14.2 ms, GC/1k req 3 | GC 6% CPU, 25.2 KiB alloc/req, ≤ 320 goroutines |
| 250 | GC 5% CPU, 31.7 KiB alloc/req, ≤ 270 goroutines | ELU 1.00, loop delay p99 ≤ 45.4 ms, GC/1k req 3 | GC 8% CPU, 25.5 KiB alloc/req, ≤ 771 goroutines |
| 500 | GC 5% CPU, 31.6 KiB alloc/req, ≤ 520 goroutines | ELU 1.00, loop delay p99 ≤ 113 ms, GC/1k req 3 | GC 10% CPU, 25.4 KiB alloc/req, ≤ 1,522 goroutines |
| 1000 | GC 5% CPU, 31.6 KiB alloc/req, ≤ 1,019 goroutines | ELU 1.00, loop delay p99 ≤ 162 ms, GC/1k req 3 | GC 14% CPU, 25.3 KiB alloc/req, ≤ 3,019 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | ≈1.02× | 0.82× | 0.84× | 1.04× | 1.31× | 1.36× |
| 10 | 0.83× | 1.05× | 0.87× | 1.45× | 0.96× | 1.38× |
| 50 | 0.82× | 1.11× | 0.91× | 1.53× | 0.79× | 1.22× |
| 100 | 0.83× | 1.06× | 0.89× | 1.48× | 0.92× | 1.36× |
| 250 | 0.85× | 1.01× | 0.86× | 1.42× | 0.99× | 1.41× |
| 500 | 0.85× | 0.97× | 0.83× | 1.22× | 1.26× | 1.53× |
| 1000 | 0.87× | 0.93× | 0.81× | 1.16× | 1.41× | 1.63× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 16,543 | 16,592 | 10,783 |
| sent | 96 | 96 | 85 |

## list-products (limit=100): memory mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 3,487 ±0% | 3,594 ±1% | 3,041 ±2% |
| 10 | 6,881 ±1% | 8,805 ±1% | 8,449 ±2% |
| 50 | 6,672 ±1% | 8,442 ±0% | 8,071 ±1% |
| 100 | 6,613 ±0% | 8,436 ±1% | 7,845 ±2% |
| 250 | 6,408 ±0% | 8,113 ±1% | 7,372 ±0% |
| 500 | 6,421 ±1% | 8,032 ±0% | 7,089 ±1% |
| 1000 | 6,338 ±1% | 7,935 ±0% ⚠1200err ⚠1200t/o | 6,704 ±0% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.22 / 0.34 / 0.68 | 0.21 / 0.33 / 0.55 | 0.23 / 0.44 / 0.77 |
| 10 | 1.33 / 3.06 / 4.30 | 0.90 / 1.93 / 2.71 | 0.89 / 2.18 / 3.72 |
| 50 | 7.18 / 14.8 / 18.4 | 5.42 / 9.30 / 12.3 | 5.46 / 10.8 / 13.9 |
| 100 | 14.9 / 27.3 / 32.7 | 11.1 / 16.6 / 21.8 | 11.5 / 21.8 / 26.8 |
| 250 | 38.9 / 61.4 / 70.8 | 29.0 / 41.9 / 48.0 | 32.8 / 52.1 / 63.3 |
| 500 | 83.1 / 106 / 119 | 58.8 / 79.4 / 90.7 | 68.5 / 111 / 134 |
| 1000 | 169 / 208 / 242 | 93.8 / 128 / 170 | 145 / 219 / 273 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.63 / 0.180 / 92 | 0.59 / 0.163 / 209 | 0.57 / 0.186 / 100 |
| 10 | 1.00 / 0.145 / 102 | 1.00 / 0.113 / 209 | 0.96 / 0.113 / 114 |
| 50 | 1.00 / 0.149 / 101 | 1.00 / 0.118 / 209 | 0.99 / 0.123 / 115 |
| 100 | 1.00 / 0.150 / 105 | 1.00 / 0.119 / 209 | 0.99 / 0.126 / 121 |
| 250 | 0.99 / 0.155 / 109 | 1.00 / 0.123 / 209 | 0.99 / 0.135 / 139 |
| 500 | 0.99 / 0.155 / 117 | 1.00 / 0.124 / 209 | 0.99 / 0.140 / 151 |
| 1000 | 0.99 / 0.157 / 131 | 1.00 / 0.126 / 209 | 0.99 / 0.148 / 182 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 5% CPU, 65.0 KiB alloc/req, ≤ 18 goroutines | ELU 0.57, loop delay p99 ≤ 4.92 ms, GC/1k req 11 | GC 4% CPU, 45.9 KiB alloc/req, ≤ 22 goroutines |
| 10 | GC 6% CPU, 65.0 KiB alloc/req, ≤ 28 goroutines | ELU 1.00, loop delay p99 ≤ 9.26 ms, GC/1k req 5 | GC 7% CPU, 45.7 KiB alloc/req, ≤ 49 goroutines |
| 50 | GC 6% CPU, 64.9 KiB alloc/req, ≤ 68 goroutines | ELU 1.00, loop delay p99 ≤ 17.0 ms, GC/1k req 5 | GC 7% CPU, 45.9 KiB alloc/req, ≤ 170 goroutines |
| 100 | GC 6% CPU, 65.0 KiB alloc/req, ≤ 119 goroutines | ELU 1.00, loop delay p99 ≤ 19.5 ms, GC/1k req 5 | GC 8% CPU, 46.0 KiB alloc/req, ≤ 320 goroutines |
| 250 | GC 6% CPU, 64.9 KiB alloc/req, ≤ 271 goroutines | ELU 1.00, loop delay p99 ≤ 51.2 ms, GC/1k req 5 | GC 8% CPU, 46.5 KiB alloc/req, ≤ 773 goroutines |
| 500 | GC 6% CPU, 64.9 KiB alloc/req, ≤ 519 goroutines | ELU 1.00, loop delay p99 ≤ 111 ms, GC/1k req 5 | GC 12% CPU, 46.3 KiB alloc/req, ≤ 1,521 goroutines |
| 1000 | GC 6% CPU, 64.5 KiB alloc/req, ≤ 1,020 goroutines | ELU 1.00, loop delay p99 ≤ 180 ms, GC/1k req 5 | GC 15% CPU, 46.1 KiB alloc/req, ≤ 3,023 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 0.97× | 0.87× | 0.85× | 1.24× | 1.13× | 1.40× |
| 10 | 0.78× | 1.23× | 0.96× | 1.59× | 0.87× | 1.37× |
| 50 | 0.79× | 1.21× | 0.96× | 1.49× | 0.76× | 1.13× |
| 100 | 0.78× | 1.19× | 0.93× | 1.50× | 0.82× | 1.23× |
| 250 | 0.79× | 1.15× | 0.91× | 1.48× | 0.89× | 1.32× |
| 500 | 0.80× | 1.10× | 0.88× | 1.32× | 1.13× | 1.48× |
| 1000 | 0.80× | 1.06× | 0.84× | 1.42× | 1.13× | 1.60× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 33,003 | 33,052 | 21,499 |
| sent | 97 | 97 | 99 |

## list-products (limit=500): memory mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 1,326 ±1% | 1,321 ±1% | 1,114 ±12% |
| 10 | 1,696 ±0% | 1,735 ±1% | 1,753 ±1% |
| 50 | 1,672 ±1% | 1,752 ±1% | 1,747 ±0% |
| 100 | 1,684 ±0% | 1,750 ±1% | 1,753 ±1% |
| 250 | 1,689 ±1% | 1,764 ±1% | 1,717 ±1% |
| 500 | 1,687 ±1% | 1,793 ±0% ⚠1318err ⚠1318t/o | 1,697 ±2% |
| 1000 | 1,737 ±1% | 1,839 ±1% ⚠2212err ⚠2212t/o | 277 ±5% ⚠435err ⚠435t/o |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.63 / 1.08 / 1.76 | 0.63 / 1.10 / 1.88 | 0.68 / 1.35 / 2.04 |
| 10 | 5.41 / 13.2 / 17.0 | 4.82 / 10.2 / 14.3 | 5.05 / 8.70 / 11.3 |
| 50 | 30.0 / 50.2 / 58.2 | 26.9 / 36.2 / 61.5 | 27.0 / 44.4 / 56.0 |
| 100 | 63.7 / 83.1 / 89.4 | 54.6 / 70.6 / 119 | 55.3 / 82.4 / 97.5 |
| 250 | 168 / 193 / 207 | 133 / 172 / 213 | 139 / 219 / 263 |
| 500 | 342 / 382 / 402 | 142 / 191 / 9,999 | 290 / 383 / 555 |
| 1000 | 673 / 723 / 752 | 357 / 438 / 9,989 | 3,653 / 7,162 / 7,274 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.81 / 0.613 / 97 | 0.79 / 0.595 / 237 | 0.55 / 0.491 / 928 |
| 10 | 0.99 / 0.586 / 106 | 1.00 / 0.574 / 244 | 0.99 / 0.564 / 142 |
| 50 | 0.99 / 0.594 / 109 | 1.00 / 0.570 / 236 | 0.99 / 0.567 / 230 |
| 100 | 1.00 / 0.592 / 110 | 1.00 / 0.570 / 244 | 0.99 / 0.563 / 335 |
| 250 | 1.00 / 0.591 / 115 | 1.00 / 0.566 / 239 | 0.99 / 0.574 / 447 |
| 500 | 1.00 / 0.591 / 122 | 0.99 / 0.554 / 237 | 0.99 / 0.584 / 518 |
| 1000 | 1.00 / 0.574 / 138 | 0.99 / 0.541 / 240 | 0.36 / 1.211 / 1,006 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 7% CPU, 282.0 KiB alloc/req, ≤ 18 goroutines | ELU 0.78, loop delay p99 ≤ 24.1 ms, GC/1k req 32 | GC 8% CPU, 206.3 KiB alloc/req, ≤ 19 goroutines |
| 10 | GC 6% CPU, 281.6 KiB alloc/req, ≤ 28 goroutines | ELU 1.00, loop delay p99 ≤ 37.2 ms, GC/1k req 26 | GC 5% CPU, 205.8 KiB alloc/req, ≤ 50 goroutines |
| 50 | GC 6% CPU, 281.4 KiB alloc/req, ≤ 68 goroutines | ELU 1.00, loop delay p99 ≤ 75.5 ms, GC/1k req 24 | GC 4% CPU, 211.5 KiB alloc/req, ≤ 172 goroutines |
| 100 | GC 6% CPU, 280.8 KiB alloc/req, ≤ 120 goroutines | ELU 1.00, loop delay p99 ≤ 124 ms, GC/1k req 22 | GC 3% CPU, 210.4 KiB alloc/req, ≤ 320 goroutines |
| 250 | GC 6% CPU, 279.6 KiB alloc/req, ≤ 270 goroutines | ELU 1.00, loop delay p99 ≤ 223 ms, GC/1k req 22 | GC 2% CPU, 207.7 KiB alloc/req, ≤ 769 goroutines |
| 500 | GC 7% CPU, 277.8 KiB alloc/req, ≤ 519 goroutines | ELU 1.00, loop delay p99 ≤ 208 ms, GC/1k req 29 | GC 3% CPU, 204.9 KiB alloc/req, ≤ 1,521 goroutines |
| 1000 | GC 7% CPU, 273.8 KiB alloc/req, ≤ 1,018 goroutines | ELU 1.00, loop delay p99 ≤ 467 ms, GC/1k req 17 | GC 11% CPU, 193.1 KiB alloc/req, ≤ 3,149 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | ≈1.00× | 0.84× | 0.84× | 0.94× | 1.16× | 1.09× |
| 10 | 0.98× | 1.03× | ≈1.01× | 1.18× | 0.66× | 0.79× |
| 50 | 0.95× | 1.05× | ≈1.00× | 0.95× | 0.96× | 0.91× |
| 100 | 0.96× | 1.04× | ≈1.00× | 0.75× | 1.09× | 0.82× |
| 250 | 0.96× | ≈1.02× | 0.97× | 0.97× | 1.28× | 1.24× |
| 500 | 0.94× | ≈1.01× | 0.95× | 0.04× | 1.38× | 0.06× |
| 1000 | 0.94× | 0.16× | 0.15× | 0.08× | 9.68× | 0.73× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 164,678 | 164,727 | 107,197 |
| sent | 98 | 98 | 117 |

## search-products (all terms, limit=server default (50)): memory mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 4,002 ±1% | 3,736 ±2% | 3,352 ±16% |
| 10 | 10,036 ±1% | 10,394 ±1% | 10,619 ±0% |
| 50 | 9,621 ±1% | 9,978 ±1% | 10,110 ±1% |
| 100 | 9,461 ±0% | 9,800 ±1% | 9,690 ±0% |
| 250 | 9,168 ±0% | 9,404 ±0% | 8,992 ±1% |
| 500 | 9,029 ±0% | 9,226 ±0% | 8,465 ±0% |
| 1000 | 8,976 ±0% | 9,000 ±1% ⚠1282err ⚠1282t/o | 8,089 ±2% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.18 / 0.29 / 0.60 | 0.20 / 0.33 / 0.58 | 0.20 / 0.40 / 0.70 |
| 10 | 0.88 / 2.11 / 3.09 | 0.75 / 1.62 / 2.44 | 0.69 / 1.82 / 3.19 |
| 50 | 4.87 / 10.6 / 14.0 | 4.57 / 8.12 / 10.4 | 4.18 / 9.01 / 11.8 |
| 100 | 10.1 / 20.3 / 25.0 | 9.50 / 15.2 / 19.4 | 9.12 / 18.4 / 23.0 |
| 250 | 26.7 / 46.0 / 55.1 | 25.1 / 36.7 / 43.1 | 25.1 / 48.2 / 59.7 |
| 500 | 56.5 / 81.3 / 95.4 | 50.8 / 71.0 / 83.7 | 53.4 / 102 / 121 |
| 1000 | 118 / 151 / 175 | 86.3 / 132 / 158 | 118 / 197 / 249 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.56 / 0.140 / 92 | 0.54 / 0.149 / 210 | 0.57 / 0.169 / 932 |
| 10 | 1.00 / 0.099 / 100 | 0.99 / 0.096 / 210 | 0.96 / 0.091 / 121 |
| 50 | 1.00 / 0.104 / 100 | 1.00 / 0.100 / 210 | 0.99 / 0.098 / 121 |
| 100 | 0.99 / 0.105 / 101 | 1.00 / 0.102 / 210 | 0.99 / 0.103 / 124 |
| 250 | 0.99 / 0.108 / 106 | 1.00 / 0.106 / 210 | 0.99 / 0.111 / 139 |
| 500 | 1.00 / 0.110 / 113 | 1.00 / 0.108 / 210 | 0.99 / 0.117 / 150 |
| 1000 | 1.00 / 0.111 / 128 | 1.00 / 0.111 / 210 | 0.99 / 0.122 / 196 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 5% CPU, 42.0 KiB alloc/req, ≤ 18 goroutines | ELU 0.52, loop delay p99 ≤ 13.8 ms, GC/1k req 11 | GC 4% CPU, 37.3 KiB alloc/req, ≤ 22 goroutines |
| 10 | GC 5% CPU, 42.0 KiB alloc/req, ≤ 27 goroutines | ELU 1.00, loop delay p99 ≤ 8.36 ms, GC/1k req 5 | GC 7% CPU, 37.2 KiB alloc/req, ≤ 50 goroutines |
| 50 | GC 5% CPU, 42.0 KiB alloc/req, ≤ 68 goroutines | ELU 1.00, loop delay p99 ≤ 12.2 ms, GC/1k req 4 | GC 7% CPU, 37.3 KiB alloc/req, ≤ 167 goroutines |
| 100 | GC 5% CPU, 42.0 KiB alloc/req, ≤ 118 goroutines | ELU 1.00, loop delay p99 ≤ 23.9 ms, GC/1k req 4 | GC 8% CPU, 37.4 KiB alloc/req, ≤ 319 goroutines |
| 250 | GC 6% CPU, 42.0 KiB alloc/req, ≤ 272 goroutines | ELU 1.00, loop delay p99 ≤ 73.3 ms, GC/1k req 5 | GC 10% CPU, 37.7 KiB alloc/req, ≤ 771 goroutines |
| 500 | GC 6% CPU, 42.0 KiB alloc/req, ≤ 520 goroutines | ELU 1.00, loop delay p99 ≤ 114 ms, GC/1k req 5 | GC 14% CPU, 37.6 KiB alloc/req, ≤ 1,522 goroutines |
| 1000 | GC 6% CPU, 41.8 KiB alloc/req, ≤ 1,020 goroutines | ELU 1.00, loop delay p99 ≤ 187 ms, GC/1k req 4 | GC 16% CPU, 37.5 KiB alloc/req, ≤ 3,021 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1.07× | 0.84× | 0.90× | 1.03× | 1.18× | 1.21× |
| 10 | 0.97× | 1.06× | 1.02× | 1.27× | 1.03× | 1.31× |
| 50 | 0.96× | 1.05× | ≈1.01× | 1.34× | 0.84× | 1.13× |
| 100 | 0.97× | 1.02× | 0.99× | 1.29× | 0.92× | 1.19× |
| 250 | 0.97× | 0.98× | 0.96× | 1.28× | 1.08× | 1.38× |
| 500 | 0.98× | 0.94× | 0.92× | 1.14× | 1.27× | 1.44× |
| 1000 | ≈1.00× | 0.90× | 0.90× | 1.11× | 1.42× | 1.58× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 15,748 | 15,797 | 9,978 |
| sent | 92 | 92 | 89 |

## get-product: postgres mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 3,370 ±12% | 2,661 ±2% | 2,651 ±12% |
| 10 | 17,147 ±14% | 10,709 ±8% | 9,519 ±4% |
| 50 | 13,262 ±17% | 10,040 ±9% | 10,803 ±16% |
| 100 | 16,525 ±2% | 9,416 ±1% | 10,797 ±10% |
| 250 | 16,242 ±5% | 9,092 ±10% | 9,931 ±16% |
| 500 | 14,241 ±11% | 8,286 ±7% | 9,482 ±3% |
| 1000 | 14,344 ±1% | 7,352 ±1% | 8,823 ±7% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.22 / 0.41 / 0.67 | 0.28 / 0.58 / 1.08 | 0.28 / 0.50 / 0.83 |
| 10 | 0.46 / 1.06 / 1.65 | 0.77 / 1.60 / 2.62 | 0.83 / 1.93 / 3.08 |
| 50 | 2.93 / 8.05 / 12.7 | 4.41 / 8.25 / 11.7 | 4.00 / 8.60 / 14.5 |
| 100 | 5.39 / 9.96 / 13.7 | 9.41 / 16.5 / 21.6 | 8.05 / 15.6 / 19.9 |
| 250 | 13.8 / 24.2 / 30.1 | 24.9 / 41.6 / 51.5 | 22.2 / 39.5 / 48.3 |
| 500 | 31.6 / 57.3 / 70.7 | 55.0 / 86.8 / 112 | 47.3 / 80.8 / 96.9 |
| 1000 | 63.1 / 108 / 131 | 133 / 188 / 216 | 106 / 168 / 234 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.38 / 0.114 / 21 | 0.49 / 0.178 / 143 | 0.55 / 0.207 / 37 |
| 10 | 0.98 / 0.057 / 21 | 1.00 / 0.093 / 121 | 0.92 / 0.097 / 35 |
| 50 | 1.00 / 0.075 / 21 | 1.00 / 0.100 / 127 | 0.99 / 0.092 / 36 |
| 100 | 1.00 / 0.060 / 23 | 0.99 / 0.106 / 127 | 1.00 / 0.092 / 41 |
| 250 | 1.00 / 0.061 / 29 | 0.99 / 0.109 / 130 | 1.00 / 0.100 / 54 |
| 500 | 1.00 / 0.070 / 40 | 0.99 / 0.120 / 129 | 1.00 / 0.105 / 70 |
| 1000 | 0.99 / 0.069 / 61 | 1.00 / 0.136 / 144 | 1.00 / 0.113 / 113 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 2% CPU, 4.4 KiB alloc/req, ≤ 21 goroutines | ELU 0.46, loop delay p99 ≤ 3.05 ms, GC/1k req 6 | GC 2% CPU, 7.0 KiB alloc/req, ≤ 25 goroutines |
| 10 | GC 3% CPU, 4.3 KiB alloc/req, ≤ 46 goroutines | ELU 1.00, loop delay p99 ≤ 6.56 ms, GC/1k req 1 | GC 4% CPU, 6.9 KiB alloc/req, ≤ 77 goroutines |
| 50 | GC 4% CPU, 4.5 KiB alloc/req, ≤ 138 goroutines | ELU 1.00, loop delay p99 ≤ 20.1 ms, GC/1k req 1 | GC 3% CPU, 7.0 KiB alloc/req, ≤ 235 goroutines |
| 100 | GC 3% CPU, 4.5 KiB alloc/req, ≤ 235 goroutines | ELU 1.00, loop delay p99 ≤ 9.77 ms, GC/1k req 1 | GC 4% CPU, 7.3 KiB alloc/req, ≤ 456 goroutines |
| 250 | GC 3% CPU, 4.5 KiB alloc/req, ≤ 536 goroutines | ELU 1.00, loop delay p99 ≤ 18.3 ms, GC/1k req 1 | GC 4% CPU, 7.2 KiB alloc/req, ≤ 1,039 goroutines |
| 500 | GC 3% CPU, 4.6 KiB alloc/req, ≤ 1,036 goroutines | ELU 1.00, loop delay p99 ≤ 16.4 ms, GC/1k req 2 | GC 5% CPU, 7.2 KiB alloc/req, ≤ 2,037 goroutines |
| 1000 | GC 3% CPU, 4.5 KiB alloc/req, ≤ 2,036 goroutines | ELU 1.00, loop delay p99 ≤ 27.4 ms, GC/1k req 2 | GC 6% CPU, 7.2 KiB alloc/req, ≤ 4,036 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1.27× | 0.79× | ≈1.00× | 0.62× | 1.24× | 0.77× |
| 10 | 1.60× | 0.56× | ≈0.89× | 0.63× | 1.87× | 1.18× |
| 50 | 1.32× | 0.81× | ≈1.08× | 1.09× | 1.14× | 1.24× |
| 100 | 1.75× | 0.65× | ≈1.15× | 0.63× | 1.45× | 0.92× |
| 250 | 1.79× | 0.61× | ≈1.09× | 0.59× | 1.60× | 0.94× |
| 500 | 1.72× | 0.67× | 1.14× | 0.63× | 1.37× | 0.87× |
| 1000 | 1.95× | 0.62× | 1.20× | 0.61× | 1.78× | 1.08× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 453 | 502 | 295 |
| sent | 82 | 82 | 84 |

## list-products (limit=10): postgres mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 2,760 ±1% | 2,203 ±5% | 2,233 ±4% |
| 10 | 10,874 ±11% | 7,933 ±12% | 7,951 ±10% |
| 50 | 10,279 ±13% | 7,790 ±3% | 8,081 ±4% |
| 100 | 10,568 ±1% | 7,201 ±2% | 7,758 ±2% |
| 250 | 10,378 ±1% | 6,944 ±1% | 7,301 ±3% |
| 500 | 9,986 ±1% | 5,816 ±1% | 6,980 ±2% |
| 1000 | 9,839 ±2% | 5,405 ±5% | 6,776 ±3% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.28 / 0.50 / 0.76 | 0.34 / 0.67 / 1.13 | 0.34 / 0.63 / 1.00 |
| 10 | 0.75 / 1.79 / 2.66 | 1.08 / 2.01 / 3.09 | 0.96 / 2.57 / 3.82 |
| 50 | 4.32 / 8.19 / 11.3 | 5.89 / 9.62 / 13.1 | 5.43 / 10.6 / 14.0 |
| 100 | 8.84 / 14.8 / 19.7 | 12.7 / 20.7 / 26.5 | 11.5 / 20.5 / 26.2 |
| 250 | 22.6 / 36.2 / 43.6 | 33.0 / 52.6 / 64.6 | 33.0 / 51.5 / 61.8 |
| 500 | 46.4 / 73.7 / 82.2 | 81.4 / 117 / 136 | 69.8 / 108 / 124 |
| 1000 | 94.2 / 149 / 169 | 179 / 242 / 276 | 145 / 207 / 244 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.42 / 0.149 / 23 | 0.47 / 0.214 / 145 | 0.54 / 0.240 / 40 |
| 10 | 0.99 / 0.091 / 23 | 1.00 / 0.126 / 125 | 0.99 / 0.125 / 39 |
| 50 | 0.99 / 0.097 / 23 | 1.00 / 0.128 / 130 | 1.00 / 0.123 / 37 |
| 100 | 0.99 / 0.094 / 25 | 1.00 / 0.139 / 134 | 0.99 / 0.128 / 41 |
| 250 | 1.00 / 0.096 / 31 | 1.00 / 0.143 / 134 | 0.99 / 0.136 / 52 |
| 500 | 1.00 / 0.100 / 41 | 1.00 / 0.171 / 140 | 0.99 / 0.143 / 69 |
| 1000 | 1.00 / 0.101 / 64 | 1.00 / 0.184 / 145 | 1.00 / 0.147 / 105 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 5% CPU, 15.1 KiB alloc/req, ≤ 21 goroutines | ELU 0.45, loop delay p99 ≤ 4.18 ms, GC/1k req 4 | GC 4% CPU, 15.8 KiB alloc/req, ≤ 25 goroutines |
| 10 | GC 7% CPU, 15.1 KiB alloc/req, ≤ 53 goroutines | ELU 1.00, loop delay p99 ≤ 13.8 ms, GC/1k req 3 | GC 7% CPU, 15.8 KiB alloc/req, ≤ 70 goroutines |
| 50 | GC 7% CPU, 15.2 KiB alloc/req, ≤ 143 goroutines | ELU 1.00, loop delay p99 ≤ 12.1 ms, GC/1k req 2 | GC 7% CPU, 16.3 KiB alloc/req, ≤ 246 goroutines |
| 100 | GC 8% CPU, 15.2 KiB alloc/req, ≤ 240 goroutines | ELU 1.00, loop delay p99 ≤ 11.6 ms, GC/1k req 3 | GC 7% CPU, 16.2 KiB alloc/req, ≤ 459 goroutines |
| 250 | GC 7% CPU, 15.2 KiB alloc/req, ≤ 538 goroutines | ELU 1.00, loop delay p99 ≤ 10.1 ms, GC/1k req 3 | GC 8% CPU, 16.2 KiB alloc/req, ≤ 1,037 goroutines |
| 500 | GC 7% CPU, 15.3 KiB alloc/req, ≤ 1,037 goroutines | ELU 1.00, loop delay p99 ≤ 31.3 ms, GC/1k req 4 | GC 10% CPU, 16.2 KiB alloc/req, ≤ 2,038 goroutines |
| 1000 | GC 7% CPU, 15.2 KiB alloc/req, ≤ 2,038 goroutines | ELU 1.00, loop delay p99 ≤ 28.3 ms, GC/1k req 5 | GC 11% CPU, 16.1 KiB alloc/req, ≤ 4,037 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1.25× | 0.81× | ≈1.01× | 0.67× | 1.31× | 0.88× |
| 10 | 1.37× | ≈0.73× | ≈1.00× | 0.86× | 1.44× | 1.24× |
| 50 | 1.32× | 0.79× | ≈1.04× | 0.86× | 1.24× | 1.07× |
| 100 | 1.47× | 0.73× | 1.08× | 0.74× | 1.33× | 0.99× |
| 250 | 1.49× | 0.70× | ≈1.05× | 0.67× | 1.42× | 0.96× |
| 500 | 1.72× | 0.70× | 1.20× | 0.60× | 1.51× | 0.91× |
| 1000 | 1.82× | 0.69× | 1.25× | 0.61× | 1.44× | 0.88× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 3,415 | 3,464 | 2,217 |
| sent | 95 | 95 | 85 |

## list-products (limit=50): postgres mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 2,021 ±1% | 1,691 ±2% | 1,731 ±0% |
| 10 | 4,591 ±0% | 4,524 ±1% | 4,438 ±1% |
| 50 | 4,523 ±0% | 4,343 ±1% | 4,362 ±2% |
| 100 | 4,508 ±1% | 4,308 ±1% | 4,215 ±1% |
| 250 | 4,478 ±1% | 3,212 ±1% | 3,998 ±0% |
| 500 | 4,465 ±3% | 3,102 ±2% | 3,856 ±0% |
| 1000 | 4,511 ±0% | 2,952 ±1% | 3,655 ±0% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.40 / 0.69 / 1.04 | 0.49 / 0.81 / 1.31 | 0.43 / 0.90 / 1.32 |
| 10 | 1.85 / 4.18 / 5.35 | 1.99 / 3.32 / 5.02 | 1.87 / 4.48 / 6.05 |
| 50 | 10.7 / 15.9 / 19.9 | 10.5 / 16.3 / 22.4 | 11.2 / 17.4 / 21.6 |
| 100 | 21.3 / 30.0 / 36.7 | 21.8 / 31.7 / 39.4 | 22.9 / 33.2 / 39.9 |
| 250 | 53.5 / 71.7 / 85.8 | 78.1 / 101 / 117 | 59.5 / 85.8 / 103 |
| 500 | 107 / 144 / 166 | 157 / 199 / 228 | 125 / 166 / 197 |
| 1000 | 215 / 272 / 299 | 330 / 419 / 454 | 262 / 354 / 413 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.54 / 0.264 / 22 | 0.52 / 0.309 / 151 | 0.58 / 0.334 / 39 |
| 10 | 1.00 / 0.217 / 23 | 1.00 / 0.220 / 152 | 1.00 / 0.225 / 39 |
| 50 | 1.00 / 0.220 / 23 | 1.00 / 0.229 / 153 | 1.00 / 0.229 / 37 |
| 100 | 0.99 / 0.220 / 25 | 1.00 / 0.232 / 152 | 1.00 / 0.236 / 40 |
| 250 | 0.99 / 0.221 / 31 | 1.00 / 0.311 / 152 | 1.00 / 0.248 / 51 |
| 500 | 0.99 / 0.222 / 43 | 1.00 / 0.322 / 150 | 0.99 / 0.257 / 70 |
| 1000 | 0.99 / 0.221 / 66 | 1.00 / 0.338 / 151 | 0.99 / 0.271 / 100 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 10% CPU, 61.4 KiB alloc/req, ≤ 21 goroutines | ELU 0.50, loop delay p99 ≤ 4.85 ms, GC/1k req 13 | GC 10% CPU, 54.7 KiB alloc/req, ≤ 25 goroutines |
| 10 | GC 11% CPU, 61.3 KiB alloc/req, ≤ 48 goroutines | ELU 1.00, loop delay p99 ≤ 8.46 ms, GC/1k req 8 | GC 12% CPU, 55.0 KiB alloc/req, ≤ 71 goroutines |
| 50 | GC 11% CPU, 61.5 KiB alloc/req, ≤ 137 goroutines | ELU 1.00, loop delay p99 ≤ 24.2 ms, GC/1k req 7 | GC 13% CPU, 55.8 KiB alloc/req, ≤ 258 goroutines |
| 100 | GC 11% CPU, 61.5 KiB alloc/req, ≤ 237 goroutines | ELU 1.00, loop delay p99 ≤ 14.7 ms, GC/1k req 7 | GC 14% CPU, 55.5 KiB alloc/req, ≤ 459 goroutines |
| 250 | GC 12% CPU, 61.4 KiB alloc/req, ≤ 536 goroutines | ELU 1.00, loop delay p99 ≤ 28.3 ms, GC/1k req 12 | GC 16% CPU, 55.3 KiB alloc/req, ≤ 1,038 goroutines |
| 500 | GC 11% CPU, 61.3 KiB alloc/req, ≤ 1,035 goroutines | ELU 1.00, loop delay p99 ≤ 26.6 ms, GC/1k req 12 | GC 17% CPU, 55.1 KiB alloc/req, ≤ 2,037 goroutines |
| 1000 | GC 11% CPU, 61.0 KiB alloc/req, ≤ 2,037 goroutines | ELU 1.00, loop delay p99 ≤ 35.5 ms, GC/1k req 12 | GC 20% CPU, 54.7 KiB alloc/req, ≤ 4,038 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1.20× | 0.86× | 1.02× | 0.79× | 1.27× | 1.00× |
| 10 | 1.01× | 0.97× | 0.98× | 1.07× | 1.13× | 1.21× |
| 50 | 1.04× | 0.96× | ≈1.00× | 0.89× | 1.08× | 0.97× |
| 100 | 1.05× | 0.94× | 0.98× | 0.93× | 1.09× | 1.01× |
| 250 | 1.39× | 0.89× | 1.24× | 0.74× | 1.20× | 0.89× |
| 500 | 1.44× | 0.86× | 1.24× | 0.73× | 1.19× | 0.87× |
| 1000 | 1.53× | 0.81× | 1.24× | 0.66× | 1.38× | 0.91× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 16,543 | 16,592 | 10,783 |
| sent | 96 | 96 | 85 |

## list-products (limit=100): postgres mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 1,491 ±1% | 1,286 ±2% | 1,350 ±1% |
| 10 | 2,791 ±0% | 2,989 ±1% | 2,809 ±1% |
| 50 | 2,789 ±1% | 2,875 ±0% | 2,792 ±1% |
| 100 | 2,772 ±1% | 2,833 ±1% | 2,781 ±1% |
| 250 | 2,815 ±1% | 2,054 ±0% | 2,679 ±0% |
| 500 | 2,834 ±0% | 2,025 ±1% | 2,602 ±1% |
| 1000 | 2,825 ±0% | 1,964 ±2% | 2,464 ±0% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.56 / 0.89 / 1.36 | 0.66 / 1.07 / 1.55 | 0.57 / 1.15 / 1.61 |
| 10 | 3.14 / 6.29 / 8.09 | 3.09 / 4.72 / 6.76 | 3.07 / 6.73 / 8.69 |
| 50 | 17.6 / 25.1 / 30.9 | 16.1 / 23.8 / 32.0 | 17.4 / 26.4 / 33.7 |
| 100 | 35.1 / 46.5 / 54.5 | 33.5 / 47.1 / 54.3 | 35.0 / 48.0 / 56.2 |
| 250 | 86.7 / 109 / 119 | 120 / 148 / 166 | 90.8 / 120 / 142 |
| 500 | 172 / 215 / 229 | 244 / 299 / 329 | 187 / 236 / 261 |
| 1000 | 346 / 414 / 444 | 513 / 578 / 609 | 402 / 484 / 519 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.61 / 0.410 / 22 | 0.57 / 0.437 / 153 | 0.62 / 0.460 / 39 |
| 10 | 0.99 / 0.356 / 23 | 1.00 / 0.334 / 153 | 0.99 / 0.353 / 39 |
| 50 | 1.00 / 0.357 / 23 | 1.00 / 0.348 / 153 | 0.99 / 0.355 / 38 |
| 100 | 1.00 / 0.359 / 26 | 1.00 / 0.351 / 153 | 0.99 / 0.357 / 41 |
| 250 | 1.00 / 0.354 / 33 | 1.00 / 0.484 / 150 | 0.99 / 0.371 / 53 |
| 500 | 1.00 / 0.352 / 44 | 1.00 / 0.490 / 150 | 0.99 / 0.383 / 73 |
| 1000 | 1.00 / 0.351 / 69 | 1.00 / 0.508 / 150 | 1.00 / 0.404 / 103 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 13% CPU, 123.4 KiB alloc/req, ≤ 21 goroutines | ELU 0.55, loop delay p99 ≤ 7.28 ms, GC/1k req 16 | GC 13% CPU, 104.1 KiB alloc/req, ≤ 25 goroutines |
| 10 | GC 12% CPU, 123.4 KiB alloc/req, ≤ 50 goroutines | ELU 1.00, loop delay p99 ≤ 7.02 ms, GC/1k req 15 | GC 14% CPU, 104.9 KiB alloc/req, ≤ 72 goroutines |
| 50 | GC 13% CPU, 123.4 KiB alloc/req, ≤ 145 goroutines | ELU 1.00, loop delay p99 ≤ 23.3 ms, GC/1k req 13 | GC 14% CPU, 105.6 KiB alloc/req, ≤ 259 goroutines |
| 100 | GC 13% CPU, 123.5 KiB alloc/req, ≤ 247 goroutines | ELU 1.00, loop delay p99 ≤ 21.1 ms, GC/1k req 13 | GC 16% CPU, 105.5 KiB alloc/req, ≤ 459 goroutines |
| 250 | GC 13% CPU, 123.2 KiB alloc/req, ≤ 537 goroutines | ELU 1.00, loop delay p99 ≤ 38.3 ms, GC/1k req 20 | GC 19% CPU, 104.5 KiB alloc/req, ≤ 1,037 goroutines |
| 500 | GC 14% CPU, 122.7 KiB alloc/req, ≤ 1,036 goroutines | ELU 1.00, loop delay p99 ≤ 35.2 ms, GC/1k req 20 | GC 20% CPU, 104.1 KiB alloc/req, ≤ 2,037 goroutines |
| 1000 | GC 13% CPU, 121.5 KiB alloc/req, ≤ 2,036 goroutines | ELU 1.00, loop delay p99 ≤ 45.0 ms, GC/1k req 20 | GC 23% CPU, 102.8 KiB alloc/req, ≤ 4,038 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1.16× | 0.91× | 1.05× | 0.87× | 1.19× | 1.04× |
| 10 | 0.93× | 1.01× | 0.94× | 1.20× | 1.07× | 1.29× |
| 50 | 0.97× | ≈1.00× | 0.97× | 0.96× | 1.09× | 1.05× |
| 100 | ≈0.98× | ≈1.00× | 0.98× | 1.00× | 1.03× | 1.04× |
| 250 | 1.37× | 0.95× | 1.30× | 0.72× | 1.19× | 0.85× |
| 500 | 1.40× | 0.92× | 1.29× | 0.70× | 1.14× | 0.79× |
| 1000 | 1.44× | 0.87× | 1.25× | 0.73× | 1.17× | 0.85× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 33,003 | 33,052 | 21,499 |
| sent | 97 | 97 | 100 |

## list-products (limit=500): postgres mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 513 ±1% | 459 ±0% | 453 ±1% |
| 10 | 729 ±1% | 698 ±1% | 778 ±1% |
| 50 | 749 ±0% | 563 ±1% | 772 ±0% |
| 100 | 745 ±1% | 496 ±1% | 761 ±0% |
| 250 | 746 ±1% | 501 ±1% | 736 ±0% |
| 500 | 765 ±0% | 505 ±1% | 708 ±2% |
| 1000 | 779 ±0% | 522 ±0% | 699 ±0% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 1.81 / 2.56 / 3.37 | 2.00 / 2.95 / 3.97 | 1.92 / 3.25 / 4.02 |
| 10 | 13.6 / 21.3 / 25.8 | 13.8 / 19.4 / 26.2 | 12.5 / 20.2 / 24.2 |
| 50 | 65.0 / 94.9 / 125 | 87.2 / 120 / 137 | 63.6 / 88.5 / 102 |
| 100 | 132 / 166 / 198 | 199 / 234 / 253 | 129 / 166 / 181 |
| 250 | 335 / 388 / 432 | 506 / 561 / 575 | 339 / 402 / 422 |
| 500 | 670 / 724 / 774 | 1,033 / 1,082 / 1,197 | 719 / 806 / 853 |
| 1000 | 1,362 / 1,465 / 1,513 | 2,088 / 2,137 / 2,196 | 1,534 / 1,597 / 1,634 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.75 / 1.448 / 23 | 0.67 / 1.471 / 192 | 0.65 / 1.424 / 41 |
| 10 | 1.00 / 1.368 / 24 | 0.99 / 1.425 / 194 | 1.00 / 1.281 / 54 |
| 50 | 1.00 / 1.331 / 25 | 1.00 / 1.775 / 202 | 0.99 / 1.288 / 56 |
| 100 | 0.99 / 1.332 / 28 | 1.00 / 2.006 / 198 | 1.00 / 1.309 / 50 |
| 250 | 0.99 / 1.330 / 36 | 1.00 / 1.995 / 198 | 0.99 / 1.353 / 62 |
| 500 | 1.00 / 1.304 / 48 | 1.00 / 1.975 / 198 | 0.99 / 1.401 / 84 |
| 1000 | 1.00 / 1.277 / 74 | 1.00 / 1.910 / 205 | 1.00 / 1.423 / 128 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 15% CPU, 569.4 KiB alloc/req, ≤ 22 goroutines | ELU 0.67, loop delay p99 ≤ 14.7 ms, GC/1k req 48 | GC 18% CPU, 493.4 KiB alloc/req, ≤ 25 goroutines |
| 10 | GC 13% CPU, 569.0 KiB alloc/req, ≤ 55 goroutines | ELU 1.00, loop delay p99 ≤ 26.9 ms, GC/1k req 43 | GC 11% CPU, 550.9 KiB alloc/req, ≤ 77 goroutines |
| 50 | GC 13% CPU, 568.5 KiB alloc/req, ≤ 135 goroutines | ELU 1.00, loop delay p99 ≤ 70.2 ms, GC/1k req 49 | GC 14% CPU, 528.6 KiB alloc/req, ≤ 257 goroutines |
| 100 | GC 13% CPU, 566.7 KiB alloc/req, ≤ 237 goroutines | ELU 1.00, loop delay p99 ≤ 60.9 ms, GC/1k req 60 | GC 15% CPU, 507.7 KiB alloc/req, ≤ 454 goroutines |
| 250 | GC 13% CPU, 560.8 KiB alloc/req, ≤ 537 goroutines | ELU 1.00, loop delay p99 ≤ 65.0 ms, GC/1k req 59 | GC 19% CPU, 498.8 KiB alloc/req, ≤ 1,036 goroutines |
| 500 | GC 14% CPU, 552.1 KiB alloc/req, ≤ 1,036 goroutines | ELU 1.00, loop delay p99 ≤ 69.7 ms, GC/1k req 57 | GC 21% CPU, 490.0 KiB alloc/req, ≤ 2,033 goroutines |
| 1000 | GC 14% CPU, 534.3 KiB alloc/req, ≤ 2,037 goroutines | ELU 1.00, loop delay p99 ≤ 69.5 ms, GC/1k req 54 | GC 24% CPU, 469.1 KiB alloc/req, ≤ 4,033 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1.12× | 0.88× | ≈0.99× | 0.85× | 1.19× | 1.01× |
| 10 | 1.04× | 1.07× | 1.11× | 0.99× | 0.94× | 0.92× |
| 50 | 1.33× | 1.03× | 1.37× | 0.91× | 0.82× | 0.75× |
| 100 | 1.50× | 1.02× | 1.53× | 0.78× | 0.92× | 0.72× |
| 250 | 1.49× | ≈0.99× | 1.47× | 0.75× | 0.98× | 0.73× |
| 500 | 1.52× | 0.93× | 1.40× | 0.65× | 1.10× | 0.71× |
| 1000 | 1.49× | 0.90× | 1.34× | 0.69× | 1.08× | 0.74× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 164,678 | 164,727 | 107,197 |
| sent | 98 | 98 | 114 |

## search-products (all terms, limit=server default (50)): postgres mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 1,034 ±1% | 951 ±0% | 966 ±0% |
| 10 | 3,852 ±1% | 3,850 ±2% | 3,375 ±2% |
| 50 | 3,824 ±2% | 3,843 ±1% | 3,486 ±1% |
| 100 | 3,870 ±2% | 3,804 ±5% | 3,249 ±9% |
| 250 | 3,756 ±5% | 3,009 ±0% | 3,208 ±1% |
| 500 | 3,667 ±2% | 2,934 ±1% | 3,118 ±2% |
| 1000 | 3,623 ±2% | 2,826 ±1% | 2,949 ±1% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.89 / 1.23 / 1.75 | 0.96 / 1.37 / 1.92 | 0.93 / 1.37 / 1.83 |
| 10 | 2.27 / 4.66 / 6.38 | 2.33 / 4.32 / 5.90 | 2.58 / 5.42 / 7.22 |
| 50 | 12.4 / 19.6 / 24.4 | 12.0 / 20.0 / 25.8 | 13.9 / 21.4 / 26.6 |
| 100 | 24.9 / 35.5 / 42.0 | 24.5 / 38.7 / 46.5 | 29.3 / 44.3 / 52.9 |
| 250 | 64.3 / 87.0 / 96.9 | 82.5 / 114 / 133 | 75.2 / 102 / 118 |
| 500 | 132 / 172 / 187 | 162 / 216 / 245 | 154 / 205 / 233 |
| 1000 | 269 / 338 / 366 | 346 / 435 / 471 | 331 / 423 / 461 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.32 / 0.315 / 22 | 0.33 / 0.346 / 185 | 0.36 / 0.375 / 38 |
| 10 | 0.95 / 0.248 / 24 | 0.98 / 0.254 / 137 | 0.96 / 0.285 / 38 |
| 50 | 0.97 / 0.253 / 24 | 0.99 / 0.258 / 143 | 0.98 / 0.281 / 37 |
| 100 | 0.97 / 0.253 / 25 | 0.99 / 0.260 / 143 | 0.98 / 0.301 / 41 |
| 250 | 0.98 / 0.260 / 32 | 1.00 / 0.331 / 149 | 0.98 / 0.304 / 53 |
| 500 | 0.97 / 0.263 / 44 | 1.00 / 0.339 / 152 | 0.98 / 0.314 / 72 |
| 1000 | 0.96 / 0.265 / 68 | 1.00 / 0.352 / 155 | 0.98 / 0.331 / 102 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 11% CPU, 71.0 KiB alloc/req, ≤ 21 goroutines | ELU 0.32, loop delay p99 ≤ 12.9 ms, GC/1k req 16 | GC 10% CPU, 66.2 KiB alloc/req, ≤ 25 goroutines |
| 10 | GC 11% CPU, 70.9 KiB alloc/req, ≤ 50 goroutines | ELU 0.97, loop delay p99 ≤ 3.97 ms, GC/1k req 8 | GC 12% CPU, 66.5 KiB alloc/req, ≤ 74 goroutines |
| 50 | GC 12% CPU, 71.1 KiB alloc/req, ≤ 156 goroutines | ELU 0.99, loop delay p99 ≤ 20.5 ms, GC/1k req 7 | GC 14% CPU, 66.9 KiB alloc/req, ≤ 257 goroutines |
| 100 | GC 12% CPU, 71.0 KiB alloc/req, ≤ 238 goroutines | ELU 0.99, loop delay p99 ≤ 20.2 ms, GC/1k req 7 | GC 14% CPU, 66.7 KiB alloc/req, ≤ 457 goroutines |
| 250 | GC 12% CPU, 70.9 KiB alloc/req, ≤ 538 goroutines | ELU 1.00, loop delay p99 ≤ 24.0 ms, GC/1k req 12 | GC 16% CPU, 66.6 KiB alloc/req, ≤ 1,039 goroutines |
| 500 | GC 12% CPU, 70.8 KiB alloc/req, ≤ 1,037 goroutines | ELU 1.00, loop delay p99 ≤ 31.2 ms, GC/1k req 12 | GC 17% CPU, 66.4 KiB alloc/req, ≤ 2,037 goroutines |
| 1000 | GC 12% CPU, 70.4 KiB alloc/req, ≤ 2,038 goroutines | ELU 1.00, loop delay p99 ≤ 32.0 ms, GC/1k req 12 | GC 20% CPU, 65.9 KiB alloc/req, ≤ 4,040 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1.09× | 0.93× | 1.01× | 0.91× | 1.05× | 0.96× |
| 10 | ≈1.00× | 0.88× | 0.88× | 1.08× | 1.13× | 1.22× |
| 50 | ≈0.99× | 0.91× | 0.91× | 0.95× | 1.09× | 1.03× |
| 100 | ≈1.02× | 0.84× | 0.85× | 0.90× | 1.26× | 1.14× |
| 250 | 1.25× | 0.85× | 1.07× | 0.73× | 1.21× | 0.88× |
| 500 | 1.25× | 0.85× | 1.06× | 0.76× | 1.24× | 0.95× |
| 1000 | 1.28× | 0.81× | 1.04× | 0.78× | 1.26× | 0.98× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 15,748 | 15,797 | 9,978 |
| sent | 92 | 92 | 89 |
