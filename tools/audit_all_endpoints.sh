#!/bin/bash
# Comprehensive API Endpoint Audit Script
# Tests all endpoints and saves responses for analysis

API_BASE="http://localhost:8080/api/v1"
OUTPUT_DIR="audit_results_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$OUTPUT_DIR"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TOTAL=0
PASSED=0
FAILED=0

test_endpoint() {
    local method=$1
    local endpoint=$2
    local name=$3
    local expected_status=${4:-200}
    
    TOTAL=$((TOTAL + 1))
    echo -n "Testing: $name ... "
    
    response=$(curl -s -w "\n%{http_code}" -X $method "$API_BASE$endpoint" 2>&1)
    status_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    # Save response
    echo "$body" > "$OUTPUT_DIR/${name}.json"
    
    if [ "$status_code" = "$expected_status" ]; then
        echo -e "${GREEN}✓${NC} ($status_code)"
        PASSED=$((PASSED + 1))
        
        # Check if response has data
        if echo "$body" | grep -q '"error"'; then
            echo "  ${YELLOW}⚠ Has error in response${NC}"
        elif echo "$body" | grep -q '{}'; then
            echo "  ${YELLOW}⚠ Empty object${NC}"
        elif echo "$body" | grep -q '\[\]'; then
            echo "  ${YELLOW}⚠ Empty array${NC}"
        fi
    else
        echo -e "${RED}✗${NC} (expected $expected_status, got $status_code)"
        FAILED=$((FAILED + 1))
        echo "  Response: $body" | head -c 200
        echo ""
    fi
}

echo "========================================="
echo "API Endpoint Audit - $(date)"
echo "========================================="
echo ""

# Health Check
echo "=== HEALTH & MONITORING ==="
test_endpoint GET "/stats/global" "global_stats"
test_endpoint GET "/stats/global/activity" "server_activity"
test_endpoint GET "/stats/server/pulse" "server_pulse"

# Global Stats
echo ""
echo "=== GLOBAL STATISTICS ==="
test_endpoint GET "/stats/weapons" "weapons_global"
test_endpoint GET "/stats/weapons/list" "weapons_list"
test_endpoint GET "/stats/maps" "maps_all"
test_endpoint GET "/stats/maps/list" "maps_list"
test_endpoint GET "/stats/maps/popularity" "maps_popularity"
test_endpoint GET "/stats/gametypes" "gametypes_all"
test_endpoint GET "/stats/gametypes/list" "gametypes_list"
test_endpoint GET "/stats/matches" "recent_matches"
test_endpoint GET "/stats/teams/performance" "faction_performance"

# Leaderboards
echo ""
echo "=== LEADERBOARDS ==="
test_endpoint GET "/stats/leaderboard" "leaderboard_global"
test_endpoint GET "/stats/leaderboard/cards" "leaderboard_cards"
test_endpoint GET "/achievements/leaderboard" "achievements_leaderboard"

# Get first weapon from list for detailed tests
FIRST_WEAPON=$(curl -s "$API_BASE/stats/weapons/list" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data['weapons'][0] if 'weapons' in data and len(data['weapons']) > 0 else '')" 2>/dev/null)
if [ ! -z "$FIRST_WEAPON" ]; then
    echo ""
    echo "=== WEAPON DETAILS (Testing with: $FIRST_WEAPON) ==="
    test_endpoint GET "/stats/weapon/$FIRST_WEAPON" "weapon_detail"
    test_endpoint GET "/stats/leaderboard/weapon/$FIRST_WEAPON" "weapon_leaderboard"
else
    echo "WARNING: No weapons found for detailed testing"
fi

# Get first map from list for detailed tests
FIRST_MAP=$(curl -s "$API_BASE/stats/maps/list" | grep -o '"[^"]*"' | head -1 | tr -d '"')
if [ ! -z "$FIRST_MAP" ]; then
    echo ""
    echo "=== MAP DETAILS (Testing with: $FIRST_MAP) ==="
    test_endpoint GET "/stats/map/$FIRST_MAP" "map_detail"
    test_endpoint GET "/stats/leaderboard/map/$FIRST_MAP" "map_leaderboard"
    test_endpoint GET "/stats/map/$FIRST_MAP/heatmap" "map_heatmap"
fi

# Get first gametype from list for detailed tests
FIRST_GAMETYPE=$(curl -s "$API_BASE/stats/gametypes/list" | grep -o '"[^"]*"' | head -1 | tr -d '"')
if [ ! -z "$FIRST_GAMETYPE" ]; then
    echo ""
    echo "=== GAMETYPE DETAILS (Testing with: $FIRST_GAMETYPE) ==="
    test_endpoint GET "/stats/gametype/$FIRST_GAMETYPE" "gametype_detail"
    test_endpoint GET "/stats/leaderboard/gametype/$FIRST_GAMETYPE" "gametype_leaderboard"
fi

# Get player GUID for player tests (from leaderboard)
FIRST_PLAYER_GUID=$(curl -s "$API_BASE/stats/leaderboard/global?limit=1" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data['players'][0]['id'] if 'players' in data and len(data['players']) > 0 else '')" 2>/dev/null)
if [ ! -z "$FIRST_PLAYER_GUID" ]; then
    echo ""
    echo "=== PLAYER STATS (Testing with GUID: $FIRST_PLAYER_GUID) ==="
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID" "player_basic"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/deep" "player_deep"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/combat" "player_combat"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/movement" "player_movement"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/stance" "player_stance"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/matches" "player_matches"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/weapons" "player_weapons"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/gametypes" "player_gametypes"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/maps" "player_maps"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/performance" "player_performance"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/playstyle" "player_playstyle"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/peak-performance" "player_peak"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/combos" "player_combos"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/vehicles" "player_vehicles"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/game-flow" "player_gameflow"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/world" "player_world"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/bots" "player_bots"
    test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/heatmap/body" "player_body_heatmap"
    
    if [ ! -z "$FIRST_MAP" ]; then
        test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/heatmap/$FIRST_MAP" "player_map_heatmap"
        test_endpoint GET "/stats/player/$FIRST_PLAYER_GUID/deaths/$FIRST_MAP" "player_death_heatmap"
    fi
else
    echo "WARNING: No player GUID found for detailed testing"
fi

# Servers
echo ""
echo "=== SERVERS ==="
test_endpoint GET "/servers/" "servers_list"
test_endpoint GET "/servers/stats" "servers_global_stats"
test_endpoint GET "/servers/rankings" "servers_rankings"

# Get first server for detailed tests
FIRST_SERVER_ID=$(curl -s "$API_BASE/servers/" | grep -o '"server_id":[0-9]*' | head -1 | cut -d':' -f2)
if [ ! -z "$FIRST_SERVER_ID" ]; then
    echo ""
    echo "=== SERVER DETAILS (Testing with ID: $FIRST_SERVER_ID) ==="
    test_endpoint GET "/servers/$FIRST_SERVER_ID" "server_detail"
    test_endpoint GET "/servers/$FIRST_SERVER_ID/live" "server_live"
    test_endpoint GET "/servers/$FIRST_SERVER_ID/player-history" "server_player_history"
    test_endpoint GET "/servers/$FIRST_SERVER_ID/peak-hours" "server_peak_hours"
    test_endpoint GET "/servers/$FIRST_SERVER_ID/top-players" "server_top_players"
    test_endpoint GET "/servers/$FIRST_SERVER_ID/players" "server_players"
    test_endpoint GET "/servers/$FIRST_SERVER_ID/maps" "server_maps"
    test_endpoint GET "/servers/$FIRST_SERVER_ID/map-rotation" "server_map_rotation"
    test_endpoint GET "/servers/$FIRST_SERVER_ID/weapons" "server_weapons"
    test_endpoint GET "/servers/$FIRST_SERVER_ID/matches" "server_matches"
    test_endpoint GET "/servers/$FIRST_SERVER_ID/activity-timeline" "server_activity_timeline"
fi

# Achievements
echo ""
echo "=== ACHIEVEMENTS ==="
test_endpoint GET "/achievements/" "achievements_list"
test_endpoint GET "/achievements/recent" "achievements_recent"
if [ ! -z "$FIRST_PLAYER_GUID" ]; then
    test_endpoint GET "/achievements/player/$FIRST_PLAYER_GUID" "player_achievements"
fi

# Tournaments
echo ""
echo "=== TOURNAMENTS ==="
test_endpoint GET "/tournaments/" "tournaments_list"

# Get first match for match tests
FIRST_MATCH_ID=$(curl -s "$API_BASE/stats/matches?limit=1" | grep -o '"match_id":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ ! -z "$FIRST_MATCH_ID" ]; then
    echo ""
    echo "=== MATCH DETAILS (Testing with ID: $FIRST_MATCH_ID) ==="
    test_endpoint GET "/stats/match/$FIRST_MATCH_ID" "match_detail"
    test_endpoint GET "/stats/match/$FIRST_MATCH_ID/advanced" "match_advanced"
    test_endpoint GET "/stats/match/$FIRST_MATCH_ID/timeline" "match_timeline"
    test_endpoint GET "/stats/match/$FIRST_MATCH_ID/heatmap" "match_heatmap"
fi

# Summary
echo ""
echo "========================================="
echo "AUDIT SUMMARY"
echo "========================================="
echo "Total Tests: $TOTAL"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo ""
echo "Results saved to: $OUTPUT_DIR/"
echo ""

# Generate summary report
cat > "$OUTPUT_DIR/SUMMARY.txt" << EOF
API Endpoint Audit Summary
Date: $(date)
=========================

Total Tests: $TOTAL
Passed: $PASSED
Failed: $FAILED
Success Rate: $(echo "scale=2; $PASSED * 100 / $TOTAL" | bc)%

Output Directory: $OUTPUT_DIR

=== ENDPOINTS TESTED ===
$(ls -1 $OUTPUT_DIR/*.json | wc -l) endpoint responses saved

=== EMPTY OR ERROR RESPONSES ===
EOF

# Check for empty responses
for file in "$OUTPUT_DIR"/*.json; do
    if grep -q '{}' "$file" || grep -q '\[\]' "$file" || grep -q '"error"' "$file"; then
        basename "$file" >> "$OUTPUT_DIR/SUMMARY.txt"
    fi
done

echo "Full summary saved to: $OUTPUT_DIR/SUMMARY.txt"
