#!/usr/bin/env bash
# Experiment 2: large payloads and streaming. Runs the whole matrix and writes
# a report. Design and rationale: docs/experiment-2-design.md.
#
#   scripts/run-x2.sh [options]
#
# Options (defaults in brackets):
#   --session NAME        results/<NAME> [x2-<UTC timestamp>]
#   --parts "a b"         a: large payloads; b: streaming (export, import, quote, feed)
#   --repeat 3            runs per point (median and spread are reported)
#   --warmup 5s --duration 15s --cooldown 2
#   --networks "lan dc wan"        part A network profiles
#   --export-networks "lan wan"    part B export network profiles
#   --server-cpuset 2 --loadgen-cpuset 3-11
#   --quick               tiny matrix with short runs, to check the setup (~10 min)
#   --skip-verify         do not run the equivalence check first
#   --dry-run             print the plan and the estimated duration only
#
# Network profiles (netem delay + tbf rate on every server and on the load
# generator, so both directions are shaped; RTT = 2 x delay):
#   lan  no shaping (container to container, ~0.05 ms RTT)
#   dc   1 Gbit/s, 0.5 ms RTT
#   wan  100 Mbit/s, 20 ms RTT
#
# Output in results/<NAME>/: results.jsonl (one line per run), verify.log,
# codec-go.jsonl, codec-node.jsonl, environment.json, events.log, and
# report.md / report.csv (scripts/report-x2.mjs).
#
# Keep the machine otherwise idle, plugged in, with sleep and screen-off
# disabled (see README "Running experiment 2").
set -euo pipefail
export MSYS_NO_PATHCONV=1

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

SESSION="x2-$(date -u +%Y%m%dT%H%M%SZ)"
PARTS="a b" REPEAT=3 WARMUP=5s DURATION=15s COOLDOWN=2
NETWORKS="lan dc wan" EXPORT_NETWORKS="lan wan"
SERVER_SET=2 LOADGEN_SET=3-11 QUICK=0 SKIP_VERIFY=0 DRY_RUN=0
while (($# > 0)); do
  case "$1" in
    --session) SESSION=$2; shift 2 ;;
    --parts) PARTS=$2; shift 2 ;;
    --repeat) REPEAT=$2; shift 2 ;;
    --warmup) WARMUP=$2; shift 2 ;;
    --duration) DURATION=$2; shift 2 ;;
    --cooldown) COOLDOWN=$2; shift 2 ;;
    --networks) NETWORKS=$2; shift 2 ;;
    --export-networks) EXPORT_NETWORKS=$2; shift 2 ;;
    --server-cpuset) SERVER_SET=$2; shift 2 ;;
    --loadgen-cpuset) LOADGEN_SET=$2; shift 2 ;;
    --quick) QUICK=1; shift ;;
    --skip-verify) SKIP_VERIFY=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    *) sed -n '2,32p' "$0" | sed 's/^# \{0,1\}//'; exit 1 ;;
  esac
done

# ------------------------------------------------------------------ matrix
TARGETS="go-rest ts-rest go-grpc"
A_PROFILES="text numeric binary" A_COUNTS="10 250 4000" A_GZIP="false true" A_CONC="1 16"
EXPORT_COUNTS="1000 10000 100000" EXPORT_CONC="1 8"
IMPORT_COUNTS="1000 10000" IMPORT_CONC="1 8"
QUOTE_CONC="1 16 128"
FEED_SUBS="100 1000 2500 5000" FEED_RATE=20
if ((QUICK)); then
  REPEAT=1 WARMUP=2s DURATION=4s COOLDOWN=1 NETWORKS="lan wan" EXPORT_NETWORKS="wan"
  A_PROFILES="numeric" A_COUNTS="250" A_CONC="16"
  EXPORT_COUNTS="10000" EXPORT_CONC="1" IMPORT_COUNTS="1000" IMPORT_CONC="8" QUOTE_CONC="16" FEED_SUBS="500"
fi

log() { printf '==> [%s] %s\n' "$(date +%H:%M:%S)" "$*" >&2; }
secs() { case "$1" in *m) echo $(( ${1%m} * 60 )) ;; *s) echo "${1%s}" ;; *) echo "$1" ;; esac; }
count() { set -- $1; echo $#; }
has_part() { [[ " $PARTS " == *" $1 "* ]]; }

n_targets=$(count "$TARGETS")
runs_a=0 runs_b=0
if has_part a; then
  runs_a=$(( $(count "$NETWORKS") * $(count "$A_PROFILES") * $(count "$A_COUNTS") * $(count "$A_GZIP") * $(count "$A_CONC") * n_targets * REPEAT ))
fi
if has_part b; then
  runs_b=$(( ( $(count "$EXPORT_NETWORKS") * $(count "$EXPORT_COUNTS") * 2 * $(count "$EXPORT_CONC") * n_targets \
    + $(count "$IMPORT_COUNTS") * 2 * $(count "$IMPORT_CONC") * n_targets \
    + $(count "$QUOTE_CONC") * 4 + $(count "$FEED_SUBS") * 4 ) * REPEAT ))
fi
# Measured overhead: ~4 s per run (exec, connection setup, scrapes); feed runs
# add a server restart and the subscriber ramp (~15 s).
per_run=$(( $(secs "$WARMUP") + $(secs "$DURATION") + COOLDOWN + 4 ))
feed_runs=0; has_part b && feed_runs=$(( $(count "$FEED_SUBS") * 4 * REPEAT ))
total=$(( (runs_a + runs_b) * per_run + feed_runs * 15 + 600 ))
log "session results/$SESSION: $((runs_a + runs_b)) runs (part A $runs_a, part B $runs_b), estimated ~$((total / 3600))h $((total % 3600 / 60))m"
log "parts: $PARTS | repeat $REPEAT | warm-up $WARMUP, measured $DURATION, cool-down ${COOLDOWN}s"
log "CPU: servers pinned to $SERVER_SET (one core each, never loaded at the same time), load generator on $LOADGEN_SET"
((DRY_RUN == 0)) || exit 0

OUT="results/$SESSION"
mkdir -p "$OUT"
[ -z "$(ls -A "$OUT")" ] || { log "$OUT is not empty"; exit 1; }
EVENTS="$OUT/events.log"

export COMPOSE_PATH_SEPARATOR=: COMPOSE_FILE=docker-compose.yml:docker-compose.pinned.yml
export DATA_SOURCE=none GOMEMLIMIT=900MiB SERVER_CPUSET=$SERVER_SET LOADGEN_CPUSET=$LOADGEN_SET
DC="docker compose --profile x2"
LG_IMAGE=go-vs-ts-benchmark/loadgen:local

restore_defaults() {
  log "restoring the stack to compose defaults (experiment 1, PostgreSQL)"
  $DC stop loadgen >/dev/null 2>&1 || true
  env -u COMPOSE_FILE -u COMPOSE_PATH_SEPARATOR -u DATA_SOURCE -u GOMEMLIMIT \
    docker compose up -d --wait --force-recreate >/dev/null 2>&1 || true
}
trap restore_defaults EXIT

# ------------------------------------------------------------------ network
NET=lan
net_cmd() {
  case "$1" in
    lan) echo "tc qdisc del dev eth0 root 2>/dev/null; true" ;;
    dc)  echo "tc qdisc del dev eth0 root 2>/dev/null; tc qdisc add dev eth0 root handle 1: netem delay 0.25ms limit 100000 && tc qdisc add dev eth0 parent 1:1 handle 10: tbf rate 1gbit burst 256kb latency 10ms" ;;
    wan) echo "tc qdisc del dev eth0 root 2>/dev/null; tc qdisc add dev eth0 root handle 1: netem delay 10ms limit 100000 && tc qdisc add dev eth0 parent 1:1 handle 10: tbf rate 100mbit burst 256kb latency 50ms" ;;
    *) log "unknown network profile $1"; exit 1 ;;
  esac
}
apply_net() { # profile [services]
  local svc cid
  for svc in ${2:-go-rest ts-rest go-grpc loadgen}; do
    cid=$($DC ps -q "$svc")
    docker run --rm --network "container:$cid" --cap-add NET_ADMIN "$LG_IMAGE" sh -c "$(net_cmd "$1")" >/dev/null
  done
}
set_net() { NET=$1; apply_net "$1"; log "network: $1"; echo "$(date -u +%FT%TZ) network $1" >> "$EVENTS"; }

# ------------------------------------------------------------------ servers
wait_healthy() { # services
  local svc deadline=$(( $(date +%s) + 120 ))
  for svc in "$@"; do
    until [ "$(docker inspect -f '{{.State.Health.Status}}' "$($DC ps -q "$svc")" 2>/dev/null)" = healthy ]; do
      (( $(date +%s) < deadline )) || { log "$svc did not become healthy"; return 1; }
      sleep 1
    done
  done
}
# Fresh server processes (same containers, so the network shaping stays).
restart_servers() { # [services]
  local svcs=${1:-"go-rest ts-rest go-grpc"}
  $DC restart $svcs >/dev/null 2>&1
  wait_healthy $svcs
  apply_net "$NET" "$svcs"
}
restarts() { docker inspect -f '{{.RestartCount}}' "$($DC ps -q "$1")"; }

# ------------------------------------------------------------------ runs
RUN_NO=0
# run TARGET VARIANT PART "labels-json-fragment" loadgen-args...
run() {
  local target=$1 variant=$2 part=$3 extra=$4; shift 4
  RUN_NO=$((RUN_NO + 1))
  local before line rc=0
  before=$(restarts "$target")
  local labels="{\"session\":\"$SESSION\",\"part\":\"$part\",\"network\":\"$NET\",\"variant\":\"$variant\"$extra}"
  line=$($DC exec -T loadgen loadgen run --target "$target" --warmup "$WARMUP" --duration "$DURATION" \
    --labels "$labels" "$@" 2>>"$EVENTS") || rc=$?
  if ((rc != 0)) || [ -z "$line" ]; then
    log "run $RUN_NO FAILED ($variant $*), see events.log"
    echo "$(date -u +%FT%TZ) run failed: $variant $* $labels" >> "$EVENTS"
    line="{\"failed\":true,\"session\":\"$SESSION\",\"part\":\"$part\",\"network\":\"$NET\",\"variant\":\"$variant\",\"target\":\"$target\",\"args\":\"$*\"${extra}}"
  fi
  if [ "$(restarts "$target")" != "$before" ]; then
    log "$target restarted during run $RUN_NO (OOM-killed?)"
    echo "$(date -u +%FT%TZ) $target restarted during: $variant $*" >> "$EVENTS"
    line="{\"server_restarted\":true,${line#\{}"
    wait_healthy "$target"
    apply_net "$NET" "$target"
  fi
  echo "$line" >> "$OUT/results.jsonl"
  sleep "$COOLDOWN"
}
progress() { log "$1 (run $RUN_NO done, $(( ($(date +%s) - STARTED) / 60 )) min elapsed)"; }

# ------------------------------------------------------------------ setup
log "building images"
$DC build go-rest go-grpc ts-rest loadgen >/dev/null 2>&1
log "starting servers (DATA_SOURCE=none, GOMEMLIMIT=900MiB, CPU $SERVER_SET) and the load generator (CPU $LOADGEN_SET)"
docker compose stop postgres k6 >/dev/null 2>&1 || true # idle CPUs; restored at the end
$DC up -d --no-deps --force-recreate --wait go-rest ts-rest go-grpc loadgen >/dev/null 2>&1
apply_net lan

{
  echo "{"
  echo "  \"session\": \"$SESSION\", \"started_at\": \"$(date -u +%FT%TZ)\","
  echo "  \"host_os\": \"$(uname -s 2>/dev/null) $(uname -r 2>/dev/null)\","
  echo "  \"docker\": $(docker version -f '{"server": "{{.Server.Version}}", "kernel": "{{.Server.KernelVersion}}", "os": "{{.Server.Os}}/{{.Server.Arch}}"}'),"
  echo "  \"docker_vm\": $(docker info -f '{"cpus": {{.NCPU}}, "memory_bytes": {{.MemTotal}}}'),"
  echo "  \"cpu_model\": \"$($DC exec -T loadgen sh -c "grep -m1 'model name' /proc/cpuinfo | cut -d: -f2" | sed 's/^ *//' | tr -d '\r')\","
  echo "  \"server_cpuset\": \"$SERVER_SET\", \"loadgen_cpuset\": \"$LOADGEN_SET\", \"server_memory\": \"1g\", \"gomemlimit\": \"900MiB\","
  echo "  \"gzip_level\": 1, \"data_source\": \"none\","
  echo "  \"networks\": {\"lan\": \"$(net_cmd lan)\", \"dc\": \"$(net_cmd dc)\", \"wan\": \"$(net_cmd wan)\"},"
  echo "  \"images\": {"
  for i in go-rest go-grpc ts-rest loadgen; do
    sep=,; [ $i = loadgen ] && sep=
    echo "    \"$i\": \"$(docker image inspect -f '{{.Id}}' go-vs-ts-benchmark/$i:local)\"$sep"
  done
  echo "  },"
  echo "  \"options\": {\"parts\": \"$PARTS\", \"repeat\": $REPEAT, \"warmup\": \"$WARMUP\", \"duration\": \"$DURATION\", \"cooldown\": $COOLDOWN, \"quick\": $QUICK}"
  echo "}"
} > "$OUT/environment.json"

if ((SKIP_VERIFY == 0)); then
  log "equivalence check (loadgen verify)"
  $DC exec -T loadgen loadgen verify > "$OUT/verify.log" 2>&1 || { log "verify FAILED; see $OUT/verify.log"; exit 1; }
  tail -1 "$OUT/verify.log" >&2
fi

log "codec micro-benchmarks (one core, CPU $SERVER_SET)"
docker run --rm --cpuset-cpus "$SERVER_SET" "$LG_IMAGE" loadgen codec > "$OUT/codec-go.jsonl"
docker run --rm --cpuset-cpus "$SERVER_SET" --entrypoint node go-vs-ts-benchmark/ts-rest:local dist/x2/codec-bench.js > "$OUT/codec-node.jsonl"

STARTED=$(date +%s)

# ------------------------------------------------------------------ part A
# Order: network -> repeat (fresh servers) -> point (ascending size) -> target,
# so every target sees the same drift and repeats are spread over the session.
if has_part a; then
  for net in $NETWORKS; do
    set_net "$net"
    for r in $(seq 1 "$REPEAT"); do
      restart_servers
      progress "part A, $net, repeat $r"
      for profile in $A_PROFILES; do for n in $A_COUNTS; do for gz in $A_GZIP; do for c in $A_CONC; do
        for t in $TARGETS; do
          gzflag=(); [ "$gz" = true ] && gzflag=(--gzip)
          run "$t" "$t" A ",\"repeat\":$r" --scenario payload --profile "$profile" --count "$n" --conc "$c" "${gzflag[@]}"
        done
      done; done; done; done
    done
  done
fi

# ------------------------------------------------------------------ part B
if has_part b; then
  for net in $EXPORT_NETWORKS; do
    set_net "$net"
    for r in $(seq 1 "$REPEAT"); do
      restart_servers
      progress "part B export, $net, repeat $r"
      for n in $EXPORT_COUNTS; do for mode in unary stream; do for c in $EXPORT_CONC; do for t in $TARGETS; do
        run "$t" "$t" B ",\"repeat\":$r" --scenario export --mode "$mode" --count "$n" --conc "$c"
      done; done; done; done
    done
  done

  set_net lan
  for r in $(seq 1 "$REPEAT"); do
    restart_servers
    progress "part B import + quote, repeat $r"
    for n in $IMPORT_COUNTS; do for mode in unary stream; do for c in $IMPORT_CONC; do for t in $TARGETS; do
      run "$t" "$t" B ",\"repeat\":$r" --scenario import --mode "$mode" --count "$n" --conc "$c"
    done; done; done; done
    for c in $QUOTE_CONC; do
      run go-rest go-rest B ",\"repeat\":$r" --scenario quote --mode unary --conc "$c"
      run ts-rest ts-rest B ",\"repeat\":$r" --scenario quote --mode unary --conc "$c"
      run go-grpc go-grpc B ",\"repeat\":$r" --scenario quote --mode unary --conc "$c"
      run go-grpc go-grpc-stream B ",\"repeat\":$r" --scenario quote --mode stream --conc "$c"
    done
  done

  # Feed: a fresh server for every run (thousands of connections leave state behind).
  for r in $(seq 1 "$REPEAT"); do
    progress "part B feed, repeat $r"
    for subs in $FEED_SUBS; do
      for v in go-rest ts-rest go-grpc go-grpc-mux; do
        t=${v%-mux}; conns=0; [ "$v" = go-grpc-mux ] && conns=$(( (subs + 99) / 100 ))
        restart_servers "$t"
        run "$t" "$v" B ",\"repeat\":$r" --scenario feed --subscribers "$subs" --rate "$FEED_RATE" --conns "$conns"
      done
    done
  done
fi

apply_net lan
log "all $RUN_NO runs finished in $(( ($(date +%s) - STARTED) / 60 )) min"
node scripts/report-x2.mjs "$OUT" > /dev/null
log "report: $OUT/report.md"
