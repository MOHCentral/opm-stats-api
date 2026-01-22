#!/bin/bash
# ==============================================================================
# MOHAA Login Token Integration Test
# ==============================================================================
# Simulates the complete login flow from tracker.scr perspective
#
# Usage:
#   ./tests/integration_login.sh
#   API_URL=http://api:8080 ./tests/integration_login.sh
#   FORUM_USER_ID=5 ./tests/integration_login.sh
# ==============================================================================

set -euo pipefail

# Configuration
API_URL="${API_URL:-http://localhost:8080}"
FORUM_USER_ID="${FORUM_USER_ID:-1}"
PLAYER_GUID="${PLAYER_GUID:-test-player-$(date +%s)}"
SERVER_NAME="${SERVER_NAME:-Test Server}"
SERVER_ADDRESS="${SERVER_ADDRESS:-127.0.0.1:12203}"
PLAYER_IP="${PLAYER_IP:-192.168.1.100}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; ((TESTS_PASSED++)); }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; ((TESTS_FAILED++)); }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

# JSON parsing helper (requires jq)
json_get() {
    echo "$1" | python3 -c "import sys, json; print(json.load(sys.stdin).get('$2', ''))" 2>/dev/null || echo ""
}

# ==============================================================================
# Test Functions
# ==============================================================================

test_api_health() {
    log_info "Testing API health..."
    
    RESPONSE=$(curl -s -w "\n%{http_code}" "$API_URL/health" 2>&1)
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | head -n-1)
    
    if [[ "$HTTP_CODE" == "200" ]] && echo "$BODY" | grep -q '"status":"ok"'; then
        log_pass "API is healthy"
        return 0
    else
        log_fail "API health check failed (HTTP $HTTP_CODE)"
        return 1
    fi
}

test_generate_token() {
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/v1/auth/device" \
        -H "Content-Type: application/json" \
        -d "{\"forum_user_id\": $FORUM_USER_ID, \"regenerate\": true}" 2>&1)
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | head -n-1)
    
    if [[ "$HTTP_CODE" == "200" ]]; then
        TOKEN=$(json_get "$BODY" "user_code")
        if [[ -n "$TOKEN" ]]; then
            echo "$TOKEN"
            return 0
        fi
    fi
    
    return 1
}

test_verify_token() {
    local TOKEN="$1"
    log_info "Verifying token '$TOKEN' (simulating tracker.scr /login)..."
    
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/v1/auth/verify" \
        -H "Content-Type: application/json" \
        -d "{
            \"token\": \"$TOKEN\",
            \"player_guid\": \"$PLAYER_GUID\",
            \"server_name\": \"$SERVER_NAME\",
            \"server_address\": \"$SERVER_ADDRESS\",
            \"player_ip\": \"$PLAYER_IP\"
        }" 2>&1)
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    BODY=$(echo "$RESPONSE" | head -n-1)
    
    if [[ "$HTTP_CODE" == "200" ]] && echo "$BODY" | grep -q '"valid":true'; then
        LINKED_USER=$(json_get "$BODY" "forum_user_id")
        log_pass "Token verified! Linked to forum user $LINKED_USER"
        return 0
    else
        log_fail "Token verification failed (HTTP $HTTP_CODE): $BODY"
        return 1
    fi
}

test_token_reuse_blocked() {
    local TOKEN="$1"
    log_info "Testing token reuse prevention..."
    
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/v1/auth/verify" \
        -H "Content-Type: application/json" \
        -d "{
            \"token\": \"$TOKEN\",
            \"player_guid\": \"$PLAYER_GUID\",
            \"server_name\": \"$SERVER_NAME\",
            \"server_address\": \"$SERVER_ADDRESS\",
            \"player_ip\": \"$PLAYER_IP\"
        }" 2>&1)
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    
    if [[ "$HTTP_CODE" == "401" ]]; then
        log_pass "Token reuse correctly blocked"
        return 0
    else
        log_fail "Token reuse should be blocked (got HTTP $HTTP_CODE)"
        return 1
    fi
}

test_invalid_token() {
    log_info "Testing invalid token rejection..."
    
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/v1/auth/verify" \
        -H "Content-Type: application/json" \
        -d "{
            \"token\": \"INVALID-TOKEN-XYZ\",
            \"player_guid\": \"$PLAYER_GUID\",
            \"server_name\": \"$SERVER_NAME\",
            \"server_address\": \"$SERVER_ADDRESS\",
            \"player_ip\": \"$PLAYER_IP\"
        }" 2>&1)
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    
    if [[ "$HTTP_CODE" == "401" ]]; then
        log_pass "Invalid token correctly rejected"
        return 0
    else
        log_fail "Invalid token should be rejected (got HTTP $HTTP_CODE)"
        return 1
    fi
}

test_missing_token() {
    log_info "Testing missing token validation..."
    
    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/api/v1/auth/verify" \
        -H "Content-Type: application/json" \
        -d "{\"player_guid\": \"$PLAYER_GUID\"}" 2>&1)
    
    HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
    
    if [[ "$HTTP_CODE" == "400" ]]; then
        log_pass "Missing token correctly rejected"
        return 0
    else
        log_fail "Missing token should return 400 (got HTTP $HTTP_CODE)"
        return 1
    fi
}

test_database_records() {
    log_info "Checking database records..."
    
    # Check if postgres container is accessible
    if ! docker ps --format '{{.Names}}' | grep -q "postgres"; then
        log_warn "PostgreSQL container not found, skipping database checks"
        return 0
    fi
    
    # Check login_tokens table
    TOKEN_COUNT=$(docker exec opm-stats-system-postgres-1 psql -U mohaa -d mohaa_stats -t -c \
        "SELECT COUNT(*) FROM login_tokens WHERE forum_user_id = $FORUM_USER_ID;" 2>/dev/null | tr -d ' ')
    
    if [[ "$TOKEN_COUNT" -gt 0 ]]; then
        log_pass "Found $TOKEN_COUNT token(s) for forum user $FORUM_USER_ID"
    else
        log_fail "No tokens found in database"
        return 1
    fi
    
    # Check login_token_history table
    HISTORY_COUNT=$(docker exec opm-stats-system-postgres-1 psql -U mohaa -d mohaa_stats -t -c \
        "SELECT COUNT(*) FROM login_token_history WHERE forum_user_id = $FORUM_USER_ID;" 2>/dev/null | tr -d ' ')
    
    if [[ "$HISTORY_COUNT" -gt 0 ]]; then
        log_pass "Found $HISTORY_COUNT history record(s) for forum user $FORUM_USER_ID"
    else
        log_warn "No history records found (may be expected for first run)"
    fi
    
    # Check smf_user_mappings table
    MAPPING=$(docker exec opm-stats-system-postgres-1 psql -U mohaa -d mohaa_stats -t -c \
        "SELECT primary_guid FROM smf_user_mappings WHERE smf_member_id = $FORUM_USER_ID;" 2>/dev/null | tr -d ' ')
    
    if [[ -n "$MAPPING" ]]; then
        log_pass "GUID mapping found: $MAPPING"
    else
        log_warn "No GUID mapping found (may be expected)"
    fi
    
    return 0
}

# ==============================================================================
# Main
# ==============================================================================

echo ""
echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║           MOHAA Login Token Integration Tests                ║${NC}"
echo -e "${BLUE}╠══════════════════════════════════════════════════════════════╣${NC}"
echo -e "${BLUE}║${NC}  API URL:       $API_URL"
echo -e "${BLUE}║${NC}  Forum User ID: $FORUM_USER_ID"
echo -e "${BLUE}║${NC}  Player GUID:   $PLAYER_GUID"
echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Run tests
test_api_health || exit 1

log_info "Generating login token for forum user $FORUM_USER_ID..."
TOKEN=$(test_generate_token)
if [[ -z "$TOKEN" ]]; then
    log_fail "Cannot continue without a token"
    exit 1
fi
log_pass "Generated token: $TOKEN"

test_verify_token "$TOKEN"
test_token_reuse_blocked "$TOKEN"
test_invalid_token
test_missing_token
test_database_records

# Summary
echo ""
echo -e "${BLUE}══════════════════════════════════════════════════════════════${NC}"
echo -e "  Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "  Tests Failed: ${RED}$TESTS_FAILED${NC}"
echo -e "${BLUE}══════════════════════════════════════════════════════════════${NC}"

if [[ "$TESTS_FAILED" -gt 0 ]]; then
    exit 1
fi

exit 0
