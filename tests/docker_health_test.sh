#!/usr/bin/env bash
# docker_health_test.sh — Verify Docker infrastructure is up and healthy
set -euo pipefail

COMPOSE_DIR="${1:-$(cd "$(dirname "$0")/../opm-stats-api" && pwd)}"
cd "$COMPOSE_DIR"

PASS=0
FAIL=0
WARN=0

pass() { echo "  ✓ $1"; PASS=$((PASS + 1)); }
fail() { echo "  ✗ $1"; FAIL=$((FAIL + 1)); }
warn() { echo "  ⚠ $1"; WARN=$((WARN + 1)); }

echo "=== Docker Infrastructure Health Check ==="
echo "Compose dir: $COMPOSE_DIR"
echo ""

# ─── Docker Compose Services ───
echo "--- Services ---"
for svc in opm-stats-postgres opm-stats-clickhouse opm-stats-redis opm-stats-mosquitto; do
    if docker compose ps --format json 2>/dev/null | grep -q "\"$svc\""; then
        state=$(docker compose ps --format json | jq -r "select(.Name==\"$svc\") | .State" 2>/dev/null || echo "unknown")
        if [[ "$state" == "running" ]]; then
            pass "$svc is running"
        else
            fail "$svc state: $state"
        fi
    else
        fail "$svc not found in compose"
    fi
done

# ─── Port Checks ───
echo ""
echo "--- Ports ---"
check_port() {
    if timeout 2 bash -c "echo >/dev/tcp/localhost/$1" 2>/dev/null; then
        pass "Port $1 ($2) is open"
    else
        fail "Port $1 ($2) is not open"
    fi
}

check_port 5432 "PostgreSQL"
check_port 8123 "ClickHouse HTTP"
check_port 9000 "ClickHouse Native"
check_port 6379 "Redis"
check_port 1883 "MQTT"

# ─── Database Connectivity ───
echo ""
echo "--- Database Connectivity ---"

# PostgreSQL
if docker exec opm-stats-postgres pg_isready -U opm -d opm_stats >/dev/null 2>&1; then
    pass "PostgreSQL pg_isready"
else
    fail "PostgreSQL pg_isready"
fi

# ClickHouse
if curl -sf "http://localhost:8123/?query=SELECT%201" >/dev/null 2>&1; then
    pass "ClickHouse SELECT 1"
else
    fail "ClickHouse SELECT 1"
fi

# Redis
if docker exec opm-stats-redis redis-cli ping 2>/dev/null | grep -q "PONG"; then
    pass "Redis PING"
else
    fail "Redis PING"
fi

# MQTT
if docker exec opm-stats-mosquitto mosquitto_sub -t test -C 1 -W 2 >/dev/null 2>&1; then
    pass "Mosquitto broker responds"
else
    # Alternative: just check port and container
    if timeout 2 bash -c "echo >/dev/tcp/localhost/1883" 2>/dev/null; then
        warn "Mosquitto port open but sub test failed"
    else
        fail "Mosquitto broker unreachable"
    fi
fi

# ─── Summary ───
echo ""
echo "=== Results: $PASS passed, $FAIL failed, $WARN warnings ==="
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
