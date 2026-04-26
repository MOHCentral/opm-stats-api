#!/usr/bin/env bash
# build_api_test.sh — Verify Go API builds and passes basic checks
set -euo pipefail

API_DIR="${1:-$(cd "$(dirname "$0")/.." && pwd)}"
cd "$API_DIR"

PASS=0
FAIL=0

pass() { echo "  ✓ $1"; PASS=$((PASS + 1)); }
fail() { echo "  ✗ $1"; FAIL=$((FAIL + 1)); }

echo "=== API Build Test ==="
echo "API dir: $API_DIR"
echo ""

# ─── Build ───
echo "--- Build ---"
if go build -o /dev/null ./cmd/api/; then
    pass "go build ./cmd/api/"
else
    fail "go build ./cmd/api/"
fi

if go build -o /dev/null ./cmd/seeder/; then
    pass "go build ./cmd/seeder/"
else
    fail "go build ./cmd/seeder/"
fi

# ─── Vet ───
echo ""
echo "--- Vet ---"
if go vet ./...; then
    pass "go vet ./..."
else
    fail "go vet ./..."
fi

# ─── Unit Tests ───
echo ""
echo "--- Unit Tests ---"
if go test ./internal/mqtt/ -v -count=1; then
    pass "MQTT subscriber unit tests"
else
    fail "MQTT subscriber unit tests"
fi

# ─── Dependencies ───
echo ""
echo "--- Dependencies ---"
if go mod verify; then
    pass "go mod verify"
else
    fail "go mod verify"
fi

if go mod tidy -diff 2>/dev/null; then
    pass "go.mod is tidy"
else
    warn_str="go.mod may need tidy"
    echo "  ⚠ $warn_str"
fi

# ─── Summary ───
echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
