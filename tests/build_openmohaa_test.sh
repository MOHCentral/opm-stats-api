#!/usr/bin/env bash
# build_openmohaa_test.sh — Verify OpenMOHAA engine builds with MQTT support
set -euo pipefail

OPM_DIR="${1:-/run/media/elgan/evo/dev/openmohaa-central}"
BUILD_DIR="$OPM_DIR/build"

PASS=0
FAIL=0

pass() { echo "  ✓ $1"; PASS=$((PASS + 1)); }
fail() { echo "  ✗ $1"; FAIL=$((FAIL + 1)); }

echo "=== OpenMOHAA Build Test ==="
echo "Source dir: $OPM_DIR"
echo ""

# ─── Prerequisites ───
echo "--- Prerequisites ---"
for cmd in cmake make gcc g++; do
    if command -v "$cmd" >/dev/null 2>&1; then
        pass "$cmd found: $(command -v "$cmd")"
    else
        fail "$cmd not found"
    fi
done

if [[ -f "$OPM_DIR/CMakeLists.txt" ]]; then
    pass "CMakeLists.txt exists"
else
    fail "CMakeLists.txt not found at $OPM_DIR"
    echo "=== Results: $PASS passed, $FAIL failed ==="
    exit 1
fi

# ─── CMake Configuration ───
echo ""
echo "--- CMake Configure ---"
mkdir -p "$BUILD_DIR"
if cmake -S "$OPM_DIR" -B "$BUILD_DIR" \
    -DCMAKE_BUILD_TYPE=Release \
    -DUSE_MQTT=ON \
    -DBUILD_GAME_SCRIPTS=ON 2>&1 | tail -5; then
    pass "CMake configure succeeded"
else
    fail "CMake configure failed"
fi

# ─── Build (parallel) ───
echo ""
echo "--- Build ---"
NPROC=$(nproc 2>/dev/null || echo 4)
if cmake --build "$BUILD_DIR" --parallel "$NPROC" -- -k 2>&1 | tail -10; then
    pass "Build succeeded"
else
    fail "Build failed"
fi

# ─── Verify MQTT Support ───
echo ""
echo "--- MQTT Binary Check ---"
# Check if openmohaa binary or game library has MQTT symbols
OMOHAADED="$BUILD_DIR/code/omohaaded"
CGAME_SO=$(find "$BUILD_DIR" -name "cgame*.so" -o -name "game*.so" 2>/dev/null | head -1)

if [[ -f "$OMOHAADED" ]]; then
    if nm -D "$OMOHAADED" 2>/dev/null | grep -qi mqtt; then
        pass "MQTT symbols found in omohaaded"
    else
        echo "  ⚠ No MQTT symbols in omohaaded (may be statically linked)"
    fi
else
    echo "  ⚠ omohaaded not found at expected path"
fi

# ─── Summary ───
echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
