#!/usr/bin/env bash
# Runs a k6 workload against one or more servers at several concurrency levels.
# k6 runs in a container on the compose network (no port-forwarding proxy).
#
#   scripts/bench.sh WORKLOAD [options]
#
# WORKLOAD: get-product | list-products | search-products
#
# Options (defaults in brackets):
#   --targets "go-rest ts-rest go-grpc"   servers to test, in this order
#   --vus "1 10 50 100 250 500 1000"      concurrency levels
#   --repeat 1                            runs per (target, level); report.sh takes the median
#   --warmup 15s                          warm-up per run (not reported)
#   --duration 30s                        measured phase per run
#   --cooldown 5                          seconds of idle between runs
#   --limit N                             list: page size [50]; search: result limit [server default]
#   --term WORD                           search: pin one term [rotate all five]
#   --label NAME                          added to the default results directory name
#   --dir PATH                            results directory [results/<timestamp>-<workload>[-<label>]]
#
# Order: repeat -> concurrency level -> target, so repeats are spread over the
# whole session and every target sees the same drift. Each run writes
#   <dir>/<target>-vus<N>-r<R>.json
# plus results.jsonl (one line per run) and environment.json (hardware,
# versions, limits, dataset checksum). Summarise with scripts/report.sh.
#
# The servers must already be running (docker compose up -d --wait); the data
# source and CPU limit are read from the running containers and recorded.
set -euo pipefail
export MSYS_NO_PATHCONV=1 # Git Bash: do not rewrite container paths

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

usage() { sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'; exit 1; }

WORKLOAD="${1:-}"; shift || true
case "$WORKLOAD" in get-product|list-products|search-products) ;; *) usage ;; esac

TARGETS="go-rest ts-rest go-grpc"
VUS_LEVELS="1 10 50 100 250 500 1000"
REPEAT=1 WARMUP=15s DURATION=30s COOLDOWN=5 LIMIT="" TERM_ARG="" LABEL="" OUT_DIR=""
while (($# > 0)); do
  case "$1" in
    --targets)  TARGETS=$2; shift 2 ;;
    --vus)      VUS_LEVELS=$2; shift 2 ;;
    --repeat)   REPEAT=$2; shift 2 ;;
    --dir)      OUT_DIR=$2; shift 2 ;;
    --warmup)   WARMUP=$2; shift 2 ;;
    --duration) DURATION=$2; shift 2 ;;
    --cooldown) COOLDOWN=$2; shift 2 ;;
    --limit)    LIMIT=$2; shift 2 ;;
    --term)     TERM_ARG=$2; shift 2 ;;
    --label)    LABEL=$2; shift 2 ;;
    *) usage ;;
  esac
done

log() { printf '==> %s\n' "$*" >&2; }

container_env() { # service, variable
  docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$(docker compose ps -q "$1")" |
    sed -n "s/^$2=//p"
}

# cpu_limit CONTAINER_ID -> "<cpus> <mode> <cpuset>"
#   quota:     --cpus (CFS quota; the container is throttled when it uses it up)
#   pinned:    --cpuset-cpus without quota (docker-compose.pinned.yml); cpus = number of listed CPUs
#   unlimited: neither
cpuset_count() { # "2,4-6" -> 4
  printf '%s' "$1" | tr ',' '\n' | awk -F- 'NF == 2 { n += $2 - $1 + 1; next } NF == 1 && $1 != "" { n++ } END { print n + 0 }'
}
cpu_limit() {
  local nano cpuset
  nano=$(docker inspect -f '{{.HostConfig.NanoCpus}}' "$1")
  cpuset=$(docker inspect -f '{{.HostConfig.CpusetCpus}}' "$1")
  if [ "$nano" != 0 ]; then echo "$(awk -v n="$nano" 'BEGIN { print n / 1e9 }') quota ${cpuset:--}"
  elif [ -n "$cpuset" ]; then echo "$(cpuset_count "$cpuset") pinned $cpuset"
  else echo "$(docker info -f '{{.NCPU}}') unlimited -"
  fi
}

# All targets must be running and healthy, with the same data source and limits.
DATA_SOURCE="" CPUS="" CPU_MODE="" SERVER_CPUSET="" MEMORY=""
for t in $TARGETS; do
  id=$(docker compose ps -q "$t" 2>/dev/null || true)
  [ -n "$id" ] || { log "service $t is not running (docker compose up -d --wait)"; exit 1; }
  health=$(docker inspect -f '{{.State.Health.Status}}' "$id")
  [ "$health" = healthy ] || { log "service $t is $health"; exit 1; }
  ds=$(container_env "$t" DATA_SOURCE)
  read -r cpus mode cpuset <<<"$(cpu_limit "$id")"
  mem=$(docker inspect -f '{{.HostConfig.Memory}}' "$id" | awk '{print $1/1048576 "MiB"}')
  if [ -n "$DATA_SOURCE" ] && { [ "$ds" != "$DATA_SOURCE" ] || [ "$cpus" != "$CPUS" ] || [ "$mode" != "$CPU_MODE" ] ||
      [ "$cpuset" != "$SERVER_CPUSET" ] || [ "$mem" != "$MEMORY" ]; }; then
    log "targets differ in DATA_SOURCE/CPU/memory ($t: $ds, $cpus CPUs $mode $cpuset, $mem); refusing to compare"; exit 1
  fi
  DATA_SOURCE=$ds CPUS=$cpus CPU_MODE=$mode SERVER_CPUSET=$cpuset MEMORY=$mem
done

# k6's CPU limit, for the saturation check: the pinned CPU count when the k6
# service has a cpuset in the effective compose configuration, else K6_CPUS.
K6_CPUSET_EFFECTIVE=$(docker compose --profile bench config k6 2>/dev/null | sed -n 's/^ *cpuset: *"\{0,1\}\([^"]*\)"\{0,1\}$/\1/p' | head -1)
if [ -n "$K6_CPUSET_EFFECTIVE" ]; then
  K6_CPUS=$(cpuset_count "$K6_CPUSET_EFFECTIVE")
fi

[[ "$REPEAT" =~ ^[1-9][0-9]*$ ]] || { log "--repeat must be a positive integer"; exit 1; }

STAMP=$(date -u +%Y%m%dT%H%M%SZ)
DIR="${OUT_DIR:-results/${STAMP}-${WORKLOAD}${LABEL:+-$LABEL}}"
mkdir -p "$DIR"
[ ! -e "$DIR/results.jsonl" ] || { log "$DIR already contains results; choose another --dir"; exit 1; }

# --- Environment capture ------------------------------------------------------
json_escape() { sed 's/\\/\\\\/g; s/"/\\"/g' | tr -d '\r\n'; }
host_cpu() {
  case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*) powershell.exe -NoProfile -Command '(Get-CimInstance Win32_Processor | Select-Object -First 1).Name' 2>/dev/null ;;
    Darwin) sysctl -n machdep.cpu.brand_string 2>/dev/null ;;
    *) sed -n 's/^model name[[:space:]]*: //p' /proc/cpuinfo 2>/dev/null | head -1 ;;
  esac
}
host_os() {
  case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*) powershell.exe -NoProfile -Command '$o = Get-CimInstance Win32_OperatingSystem; "$($o.Caption) $($o.Version)"' 2>/dev/null ;;
    Darwin) echo "macOS $(sw_vers -productVersion 2>/dev/null)" ;;
    *) (. /etc/os-release 2>/dev/null && echo "$PRETTY_NAME") || uname -sr ;;
  esac
}
psql_q() { docker compose exec -T postgres psql -X -At -U "${POSTGRES_USER:-bench}" -d "${POSTGRES_DB:-products}" -c "$1" 2>/dev/null | tr -d '\r'; }
service_log_field() { docker compose logs "$1" --no-log-prefix 2>/dev/null | sed -n "s/.*\"$2\":\"\([^\"]*\)\".*/\1/p" | tail -1; }

log "capturing environment"
DATASET_CHECKSUM=$(psql_q "SET TIME ZONE 'UTC'; SELECT md5(string_agg(concat_ws('|', id, name, description, category, price, stock, rating, created_at, updated_at), E'\n' ORDER BY id)) FROM products" | tail -1)
cat > "$DIR/environment.json" <<EOF
{
  "workload": "$WORKLOAD",
  "started_at": "$STAMP",
  "targets": "$TARGETS",
  "vus_levels": "$VUS_LEVELS",
  "repeat": $REPEAT,
  "warmup": "$WARMUP",
  "duration": "$DURATION",
  "cooldown_seconds": $COOLDOWN,
  "limit": "${LIMIT}",
  "term": "${TERM_ARG}",
  "data_source": "$DATA_SOURCE",
  "server_cpus": "$CPUS",
  "cpu_mode": "$CPU_MODE",
  "server_cpuset": "$SERVER_CPUSET",
  "server_memory": "$MEMORY",
  "db_max_conns": "$(container_env go-rest DB_MAX_CONNS)",
  "postgres_cpus": "$(cpu_limit "$(docker compose ps -q postgres)")",
  "k6_cpus": "${K6_CPUS:-4}",
  "k6_cpuset": "${K6_CPUSET_EFFECTIVE:-}",
  "host_cpu": "$(host_cpu | json_escape)",
  "host_os": "$(host_os | json_escape)",
  "docker_vm_cpu": "$(docker compose exec -T postgres sh -c "sed -n 's/^model name[[:space:]]*: //p' /proc/cpuinfo | head -1" 2>/dev/null | json_escape)",
  "docker_version": "$(docker version -f '{{.Server.Version}}')",
  "docker_os": "$(docker info -f '{{.OperatingSystem}}' | json_escape)",
  "docker_kernel": "$(docker info -f '{{.KernelVersion}}')",
  "docker_cpus": $(docker info -f '{{.NCPU}}'),
  "docker_memory_bytes": $(docker info -f '{{.MemTotal}}'),
  "go_version": "$(service_log_field go-rest go_version)",
  "node_version": "$(service_log_field ts-rest node_version)",
  "typescript_version": "$(sed -n 's/.*"typescript": "\([^"]*\)".*/\1/p' typescript-server/package.json)",
  "postgres_version": "$(psql_q 'SHOW server_version' | json_escape)",
  "k6_version": "$(docker compose run --rm --no-deps k6 version 2>/dev/null | head -1 | json_escape)",
  "dataset_products": $(psql_q 'SELECT count(*) FROM products' || echo null),
  "dataset_checksum": "$DATASET_CHECKSUM",
  "network": "k6 container -> server container on the compose bridge network (no published ports)",
  "tls": false
}
EOF
if [ "$DATASET_CHECKSUM" != "d26bc7a601e76ff91ba94bc08ebebc68" ]; then
  log "WARNING: dataset checksum $DATASET_CHECKSUM differs from the seeded dataset (run scripts/db.sh reset)"
fi

# --- Container sampling -------------------------------------------------------
# A background loop of `docker stats --no-stream` (one snapshot of every
# container every ~2 s, each line prefixed with the host time in ms). Only
# samples inside the measured window reported by k6 are used. It provides the
# k6 and PostgreSQL CPU/memory, and a cross-check of the server's own numbers;
# the authoritative server CPU/memory come from its runtime endpoint
# (result.server). A loop of short-lived calls is used instead of a streaming
# `docker stats`, because on Windows a backgrounded docker.exe cannot be
# stopped reliably from Git Bash.
K6_CONTAINER=bench-k6
K6_CPU_LIMIT=${K6_CPUS:-4}
STATS_FILE="$DIR/.docker-stats.tmp"
STATS_PID=""
if [ "$(date +%3N)" = "3N" ]; then now_ms() { echo "$(date +%s)000"; }; else now_ms() { date +%s%3N; }; fi
sample_containers() {
  while :; do
    ts=$(now_ms)
    docker stats --no-stream --format '{{.Name}} {{.CPUPerc}} {{.MemUsage}}' 2>/dev/null | sed "s/^/$ts /"
  done
}
stop_stats() { [ -z "$STATS_PID" ] || { kill "$STATS_PID" 2>/dev/null || true; wait "$STATS_PID" 2>/dev/null || true; STATS_PID=""; }; }
cleanup() { stop_stats; docker rm -f "$K6_CONTAINER" >/dev/null 2>&1 || true; rm -f "$STATS_FILE"; }
trap cleanup EXIT

# container_summary NAME START_MS END_MS
# Prints {"cpu_percent_avg":..,"cpu_percent_max":..,"mem_mb_max":..,"samples":N} or null,
# from samples taken between START+1s and END. CPU% is docker's (100 = one core).
container_summary() {
  awk -v name="$1" -v start="$2" -v end="$3" '
    $2 == name && $1 >= start + 1000 && $1 <= end {
      cpu = $3; sub(/%/, "", cpu); cpu += 0
      mem = $4; v = mem + 0
      if (mem ~ /GiB$/) v *= 1024; else if (mem ~ /KiB$/) v /= 1024; else if (mem ~ /[0-9]B$/) v /= 1048576
      sum += cpu; n++; if (cpu > max) max = cpu; if (v > memmax) memmax = v
    }
    END {
      if (n == 0) { print "null"; exit }
      printf "{\"cpu_percent_avg\":%.1f,\"cpu_percent_max\":%.1f,\"mem_mb_max\":%.1f,\"samples\":%d}", sum / n, max, memmax, n
    }' "$STATS_FILE"
}
json_field() { printf '%s' "$1" | sed -n "s/.*\"$2\":\([0-9.]*\).*/\1/p" | head -1; }

log "results: $DIR  (data source: $DATA_SOURCE, $CPUS CPU(s) $CPU_MODE ${SERVER_CPUSET} / $MEMORY per server, k6 $K6_CPU_LIMIT CPUs)"
HEADER='%-8s %5s %9s %8s %8s %8s %8s %8s %6s %5s %8s %7s %7s %6s\n'
printf "$HEADER" target vus req/s avg_ms p50_ms p90_ms p95_ms p99_ms errors t/o rx_B/req srv_cpu srv_MB k6_cpu% >&2

PG_CONTAINER=$(docker inspect -f '{{.Name}}' "$(docker compose ps -q postgres)" | sed 's|^/||')
saturated_runs=0
first=1
for rep in $(seq 1 "$REPEAT"); do
for vus in $VUS_LEVELS; do
  for t in $TARGETS; do
    ((first)) || sleep "$COOLDOWN"
    first=0
    file="$t-vus$vus-r$rep.json"
    server_container=$(docker inspect -f '{{.Name}}' "$(docker compose ps -q "$t")" | sed 's|^/||')

    docker rm -f "$K6_CONTAINER" >/dev/null 2>&1 || true
    sample_containers > "$STATS_FILE" &
    STATS_PID=$!

    out=$(docker compose run --rm --no-deps --name "$K6_CONTAINER" \
        -e TARGET="$t" -e VUS="$vus" -e WARMUP="$WARMUP" -e DURATION="$DURATION" \
        ${LIMIT:+-e LIMIT="$LIMIT"} ${TERM_ARG:+-e TERM="$TERM_ARG"} \
        -e DATA_SOURCE_LABEL="$DATA_SOURCE" -e SERVER_CPUS_LABEL="$CPUS" -e CPU_MODE_LABEL="$CPU_MODE" \
        -e REPEAT_INDEX="$rep" -e RUN_STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
        k6 run -q "/scripts/$WORKLOAD.js" 2>&1) || { log "k6 failed for $t at $vus VUs:"; echo "$out" >&2; exit 1; }
    stop_stats

    line=$(printf '%s\n' "$out" | sed -n 's/^RESULT //p')
    [ -n "$line" ] || { log "no result for $t at $vus VUs:"; echo "$out" >&2; exit 1; }

    win_start=$(printf '%s' "$line" | sed -n 's/.*"measure_window_ms":{"start":\([0-9]*\),"end":\([0-9]*\)}.*/\1/p')
    win_end=$(printf '%s' "$line" | sed -n 's/.*"measure_window_ms":{"start":\([0-9]*\),"end":\([0-9]*\)}.*/\2/p')
    k6_stats=$(container_summary "$K6_CONTAINER" "${win_start:-0}" "${win_end:-0}")
    k6_avg=$(json_field "$k6_stats" cpu_percent_avg)
    # k6 is treated as the bottleneck when it averages over 75% of its CPU limit
    # (docker stats reads ~15% below process CPU accounting, so 75% is conservative).
    k6_bound=$(awk -v a="${k6_avg:-0}" -v l="$K6_CPU_LIMIT" 'BEGIN { print (a > 75 * l) ? "true" : "false" }')
    containers="{\"server\":$(container_summary "$server_container" "${win_start:-0}" "${win_end:-0}"),\"k6\":$k6_stats,\"postgres\":$(container_summary "$PG_CONTAINER" "${win_start:-0}" "${win_end:-0}"),\"k6_cpu_limit\":$K6_CPU_LIMIT,\"k6_cpu_bound\":$k6_bound}"
    merged="${line%\}},\"containers\":$containers}"
    printf '%s\n' "$merged" > "$DIR/$file"
    printf '%s\n' "$merged" >> "$DIR/results.jsonl"

    # shellcheck disable=SC2046
    printf "$HEADER" $(printf '%s\n' "$out" | sed -n 's/^ROW //p') "${k6_avg:--}" >&2
    if [ "$k6_bound" = true ]; then
      saturated_runs=$((saturated_runs + 1))
      log "WARNING: k6 averaged ${k6_avg}% CPU (limit $((K6_CPU_LIMIT * 100))%): this run may measure the client, not $t. Raise K6_CPUS."
    fi
  done
done
done
rm -f "$STATS_FILE"
log "done: $DIR"
((saturated_runs == 0)) || log "$saturated_runs run(s) flagged k6_cpu_bound; see README (Client saturation)"
