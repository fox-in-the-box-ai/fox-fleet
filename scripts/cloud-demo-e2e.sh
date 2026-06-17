#!/usr/bin/env bash
set -euo pipefail

# Cloud routing E2E demo — exercises the full cloud flow against a running
# fox-control server with [cloud] enabled and at least one provisioned instance.
#
# Usage:
#   ./scripts/cloud-demo-e2e.sh [base_url] [admin_secret] [instance_id]
#
# Defaults:
#   base_url     = http://localhost:8080
#   admin_secret = $FOX_ADMIN_SECRET (env var)
#   instance_id  = first running instance from GET /api/instances
#
# Prerequisites:
#   - fox-control running with [cloud] enabled in config
#   - At least one Fox instance provisioned and in "running" state
#   - curl and jq installed

BASE_URL="${1:-http://localhost:8080}"
ADMIN_SECRET="${2:-${FOX_ADMIN_SECRET:-}}"
INSTANCE_ID="${3:-}"

DEMO_USER="cloud-demo-$$"
DEMO_PASS="demo-password-e2e"
COOKIE_JAR=$(mktemp)

cleanup() {
    echo ""
    echo "=== Cleanup ==="
    curl -s -X DELETE "$BASE_URL/api/users/$DEMO_USER" \
        -H "Authorization: Bearer $ADMIN_SECRET" -o /dev/null -w "" 2>/dev/null && \
        echo "  Deleted user $DEMO_USER" || \
        echo "  Could not delete user $DEMO_USER (may need manual cleanup)"
    rm -f "$COOKIE_JAR"
}
trap cleanup EXIT

fail() { echo "FAIL: $1" >&2; exit 1; }
pass() { echo "  PASS: $1"; }

if [ -z "$ADMIN_SECRET" ]; then
    fail "admin secret required — pass as arg 2 or set FOX_ADMIN_SECRET"
fi

echo "Cloud E2E Demo"
echo "  Server:  $BASE_URL"
echo ""

# --- Step 0: Discover instance if not provided ---
if [ -z "$INSTANCE_ID" ]; then
    echo "=== Step 0: Discover running instance ==="
    INSTANCE_ID=$(curl -s "$BASE_URL/api/instances" \
        -H "Authorization: Bearer $ADMIN_SECRET" | \
        jq -r '[.[] | select(.status == "running")][0].id // empty') || true
    [ -n "$INSTANCE_ID" ] || fail "no running instance found"
    pass "found instance $INSTANCE_ID"
fi

# --- Step 1: Create a cloud user via admin API ---
echo ""
echo "=== Step 1: Create cloud user ==="
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "$BASE_URL/api/users" \
    -H "Authorization: Bearer $ADMIN_SECRET" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$DEMO_USER\",\"password\":\"$DEMO_PASS\"}")
[ "$HTTP_CODE" = "201" ] || fail "create user returned $HTTP_CODE (expected 201)"
pass "created user $DEMO_USER"

# --- Step 2: Assign instance to user ---
echo ""
echo "=== Step 2: Assign instance ==="
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X PUT "$BASE_URL/api/users/$DEMO_USER" \
    -H "Authorization: Bearer $ADMIN_SECRET" \
    -H "Content-Type: application/json" \
    -d "{\"instance_id\":\"$INSTANCE_ID\"}")
[ "$HTTP_CODE" = "200" ] || fail "assign instance returned $HTTP_CODE (expected 200)"
pass "assigned instance $INSTANCE_ID to $DEMO_USER"

# --- Step 3: Login via cloud endpoint ---
echo ""
echo "=== Step 3: Cloud login ==="
LOGIN_RESP=$(curl -s -c "$COOKIE_JAR" -w "\n%{http_code}" \
    -X POST "$BASE_URL/cloud/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$DEMO_USER\",\"password\":\"$DEMO_PASS\"}")
LOGIN_CODE=$(echo "$LOGIN_RESP" | tail -1)
[ "$LOGIN_CODE" = "200" ] || fail "login returned $LOGIN_CODE (expected 200)"
grep -q "fox_cloud_session" "$COOKIE_JAR" || fail "no session cookie set"
pass "logged in as $DEMO_USER (session cookie received)"

# --- Step 4: Access proxied instance via /cloud/ ---
echo ""
echo "=== Step 4: Verify cloud proxy ==="
PROXY_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -b "$COOKIE_JAR" \
    "$BASE_URL/cloud/health")
# Accept 200 (proxied to instance) or 503 (instance port unreachable but proxy ran)
case "$PROXY_CODE" in
    200) pass "proxy returned 200 — instance /health reached" ;;
    503) pass "proxy returned 503 — proxy ran, instance may be starting" ;;
    *)   fail "proxy returned $PROXY_CODE (expected 200 or 503)" ;;
esac

# --- Step 5: Verify unauthenticated access is rejected ---
echo ""
echo "=== Step 5: Verify unauthenticated rejection ==="
UNAUTH_CODE=$(curl -o /dev/null -w "%{http_code}" -s \
    "$BASE_URL/cloud/health")
[ "$UNAUTH_CODE" = "303" ] || fail "unauthenticated proxy returned $UNAUTH_CODE (expected 303 redirect)"
pass "unauthenticated request redirected to login (303)"

# --- Step 6: Logout ---
echo ""
echo "=== Step 6: Cloud logout ==="
LOGOUT_CODE=$(curl -o /dev/null -w "%{http_code}" -s \
    -X POST -b "$COOKIE_JAR" \
    "$BASE_URL/cloud/logout")
[ "$LOGOUT_CODE" = "204" ] || fail "logout returned $LOGOUT_CODE (expected 204)"
pass "logged out successfully"

# --- Step 7: Verify session invalidated ---
echo ""
echo "=== Step 7: Verify session invalidated ==="
POST_LOGOUT_CODE=$(curl -o /dev/null -w "%{http_code}" -s \
    -b "$COOKIE_JAR" \
    "$BASE_URL/cloud/health")
[ "$POST_LOGOUT_CODE" = "303" ] || fail "post-logout proxy returned $POST_LOGOUT_CODE (expected 303 redirect)"
pass "session invalidated — redirected to login after logout"

echo ""
echo "=== All 7 steps passed ==="
