#!/usr/bin/env bash
# Black-box equivalence check of all three servers, in both data-source modes.
#
#   scripts/equivalence.sh              postgres and memory
#   scripts/equivalence.sh memory       one mode only
#
# Tests the same Docker images that are benchmarked (built by docker compose),
# against its own database (products_equiv, 100k products), so the benchmark
# database and the running benchmark servers are never touched. For each mode:
#   1. reset and seed products_equiv
#   2. start the go-rest, go-grpc and ts-rest images as equiv-* containers
#   3. run go-server/equivalence against them
#   4. remove the containers
# Exits non-zero if any mode fails.
set -euo pipefail
export MSYS_NO_PATHCONV=1 # Git Bash: do not rewrite container paths

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if (($# > 0)); then MODES=("$@"); else MODES=(postgres memory); fi
NETWORK="go-vs-ts-benchmark_default"
EQUIV_DB="products_equiv"
DB_URL="postgres://${POSTGRES_USER:-bench}:${POSTGRES_PASSWORD:-bench}@postgres:5432/${EQUIV_DB}"
CONTAINERS=(equiv-go-rest equiv-go-grpc equiv-ts-rest)

log() { printf '==> %s\n' "$*" >&2; }
quiet_compose() { grep -v -E '^ ?(Container|Network|Volume|Image) ' >&2 || true; }

stop_servers() { docker rm -f "${CONTAINERS[@]}" >/dev/null 2>&1 || true; }
trap stop_servers EXIT

scripts/db.sh up >/dev/null
log "building images"
docker compose build --quiet go-rest go-grpc ts-rest

# Waits for the image's HEALTHCHECK to report healthy.
wait_healthy() {
  local name=$1 state
  for _ in $(seq 1 120); do
    state=$(docker inspect -f '{{.State.Status}}/{{.State.Health.Status}}' "$name" 2>/dev/null || echo missing)
    case "$state" in
      running/healthy) return 0 ;;
      running/*) sleep 1 ;;
      *) docker logs "$name" >&2 || true; log "$name: $state"; return 1 ;;
    esac
  done
  log "$name did not become healthy"; return 1
}

failed=()
for mode in "${MODES[@]}"; do
  log "mode: $mode"
  DB_NAME=$EQUIV_DB scripts/db.sh create >/dev/null 2>&1
  DB_NAME=$EQUIV_DB scripts/db.sh reset >/dev/null 2>&1

  stop_servers
  for svc in go-rest go-grpc ts-rest; do
    docker run -d --name "equiv-$svc" --network "$NETWORK" \
      -e DATA_SOURCE="$mode" -e DATABASE_URL="$DB_URL" \
      "go-vs-ts-benchmark/$svc:local" >/dev/null
  done
  for c in "${CONTAINERS[@]}"; do wait_healthy "$c"; done

  if docker compose run --rm \
      -e EQUIV_GO_REST_URL=http://equiv-go-rest:8080 \
      -e EQUIV_TS_REST_URL=http://equiv-ts-rest:8080 \
      -e EQUIV_GRPC_ADDR=equiv-go-grpc:9090 \
      go go test -count=1 -v ./equivalence/ 2> >(quiet_compose); then
    log "mode $mode: PASS"
  else
    log "mode $mode: FAIL"; failed+=("$mode")
  fi
  stop_servers
done

if ((${#failed[@]} > 0)); then
  log "equivalence FAILED for: ${failed[*]}"; exit 1
fi
log "equivalence passed for: ${MODES[*]}"
