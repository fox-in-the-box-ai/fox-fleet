#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
FLEET_PORT="${FLEET_PORT:-19090}"
FLEET_DATA="${TMPDIR:-/tmp}/fox-fleet-e2e-$$"
FLEET_PID=""

export E2E_USERNAME="e2e-admin"
export E2E_PASSWORD="e2e-password-8f3a"
ADMIN_SECRET="e2e-admin-secret-00ff"
SIGNING_KEY="e2e-signing-key-must-be-32bytes!"

cleanup() {
  if [ -n "$FLEET_PID" ] && kill -0 "$FLEET_PID" 2>/dev/null; then
    kill "$FLEET_PID" 2>/dev/null || true
    wait "$FLEET_PID" 2>/dev/null || true
  fi
  rm -rf "$FLEET_DATA"
  rm -f "$REPO_ROOT/fox-control"
}
trap cleanup EXIT

echo "==> Building fox-control..."
cd "$REPO_ROOT"
go build -ldflags "-X main.buildVersion=e2e" -o "$REPO_ROOT/fox-control" ./cmd/fox-control

echo "==> Preparing data dir..."
mkdir -p "$FLEET_DATA"

DOCKER_SOCKET="/var/run/docker.sock"
if [ -S "$HOME/.colima/default/docker.sock" ]; then
  DOCKER_SOCKET="$HOME/.colima/default/docker.sock"
fi

cat > "$FLEET_DATA/config.toml" <<TOML
[control]
listen = "127.0.0.1:${FLEET_PORT}"
data_root = "${FLEET_DATA}"

[auth]
admin_secret = "${ADMIN_SECRET}"
signing_key = "${SIGNING_KEY}"
instance_password = "e2e-inst-pass"

[docker]
image = "ghcr.io/fox-in-the-box-ai/cloud:stable"
socket = "${DOCKER_SOCKET}"

[cloud]
enabled = true
domain = "fleet.test"
login_rate_limit = 100

[qdrant]
enabled = false

[data_plane]
enabled = false
TOML

echo "==> Starting fox-control (port $FLEET_PORT)..."
"$REPO_ROOT/fox-control" serve --config "$FLEET_DATA/config.toml" &
FLEET_PID=$!

echo "==> Waiting for /healthz..."
for i in $(seq 1 30); do
  if curl -sf "http://127.0.0.1:${FLEET_PORT}/healthz" > /dev/null 2>&1; then
    echo "    Fleet healthy after ${i}s"
    break
  fi
  if ! kill -0 "$FLEET_PID" 2>/dev/null; then
    echo "ERROR: fox-control exited prematurely"
    exit 1
  fi
  sleep 1
done

if ! curl -sf "http://127.0.0.1:${FLEET_PORT}/healthz" > /dev/null 2>&1; then
  echo "ERROR: Fleet not healthy after 30s"
  exit 1
fi

echo "==> Creating test user..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST "http://127.0.0.1:${FLEET_PORT}/api/users" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${ADMIN_SECRET}" \
  -d "{\"username\":\"${E2E_USERNAME}\",\"password\":\"${E2E_PASSWORD}\"}")

if [ "$HTTP_CODE" = "201" ] || [ "$HTTP_CODE" = "409" ]; then
  echo "    User ready (HTTP $HTTP_CODE)"
else
  echo "ERROR: Failed to create user (HTTP $HTTP_CODE)"
  exit 1
fi

export FLEET_BASE_URL="http://127.0.0.1:${FLEET_PORT}"
echo "==> Fleet ready at $FLEET_BASE_URL"
echo ""
echo "==> Running Playwright tests..."
cd "$SCRIPT_DIR"
npx playwright test "$@"
EXIT_CODE=$?

echo "==> Tests finished (exit code $EXIT_CODE)"
exit $EXIT_CODE
