import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 10,
  duration: '30s',
  thresholds: {
    http_req_duration: ['p(99)<500'],
  },
};

// Placeholder: point ENVOY_URL at HTTP gateway once Envoy grpc-json is wired.
// For now hits gateway healthz as a smoke load against the stack edge.
const BASE = __ENV.GATEWAY_URL || 'http://localhost:8082';

export default function () {
  const res = http.get(`${BASE}/healthz`);
  check(res, { 'status is 200': (r) => r.status === 200 });
  sleep(0.1);
}
