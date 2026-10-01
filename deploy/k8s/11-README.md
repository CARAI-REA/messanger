# Messanger Kubernetes scaffold (future cluster)

Local development uses **Docker Compose + Colima only**.  
These manifests are for a later staging/prod cluster — do not require kind/minikube locally.

## Apply order

```bash
# Secrets: copy example → local (gitignored) or use External Secrets
cp 00-namespace-secrets.example.yaml 00-namespace-secrets.local.yaml
# edit, then:
kubectl apply -f 00-namespace-secrets.local.yaml

kubectl apply -f envoy-configmap.yaml       # includes grpc-json descriptor
kubectl apply -f 09-infra.yaml              # staging stubs only; prod = managed
kubectl apply -f 01-user.yaml
kubectl apply -f 02-auth.yaml
kubectl apply -f 03-chat.yaml
kubectl apply -f 04-gateway.yaml            # includes preStop drain
kubectl apply -f 05-media.yaml
kubectl apply -f 06-search.yaml
kubectl apply -f 07-notify.yaml
kubectl apply -f 08-envoy.yaml
kubectl apply -f 10-network-policies.yaml
kubectl apply -f 12-hpa.yaml
kubectl apply -f 13-pdb.yaml
```

## Production notes

- **Scrape metrics** via Prometheus sidecar/DaemonSet on the private network; do not expose `:910x` publicly. Prefer `METRICS_HOST=127.0.0.1` + localhost scrape, or NetworkPolicy allow-list from Prometheus pods.
- **Gateway drain**: Deployment `preStop` calls `POST /internal/drain` so WS connections wind down before SIGTERM.
- **Managed Kafka**: replication factor ≥ 3; monitor consumer lag alerts in `deploy/observability/alerts.yml`.
- **Managed Postgres / Redis / S3 / OpenSearch**: point env at managed endpoints (`deploy/env/.env.production.example`); Chainguard MinIO is for Compose only.
- **Edge**: TLS at nginx ([../nginx/messanger.conf.example](../nginx/messanger.conf.example)) → Envoy `:8080` (REST) + gateway `:8082` (WSS).

Images expected as `messanger/<svc>:latest` (same binaries as Compose Dockerfiles).

See also: [../PROD_CHECKLIST.md](../PROD_CHECKLIST.md), [../SECRETS.md](../SECRETS.md).
