# Messanger

Go messenger monorepo following VKS callService patterns.

## Services

| Service | Port | Role |
|---------|------|------|
| user | 50051 | Users + credentials |
| auth | 50050 | Login / JWT / refresh |
| chat | 50052 | Chats, messages, outbox → Kafka + Redis realtime |
| gateway | 8082 | WebSocket gateway (`/ws`), presence, typing |
| media | 50053 | MinIO presigned uploads |
| search | 50054 | OpenSearch indexing + SearchMessages |
| notify | 50055 | Device registry + mock push |

## Quick start (Colima + Docker Compose)

```bash
# Docker runtime
colima start --cpu 4 --memory 8
docker context use colima
unset DOCKER_HOST

# Images that need Chainguard (official minio/* is pull-denied on some hosts)
docker pull chainguard/minio:latest
docker pull chainguard/minio-client:latest

task env:generate
task up-core
task up-user
task up-auth
task up-chat
task up-media
task up-search
task up-notify
task up-gateway
task up-envoy
task up-observability
task up-web
# or: task up-all

task smoke
```

Web UI (Telegram-style): http://localhost:8088 after `task up-web`.

Register with a unique `@username` (letters/digits/underscore). Find people via search or New message; groups via New group.

Gateway scale check: `task up-gateway-scaled`

Local ports: see [deploy/LOCAL_ACCESS.md](deploy/LOCAL_ACCESS.md).
K8s manifests (future cluster only): [deploy/k8s/](deploy/k8s/).

## Build locally

```bash
task build
# or
for m in user auth chat gateway media search notify; do (cd $m && go build ./cmd); done
```

## Module layout

Each service is a short Go module (`module chat`, …) with:

```
replace github.com/CARAI-REA/messanger/platform => ../platform
replace github.com/CARAI-REA/messanger/shared => ../shared
```

Config is loaded from `deploy/compose/<svc>/.env` via godotenv (unprefixed env vars).

## Realtime path

1. Chat writes `events.v1.ChatRealtimeEvent` into Postgres outbox
2. Outbox worker publishes to Kafka `chat.events` (staged `kafka_published_at`)
3. Same worker `PUBLISH`es payload to Redis channel `chat:realtime` (staged `realtime_published_at`)
4. Gateway subscribes and fans out over WebSocket **only to chat members**
5. search/notify consume Kafka for index/push (offline users only for push)

## Production deployment

Colima + Compose is **dev only**. For staging/prod:

1. Fill secrets from [deploy/env/.env.production.example](deploy/env/.env.production.example) into External Secrets / sealed secrets (see [deploy/SECRETS.md](deploy/SECRETS.md)).
2. Apply k8s manifests per [deploy/k8s/11-README.md](deploy/k8s/11-README.md) (NetworkPolicy, Envoy ConfigMap, gateway drain).
3. Terminate TLS at nginx using [deploy/nginx/messanger.conf.example](deploy/nginx/messanger.conf.example).
4. Run through [deploy/PROD_CHECKLIST.md](deploy/PROD_CHECKLIST.md) including load gate ([scripts/load/README.md](scripts/load/README.md)).
5. Regenerate Envoy descriptor after proto changes: `task proto:descriptor`.

Compose prod overlay (no public gRPC ports):  
`docker compose -f deploy/compose/user/docker-compose.yml -f deploy/compose/docker-compose.prod.yml ...`

### Prod-ready criteria

Backend is **prod-ready in code** when:

- Realtime membership hot-updates + transactional outbox + idempotent SendMessage
- Server gRPC TLS wired (on in prod secrets; off in local compose) + real FCM provider
- Gateway `/healthz` vs `/readyz` + localhost drain + WS Prometheus metrics
- CI: unit (`-short`) → proto-lint → testcontainers integration → compose e2e (WS + smoke)
- Checklist [deploy/PROD_CHECKLIST.md](deploy/PROD_CHECKLIST.md) passed on staging

Remaining ops (out of repo): managed PG/Kafka/Redis/S3, vault secrets, TLS certs, DNS/ingress.
