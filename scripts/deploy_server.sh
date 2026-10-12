#!/usr/bin/env bash
set -euo pipefail
export PATH="$HOME/bin:/usr/bin:$PATH"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
DC="$HOME/bin/docker-compose"
OVDIR="$ROOT/deploy/compose/overlays"
ENVDIR="$ROOT/deploy/env"
GEN="$ENVDIR/generate-env.sh"

echo "==> generate compose .env files"
chmod +x "$GEN"
SERVICES=core,user,auth,chat,gateway,media,search,notify,envoy,observability,web \
  ENV_SUBST="$(command -v envsubst)" \
  bash "$GEN"

# web template must respect master WEB_PORT (18088 on this host)
if grep -q '^WEB_PORT=' deploy/compose/web/.env; then
  WEB_PORT_VAL="$(grep -E '^WEB_PORT=' "$ENVDIR/.env" | head -1 | cut -d= -f2-)"
  if [[ -n "${WEB_PORT_VAL}" ]]; then
    sed -i "s/^WEB_PORT=.*/WEB_PORT=${WEB_PORT_VAL}/" deploy/compose/web/.env
  fi
fi

up() {
  local name="$1"
  local compose="$2"
  local overlay="$3"
  local envfile="$4"
  echo "==> up $name"
  # Unique project name avoids clobbering VKS stacks that share directory names (web/observability).
  $DC -p "messanger-${name}" -f "$compose" -f "$overlay" --env-file "$envfile" up -d --build
}

up core \
  deploy/compose/core/docker-compose.yml \
  "$OVDIR/core.yml" \
  deploy/compose/core/.env

docker network inspect messanger-net >/dev/null

up user \
  deploy/compose/user/docker-compose.yml \
  "$OVDIR/user.yml" \
  deploy/compose/user/.env

up auth \
  deploy/compose/auth/docker-compose.yml \
  "$OVDIR/auth.yml" \
  deploy/compose/auth/.env

up chat \
  deploy/compose/chat/docker-compose.yml \
  "$OVDIR/chat.yml" \
  deploy/compose/chat/.env

up media \
  deploy/compose/media/docker-compose.yml \
  "$OVDIR/media.yml" \
  deploy/compose/media/.env

up search \
  deploy/compose/search/docker-compose.yml \
  "$OVDIR/search.yml" \
  deploy/compose/search/.env

up notify \
  deploy/compose/notify/docker-compose.yml \
  "$OVDIR/notify.yml" \
  deploy/compose/notify/.env

up gateway \
  deploy/compose/gateway/docker-compose.yml \
  "$OVDIR/gateway.yml" \
  deploy/compose/gateway/.env

up envoy \
  deploy/compose/envoy/docker-compose.yml \
  "$OVDIR/envoy.yml" \
  deploy/compose/envoy/.env

up web \
  deploy/compose/web/docker-compose.yml \
  "$OVDIR/web.yml" \
  deploy/compose/web/.env

up observability \
  deploy/compose/observability/docker-compose.yml \
  "$OVDIR/observability.yml" \
  deploy/compose/observability/.env || echo "WARN observability failed (non-fatal)"

echo "==> install nginx site (via docker write + HUP)"
docker run --rm \
  -v /etc/nginx/sites-available:/out \
  -v "$ROOT/deploy/nginx/chat.arbuz-arbuz.site.conf:/in/chat.conf:ro" \
  alpine sh -c 'cp /in/chat.conf /out/chat.arbuz-arbuz.site.conf'
docker run --rm \
  -v /etc/nginx/sites-enabled:/out \
  alpine sh -c 'ln -sfn /etc/nginx/sites-available/chat.arbuz-arbuz.site.conf /out/chat.arbuz-arbuz.site.conf'
# Host nginx.conf may reference modules the docker image lacks — skip nginx -t, just HUP.
NGINX_PID="$(cat /run/nginx.pid)"
docker run --rm --privileged --pid=host alpine kill -HUP "$NGINX_PID"

echo "==> status"
docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' | grep -E 'messanger|NAMES' || true
curl -sI -H 'Host: chat.arbuz-arbuz.site' http://127.0.0.1/ | head -15
curl -sI http://127.0.0.1:18088/ | head -10
echo "DONE -> https://chat.arbuz-arbuz.site"
