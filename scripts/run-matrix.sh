#!/usr/bin/env bash
# Runs the full benchmark matrix in one go and writes a report.
#
#   scripts/run-matrix.sh [options]
#
# Options (defaults in brackets):
#   --session NAME            results/<NAME> [matrix-<UTC timestamp>]
#   --modes "postgres memory"
#   --workloads "get-product list-products search-products"
#   --list-limits "10 50 100 500"
#   --vus "1 10 50 100 250 500 1000"
#   --repeat 3                runs per point (median and spread are reported)
#   --warmup 15s --duration 30s --cooldown 5
#   --cpu-mode pinned         pinned: CPU sets, no quota (docker-compose.pinned.yml)
#                             quota:  CFS quotas (--cpus), as in Kubernetes CPU limits
#   pinned mode:
#     --server-cpusets "2"    one pass per set, e.g. "2 2-3" for 1 and 2 CPUs
#     --postgres-cpuset 3-5
#     --k6-cpuset 6-11            k6 CPUs in postgres mode
#     --k6-cpuset-memory 3-11     k6 CPUs in memory mode (PostgreSQL is idle there)
#   quota mode:
#     --cpus "1"              one pass per value
#     --k6-cpus 6
#   --skip-equivalence        do not run scripts/equivalence.sh first
#   --dry-run                 print the plan and the estimated duration only
#
# Per pass (CPU limit x data mode): reset the database, recreate the three
# servers with that configuration (fresh processes), then run every workload
# with scripts/bench.sh. At the end the stack is restored to the compose
# defaults and scripts/report.sh writes results/<NAME>/report.md and report.csv.
#
# Keep the machine otherwise idle while this runs: other load on the host
# shows up as noise in the results.
set -euo pipefail
export MSYS_NO_PATHCONV=1

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SESSION="matrix-$(date -u +%Y%m%dT%H%M%SZ)"
MODES="postgres memory"
WORKLOADS="get-product list-products search-products" LIST_LIMITS="10 50 100 500"
VUS="1 10 50 100 250 500 1000" REPEAT=3 WARMUP=15s DURATION=30s COOLDOWN=5
CPU_MODE=pinned SERVER_CPUSETS="2" PG_CPUSET="3-5" K6_SET="6-11" K6_SET_MEMORY="3-11" CPU_LIMITS="1" K6_CPU_LIMIT=6
SKIP_EQUIV=0 DRY_RUN=0
while (($# > 0)); do
  case "$1" in
    --session) SESSION=$2; shift 2 ;;
    --modes) MODES=$2; shift 2 ;;
    --workloads) WORKLOADS=$2; shift 2 ;;
    --list-limits) LIST_LIMITS=$2; shift 2 ;;
    --vus) VUS=$2; shift 2 ;;
    --repeat) REPEAT=$2; shift 2 ;;
    --warmup) WARMUP=$2; shift 2 ;;
    --duration) DURATION=$2; shift 2 ;;
    --cooldown) COOLDOWN=$2; shift 2 ;;
    --cpu-mode) CPU_MODE=$2; shift 2 ;;
    --server-cpusets) SERVER_CPUSETS=$2; shift 2 ;;
    --postgres-cpuset) PG_CPUSET=$2; shift 2 ;;
    --k6-cpuset) K6_SET=$2; shift 2 ;;
    --k6-cpuset-memory) K6_SET_MEMORY=$2; shift 2 ;;
    --cpus) CPU_LIMITS=$2; shift 2 ;;
    --k6-cpus) K6_CPU_LIMIT=$2; shift 2 ;;
    --skip-equivalence) SKIP_EQUIV=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    *) sed -n '2,34p' "$0" | sed 's/^# \{0,1\}//'; exit 1 ;;
  esac
done

log() { printf '==> [%s] %s\n' "$(date +%H:%M:%S)" "$*" >&2; }
secs() { case "$1" in *ms) echo 1 ;; *m) echo $(( ${1%m} * 60 )) ;; *s) echo "${1%s}" ;; *) echo "$1" ;; esac; }
count() { set -- $1; echo $#; }
cpuset_max() { printf '%s' "$1" | tr ',-' '\n\n' | sort -n | tail -1; }

case "$CPU_MODE" in
  pinned)
    PASS_LIMITS=$SERVER_CPUSETS
    vm_cpus=$(docker info -f '{{.NCPU}}')
    for set in $SERVER_CPUSETS $PG_CPUSET $K6_SET $K6_SET_MEMORY; do
      if (( $(cpuset_max "$set") >= vm_cpus )); then
        log "CPU set $set does not exist: the Docker VM has CPUs 0-$((vm_cpus - 1)). Pass --server-cpusets/--postgres-cpuset/--k6-cpuset."
        exit 1
      fi
    done
    export COMPOSE_PATH_SEPARATOR=: COMPOSE_FILE=docker-compose.yml:docker-compose.pinned.yml
    export POSTGRES_CPUSET=$PG_CPUSET
    ;;
  quota)
    PASS_LIMITS=$CPU_LIMITS
    export K6_CPUS=$K6_CPU_LIMIT
    ;;
  *) log "--cpu-mode must be pinned or quota"; exit 1 ;;
esac

# One bench.sh invocation per workload instance (list-products once per limit).
instances=0
for w in $WORKLOADS; do
  if [ "$w" = list-products ]; then instances=$((instances + $(count "$LIST_LIMITS"))); else instances=$((instances + 1)); fi
done
passes=$(( $(count "$PASS_LIMITS") * $(count "$MODES") ))
runs=$(( passes * instances * $(count "$VUS") * 3 * REPEAT ))
# Measured overhead: ~12 s per run (k6 container start, setup, sampling),
# ~30 s per bench.sh invocation (environment capture), ~60 s per pass.
per_run=$(( $(secs "$WARMUP") + 2 + $(secs "$DURATION") + COOLDOWN + 12 ))
total=$(( runs * per_run + passes * instances * 30 + passes * 60 + 60 ))
log "session results/$SESSION: $runs runs, estimated ~$((total / 3600))h $((total % 3600 / 60))m"
if [ "$CPU_MODE" = pinned ]; then
  log "CPU: pinned; servers on CPUs [$SERVER_CPUSETS] (one pass each), postgres on $PG_CPUSET, k6 on $K6_SET (memory mode: $K6_SET_MEMORY)"
else
  log "CPU: quota; servers $CPU_LIMITS CPU(s) (one pass each), k6 $K6_CPU_LIMIT CPUs"
fi
log "modes: $MODES | workloads: $WORKLOADS | list limits: $LIST_LIMITS"
log "VUs: $VUS | repeat: $REPEAT | warm-up $WARMUP, measured $DURATION, cool-down ${COOLDOWN}s"
((DRY_RUN == 0)) || exit 0

mkdir -p "results/$SESSION"
[ -z "$(ls -A "results/$SESSION")" ] || { log "results/$SESSION is not empty"; exit 1; }

restore_defaults() {
  log "restoring the stack to compose defaults"
  env -u COMPOSE_FILE -u COMPOSE_PATH_SEPARATOR -u SERVER_CPUS -u DATA_SOURCE \
    docker compose up -d --wait --force-recreate >/dev/null 2>&1 || true
}
trap restore_defaults EXIT

docker compose up -d --build --wait >/dev/null 2>&1
if ((SKIP_EQUIV == 0)); then
  log "equivalence check"
  scripts/equivalence.sh > "results/$SESSION/equivalence.log" 2>&1 ||
    { log "equivalence FAILED; see results/$SESSION/equivalence.log"; exit 1; }
  grep -E 'requests compared|PASS' "results/$SESSION/equivalence.log" | tail -4 >&2
fi

started=$(date +%s)
for limit_value in $PASS_LIMITS; do
  if [ "$CPU_MODE" = pinned ]; then export SERVER_CPUSET=$limit_value; tag="pinned$limit_value"
  else export SERVER_CPUS=$limit_value; tag="cpus$limit_value"; fi
  for mode in $MODES; do
    if [ "$CPU_MODE" = pinned ]; then
      if [ "$mode" = memory ]; then export K6_CPUSET=$K6_SET_MEMORY; else export K6_CPUSET=$K6_SET; fi
    fi
    log "pass: $mode, servers $CPU_MODE $limit_value${K6_CPUSET:+, k6 on $K6_CPUSET}"
    scripts/db.sh reset >/dev/null 2>&1
    DATA_SOURCE=$mode docker compose up -d --wait --force-recreate go-rest ts-rest go-grpc >/dev/null 2>&1
    for w in $WORKLOADS; do
      if [ "$w" = list-products ]; then set -- $LIST_LIMITS; else set -- ""; fi
      for limit in "$@"; do
        dir="results/$SESSION/$mode-$tag/$w${limit:+-limit$limit}"
        log "$w${limit:+ limit=$limit} -> $dir"
        scripts/bench.sh "$w" --dir "$dir" --vus "$VUS" --repeat "$REPEAT" \
          --warmup "$WARMUP" --duration "$DURATION" --cooldown "$COOLDOWN" ${limit:+--limit "$limit"} \
          2> >(grep -v -E '^ ?(Container|Network) ' >&2)
      done
    done
  done
done
log "all runs finished in $(( ($(date +%s) - started) / 60 )) min"

scripts/report.sh "results/$SESSION" > /dev/null
log "report: results/$SESSION/report.md"
