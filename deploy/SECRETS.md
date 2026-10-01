# Secrets

Never commit real production secrets. Use `deploy/env/.env.template` for local/dev and
`deploy/env/.env.production.example` as the production placeholder map.

## Local vs production matrix

| Secret / setting | Local (Compose / Colima) | Production (k8s) |
|------------------|--------------------------|------------------|
| `APP_ENV` | `development` | `production` (prodguard enforced) |
| `JWT_SECRET` / `REFRESH_TOKEN_SECRET` / `SERVICE_JWT_SECRET` | weak OK for local | strong, independent; External Secrets |
| `REDIS_PASSWORD` | empty / local | required |
| Postgres | compose service, SSL off | managed PG, `POSTGRES_SSL_MODE=require` |
| `PUSH_PROVIDER` | `mock` | `fcm` + mounted `FCM_CREDENTIALS_FILE` |
| gRPC TLS (`GRPC_TLS_CERT_FILE` / `KEY`) | **off** (plaintext) | **on** server TLS; clients set `*_GRPC_TLS=true` + CA |
| `WS_ALLOWED_ORIGINS` | `*` OK | explicit allow-list |
| `GATEWAY_ALLOW_QUERY_TOKEN` | `true` OK | `false` |
| Metrics bind | `0.0.0.0` OK | `127.0.0.1` or NetworkPolicy |
| Secret delivery | `deploy/env/.env` + `task env:generate` | [external-secrets.example.yaml](k8s/external-secrets.example.yaml) |

## Required secrets

| Name | Used by | Notes |
|------|---------|-------|
| `JWT_SECRET` | auth, user, chat, gateway, media, search, notify | Access token HMAC |
| `REFRESH_TOKEN_SECRET` | auth | Refresh token HMAC — **must differ** from JWT_SECRET |
| `SERVICE_JWT_SECRET` | all internal callers | Service-to-service JWT |
| `REDIS_PASSWORD` | auth, chat, gateway, notify | Required in production |
| Postgres passwords | user, chat, media, notify | Per-database |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | media | Prefer IAM roles in cloud |
| `FCM_CREDENTIALS_FILE` | notify when `PUSH_PROVIDER=fcm` | Mount as secret volume |
| `GRPC_TLS_CERT_FILE` / `GRPC_TLS_KEY_FILE` / optional `GRPC_TLS_CLIENT_CA` | all gRPC servers | Off in compose; on in prod |
| `USER_GRPC_CA_FILE` / `CHAT_GRPC_CA_FILE` | auth→user, gateway→chat | When client TLS enabled |

## Rotation

1. Issue new secret in vault / External Secrets.
2. Roll service Deployments (new pods pick up secret).
3. For JWT rotation: dual-read window if needed; otherwise force re-login.
4. Rotate Redis password with brief maintenance or dual-password Redis ACL.
5. Invalidate old refresh JTIs by flushing `refresh:*` keys only after access TTL expires.

## Verify

```bash
# Templates render
task env:generate

# k8s example (no real values in git)
cp deploy/k8s/00-namespace-secrets.example.yaml deploy/k8s/00-namespace-secrets.local.yaml
# edit local file; it is gitignored

# Confirm production env refuses weak values
grep -n CHANGE_ME deploy/env/.env.production.example

# External Secrets skeleton
kubectl apply --dry-run=client -f deploy/k8s/external-secrets.example.yaml
```

## Kubernetes

- Example: [deploy/k8s/00-namespace-secrets.example.yaml](k8s/00-namespace-secrets.example.yaml)
- Prefer **External Secrets Operator**: [external-secrets.example.yaml](k8s/external-secrets.example.yaml)
- Envoy ConfigMap includes the protobuf descriptor: [deploy/k8s/envoy-configmap.yaml](k8s/envoy-configmap.yaml) (regenerate via `task proto:descriptor`).
