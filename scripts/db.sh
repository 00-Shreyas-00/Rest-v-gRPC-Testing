#!/usr/bin/env bash
# Database lifecycle for the benchmark. Delegates to database/manage.sh inside
# the db-init container, so no local PostgreSQL client is needed.
# Works in Linux, macOS, WSL and Git Bash.
#
#   scripts/db.sh up        start the postgres container and wait until healthy
#   scripts/db.sh create    create the database if it does not exist
#   scripts/db.sh migrate   apply pending migrations from database/migrations
#   scripts/db.sh seed      replace all products with the deterministic dataset
#   scripts/db.sh reset     drop everything, then migrate and seed
#   scripts/db.sh setup     up + create + migrate + seed-if-empty
#   scripts/db.sh verify    print row count, dataset checksum and search-term hit counts
#   scripts/db.sh psql      open an interactive psql shell
#   scripts/db.sh testdb    (re)create the products_test database with 1000 products
#
# Environment: DB_NAME (target database; default $POSTGRES_DB or products),
#              PRODUCT_COUNT (100000). DB_NAME is passed to the container only;
#              POSTGRES_DB is never changed, so compose never recreates postgres.
set -euo pipefail
export MSYS_NO_PATHCONV=1 # Git Bash: do not rewrite container paths

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

TARGET_DB="${DB_NAME:-${POSTGRES_DB:-products}}"
PRODUCT_COUNT="${PRODUCT_COUNT:-100000}"

manage() {
  docker compose run --rm -e PGDATABASE="$TARGET_DB" -e PRODUCT_COUNT="$PRODUCT_COUNT" \
    db-init "$@" 2> >(grep -v -E '^ ?(Container|Network|Volume) ' >&2)
}

case "${1:-}" in
  up)      docker compose up -d --wait postgres ;;
  create|migrate|seed|reset|verify) manage "$1" ;;
  setup)   manage init ;;
  psql)    docker compose exec postgres psql -U "${POSTGRES_USER:-bench}" -d "$TARGET_DB" ;;
  testdb)
    # Separate database for integration tests, so tests never touch benchmark data.
    TARGET_DB=products_test PRODUCT_COUNT=1000
    manage create; manage reset ;;
  *)
    sed -n '2,18p' "$0" | sed 's/^# \{0,1\}//'
    exit 1
    ;;
esac
