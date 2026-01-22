#!/bin/bash

# Comprehensive API Endpoint Testing Script
# Tests ALL 29+ dashboard stat categories

BASE_URL="http://localhost:8080/api/v1"
# Use Elgan's GUID from database
GUID="72750883-29ae-4377-85c4-9367f1f89d1a"
MAP="v2_rocket"
SERVER_ID="server_001"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

total_tests=0
passed_tests=0
failed_tests=0

test_endpoint() {
    local name="$1"
    local url="$2"
    local expected_non_empty="$3"  # "yes" or "no"
    
    total_tests=$((total_tests + 1))
    echo -e "${BLUE}[$total_tests]${NC} Testing: $name"
    echo -e "   URL: $url"
    
    response=$(curl -s -w "\n%{http_code}" "$url" 2>/dev/null)
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n-1)
    
    # Check HTTP status
    if [ "$http_code" != "200" ]; then
        echo -e "   ${RED}✗ FAILED${NC} - HTTP $http_code"
        failed_tests=$((failed_tests + 1))
        echo "   Response: $body"
        return 1
    fi
    
    # Check if response is valid JSON
    if ! echo "$body" | python3 -m json.tool > /dev/null 2>&1; then
        echo -e "   ${RED}✗ FAILED${NC} - Invalid JSON"
        failed_tests=$((failed_tests + 1))
        echo "   Response: $body"
        return 1
    fi
    
    # Check if empty vs non-empty
    if [ "$expected_non_empty" = "yes" ]; then
        if [ "$body" = "[]" ] || [ "$body" = "{}" ] || [ "$body" = "null" ]; then
            echo -e "   ${YELLOW}⚠ WARNING${NC} - Expected data but got empty response"
            echo "   Response: $body"
        else
            # Count items in response
            item_count=$(echo "$body" | python3 -c "import sys, json; data=json.load(sys.stdin); print(len(data) if isinstance(data, list) else 1)" 2>/dev/null || echo "?")
            echo -e "   ${GREEN}✓ PASSED${NC} - HTTP 200, $item_count items"
            passed_tests=$((passed_tests + 1))
            return 0
        fi
    fi
    
    echo -e "   ${GREEN}✓ PASSED${NC} - HTTP 200, Valid JSON"
    passed_tests=$((passed_tests + 1))
    return 0
}

echo "=========================================="
echo "API ENDPOINT COMPREHENSIVE AUDIT"
echo "=========================================="
echo ""

# 1. Health Check
echo -e "${YELLOW}=== HEALTH & STATUS ===${NC}"
test_endpoint "Health Check" "http://localhost:8080/health" "no"
echo ""

# 2. Player Stats Endpoints
echo -e "${YELLOW}=== PLAYER STATS ===${NC}"
test_endpoint "Player Profile" "$BASE_URL/stats/player/$GUID" "yes"
test_endpoint "Player Weapons" "$BASE_URL/stats/player/$GUID/weapons" "yes"
test_endpoint "Player Maps" "$BASE_URL/stats/player/$GUID/maps" "yes"
test_endpoint "Player Gametypes" "$BASE_URL/stats/player/$GUID/gametypes" "yes"
test_endpoint "Player Performance" "$BASE_URL/stats/player/$GUID/performance" "yes"
# test_endpoint "Player Session History" "$BASE_URL/stats/player/$GUID/sessions" "yes"  # NOT IMPLEMENTED YET
test_endpoint "Player Heatmap" "$BASE_URL/stats/player/$GUID/heatmap/$MAP" "no"
echo ""

# 3. Global Stats
echo -e "${YELLOW}=== GLOBAL STATS ===${NC}"
test_endpoint "Global Stats" "$BASE_URL/stats/global" "yes"
test_endpoint "Top Weapons Global" "$BASE_URL/stats/weapons" "yes"
test_endpoint "Top Maps Global" "$BASE_URL/stats/maps" "yes"
test_endpoint "Weapon Detail - Thompson" "$BASE_URL/stats/weapon/Thompson" "yes"
test_endpoint "Map Detail - V2 Rocket" "$BASE_URL/stats/map/$MAP" "yes"
echo ""

# 4. Leaderboards
echo -e "${YELLOW}=== LEADERBOARDS ===${NC}"
test_endpoint "Leaderboard - Global" "$BASE_URL/stats/leaderboard" "yes"
test_endpoint "Leaderboard - Cards" "$BASE_URL/stats/leaderboard/cards" "yes"
test_endpoint "Weapon Leaderboard - Thompson" "$BASE_URL/stats/leaderboard/weapon/Thompson" "yes"
test_endpoint "Map Leaderboard - V2 Rocket" "$BASE_URL/stats/leaderboard/map/$MAP" "yes"
echo ""

# 5. Server Stats
echo -e "${YELLOW}=== SERVER STATS ===${NC}"
test_endpoint "Server Stats" "$BASE_URL/stats/server/$SERVER_ID/stats" "no"
test_endpoint "Server Pulse" "$BASE_URL/stats/server/pulse" "yes"
test_endpoint "Server Maps" "$BASE_URL/stats/server/maps" "yes"
echo ""

# 6. Weapon Detail Pages
echo -e "${YELLOW}=== WEAPON DETAILS (redundant, tested above) ===${NC}"
# Already tested in Global Stats
echo ""

# 7. Map Detail Pages
echo -e "${YELLOW}=== MAP DETAILS (redundant, tested above) ===${NC}"
# Already tested in Global Stats
echo ""

# 8. Achievements
echo -e "${YELLOW}=== ACHIEVEMENTS ===${NC}"
test_endpoint "Player Achievements" "$BASE_URL/achievements/player/$GUID" "no"
test_endpoint "Achievement Definitions" "$BASE_URL/achievements/definitions" "no"
echo ""

# 9. Advanced Stats
echo -e "${YELLOW}=== ADVANCED ANALYTICS ===${NC}"
test_endpoint "Player Deep Stats" "$BASE_URL/stats/player/$GUID/deep" "yes"
test_endpoint "Player Playstyle" "$BASE_URL/stats/player/$GUID/playstyle" "no"
test_endpoint "Peak Performance" "$BASE_URL/stats/player/$GUID/peak-performance" "no"
test_endpoint "Combo Metrics" "$BASE_URL/stats/player/$GUID/combos" "no"
test_endpoint "Drill-Down" "$BASE_URL/stats/player/$GUID/drilldown" "no"
test_endpoint "Vehicle Stats" "$BASE_URL/stats/player/$GUID/vehicles" "no"
test_endpoint "Game Flow Stats" "$BASE_URL/stats/player/$GUID/game-flow" "no"
test_endpoint "World Stats" "$BASE_URL/stats/player/$GUID/world" "no"
test_endpoint "Bot Stats" "$BASE_URL/stats/player/$GUID/bots" "no"
echo ""

# Final Report
echo "=========================================="
echo "TEST SUMMARY"
echo "=========================================="
echo -e "Total Tests:  $total_tests"
echo -e "${GREEN}Passed:       $passed_tests${NC}"
echo -e "${RED}Failed:       $failed_tests${NC}"
echo ""

success_rate=$((passed_tests * 100 / total_tests))
if [ $success_rate -ge 80 ]; then
    echo -e "${GREEN}Success Rate: $success_rate% ✓${NC}"
elif [ $success_rate -ge 50 ]; then
    echo -e "${YELLOW}Success Rate: $success_rate% ⚠${NC}"
else
    echo -e "${RED}Success Rate: $success_rate% ✗${NC}"
fi
echo "=========================================="

exit $failed_tests
