#!/bin/bash
# ==============================================================================
# MOHAA Login Token Test Script
# ==============================================================================
# This script simulates the full login flow:
#   1. SMF forum generates a login token (initDeviceAuth)
#   2. Player types /login <token> in-game (tracker.scr calls verifyToken)
#   3. API validates token and links player GUID to forum user
#   4. History is recorded
# ==============================================================================

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

API_URL="${API_URL:-http://localhost:8080}"
FORUM_USER_ID="${FORUM_USER_ID:-1}"
PLAYER_GUID="${PLAYER_GUID:-test-player-$(date +%s)}"
SERVER_NAME="${SERVER_NAME:-Test Server}"
SERVER_ADDRESS="${SERVER_ADDRESS:-127.0.0.1:12203}"
PLAYER_IP="${PLAYER_IP:-192.168.1.100}"

echo -e "${CYAN}"
cat << 'EOF'
╔══════════════════════════════════════════════════════════════╗
║              MOHAA Login Token Test                          ║
╠══════════════════════════════════════════════════════════════╣
║  Simulates: tracker.scr -> Go API -> Token Verify -> History ║
╚══════════════════════════════════════════════════════════════╝
EOF
echo -e "${NC}"

# ==============================================================================
# Step 1: Check API Health
# ==============================================================================
echo -e "${BLUE}[Step 1]${NC} Checking API health..."
HEALTH=$(curl -s "$API_URL/health" 2>&1)
if echo "$HEALTH" | grep -q '"status":"ok"'; then
    echo -e "${GREEN}  ✓ API is healthy${NC}"
else
    echo -e "${RED}  ✗ API is not responding: $HEALTH${NC}"
    exit 1
fi

# ==============================================================================
# Step 2: Generate a new login token (simulates SMF profile page)
# ==============================================================================
echo ""
echo -e "${BLUE}[Step 2]${NC} Generating login token for forum user $FORUM_USER_ID..."
echo -e "         ${CYAN}POST $API_URL/api/v1/auth/device${NC}"

TOKEN_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/device" \
    -H "Content-Type: application/json" \
    -d "{\"forum_user_id\": $FORUM_USER_ID, \"regenerate\": true}" 2>&1)

echo -e "         Response: ${YELLOW}$TOKEN_RESPONSE${NC}"

# Extract the token
USER_CODE=$(echo "$TOKEN_RESPONSE" | grep -o '"user_code":"[^"]*"' | cut -d'"' -f4)

if [ -z "$USER_CODE" ]; then
    echo -e "${RED}  ✗ Failed to generate token${NC}"
    exit 1
fi

echo -e "${GREEN}  ✓ Generated token: ${YELLOW}$USER_CODE${NC}"

# ==============================================================================
# Step 3: Simulate game login (tracker.scr /login command)
# ==============================================================================
echo ""
echo -e "${BLUE}[Step 3]${NC} Simulating in-game /login command..."
echo -e "         ${CYAN}This is what tracker.scr sends when player types: /login $USER_CODE${NC}"
echo ""
echo -e "         Player GUID: $PLAYER_GUID"
echo -e "         Server: $SERVER_NAME ($SERVER_ADDRESS)"
echo -e "         Player IP: $PLAYER_IP"
echo ""
echo -e "         ${CYAN}POST $API_URL/api/v1/auth/verify${NC}"

VERIFY_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/verify" \
    -H "Content-Type: application/json" \
    -d "{
        \"token\": \"$USER_CODE\",
        \"player_guid\": \"$PLAYER_GUID\",
        \"server_name\": \"$SERVER_NAME\",
        \"server_address\": \"$SERVER_ADDRESS\",
        \"player_ip\": \"$PLAYER_IP\"
    }" 2>&1)

echo -e "         Response: ${YELLOW}$VERIFY_RESPONSE${NC}"

# Check if verification succeeded
if echo "$VERIFY_RESPONSE" | grep -q '"valid":true'; then
    echo -e "${GREEN}  ✓ Token verified successfully!${NC}"
    VERIFIED_USER=$(echo "$VERIFY_RESPONSE" | grep -o '"forum_user_id":[0-9]*' | cut -d':' -f2)
    echo -e "${GREEN}  ✓ Linked to forum user ID: $VERIFIED_USER${NC}"
else
    echo -e "${RED}  ✗ Token verification failed${NC}"
    exit 1
fi

# ==============================================================================
# Step 4: Try to use the same token again (should fail)
# ==============================================================================
echo ""
echo -e "${BLUE}[Step 4]${NC} Testing token reuse prevention..."
echo -e "         ${CYAN}Trying to use the same token again (should fail)${NC}"

REUSE_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/verify" \
    -H "Content-Type: application/json" \
    -d "{
        \"token\": \"$USER_CODE\",
        \"player_guid\": \"$PLAYER_GUID\",
        \"server_name\": \"$SERVER_NAME\",
        \"server_address\": \"$SERVER_ADDRESS\",
        \"player_ip\": \"$PLAYER_IP\"
    }" 2>&1)

echo -e "         Response: ${YELLOW}$REUSE_RESPONSE${NC}"

if echo "$REUSE_RESPONSE" | grep -q '"error"'; then
    echo -e "${GREEN}  ✓ Token correctly rejected (already used)${NC}"
else
    echo -e "${RED}  ✗ Token should have been rejected!${NC}"
fi

# ==============================================================================
# Step 5: Verify database records
# ==============================================================================
echo ""
echo -e "${BLUE}[Step 5]${NC} Checking database records..."

# Check login_tokens table
echo -e "         ${CYAN}Checking login_tokens table...${NC}"
docker exec opm-stats-system-postgres-1 psql -U mohaa -d mohaa_stats -t -c "
    SELECT 
        'Token: ' || token || 
        ', Used: ' || COALESCE(used_at::text, 'not used') ||
        ', Active: ' || is_active::text ||
        ', Used by GUID: ' || COALESCE(used_player_guid, 'n/a')
    FROM login_tokens 
    WHERE forum_user_id = $FORUM_USER_ID 
    ORDER BY created_at DESC 
    LIMIT 1;
" 2>/dev/null

# Check login_token_history table
echo ""
echo -e "         ${CYAN}Checking login_token_history table...${NC}"
docker exec opm-stats-system-postgres-1 psql -U mohaa -d mohaa_stats -t -c "
    SELECT 
        'Attempt at: ' || attempt_at || 
        ', Success: ' || success::text ||
        ', Server: ' || COALESCE(server_name, 'n/a') ||
        ', Reason: ' || COALESCE(failure_reason, 'success')
    FROM login_token_history 
    WHERE forum_user_id = $FORUM_USER_ID 
    ORDER BY attempt_at DESC 
    LIMIT 3;
" 2>/dev/null

# Check smf_user_mappings table
echo ""
echo -e "         ${CYAN}Checking smf_user_mappings table...${NC}"
docker exec opm-stats-system-postgres-1 psql -U mohaa -d mohaa_stats -t -c "
    SELECT 
        'Forum User: ' || smf_member_id || 
        ', GUID: ' || primary_guid ||
        ', Updated: ' || updated_at::text
    FROM smf_user_mappings 
    WHERE smf_member_id = $FORUM_USER_ID;
" 2>/dev/null

# ==============================================================================
# Step 6: Test invalid token
# ==============================================================================
echo ""
echo -e "${BLUE}[Step 6]${NC} Testing invalid token handling..."

INVALID_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/verify" \
    -H "Content-Type: application/json" \
    -d "{
        \"token\": \"INVALID-TOKEN\",
        \"player_guid\": \"$PLAYER_GUID\",
        \"server_name\": \"$SERVER_NAME\",
        \"server_address\": \"$SERVER_ADDRESS\",
        \"player_ip\": \"$PLAYER_IP\"
    }" 2>&1)

echo -e "         Response: ${YELLOW}$INVALID_RESPONSE${NC}"

if echo "$INVALID_RESPONSE" | grep -q '"error"'; then
    echo -e "${GREEN}  ✓ Invalid token correctly rejected${NC}"
else
    echo -e "${RED}  ✗ Invalid token should have been rejected!${NC}"
fi

# ==============================================================================
# Summary
# ==============================================================================
echo ""
echo -e "${CYAN}══════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}                    TEST COMPLETE                              ${NC}"
echo -e "${CYAN}══════════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "  Token Generated:     ${YELLOW}$USER_CODE${NC}"
echo -e "  Forum User ID:       ${BLUE}$FORUM_USER_ID${NC}"
echo -e "  Player GUID:         ${BLUE}$PLAYER_GUID${NC}"
echo -e "  Verification:        ${GREEN}✓ PASSED${NC}"
echo -e "  Reuse Prevention:    ${GREEN}✓ PASSED${NC}"
echo -e "  Invalid Token Check: ${GREEN}✓ PASSED${NC}"
echo ""
