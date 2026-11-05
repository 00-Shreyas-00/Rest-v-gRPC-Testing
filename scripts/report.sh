#!/usr/bin/env bash
# Builds report.md and report.csv from benchmark results.
#
#   scripts/report.sh RESULTS_DIR...            writes into the first RESULTS_DIR
#   scripts/report.sh --out DIR RESULTS_DIR...
#
# Uses a local Node.js if present, otherwise the pinned Node image.
set -euo pipefail
export MSYS_NO_PATHCONV=1 # Git Bash: do not rewrite container paths

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
(($# > 0)) || { sed -n '2,7p' "$0" | sed 's/^# \{0,1\}//'; exit 1; }

if command -v node >/dev/null 2>&1; then
  exec node scripts/report.mjs "$@"
fi
exec docker run --rm -v "$(pwd -W 2>/dev/null || pwd):/repo" -w /repo node:24.22.0-bookworm-slim \
  node scripts/report.mjs "$@"
