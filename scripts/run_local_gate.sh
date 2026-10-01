#!/usr/bin/env bash
# Full local proof gate: unit + integration + compose smoke/e2e (+ optional k6).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export PATH="$ROOT/bin:$PATH"

# Colima / Docker Desktop
if ! docker info >/dev/null 2>&1; then
  echo "Docker daemon not reachable. Start Colima: colima start --cpu 4 --memory 8 --disk 100 --network-address"
  exit 1
fi

# Colima testcontainers knobs (harmless on Linux CI)
if docker context show 2>/dev/null | grep -q colima || [[ "${DOCKER_HOST:-}" == *colima* ]]; then
  export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="${TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE:-/var/run/docker.sock}"
  export TESTCONTAINERS_RYUK_DISABLED="${TESTCONTAINERS_RYUK_DISABLED:-true}"
  if [[ -z "${TESTCONTAINERS_HOST_OVERRIDE:-}" ]] && command -v colima >/dev/null; then
    ADDR=$(colima list -j 2>/dev/null | python3 -c "import sys,json; print(json.load(sys.stdin).get('address',''))" 2>/dev/null || true)
    [[ -n "$ADDR" ]] && export TESTCONTAINERS_HOST_OVERRIDE="$ADDR"
  fi
fi

echo "== vendor =="
bash scripts/ci_vendor.sh

echo "== unit (-short) =="
(cd platform && go test ./... -count=1 -short)
for m in user auth chat gateway media search notify; do
  (cd "$m" && go test -mod=vendor ./... -count=1 -short)
done

echo "== integration =="
(cd chat && go test -mod=vendor ./internal/repository/outbox/ -run TestOutboxSkipLockedStagedPublish -count=1 -timeout 10m)
(cd gateway && go test -mod=vendor ./internal/integration/ -count=1 -timeout 10m)
(cd auth && go test -mod=vendor ./internal/integration/ -count=1)

echo "== compose up =="
bash scripts/ci_prepare_env.sh
task up-all
for i in $(seq 1 60); do
  if curl -sf --max-time 2 http://127.0.0.1:8082/readyz >/dev/null; then
    echo gateway_ready
    break
  fi
  sleep 5
  if [[ $i -eq 60 ]]; then
    echo "gateway not ready"; docker ps -a; exit 1
  fi
done

echo "== smoke =="
task smoke

echo "== e2e =="
task e2e

if command -v k6 >/dev/null; then
  echo "== k6 ws_fanout (short) =="
  # Obtain token via grpcurl login helper is environment-specific; skip if ACCESS_TOKEN unset.
  if [[ -n "${ACCESS_TOKEN:-}" ]]; then
    k6 run --vus 5 --duration 15s -e ACCESS_TOKEN="$ACCESS_TOKEN" scripts/load/ws_fanout.js
  else
    echo "SKIP k6 (set ACCESS_TOKEN to run)"
  fi
else
  echo "SKIP k6 (not installed)"
fi

echo "LOCAL_GATE_OK"
