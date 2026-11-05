#!/bin/sh
# Database management, run inside the PostgreSQL image (it provides psql).
# Used by the db-init compose service and by scripts/db.sh; this is the only
# implementation of migrate/seed/reset.
#
#   manage.sh init      wait for the server, create DB, migrate, seed if empty
#   manage.sh create    create the database if it does not exist
#   manage.sh migrate   apply pending migrations
#   manage.sh seed      replace all products with the deterministic dataset
#   manage.sh reset     drop the schema, migrate, seed
#   manage.sh verify    row count, dataset checksum, search hit counts
#
# Connection: standard libpq variables (PGHOST, PGUSER, PGPASSWORD, PGDATABASE).
# PRODUCT_COUNT sets the dataset size (default 100000).
set -eu

: "${PGDATABASE:=products}"
: "${PRODUCT_COUNT:=100000}"
DIR="$(cd "$(dirname "$0")" && pwd)"
export PGOPTIONS='-c client_min_messages=warning'

log() { printf '==> %s\n' "$*" >&2; }
psql_db() { psql -X -q -v ON_ERROR_STOP=1 "$@"; }

wait_ready() {
  i=0
  until pg_isready -q -d postgres; do
    i=$((i + 1))
    [ "$i" -lt 60 ] || { log "postgres not ready"; exit 1; }
    sleep 1
  done
}

cmd_create() {
  log "ensuring database '$PGDATABASE' exists"
  psql_db -d postgres -v dbname="$PGDATABASE" <<'SQL'
SELECT format('CREATE DATABASE %I', :'dbname')
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = :'dbname') \gexec
SQL
}

cmd_migrate() {
  psql_db -c "CREATE TABLE IF NOT EXISTS schema_migrations (
                version    TEXT PRIMARY KEY,
                applied_at TIMESTAMPTZ NOT NULL DEFAULT now())"
  applied="$(psql_db -At -c "SELECT version FROM schema_migrations")"
  for file in "$DIR"/migrations/*.sql; do
    version="$(basename "$file" .sql)"
    if printf '%s\n' "$applied" | grep -qxF "$version"; then
      continue
    fi
    log "applying migration $version"
    # A migration and its bookkeeping row commit atomically.
    {
      echo "BEGIN;"
      cat "$file"
      echo
      echo "INSERT INTO schema_migrations (version) VALUES ('$version');"
      echo "COMMIT;"
    } | psql_db
  done
  log "migrations up to date"
}

cmd_seed() {
  log "seeding $PRODUCT_COUNT products into '$PGDATABASE'"
  psql_db -v product_count="$PRODUCT_COUNT" -f "$DIR/seed/seed.sql"
  log "seed complete"
}

cmd_reset() {
  log "dropping schema public in '$PGDATABASE'"
  psql_db <<SQL
DROP SCHEMA public CASCADE;
CREATE SCHEMA public AUTHORIZATION "$PGUSER";
SQL
  cmd_migrate
  cmd_seed
}

cmd_init() {
  wait_ready
  cmd_create
  cmd_migrate
  count="$(psql_db -At -c "SELECT count(*) FROM products")"
  if [ "$count" = "0" ]; then
    cmd_seed
  else
    log "products already present ($count rows); not reseeding (use scripts/db.sh reset)"
  fi
}

cmd_verify() {
  psql_db <<'SQL'
SET TIME ZONE 'UTC';
\echo 'dataset:'
SELECT count(*)                                         AS products,
       min(id)                                          AS min_id,
       max(id)                                          AS max_id,
       md5(string_agg(concat_ws('|', id, name, description, category, price,
                                stock, rating, created_at, updated_at),
                      E'\n' ORDER BY id))               AS checksum
FROM products;
\echo 'search-term hit counts (name ILIKE %term%):'
SELECT t.term, count(p.id) AS matches
FROM unnest(ARRAY['keyboard','phone','laptop','monitor','mouse']) AS t(term)
LEFT JOIN products p ON p.name ILIKE '%' || t.term || '%'
GROUP BY t.term
ORDER BY t.term;
\echo 'description length (bytes):'
SELECT min(octet_length(description)) AS min,
       round(avg(octet_length(description))) AS avg,
       max(octet_length(description)) AS max
FROM products;
SQL
}

case "${1:-}" in
  init)    cmd_init ;;
  create)  cmd_create ;;
  migrate) cmd_migrate ;;
  seed)    cmd_seed ;;
  reset)   cmd_reset ;;
  verify)  cmd_verify ;;
  *) sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'; exit 1 ;;
esac
