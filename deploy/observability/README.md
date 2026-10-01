# Messanger observability

- Prometheus scrapes service `:910x` metrics (see compose `prometheus.yml`).
- Alerts: `alerts.yml` (outbox backlog, auth errors, search index, WS delta).
- Grafana dashboard: `grafana-dashboard.json` (auto-provisioned in compose).

Key metrics:
- `outbox_pending{service=}`
- `outbox_publish_errors_total{service=,stage=}`
- `gateway_ws_connections`
- `gateway_ws_messages_total{direction=}`
- `gateway_ws_disconnects_total`
- `gateway_fanout_errors_total`
- `notify_push_sent_total{provider=,result=}`
- `auth_grpc_requests_total{method=,code=}`
- `search_index_errors_total`

Probes: gateway liveness `/healthz` (always 200), readiness `/readyz` (503 when draining / Redis down).
Prometheus `evaluation_interval` is set in compose; scrape configs stay in `prometheus.yml`.
