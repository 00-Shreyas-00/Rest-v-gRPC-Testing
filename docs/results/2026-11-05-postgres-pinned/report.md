# Benchmark report

Generated 2026-11-05T15:27:05.612Z from results/final-20261105.

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
| k6 CPU set | 6-11 |
| Server memory limit | 1024MiB |
| PostgreSQL CPU limit | 3 pinned 3-5 |
| k6 CPU limit | 6 |
| DB pool per server | 20 |
| Data source | postgres |
| Products / dataset checksum | 100000 / d26bc7a601e76ff91ba94bc08ebebc68 |
| Warm-up / measured / cool-down | 10s / 20s / 3 s |
| Repeats per point | 3 |
| Network | k6 container -> server container on the compose bridge network (no published ports) |
| TLS | false |

Runs: 297. Runs where k6 was CPU-bound: 0. Total errors: 4759.

Legend: ⚠k6 = k6 was CPU-bound in at least one repeat (client limit, not a server result); ⚠Nerr / ⚠Nt/o = errors / timeouts summed over repeats.

## get-product: postgres mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 1,327 ±12% | 853 ±7% | 1,075 ±14% |
| 10 | 9,060 ±17% | 5,438 ±7% | 5,291 ±8% |
| 50 | 9,802 ±7% | 5,546 ±2% | 6,264 ±12% |
| 100 | 8,854 ±9% | 5,045 ±11% | 6,032 ±10% |
| 250 | 9,367 ±9% | 4,597 ±2% | 5,884 ±2% |
| 500 | 8,748 ±1% | 4,517 ±4% | 5,427 ±2% |
| 1000 | 8,379 ±2% | 3,722 ±4% | 5,434 ±1% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.51 / 1.29 / 1.86 | 0.85 / 1.93 / 2.78 | 0.61 / 1.56 / 2.25 |
| 10 | 0.85 / 2.02 / 3.28 | 1.52 / 3.18 / 4.78 | 1.57 / 3.17 / 4.54 |
| 50 | 4.46 / 8.92 / 13.0 | 8.18 / 13.5 / 16.8 | 7.24 / 13.7 / 17.6 |
| 100 | 9.81 / 19.2 / 23.4 | 18.1 / 30.1 / 37.1 | 15.1 / 27.1 / 34.6 |
| 250 | 24.8 / 40.9 / 55.4 | 52.4 / 73.7 / 103 | 40.3 / 60.7 / 71.0 |
| 500 | 53.8 / 80.4 / 108 | 109 / 137 / 151 | 87.6 / 137 / 159 |
| 1000 | 117 / 156 / 193 | 262 / 355 / 424 | 170 / 271 / 297 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.41 / 0.311 / 21 | 0.50 / 0.585 / 145 | 0.56 / 0.524 / 39 |
| 10 | 0.97 / 0.107 / 21 | 1.00 / 0.184 / 123 | 0.92 / 0.175 / 36 |
| 50 | 0.98 / 0.100 / 21 | 1.00 / 0.179 / 127 | 0.99 / 0.159 / 36 |
| 100 | 0.98 / 0.111 / 23 | 1.00 / 0.198 / 130 | 0.99 / 0.165 / 41 |
| 250 | 0.98 / 0.105 / 30 | 1.00 / 0.217 / 130 | 1.00 / 0.169 / 52 |
| 500 | 0.98 / 0.112 / 40 | 1.00 / 0.221 / 131 | 0.99 / 0.183 / 75 |
| 1000 | 0.98 / 0.118 / 61 | 0.99 / 0.266 / 145 | 0.99 / 0.183 / 106 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 1% CPU, 4.4 KiB alloc/req, ≤ 21 goroutines | ELU 0.47, loop delay p99 ≤ 23.8 ms, GC/1k req 9 | GC 2% CPU, 6.9 KiB alloc/req, ≤ 25 goroutines |
| 10 | GC 3% CPU, 4.4 KiB alloc/req, ≤ 47 goroutines | ELU 1.00, loop delay p99 ≤ 9.10 ms, GC/1k req 2 | GC 3% CPU, 7.0 KiB alloc/req, ≤ 67 goroutines |
| 50 | GC 4% CPU, 4.5 KiB alloc/req, ≤ 137 goroutines | ELU 1.00, loop delay p99 ≤ 11.7 ms, GC/1k req 1 | GC 3% CPU, 7.0 KiB alloc/req, ≤ 237 goroutines |
| 100 | GC 3% CPU, 4.5 KiB alloc/req, ≤ 233 goroutines | ELU 1.00, loop delay p99 ≤ 16.4 ms, GC/1k req 1 | GC 4% CPU, 7.3 KiB alloc/req, ≤ 433 goroutines |
| 250 | GC 3% CPU, 4.5 KiB alloc/req, ≤ 537 goroutines | ELU 1.00, loop delay p99 ≤ 18.6 ms, GC/1k req 1 | GC 5% CPU, 7.3 KiB alloc/req, ≤ 1,040 goroutines |
| 500 | GC 3% CPU, 4.5 KiB alloc/req, ≤ 1,035 goroutines | ELU 1.00, loop delay p99 ≤ 18.2 ms, GC/1k req 2 | GC 5% CPU, 7.2 KiB alloc/req, ≤ 2,030 goroutines |
| 1000 | GC 3% CPU, 4.5 KiB alloc/req, ≤ 2,035 goroutines | ELU 1.00, loop delay p99 ≤ 53.2 ms, GC/1k req 2 | GC 6% CPU, 7.4 KiB alloc/req, ≤ 4,033 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1.56× | 0.81× | ≈1.26× | 0.67× | 1.21× | 0.81× |
| 10 | 1.67× | 0.58× | ≈0.97× | 0.69× | 1.39× | 0.95× |
| 50 | 1.77× | 0.64× | ≈1.13× | 0.77× | 1.35× | 1.04× |
| 100 | 1.75× | 0.68× | ≈1.20× | 0.63× | 1.48× | 0.93× |
| 250 | 2.04× | 0.63× | 1.28× | 0.54× | 1.28× | 0.69× |
| 500 | 1.94× | 0.62× | 1.20× | 0.71× | 1.47× | 1.05× |
| 1000 | 2.25× | 0.65× | 1.46× | 0.45× | 1.54× | 0.70× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 453 | 502 | 295 |
| sent | 82 | 82 | 85 |

## list-products (limit=10): postgres mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 936 ±54% | 638 ±46% | 1,024 ±44% |
| 10 | 5,321 ±49% | 4,083 ±50% | 4,304 ±49% |
| 50 | 5,651 ±46% | 3,216 ±56% | 3,563 ±55% |
| 100 | 915 ±201% | 276 ±458% | 440 ±319% |
| 250 | 673 ±281% | 378 ±298% | 2,671 ±81% |
| 500 | 4,053 ±91% | 2,968 ±63% | 3,809 ±78% |
| 1000 | 5,350 ±84% | 2,735 ±91% ⚠1472err ⚠1472t/o | 3,699 ±81% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.73 / 1.94 / 2.65 | 1.16 / 2.73 / 3.75 | 0.65 / 1.67 / 2.39 |
| 10 | 1.45 / 3.87 / 6.05 | 2.00 / 4.41 / 6.80 | 1.78 / 4.83 / 7.05 |
| 50 | 7.74 / 17.4 / 27.6 | 14.2 / 25.9 / 35.6 | 12.7 / 24.6 / 32.1 |
| 100 | 25.7 / 351 / 391 | 366 / 499 / 777 | 216 / 354 / 415 |
| 250 | 372 / 525 / 586 | 622 / 903 / 1,213 | 87.8 / 146 / 190 |
| 500 | 116 / 183 / 244 | 165 / 225 / 269 | 131 / 173 / 196 |
| 1000 | 177 / 256 / 308 | 352 / 514 / 638 | 267 / 377 / 448 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.43 / 0.454 / 22 | 0.49 / 0.751 / 146 | 0.55 / 0.531 / 39 |
| 10 | 0.98 / 0.185 / 23 | 0.99 / 0.243 / 126 | 0.99 / 0.230 / 38 |
| 50 | 0.98 / 0.174 / 23 | 0.99 / 0.309 / 125 | 0.99 / 0.279 / 37 |
| 100 | 0.97 / 0.963 / 24 | 0.99 / 3.591 / 128 | 0.98 / 2.218 / 40 |
| 250 | 0.98 / 1.416 / 31 | 0.99 / 2.562 / 131 | 0.99 / 0.371 / 52 |
| 500 | 0.98 / 0.241 / 42 | 1.00 / 0.336 / 141 | 0.99 / 0.261 / 72 |
| 1000 | 0.99 / 0.184 / 64 | 1.00 / 0.365 / 147 | 0.99 / 0.269 / 116 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 4% CPU, 15.0 KiB alloc/req, ≤ 20 goroutines | ELU 0.46, loop delay p99 ≤ 7.98 ms, GC/1k req 17 | GC 3% CPU, 15.8 KiB alloc/req, ≤ 25 goroutines |
| 10 | GC 6% CPU, 15.0 KiB alloc/req, ≤ 48 goroutines | ELU 1.00, loop delay p99 ≤ 47.0 ms, GC/1k req 4 | GC 6% CPU, 15.9 KiB alloc/req, ≤ 76 goroutines |
| 50 | GC 7% CPU, 15.2 KiB alloc/req, ≤ 136 goroutines | ELU 1.00, loop delay p99 ≤ 369 ms, GC/1k req 3 | GC 6% CPU, 16.2 KiB alloc/req, ≤ 235 goroutines |
| 100 | GC 5% CPU, 15.5 KiB alloc/req, ≤ 237 goroutines | ELU 1.00, loop delay p99 ≤ 360 ms, GC/1k req 4 | GC 6% CPU, 16.4 KiB alloc/req, ≤ 437 goroutines |
| 250 | GC 6% CPU, 15.1 KiB alloc/req, ≤ 536 goroutines | ELU 1.00, loop delay p99 ≤ 298 ms, GC/1k req 3 | GC 8% CPU, 16.1 KiB alloc/req, ≤ 1,036 goroutines |
| 500 | GC 6% CPU, 15.1 KiB alloc/req, ≤ 1,038 goroutines | ELU 1.00, loop delay p99 ≤ 196 ms, GC/1k req 4 | GC 10% CPU, 16.1 KiB alloc/req, ≤ 2,040 goroutines |
| 1000 | GC 7% CPU, 15.2 KiB alloc/req, ≤ 2,038 goroutines | ELU 1.00, loop delay p99 ≤ 138 ms, GC/1k req 4 | GC 11% CPU, 16.3 KiB alloc/req, ≤ 4,038 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | ≈1.47× | ≈1.09× | ≈1.61× | 0.71× | 0.90× | 0.64× |
| 10 | ≈1.30× | ≈0.81× | ≈1.05× | 0.89× | 1.16× | 1.04× |
| 50 | ≈1.76× | ≈0.63× | ≈1.11× | 0.77× | 1.16× | 0.90× |
| 100 | ≈3.31× | ≈0.48× | ≈1.59× | 0.50× | 1.06× | 0.53× |
| 250 | ≈1.78× | ≈3.97× | ≈7.07× | 0.48× | 0.33× | 0.16× |
| 500 | ≈1.37× | ≈0.94× | ≈1.28× | 0.91× | 0.80× | 0.73× |
| 1000 | ≈1.96× | ≈0.69× | ≈1.35× | 0.48× | 1.45× | 0.70× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 3,415 | 3,464 | 2,217 |
| sent | 95 | 95 | 87 |

## list-products (limit=50): postgres mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 1,998 ±28% | 1,668 ±32% | 1,722 ±25% |
| 10 | 4,351 ±23% | 2,444 ±46% | 2,591 ±41% |
| 50 | 2,548 ±32% | 2,543 ±39% | 2,663 ±32% |
| 100 | 2,577 ±39% | 2,350 ±49% | 2,482 ±36% |
| 250 | 2,578 ±40% | 1,759 ±36% | 2,414 ±25% |
| 500 | 2,522 ±38% | 1,712 ±43% | 2,281 ±34% |
| 1000 | 2,601 ±38% | 1,667 ±42% | 2,129 ±37% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.40 / 0.71 / 1.10 | 0.50 / 0.83 / 1.27 | 0.44 / 0.90 / 1.33 |
| 10 | 1.90 / 4.51 / 6.71 | 3.61 / 6.44 / 9.87 | 3.26 / 7.38 / 9.73 |
| 50 | 18.9 / 28.3 / 35.9 | 18.3 / 27.8 / 34.5 | 18.1 / 28.6 / 34.0 |
| 100 | 37.6 / 52.1 / 59.7 | 39.6 / 58.4 / 75.1 | 38.7 / 55.1 / 65.0 |
| 250 | 95.2 / 119 / 130 | 141 / 189 / 230 | 102 / 132 / 147 |
| 500 | 195 / 244 / 300 | 287 / 361 / 501 | 217 / 268 / 293 |
| 1000 | 383 / 444 / 491 | 599 / 742 / 821 | 463 / 591 / 628 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.53 / 0.268 / 22 | 0.51 / 0.318 / 152 | 0.59 / 0.334 / 39 |
| 10 | 0.99 / 0.229 / 23 | 1.00 / 0.405 / 153 | 0.99 / 0.383 / 38 |
| 50 | 1.00 / 0.391 / 23 | 1.00 / 0.393 / 152 | 0.99 / 0.369 / 36 |
| 100 | 1.00 / 0.386 / 25 | 1.00 / 0.422 / 152 | 0.99 / 0.400 / 39 |
| 250 | 1.00 / 0.386 / 32 | 0.99 / 0.563 / 147 | 0.99 / 0.411 / 51 |
| 500 | 1.00 / 0.395 / 43 | 0.99 / 0.578 / 147 | 0.99 / 0.434 / 70 |
| 1000 | 1.00 / 0.383 / 67 | 0.99 / 0.594 / 150 | 0.99 / 0.465 / 116 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 11% CPU, 61.2 KiB alloc/req, ≤ 21 goroutines | ELU 0.50, loop delay p99 ≤ 6.43 ms, GC/1k req 8 | GC 9% CPU, 54.7 KiB alloc/req, ≤ 25 goroutines |
| 10 | GC 10% CPU, 61.2 KiB alloc/req, ≤ 51 goroutines | ELU 1.00, loop delay p99 ≤ 13.3 ms, GC/1k req 8 | GC 11% CPU, 54.9 KiB alloc/req, ≤ 70 goroutines |
| 50 | GC 10% CPU, 61.4 KiB alloc/req, ≤ 135 goroutines | ELU 1.00, loop delay p99 ≤ 33.7 ms, GC/1k req 7 | GC 12% CPU, 55.7 KiB alloc/req, ≤ 239 goroutines |
| 100 | GC 10% CPU, 61.4 KiB alloc/req, ≤ 258 goroutines | ELU 1.00, loop delay p99 ≤ 36.7 ms, GC/1k req 7 | GC 14% CPU, 55.5 KiB alloc/req, ≤ 453 goroutines |
| 250 | GC 11% CPU, 61.3 KiB alloc/req, ≤ 537 goroutines | ELU 1.00, loop delay p99 ≤ 46.3 ms, GC/1k req 12 | GC 15% CPU, 55.0 KiB alloc/req, ≤ 1,037 goroutines |
| 500 | GC 11% CPU, 60.9 KiB alloc/req, ≤ 1,038 goroutines | ELU 1.00, loop delay p99 ≤ 52.5 ms, GC/1k req 12 | GC 17% CPU, 54.9 KiB alloc/req, ≤ 2,039 goroutines |
| 1000 | GC 10% CPU, 60.6 KiB alloc/req, ≤ 2,038 goroutines | ELU 1.00, loop delay p99 ≤ 62.5 ms, GC/1k req 11 | GC 18% CPU, 54.5 KiB alloc/req, ≤ 4,038 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | ≈1.20× | ≈0.86× | ≈1.03× | 0.87× | 1.21× | 1.05× |
| 10 | ≈1.78× | ≈0.60× | ≈1.06× | 0.68× | 1.45× | 0.99× |
| 50 | ≈1.00× | ≈1.05× | ≈1.05× | 1.04× | 0.95× | 0.99× |
| 100 | ≈1.10× | ≈0.96× | ≈1.06× | 0.80× | 1.09× | 0.87× |
| 250 | ≈1.47× | ≈0.94× | ≈1.37× | 0.56× | 1.14× | 0.64× |
| 500 | ≈1.47× | ≈0.90× | ≈1.33× | 0.60× | 0.98× | 0.58× |
| 1000 | ≈1.56× | ≈0.82× | ≈1.28× | 0.60× | 1.28× | 0.77× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 16,543 | 16,592 | 10,783 |
| sent | 96 | 96 | 86 |

## list-products (limit=100): postgres mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 607 ±13% | 473 ±3% | 703 ±6% |
| 10 | 1,197 ±28% | 1,580 ±3% | 1,569 ±3% |
| 50 | 1,438 ±4% | 1,580 ±6% | 1,665 ±1% |
| 100 | 1,481 ±3% | 1,524 ±2% | 1,638 ±2% |
| 250 | 1,479 ±2% | 1,119 ±2% | 1,500 ±0% |
| 500 | 1,495 ±0% | 1,088 ±0% | 1,813 ±19% |
| 1000 | 2,159 ±28% | 1,525 ±28% | 1,467 ±42% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 1.41 / 2.44 / 3.52 | 1.87 / 3.00 / 4.18 | 1.07 / 2.41 / 3.48 |
| 10 | 7.43 / 15.2 / 20.2 | 5.70 / 9.92 / 15.1 | 5.63 / 11.7 / 15.0 |
| 50 | 33.2 / 53.3 / 71.0 | 28.7 / 48.8 / 60.1 | 29.1 / 44.5 / 52.5 |
| 100 | 64.0 / 96.9 / 118 | 60.8 / 92.6 / 116 | 59.3 / 83.5 / 95.9 |
| 250 | 162 / 221 / 268 | 217 / 292 / 348 | 164 / 211 / 235 |
| 500 | 329 / 405 / 512 | 453 / 585 / 626 | 283 / 361 / 398 |
| 1000 | 502 / 591 / 649 | 732 / 820 / 846 | 692 / 792 / 831 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.59 / 0.970 / 22 | 0.58 / 1.188 / 153 | 0.62 / 0.882 / 38 |
| 10 | 0.99 / 0.823 / 23 | 1.00 / 0.631 / 134 | 1.00 / 0.635 / 39 |
| 50 | 0.99 / 0.685 / 23 | 1.00 / 0.630 / 140 | 1.00 / 0.598 / 38 |
| 100 | 0.99 / 0.672 / 26 | 1.00 / 0.653 / 142 | 1.00 / 0.607 / 41 |
| 250 | 0.99 / 0.669 / 32 | 0.99 / 0.888 / 149 | 0.99 / 0.662 / 52 |
| 500 | 0.99 / 0.660 / 44 | 0.99 / 0.914 / 149 | 0.99 / 0.570 / 69 |
| 1000 | 0.99 / 0.498 / 68 | 0.99 / 0.706 / 153 | 0.99 / 0.678 / 122 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 12% CPU, 123.2 KiB alloc/req, ≤ 21 goroutines | ELU 0.56, loop delay p99 ≤ 7.00 ms, GC/1k req 44 | GC 11% CPU, 103.8 KiB alloc/req, ≤ 25 goroutines |
| 10 | GC 11% CPU, 122.7 KiB alloc/req, ≤ 51 goroutines | ELU 1.00, loop delay p99 ≤ 14.8 ms, GC/1k req 15 | GC 12% CPU, 104.9 KiB alloc/req, ≤ 74 goroutines |
| 50 | GC 12% CPU, 123.0 KiB alloc/req, ≤ 143 goroutines | ELU 1.00, loop delay p99 ≤ 38.1 ms, GC/1k req 13 | GC 13% CPU, 105.7 KiB alloc/req, ≤ 239 goroutines |
| 100 | GC 12% CPU, 122.9 KiB alloc/req, ≤ 237 goroutines | ELU 1.00, loop delay p99 ≤ 35.4 ms, GC/1k req 13 | GC 15% CPU, 105.4 KiB alloc/req, ≤ 438 goroutines |
| 250 | GC 12% CPU, 122.7 KiB alloc/req, ≤ 536 goroutines | ELU 1.00, loop delay p99 ≤ 80.4 ms, GC/1k req 20 | GC 17% CPU, 104.6 KiB alloc/req, ≤ 1,034 goroutines |
| 500 | GC 12% CPU, 121.5 KiB alloc/req, ≤ 1,034 goroutines | ELU 1.00, loop delay p99 ≤ 45.8 ms, GC/1k req 20 | GC 20% CPU, 103.6 KiB alloc/req, ≤ 2,040 goroutines |
| 1000 | GC 13% CPU, 120.6 KiB alloc/req, ≤ 2,035 goroutines | ELU 1.00, loop delay p99 ≤ 47.2 ms, GC/1k req 19 | GC 21% CPU, 103.0 KiB alloc/req, ≤ 4,032 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 1.28× | ≈1.16× | 1.49× | 0.84× | 0.99× | 0.83× |
| 10 | ≈0.76× | ≈1.31× | ≈0.99× | 1.34× | 0.74× | 0.99× |
| 50 | ≈0.91× | 1.16× | ≈1.05× | 1.18× | 0.74× | 0.87× |
| 100 | 0.97× | 1.11× | 1.07× | 1.02× | 0.81× | 0.83× |
| 250 | 1.32× | ≈1.01× | 1.34× | 0.77× | 0.88× | 0.67× |
| 500 | 1.37× | ≈1.21× | 1.67× | 0.82× | 0.78× | 0.64× |
| 1000 | ≈1.42× | ≈0.68× | ≈0.96× | 0.77× | 1.28× | 0.98× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 33,003 | 33,052 | 21,499 |
| sent | 97 | 97 | 100 |

## list-products (limit=500): postgres mode, 1 CPU per server (pinned)

**Throughput** (requests/s, median of repeats, ± half the min–max range):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 324 ±34% | 312 ±38% | 327 ±33% |
| 10 | 536 ±33% | 517 ±32% | 582 ±29% |
| 50 | 539 ±35% | 294 ±47% | 447 ±39% |
| 100 | 387 ±46% | 274 ±40% | 400 ±1% |
| 250 | 373 ±2% | 267 ±2% | 404 ±5% |
| 500 | 379 ±2% | 272 ±9% ⚠439err ⚠439t/o | 391 ±8% |
| 1000 | 390 ±2% | 279 ±24% ⚠2848err ⚠2848t/o | 389 ±28% |

**Latency** (ms, median of repeats: p50 / p95 / p99):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 3.10 / 5.49 / 7.29 | 3.34 / 5.46 / 7.55 | 2.94 / 5.26 / 6.92 |
| 10 | 20.3 / 34.4 / 43.0 | 20.5 / 31.2 / 45.0 | 18.3 / 29.6 / 34.9 |
| 50 | 105 / 145 / 169 | 167 / 232 / 267 | 110 / 151 / 175 |
| 100 | 258 / 328 / 359 | 368 / 426 / 516 | 243 / 343 / 379 |
| 250 | 689 / 815 / 905 | 979 / 1,122 / 1,212 | 613 / 819 / 877 |
| 500 | 1,390 / 1,529 / 1,631 | 1,968 / 2,805 / 8,694 | 1,338 / 1,519 / 1,551 |
| 1000 | 2,880 / 3,104 / 3,144 | 2,221 / 9,974 / 10,000 | 2,815 / 3,503 / 3,589 |

**Server resources** (CPU cores used / CPU ms per request / peak RSS MB):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | 0.72 / 2.509 / 24 | 0.67 / 2.518 / 179 | 0.64 / 2.216 / 41 |
| 10 | 0.99 / 2.079 / 25 | 0.99 / 2.140 / 180 | 0.99 / 1.868 / 54 |
| 50 | 1.00 / 2.109 / 26 | 0.99 / 3.375 / 185 | 0.99 / 2.222 / 79 |
| 100 | 1.00 / 2.573 / 28 | 0.99 / 3.613 / 185 | 0.99 / 2.480 / 50 |
| 250 | 0.99 / 2.672 / 36 | 0.99 / 3.732 / 179 | 0.99 / 2.455 / 62 |
| 500 | 0.99 / 2.602 / 50 | 0.99 / 3.646 / 179 | 0.99 / 2.534 / 85 |
| 1000 | 0.99 / 2.543 / 77 | 0.99 / 3.550 / 187 | 0.99 / 2.546 / 150 |

**Runtime detail** (Go: GC share of CPU, heap allocation per request, goroutines; Node: event-loop utilization and delay, GCs per 1000 requests):

| VUs | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| 1 | GC 15% CPU, 568.2 KiB alloc/req, ≤ 21 goroutines | ELU 0.67, loop delay p99 ≤ 14.5 ms, GC/1k req 59 | GC 17% CPU, 492.4 KiB alloc/req, ≤ 22 goroutines |
| 10 | GC 13% CPU, 567.6 KiB alloc/req, ≤ 47 goroutines | ELU 1.00, loop delay p99 ≤ 36.1 ms, GC/1k req 43 | GC 11% CPU, 546.9 KiB alloc/req, ≤ 71 goroutines |
| 50 | GC 13% CPU, 567.1 KiB alloc/req, ≤ 156 goroutines | ELU 1.00, loop delay p99 ≤ 155 ms, GC/1k req 55 | GC 13% CPU, 514.1 KiB alloc/req, ≤ 243 goroutines |
| 100 | GC 13% CPU, 563.4 KiB alloc/req, ≤ 240 goroutines | ELU 1.00, loop delay p99 ≤ 163 ms, GC/1k req 64 | GC 15% CPU, 505.4 KiB alloc/req, ≤ 456 goroutines |
| 250 | GC 13% CPU, 551.7 KiB alloc/req, ≤ 537 goroutines | ELU 1.00, loop delay p99 ≤ 163 ms, GC/1k req 63 | GC 17% CPU, 492.1 KiB alloc/req, ≤ 1,036 goroutines |
| 500 | GC 13% CPU, 532.9 KiB alloc/req, ≤ 1,036 goroutines | ELU 1.00, loop delay p99 ≤ 177 ms, GC/1k req 60 | GC 19% CPU, 474.7 KiB alloc/req, ≤ 2,035 goroutines |
| 1000 | GC 13% CPU, 498.9 KiB alloc/req, ≤ 2,073 goroutines | ELU 1.00, loop delay p99 ≤ 185 ms, GC/1k req 58 | GC 21% CPU, 445.1 KiB alloc/req, ≤ 4,034 goroutines |

**Experiments** (throughput ratio, and p99 latency ratio; >1× means the first is faster in throughput / slower in latency; ≈ marks ratios whose min–max ranges overlap, i.e. within run-to-run noise):

| VUs | A: Go REST ÷ TS REST req/s | B: Go gRPC ÷ Go REST req/s | C: Go gRPC ÷ TS REST req/s | A p99 | B p99 | C p99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | ≈1.04× | ≈1.01× | ≈1.05× | 0.97× | 0.95× | 0.92× |
| 10 | ≈1.04× | ≈1.09× | ≈1.13× | 0.96× | 0.81× | 0.78× |
| 50 | ≈1.83× | ≈0.83× | ≈1.52× | 0.63× | 1.03× | 0.65× |
| 100 | ≈1.41× | ≈1.03× | ≈1.46× | 0.70× | 1.06× | 0.73× |
| 250 | 1.40× | ≈1.09× | 1.51× | 0.75× | 0.97× | 0.72× |
| 500 | 1.40× | ≈1.03× | 1.44× | 0.19× | 0.95× | 0.18× |
| 1000 | ≈1.40× | ≈1.00× | ≈1.39× | 0.31× | 1.14× | 0.36× |

**Bytes on the wire per request** (median; includes HTTP/1.1 or HTTP/2 framing and headers):

|  | Go REST | TS REST | Go gRPC |
| --- | ---: | ---: | ---: |
| received | 164,678 | 164,727 | 107,197 |
| sent | 98 | 98 | 116 |
