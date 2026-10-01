# Local access

After `task up-all`:

| Component | URL / Port |
|-----------|------------|
| Envoy HTTP | http://localhost:8080 |
| Gateway WS | ws://localhost:8082/ws |
| User gRPC | localhost:50051 |
| Auth gRPC | localhost:50050 |
| Chat gRPC | localhost:50052 |
| Media gRPC | localhost:50053 |
| Search gRPC | localhost:50054 |
| Notify gRPC | localhost:50055 |
| Kafka UI | http://localhost:8089 |
| MinIO API | http://localhost:9000 |
| MinIO Console | http://localhost:9001 |
| OpenSearch | http://localhost:9200 |
| Grafana | http://localhost:3000 (admin/admin) |
| Prometheus | http://localhost:9095 |

Generate env: `task env:generate`
