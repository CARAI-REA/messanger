#!/usr/bin/env bash
# Vendor all Go service modules (needed for Docker -mod=vendor builds; vendor/ is gitignored).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

for m in platform shared user auth chat gateway media search notify; do
  if [[ -f "$m/go.mod" ]]; then
    echo "== go mod vendor: $m =="
    (cd "$m" && go mod tidy && go mod vendor)
  fi
done

if [[ -f scripts/e2e_full_stack/go.mod ]]; then
  echo "== go mod tidy: e2e_full_stack =="
  (cd scripts/e2e_full_stack && go mod tidy)
fi

echo "vendor ok"
