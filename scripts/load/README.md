# Load testing (k6)

Install: https://k6.io/docs/get-started/installation/ (`brew install k6`)

## Prerequisites

```bash
task up-all
# or at least: up-core, up-user, up-auth, up-chat, up-gateway
```

Obtain an access token (Authorization header — query `?token=` is disabled in production):

```bash
EMAIL=load@example.com
# create + login via grpcurl, then:
export ACCESS_TOKEN=...
export WS_URL=ws://localhost:8082/ws
```

## Scenarios

```bash
k6 run -e ACCESS_TOKEN="$ACCESS_TOKEN" scripts/load/send_message.js
k6 run -e ACCESS_TOKEN="$ACCESS_TOKEN" -e WS_URL="$WS_URL" scripts/load/ws_fanout.js
```

## Target SLO

- SendMessage p99 ≤ 100ms (no media)
- Realtime fanout p99 ≤ 200ms
- ListMessages(limit=50) p99 ≤ 50ms

## Baseline (Colima local)

Recorded 2026-10-01 on Colima (`4 CPU / 8GiB`, `--network-address`), stack via `task up-all`.

| Scenario | Date | Hardware | p50 | p99 | Notes |
|----------|------|----------|-----|-----|-------|
| ws_fanout (10 VUs / 20s) | 2026-10-01 | Colima 4c/8G | ws_connecting ~7.1ms | ~15.3ms (p95) | checks 40/40 OK; header auth |
| send_message | — | — | — | — | Optional; re-run on staging before prod |

Manual release gate: attach k6 summary to the release ticket and tick the load item in `deploy/PROD_CHECKLIST.md`.
