import ws from 'k6/ws';
import { check } from 'k6';

export const options = {
  vus: 20,
  duration: '30s',
};

const BASE = __ENV.WS_URL || 'ws://localhost:8082/ws';
const TOKEN = __ENV.ACCESS_TOKEN || '';

export default function () {
  const params = TOKEN
    ? { headers: { Authorization: `Bearer ${TOKEN}` } }
    : {};
  const res = ws.connect(BASE, params, function (socket) {
    socket.on('open', () => {
      socket.send(JSON.stringify({ type: 'ping' }));
    });
    socket.setTimeout(() => socket.close(), 5000);
  });
  check(res, { connected: (r) => r && r.status === 101 });
}
