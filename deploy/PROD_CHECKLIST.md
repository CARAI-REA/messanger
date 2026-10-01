# Production checklist

## Pre-flight

- [ ] `APP_ENV=production` on all services (prodguard must pass on boot)
- [ ] Strong secrets: `JWT_SECRET`, `REFRESH_TOKEN_SECRET`, `SERVICE_JWT_SECRET` (independent values)
- [ ] Redis password set on auth/chat/gateway/notify
- [ ] Postgres SSL (`POSTGRES_SSL_MODE=require`) + managed Postgres backups
- [ ] Kafka RF ≥ 3, topic retention sized for outbox lag recovery
- [ ] S3/MinIO credentials rotated; bucket private; MIME/size limits set
- [ ] Metrics bound to `127.0.0.1` or scraped only via NetworkPolicy
- [ ] `WS_ALLOWED_ORIGINS` is an explicit allow-list (never `*`)
- [ ] `GATEWAY_ALLOW_QUERY_TOKEN=false` (Authorization header only)
- [ ] Server gRPC TLS on (`GRPC_TLS_CERT_FILE` / `GRPC_TLS_KEY_FILE`); clients `*_GRPC_TLS=true` + CA
- [ ] `PUSH_PROVIDER=fcm` with mounted credentials; invalid tokens unregister
- [ ] TLS edge: nginx ([deploy/nginx/messanger.conf.example](nginx/messanger.conf.example)) or ingress
- [ ] Envoy grpc-json descriptor applied (`task proto:descriptor`)
- [ ] ValidateCredentials **not** in public Envoy HTTP routes
- [ ] NetworkPolicies applied ([deploy/k8s/10-network-policies.yaml](k8s/10-network-policies.yaml))
- [ ] HPA/PDB applied ([12-hpa.yaml](k8s/12-hpa.yaml), [13-pdb.yaml](k8s/13-pdb.yaml))
- [ ] Gateway readiness `/readyz`, liveness `/healthz`, `preStop` → `/internal/drain` (localhost)
- [ ] Grafana dashboards + Prometheus alerts loaded (`gateway_ws_connections` panel present)
- [ ] CI green: unit + integration (testcontainers) + compose-e2e
- [ ] Smoke + load gates green on staging

## Verify commands (phases A–D)

```bash
# A — correctness unit
(cd gateway && go test -mod=vendor -short ./internal/hub/ ./internal/api/ws/v1/)
(cd chat && go test -mod=vendor -short ./internal/service/chat/ ./internal/repository/outbox/)
(cd notify && go test -mod=vendor -short ./internal/push/)

# B — TLS flags present (compose keeps TLS off)
grep -R GRPC_TLS_CERT_FILE user chat auth media search notify -n | head
grep CHAT_GRPC_CA_FILE gateway/internal/config -n

# C — probes locally
curl -sS -o /dev/null -w '%{http_code}\n' http://localhost:8082/healthz   # 200
curl -sS -o /dev/null -w '%{http_code}\n' http://localhost:8082/readyz    # 200
curl -sS http://127.0.0.1:9104/metrics | grep gateway_ws_connections

# D — integration + e2e
(cd chat && go test -mod=vendor ./internal/repository/outbox/ -run TestOutboxSkipLocked -count=1)
(cd gateway && go test -mod=vendor ./internal/integration/ -count=1)
task smoke
(cd scripts/e2e_full_stack && go run .)

# Prodguard smoke (must fail with weak secrets)
APP_ENV=production JWT_SECRET=secret REFRESH_TOKEN_SECRET=secret SERVICE_JWT_SECRET=secret \
  REDIS_PASSWORD=x go run ./auth/cmd # expect config error

# Descriptor present
test -s deploy/envoy/messanger_descriptor.pb

# Manual load gate
k6 run -e ACCESS_TOKEN="$ACCESS_TOKEN" scripts/load/ws_fanout.js   # see scripts/load/README.md
```

## Local vs production

| Area | Local (Colima Compose) | Production |
|------|------------------------|------------|
| Runtime | `task up-all` | k8s + managed PG/Kafka/Redis/S3 |
| Secrets | `deploy/env/.env` | External Secrets / sealed |
| Edge | plaintext envoy:8080 | nginx TLS → envoy + WSS gateway |
| gRPC | published for smoke | ClusterIP + server TLS + NetworkPolicy |
| Push | `PUSH_PROVIDER=mock` | `fcm` |

Local development uses Docker Compose only. Kubernetes manifests under `deploy/k8s/` are for staging/prod clusters.
Managed cloud provisioning remains an ops step outside this repo.
