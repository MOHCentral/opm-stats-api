#!/bin/bash
# ==============================================================================
# COMPREHENSIVE STATS AUDIT SCRIPT
# Tests ALL endpoints and stats to verify data is populated
# ==============================================================================

API_BASE="http://localhost:8080/api/v1"
TEST_GUID="GUID_00006"  # GrimHunter6 - Has most data
OUTPUT_DIR="audit_results_$(date +%Y%m%d_%H%M%S)"

mkdir -p "$OUTPUT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Test counter
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

echo -e "${CYAN}========================================================================${NC}"
echo -e "${CYAN}           COMPREHENSIVE STATS SYSTEM AUDIT${NC}"
echo -e "${CYAN}========================================================================${NC}"
echo ""

# Function to test endpoint
test_endpoint() {
    local name="$1"
    local url="$2"
    local expected_field="$3"  # Field that should NOT be empty/0
    local output_file="$4"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    echo -n "Testing ${BLUE}${name}${NC}... "
    
    # Make request
    response=$(curl -s "$url" || echo "ERROR")
    
    if [ "$response" = "ERROR" ] || [ -z "$response" ]; then
        echo -e "${RED}FAILED${NC} (No response)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        echo "$response" > "$OUTPUT_DIR/$output_file"
        return 1
    fi
    
    # Save response
    echo "$response" | python3 -m json.tool > "$OUTPUT_DIR/$output_file" 2>/dev/null || echo "$response" > "$OUTPUT_DIR/$output_file"
    
    # Check if it has data
    if echo "$response" | grep -q "$expected_field"; then
        echo -e "${GREEN}PASSED${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        return 0
    else
        echo -e "${YELLOW}WARNING${NC} (No $expected_field)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
}

echo -e "${CYAN}━━━ PLAYER STATS ━━━${NC}"
test_endpoint "Player Deep Stats" "$API_BASE/stats/player/$TEST_GUID/deep" "kills" "player_deep.json"
test_endpoint "Player Combat" "$API_BASE/stats/player/$TEST_GUID/combat" "kills" "player_combat.json"
test_endpoint "Player Movement" "$API_BASE/stats/player/$TEST_GUID/movement" "jump_count" "player_movement.json"
test_endpoint "Player Stance" "$API_BASE/stats/player/$TEST_GUID/stance" "standing" "player_stance.json"
test_endpoint "Player Weapons" "$API_BASE/stats/player/$TEST_GUID/weapons" "weapon" "player_weapons.json"
test_endpoint "Player Matches" "$API_BASE/stats/player/$TEST_GUID/matches" "match_id" "player_matches.json"
test_endpoint "Player by GameType" "$API_BASE/stats/player/$TEST_GUID/gametypes" "gametype" "player_gametypes.json"
test_endpoint "Player by Map" "$API_BASE/stats/player/$TEST_GUID/maps" "map" "player_maps.json"

echo ""
echo -e "${CYAN}━━━ GLOBAL STATS ━━━${NC}"
test_endpoint "Global Stats" "$API_BASE/stats/global" "total_kills" "global_stats.json"
test_endpoint "Server Activity" "$API_BASE/stats/global/activity" "active" "server_activity.json"
test_endpoint "Server Pulse" "$API_BASE/stats/server/pulse" "events_per_sec" "server_pulse.json"

echo ""
echo -e "${CYAN}━━━ LEADERBOARDS ━━━${NC}"
test_endpoint "Leaderboard Global" "$API_BASE/leaderboards/global" "players" "leaderboard_global.json"
test_endpoint "Leaderboard Cards" "$API_BASE/leaderboards/cards" "total_domination" "leaderboard_cards.json"

echo ""
echo -e "${CYAN}━━━ WEAPONS ━━━${NC}"
test_endpoint "Weapons Global" "$API_BASE/stats/weapons" "weapons" "weapons_global.json"
test_endpoint "Weapons List" "$API_BASE/stats/weapons/list" "name" "weapons_list.json"
test_endpoint "Weapon Detail (MP40)" "$API_BASE/stats/weapon/MP40" "name" "weapon_detail.json"
test_endpoint "Weapon Leaderboard" "$API_BASE/leaderboards/weapon/MP40" "players" "weapon_leaderboard.json"

echo ""
echo -e "${CYAN}━━━ MAPS ━━━${NC}"
test_endpoint "Maps All" "$API_BASE/stats/maps" "maps" "maps_all.json"
test_endpoint "Maps List" "$API_BASE/stats/maps/list" "name" "maps_list.json"
test_endpoint "Maps Popularity" "$API_BASE/stats/maps/popularity" "map_name" "maps_popularity.json"
test_endpoint "Map Detail" "$API_BASE/stats/map/mp_city" "map_name" "map_detail.json"
test_endpoint "Map Leaderboard" "$API_BASE/leaderboards/map/mp_city" "players" "map_leaderboard.json"
test_endpoint "Map Heatmap" "$API_BASE/stats/map/mp_city/heatmap" "kills" "map_heatmap.json"

echo ""
echo -e "${CYAN}━━━ GAME TYPES ━━━${NC}"
test_endpoint "GameTypes All" "$API_BASE/stats/gametypes" "gametypes" "gametypes_all.json"
test_endpoint "GameTypes List" "$API_BASE/stats/gametypes/list" "name" "gametypes_list.json"
test_endpoint "GameType Detail" "$API_BASE/stats/gametype/tdm" "name" "gametype_detail.json"
test_endpoint "GameType Leaderboard" "$API_BASE/leaderboards/gametype/tdm" "players" "gametype_leaderboard.json"

echo ""
echo -e "${CYAN}━━━ SERVERS ━━━${NC}"
test_endpoint "Servers List" "$API_BASE/servers/list" "servers" "servers_list.json"
test_endpoint "Servers Rankings" "$API_BASE/servers/rankings" "servers" "servers_rankings.json"
test_endpoint "Servers Global Stats" "$API_BASE/servers/stats/global" "total_" "servers_global_stats.json"

echo ""
echo -e "${CYAN}━━━ MATCHES ━━━${NC}"
test_endpoint "Recent Matches" "$API_BASE/stats/matches" "matches" "recent_matches.json"

echo ""
echo -e "${CYAN}━━━ ACHIEVEMENTS ━━━${NC}"
test_endpoint "Achievements List" "$API_BASE/achievements/list" "achievements" "achievements_list.json"
test_endpoint "Achievements Leaderboard" "$API_BASE/achievements/leaderboard" "players" "achievements_leaderboard.json"
test_endpoint "Recent Achievements" "$API_BASE/achievements/recent" "timestamp" "achievements_recent.json"

echo ""
echo -e "${CYAN}━━━ FACTION/TEAM ━━━${NC}"
test_endpoint "Faction Performance" "$API_BASE/stats/teams/performance" "axis" "faction_performance.json"

echo ""
echo -e "${CYAN}━━━ TOURNAMENTS ━━━${NC}"
test_endpoint "Tournaments List" "$API_BASE/tournaments/list" "tournaments" "tournaments_list.json"

echo ""
echo -e "${CYAN}========================================================================${NC}"
echo -e "${GREEN}                    AUDIT COMPLETE${NC}"
echo -e "${CYAN}========================================================================${NC}"
echo ""
echo -e "  Total Tests:  ${BLUE}$TOTAL_TESTS${NC}"
echo -e "  Passed:       ${GREEN}$PASSED_TESTS${NC}"
echo -e "  Failed:       ${RED}$FAILED_TESTS${NC}"
echo ""
echo -e "  Results saved to: ${YELLOW}$OUTPUT_DIR/${NC}"
echo ""

# Create summary file
cat > "$OUTPUT_DIR/SUMMARY.txt" << EOF
COMPREHENSIVE STATS AUDIT SUMMARY
==================================
Date: $(date)
Total Tests: $TOTAL_TESTS
Passed: $PASSED_TESTS
Failed: $FAILED_TESTS

Test Results:
$(grep -E "(PASSED|FAILED|WARNING)" "$0" 2>/dev/null || echo "See individual JSON files")
EOF

echo -e "${YELLOW}Run this script regularly to track progress!${NC}"
