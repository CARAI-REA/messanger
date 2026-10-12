#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
export PATH="$ROOT/bin:$PATH"

echo "== messenger smoke =="

check_tcp() {
  local host="$1" port="$2" name="$3"
  if nc -z "$host" "$port" 2>/dev/null; then
    echo "OK  $name ($host:$port)"
  else
    echo "FAIL $name ($host:$port) not reachable"
    exit 1
  fi
}

check_tcp localhost 50051 user
check_tcp localhost 50050 auth
check_tcp localhost 50052 chat
check_tcp localhost 8082 gateway
check_tcp localhost 50053 media
check_tcp localhost 50054 search
check_tcp localhost 50055 notify

HZ=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8082/healthz || true)
if [[ "$HZ" == "200" ]]; then
  echo "OK  gateway /healthz"
else
  echo "FAIL gateway /healthz ($HZ)"
  exit 1
fi

RZ=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8082/readyz || true)
if [[ "$RZ" == "200" ]]; then
  echo "OK  gateway /readyz"
else
  echo "FAIL gateway /readyz ($RZ)"
  exit 1
fi

EMAIL="smoke_$(date +%s)@example.com"
UNAME="smoke_$(date +%s)"
PASS="password123"
PEER_EMAIL="smoke_peer_$(date +%s)@example.com"
PEER_UNAME="peer_$(date +%s)"

echo "-- Create user --"
CREATE_OUT=$(grpcurl -plaintext -d "{\"user_info\":{\"name\":\"Smoke\",\"email\":\"$EMAIL\",\"username\":\"$UNAME\"},\"password\":\"$PASS\",\"password_confirm\":\"$PASS\"}" \
  localhost:50051 user.v1.UserService/Create)
echo "$CREATE_OUT"
USER_ID=$(echo "$CREATE_OUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',0))")

echo "-- Create peer --"
PEER_OUT=$(grpcurl -plaintext -d "{\"user_info\":{\"name\":\"Peer\",\"email\":\"$PEER_EMAIL\",\"username\":\"$PEER_UNAME\"},\"password\":\"$PASS\",\"password_confirm\":\"$PASS\"}" \
  localhost:50051 user.v1.UserService/Create)
PEER_ID=$(echo "$PEER_OUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',0))")
echo "OK  peer_id=$PEER_ID"

echo "-- Login --"
LOGIN_OUT=$(grpcurl -plaintext -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}" \
  localhost:50050 auth.v1.AuthService/Login)
ACCESS=$(echo "$LOGIN_OUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('accessToken',''))")
REFRESH=$(echo "$LOGIN_OUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('refreshToken',''))")
if [[ -z "$ACCESS" ]]; then
  echo "FAIL no access token"
  exit 1
fi
echo "OK  got access token (user_id=$USER_ID)"

echo "-- GetOrCreateDirect --"
CHAT_OUT=$(grpcurl -plaintext \
  -H "authorization: Bearer $ACCESS" \
  -d "{\"peer_user_id\":$PEER_ID}" \
  localhost:50052 chat.v1.ChatService/GetOrCreateDirectChat)
CHAT_ID=$(echo "$CHAT_OUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('chatId',0))")
echo "OK  chat_id=$CHAT_ID"

echo "-- Send message --"
MSG_OUT=$(grpcurl -plaintext \
  -H "authorization: Bearer $ACCESS" \
  -d "{\"chat_id\":$CHAT_ID,\"text\":\"hello smoke\",\"idempotency_key\":\"smoke-1\"}" \
  localhost:50052 chat.v1.ChatService/SendMessage)
MSG_ID=$(echo "$MSG_OUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('messageId',0))")

echo "-- Idempotent SendMessage --"
MSG2_OUT=$(grpcurl -plaintext \
  -H "authorization: Bearer $ACCESS" \
  -d "{\"chat_id\":$CHAT_ID,\"text\":\"hello smoke\",\"idempotency_key\":\"smoke-1\"}" \
  localhost:50052 chat.v1.ChatService/SendMessage)
MSG2_ID=$(echo "$MSG2_OUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('messageId',0))")
if [[ "$MSG_ID" != "$MSG2_ID" ]]; then
  echo "FAIL idempotency: $MSG_ID vs $MSG2_ID"
  exit 1
fi
echo "OK  idempotent message_id=$MSG_ID"

echo "-- WS connect (Authorization header) --"
export SMOKE_ACCESS="$ACCESS"
python3 - <<'PY'
import base64, os, socket

token = os.environ["SMOKE_ACCESS"]
key = base64.b64encode(os.urandom(16)).decode("ascii")
req = (
    "GET /ws HTTP/1.1\r\n"
    "Host: localhost:8082\r\n"
    "Upgrade: websocket\r\n"
    "Connection: Upgrade\r\n"
    f"Sec-WebSocket-Key: {key}\r\n"
    "Sec-WebSocket-Version: 13\r\n"
    f"Authorization: Bearer {token}\r\n"
    "\r\n"
)
with socket.create_connection(("127.0.0.1", 8082), timeout=5) as s:
    s.sendall(req.encode())
    buf = b""
    while b"\r\n\r\n" not in buf and len(buf) < 8192:
        chunk = s.recv(1024)
        if not chunk:
            break
        buf += chunk
head = buf.split(b"\r\n\r\n", 1)[0].decode("latin1", errors="replace")
status = head.split("\r\n", 1)[0]
if "101" not in status:
    raise SystemExit(f"FAIL ws upgrade: {status}\n{head[:300]}")
print("OK  ws connected with Authorization header")
PY

# Optional second gateway replica (docker compose --scale gateway=2 may not publish second host port)
if nc -z localhost 8083 2>/dev/null; then
  echo "-- Multi-gateway WS (:8083) --"
  export SMOKE_ACCESS="$ACCESS"
  python3 - <<'PY'
import base64, os, socket

token = os.environ["SMOKE_ACCESS"]
key = base64.b64encode(os.urandom(16)).decode("ascii")
req = (
    "GET /ws HTTP/1.1\r\n"
    "Host: localhost:8083\r\n"
    "Upgrade: websocket\r\n"
    "Connection: Upgrade\r\n"
    f"Sec-WebSocket-Key: {key}\r\n"
    "Sec-WebSocket-Version: 13\r\n"
    f"Authorization: Bearer {token}\r\n"
    "\r\n"
)
with socket.create_connection(("127.0.0.1", 8083), timeout=5) as s:
    s.sendall(req.encode())
    buf = b""
    while b"\r\n\r\n" not in buf and len(buf) < 8192:
        chunk = s.recv(1024)
        if not chunk:
            break
        buf += chunk
head = buf.split(b"\r\n\r\n", 1)[0].decode("latin1", errors="replace")
status = head.split("\r\n", 1)[0]
if "101" not in status:
    raise SystemExit(f"FAIL ws upgrade on :8083: {status}\n{head[:300]}")
print("OK  second gateway replica reachable")
PY
else
  echo "SKIP multi-gateway :8083 (run task up-gateway-scaled with distinct host ports to exercise Redis fanout)"
fi

echo "-- List messages --"
grpcurl -plaintext \
  -H "authorization: Bearer $ACCESS" \
  -d "{\"chat_id\":$CHAT_ID,\"limit\":10}" \
  localhost:50052 chat.v1.ChatService/ListMessages >/dev/null
echo "OK  list messages"

echo "-- MarkRead --"
grpcurl -plaintext \
  -H "authorization: Bearer $ACCESS" \
  -d "{\"chat_id\":$CHAT_ID,\"message_id\":$MSG_ID}" \
  localhost:50052 chat.v1.ChatService/MarkRead >/dev/null
echo "OK  mark read"

echo "-- Refresh rotation --"
REFRESH_OUT=$(grpcurl -plaintext -d "{\"refresh_token\":\"$REFRESH\"}" \
  localhost:50050 auth.v1.AuthService/GetRefreshToken)
NEW_REFRESH=$(echo "$REFRESH_OUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('refreshToken',''))")
if [[ -z "$NEW_REFRESH" || "$NEW_REFRESH" == "$REFRESH" ]]; then
  echo "FAIL refresh rotation"
  exit 1
fi
echo "OK  refresh rotated"

# HTTP via Envoy (optional if up)
if nc -z localhost 8080 2>/dev/null; then
  echo "-- Envoy REST login --"
  HTTP_LOGIN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
    -H 'content-type: application/json' \
    -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}")
  HTTP_ACCESS=$(echo "$HTTP_LOGIN" | python3 -c "import sys,json; print(json.load(sys.stdin).get('accessToken',''))" 2>/dev/null || true)
  if [[ -n "$HTTP_ACCESS" ]]; then
    echo "OK  envoy /api/v1/auth/login"
    curl -s -H "authorization: Bearer $HTTP_ACCESS" "http://localhost:8080/api/v1/chats?limit=10" >/dev/null || true
    echo "OK  envoy list chats (best-effort)"
    curl -s -X POST http://localhost:8080/api/v1/media/uploads \
      -H "authorization: Bearer $HTTP_ACCESS" \
      -H 'content-type: application/json' \
      -d '{"filename":"smoke.txt","mime":"text/plain","sizeBytes":12}' >/dev/null || true
    echo "OK  envoy media InitUpload (best-effort)"
    for i in 1 2 3 4 5; do
      SEARCH=$(curl -s -H "authorization: Bearer $HTTP_ACCESS" \
        "http://localhost:8080/api/v1/search/messages?query=hello&chatId=$CHAT_ID&limit=5" || true)
      if echo "$SEARCH" | grep -q hits; then
        echo "OK  envoy search"
        break
      fi
      sleep 1
    done
  else
    echo "WARN envoy login returned unexpected payload (skipping REST checks)"
  fi
else
  echo "SKIP envoy :8080 not up"
fi

echo "smoke finished OK"
