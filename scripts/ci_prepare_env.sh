#!/usr/bin/env bash
# Prepare compose .env files for CI (templates are committed; .env is gitignored).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ ! -f deploy/env/.env ]]; then
  if [[ -f deploy/env/.env.template ]]; then
    cp deploy/env/.env.template deploy/env/.env
  elif [[ -f deploy/env/.env.example ]]; then
    cp deploy/env/.env.example deploy/env/.env
  else
    echo "WARN: no deploy/env/.env template found"
  fi
fi

# Force development so prodguard does not block containers.
if [[ -f deploy/env/.env ]]; then
  sed -i.bak -E \
    -e 's/^(APP_ENV|USER_APP_ENV|AUTH_APP_ENV|CHAT_APP_ENV|GATEWAY_APP_ENV)=.*/\1=development/' \
    deploy/env/.env || true
  rm -f deploy/env/.env.bak
fi

if command -v task >/dev/null 2>&1; then
  task env:generate
  exit 0
fi

if [[ -x ./bin/task ]]; then
  ./bin/task env:generate
  exit 0
fi

echo "task not found; relying on committed compose .env files if present"
