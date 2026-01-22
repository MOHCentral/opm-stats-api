#!/bin/bash
# Comprehensive Stats Fix and Diagnostic Script

echo "=================================================="
echo "SMF Stats Comprehensive Diagnostic"
echo "=================================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

TEST_USER_ID=1
TEST_GUID=""

# Check MySQL SMF database
echo "[1] Checking SMF database..."
echo "------------------------------"

# Get linked GUID from SMF
TEST_GUID=$(mysql -u smf -p'smf_password' smf -N -e "SELECT guid FROM smf_mohaa_identities WHERE id_member = $TEST_USER_ID LIMIT 1" 2>/dev/null)

if [ -z "$TEST_GUID" ]; then
    echo -e "${RED}✗ User $TEST_USER_ID has NO linked GUID!${NC}"
    echo "  Fix: php /var/www/smf/smf-mohaa/link_identity_manual.php"
    TEST_GUID="UNKNOWN"
else
    echo -e "${GREEN}✓ User $TEST_USER_ID linked to GUID: $TEST_GUID${NC}"
fi

# Check achievement unlocks
ACH_COUNT=$(mysql -u smf -p'smf_password' smf -N -e "SELECT COUNT(*) FROM smf_mohaa_player_achievements WHERE smf_member_id = $TEST_USER_ID AND unlocked = 1" 2>/dev/null)
echo "  Achievements unlocked: $ACH_COUNT"

# Check PostgreSQL
echo ""
echo "[2] Checking PostgreSQL (mohaa_stats)..."
echo "------------------------------"

PG_GUID=$(docker exec opm-stats-system-postgres-1 psql -U mohaa -d mohaa_stats -t -c "SELECT guid FROM player_guid_registry WHERE smf_member_id = $TEST_USER_ID LIMIT 1" 2>/dev/null | xargs)

if [ -z "$PG_GUID" ]; then
    echo -e "${YELLOW}⚠ No GUID in PostgreSQL for user $TEST_USER_ID${NC}"
else
    echo -e "${GREEN}✓ PostgreSQL has GUID: $PG_GUID (SMF ID: $TEST_USER_ID)${NC}"
fi

# Check total achievements in database
TOTAL_ACH=$(docker exec opm-stats-system-postgres-1 psql -U mohaa -d mohaa_stats -t -c "SELECT COUNT(*) FROM mohaa_achievements" 2>/dev/null | xargs)
echo "  Total achievements defined: $TOTAL_ACH"

# Check ClickHouse
echo ""
echo "[3] Checking ClickHouse (events)..."
echo "------------------------------"

if [ "$TEST_GUID" != "UNKNOWN" ]; then
    KILL_COUNT=$(docker exec opm-stats-system-clickhouse-1 clickhouse-client --query="SELECT COUNT(*) FROM mohaa_stats.raw_events WHERE actor_guid = '$TEST_GUID' AND event_type = 'kill'" 2>/dev/null)
    TOTAL_EVENTS=$(docker exec opm-stats-system-clickhouse-1 clickhouse-client --query="SELECT COUNT(*) FROM mohaa_stats.raw_events WHERE actor_guid = '$TEST_GUID'" 2>/dev/null)
    
    echo "  Total events for GUID: $TOTAL_EVENTS"
    echo "  Kill events: $KILL_COUNT"
    
    # Check for win/loss events
    echo "  Event type distribution:"
    docker exec opm-stats-system-clickhouse-1 clickhouse-client --query="SELECT event_type, COUNT(*) as count FROM mohaa_stats.raw_events GROUP BY event_type ORDER BY count DESC LIMIT 15" 2>/dev/null | sed 's/^/    /'
else
    echo -e "${YELLOW}  Skipping (no GUID)${NC}"
fi

# Test API
echo ""
echo "[4] Testing API endpoints..."
echo "------------------------------"

API_HEALTH=$(curl -s http://localhost:8080/health | jq -r '.status' 2>/dev/null)
if [ "$API_HEALTH" == "ok" ]; then
    echo -e "${GREEN}✓ API Health: OK${NC}"
else
    echo -e "${RED}✗ API Health: $API_HEALTH${NC}"
fi

if [ "$TEST_GUID" != "UNKNOWN" ]; then
    # Test player stats endpoint
    PLAYER_STATS=$(curl -s "http://localhost:8080/stats/player/$TEST_GUID" 2>/dev/null)
    if [ ! -z "$PLAYER_STATS" ]; then
        KILLS=$(echo "$PLAYER_STATS" | jq -r '.player.total_kills // 0' 2>/dev/null)
        DEATHS=$(echo "$PLAYER_STATS" | jq -r '.player.total_deaths // 0' 2>/dev/null)
        echo "  API Player Stats: $KILLS kills, $DEATHS deaths"
    else
        echo -e "${YELLOW}  ⚠ Player stats endpoint returned empty${NC}"
    fi
fi

# Check for wins calculation issue
echo ""
echo "[5] Diagnosing WINS issue..."
echo "------------------------------"

# Check if gametypes endpoint has data
if [ "$TEST_GUID" != "UNKNOWN" ]; then
    GAMETYPES=$(curl -s "http://localhost:8080/stats/player/$TEST_GUID/gametypes" 2>/dev/null)
    if [ ! -z "$GAMETYPES" ]; then
        echo "  Gametypes data:"
        echo "$GAMETYPES" | jq -r '.gametypes[] | "    \(.gametype): \(.matches_played // 0) played, \(.matches_won // 0) won"' 2>/dev/null || echo "    No gametype data"
    else
        echo -e "${YELLOW}  ⚠ No gametype data${NC}"
    fi
    
    # Check ClickHouse for team_win events
    TEAM_WINS=$(docker exec opm-stats-system-clickhouse-1 clickhouse-client --query="SELECT COUNT(*) FROM mohaa_stats.raw_events WHERE event_type = 'team_win'" 2>/dev/null)
    echo "  Total team_win events in database: $TEAM_WINS"
    
    if [ "$TEAM_WINS" == "0" ]; then
        echo -e "${RED}  ✗ NO team_win events found - this is why wins = 0!${NC}"
    fi
fi

# Summary and recommendations
echo ""
echo "=================================================="
echo "SUMMARY & FIX RECOMMENDATIONS"
echo "=================================================="
echo ""

if [ "$TEST_GUID" == "UNKNOWN" ]; then
    echo -e "${RED}[CRITICAL] User identity not linked${NC}"
    echo "  Fix: mysql -u smf -p'smf_password' smf -e \"INSERT INTO smf_mohaa_identities (id_member, guid, linked_at) VALUES ($TEST_USER_ID, 'YOUR_GUID', NOW())\""
    echo ""
fi

if [ "${TOTAL_EVENTS:-0}" -lt "100" ]; then
    echo -e "${YELLOW}[WARNING] Low event count ($TOTAL_EVENTS)${NC}"
    echo "  Fix: Run seeder to generate test data"
    echo "  cd /home/elgan/dev/opm-stats-system && ./bin/seeder -player-guid=\"$TEST_GUID\" -events=2000"
    echo ""
fi

if [ "${TEAM_WINS:-0}" == "0" ]; then
    echo -e "${RED}[CRITICAL] No team_win events - wins will always be 0${NC}"
    echo "  Root cause: Game not sending team_win events OR seeder not generating them"
    echo "  Fix: Update seeder to include team_win, match_end, round_end events"
    echo ""
fi

if [ "${ACH_COUNT:-0}" == "0" ]; then
    echo -e "${YELLOW}[INFO] No achievements unlocked yet${NC}"
    echo "  Test: Send a kill event to trigger achievement worker"
    echo "  curl -H 'Authorization: dev-seed-token' -X POST http://localhost:8080/api/v1/ingest/events \\"
    echo "    -d 'event_type=kill&attacker_guid=$TEST_GUID&attacker_smf_id=$TEST_USER_ID&timestamp=$(date +%s)'"
    echo ""
fi

echo "Next steps:"
echo "  1. Run identity linker if needed"
echo "  2. Update seeder to include team_win events"
echo "  3. Re-seed data with wins/losses"
echo "  4. Re-run this diagnostic to verify fixes"
