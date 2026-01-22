#!/bin/bash
# ============================================================================
# MOHAA Stats API - Event Simulator
# Simulates tracker.scr sending events to the Go API
# ============================================================================

API_URL="${API_URL:-http://localhost:8080}"
SERVER_TOKEN="${SERVER_TOKEN:-dev-server-token}"
SERVER_ID="${SERVER_ID:-test-server-01}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=============================================="
echo "MOHAA Stats Event Simulator"
echo "API URL: $API_URL"
echo "Server ID: $SERVER_ID"
echo "=============================================="

# Function to send event
send_event() {
    local data="$1"
    local full_data="${data}&server_token=${SERVER_TOKEN}&server_id=${SERVER_ID}"
    
    response=$(curl -s -w "\n%{http_code}" -X POST \
        -H "X-Server-Token: ${SERVER_TOKEN}" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "$full_data" \
        "${API_URL}/api/v1/ingest/events")
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" == "202" ] || [ "$http_code" == "200" ]; then
        echo -e "${GREEN}✓${NC} Event sent (HTTP $http_code)"
    else
        echo -e "${RED}✗${NC} Event failed (HTTP $http_code): $body"
    fi
}

# Function to format timestamp
timestamp() {
    date +%s.%N
}

# Generate some player GUIDs
PLAYER1="player1-guid-abcd1234"
PLAYER2="player2-guid-efgh5678"
PLAYER3="player3-guid-ijkl9012"
PLAYER4="player4-guid-mnop3456"

echo ""
echo "=== Sending Match Start ==="
send_event "type=match_start&match_id=test_match_$(date +%s)&session_id=sess_test&map_name=obj/obj_team1&gametype=obj&timestamp=$(timestamp)"

echo ""
echo "=== Simulating Player Connects ==="
for player in $PLAYER1 $PLAYER2 $PLAYER3 $PLAYER4; do
    send_event "type=connect&match_id=test_match&player_guid=$player&player_name=TestPlayer_${player:0:8}&timestamp=$(timestamp)"
    sleep 0.1
done

echo ""
echo "=== Simulating Spawns ==="
for player in $PLAYER1 $PLAYER2 $PLAYER3 $PLAYER4; do
    pos_x=$((RANDOM % 1000))
    pos_y=$((RANDOM % 1000))
    pos_z=$((RANDOM % 100))
    send_event "type=spawn&match_id=test_match&player_guid=$player&player_name=TestPlayer_${player:0:8}&pos_x=$pos_x&pos_y=$pos_y&pos_z=$pos_z&team=$((RANDOM % 2))&timestamp=$(timestamp)"
    sleep 0.05
done

echo ""
echo "=== Simulating Combat Events ==="

# Generate random kills
for i in {1..20}; do
    # Pick random killer and victim
    killers=($PLAYER1 $PLAYER2 $PLAYER3 $PLAYER4)
    killer=${killers[$((RANDOM % 4))]}
    victim=${killers[$((RANDOM % 4))]}
    
    # Ensure killer != victim
    while [ "$killer" == "$victim" ]; do
        victim=${killers[$((RANDOM % 4))]}
    done
    
    weapons=("kar98" "thompson" "mp40" "stg44" "grenade" "m1garand")
    weapon=${weapons[$((RANDOM % ${#weapons[@]}))]}
    
    hitlocs=("head" "torso" "left_arm" "right_arm" "left_leg" "right_leg")
    hitloc=${hitlocs[$((RANDOM % ${#hitlocs[@]}))]}
    
    # Positions
    killer_x=$((RANDOM % 1000))
    killer_y=$((RANDOM % 1000))
    killer_z=$((RANDOM % 100))
    victim_x=$((RANDOM % 1000))
    victim_y=$((RANDOM % 1000))
    victim_z=$((RANDOM % 100))
    
    echo "Kill #$i: $killer -> $victim ($weapon, $hitloc)"
    send_event "type=kill&match_id=test_match&attacker_guid=$killer&attacker_name=TestPlayer_${killer:0:8}&victim_guid=$victim&victim_name=TestPlayer_${victim:0:8}&weapon=$weapon&hitloc=$hitloc&attacker_x=$killer_x&attacker_y=$killer_y&attacker_z=$killer_z&victim_x=$victim_x&victim_y=$victim_y&victim_z=$victim_z&timestamp=$(timestamp)"
    
    # Also send corresponding death event
    send_event "type=death&match_id=test_match&player_guid=$victim&player_name=TestPlayer_${victim:0:8}&inflictor=$weapon&pos_x=$victim_x&pos_y=$victim_y&pos_z=$victim_z&timestamp=$(timestamp)"
    
    sleep 0.1
done

echo ""
echo "=== Simulating Headshots ==="
for i in {1..5}; do
    killers=($PLAYER1 $PLAYER2 $PLAYER3 $PLAYER4)
    killer=${killers[$((RANDOM % 4))]}
    victim=${killers[$((RANDOM % 4))]}
    while [ "$killer" == "$victim" ]; do
        victim=${killers[$((RANDOM % 4))]}
    done
    
    weapons=("kar98" "m1garand")
    weapon=${weapons[$((RANDOM % ${#weapons[@]}))]}
    
    send_event "type=headshot&match_id=test_match&player_guid=$killer&player_name=TestPlayer_${killer:0:8}&victim_guid=$victim&victim_name=TestPlayer_${victim:0:8}&weapon=$weapon&timestamp=$(timestamp)"
    sleep 0.05
done

echo ""
echo "=== Simulating Weapon Events ==="
for i in {1..30}; do
    killers=($PLAYER1 $PLAYER2 $PLAYER3 $PLAYER4)
    player=${killers[$((RANDOM % 4))]}
    
    weapons=("kar98" "thompson" "mp40" "stg44" "m1garand")
    weapon=${weapons[$((RANDOM % ${#weapons[@]}))]}
    
    ammo=$((RANDOM % 30 + 1))
    
    # Random position
    pos_x=$((RANDOM % 1000))
    pos_y=$((RANDOM % 1000))
    pos_z=$((RANDOM % 100))
    pitch=$((RANDOM % 90))
    yaw=$((RANDOM % 360))
    
    send_event "type=weapon_fire&match_id=test_match&player_guid=$player&player_name=TestPlayer_${player:0:8}&weapon=$weapon&ammo_remaining=$ammo&pos_x=$pos_x&pos_y=$pos_y&pos_z=$pos_z&aim_pitch=$pitch&aim_yaw=$yaw&timestamp=$(timestamp)"
    sleep 0.02
done

echo ""
echo "=== Simulating Movement Events ==="
for i in {1..10}; do
    killers=($PLAYER1 $PLAYER2 $PLAYER3 $PLAYER4)
    player=${killers[$((RANDOM % 4))]}
    
    # Jump events
    pos_x=$((RANDOM % 1000))
    pos_y=$((RANDOM % 1000))
    pos_z=$((RANDOM % 100))
    send_event "type=jump&match_id=test_match&player_guid=$player&player_name=TestPlayer_${player:0:8}&pos_x=$pos_x&pos_y=$pos_y&pos_z=$pos_z&timestamp=$(timestamp)"
    
    # Distance tracking
    walked=$((RANDOM % 1000))
    sprinted=$((RANDOM % 500))
    driven=0
    send_event "type=distance&match_id=test_match&player_guid=$player&player_name=TestPlayer_${player:0:8}&walked=$walked&sprinted=$sprinted&driven=$driven&timestamp=$(timestamp)"
    
    sleep 0.05
done

echo ""
echo "=== Simulating Grenade Events ==="
for i in {1..5}; do
    killers=($PLAYER1 $PLAYER2 $PLAYER3 $PLAYER4)
    player=${killers[$((RANDOM % 4))]}
    
    pos_x=$((RANDOM % 1000))
    pos_y=$((RANDOM % 1000))
    pos_z=$((RANDOM % 100))
    
    send_event "type=grenade_throw&match_id=test_match&player_guid=$player&player_name=TestPlayer_${player:0:8}&pos_x=$pos_x&pos_y=$pos_y&pos_z=$pos_z&timestamp=$(timestamp)"
    sleep 0.1
    send_event "type=grenade_explode&match_id=test_match&player_guid=$player&pos_x=$((pos_x + 50))&pos_y=$((pos_y + 50))&pos_z=$pos_z&timestamp=$(timestamp)"
    
    sleep 0.05
done

echo ""
echo "=== Simulating Chat Messages ==="
messages=("gg" "nice shot" "lol" "noob" "rekt" "GG WP" "!admin" "hello everyone")
for i in {1..5}; do
    killers=($PLAYER1 $PLAYER2 $PLAYER3 $PLAYER4)
    player=${killers[$((RANDOM % 4))]}
    msg=${messages[$((RANDOM % ${#messages[@]}))]}
    
    send_event "type=chat&match_id=test_match&player_guid=$player&player_name=TestPlayer_${player:0:8}&message=$msg&timestamp=$(timestamp)"
    sleep 0.1
done

echo ""
echo "=== Sending Match End ==="
send_event "type=match_end&match_id=test_match&session_id=sess_test&map_name=obj/obj_team1&winning_team=allies&allies_score=4&axis_score=2&duration=1800&total_rounds=6&timestamp=$(timestamp)"

echo ""
echo "=== Simulating Disconnects ==="
for player in $PLAYER1 $PLAYER2 $PLAYER3 $PLAYER4; do
    send_event "type=disconnect&match_id=test_match&player_guid=$player&player_name=TestPlayer_${player:0:8}&timestamp=$(timestamp)"
    sleep 0.05
done

echo ""
echo "=============================================="
echo "Event simulation complete!"
echo "=============================================="
echo ""
echo "Summary of events sent:"
echo "  - 1 match_start"
echo "  - 4 player connects"
echo "  - 4 player spawns"  
echo "  - 20 kills + 20 deaths"
echo "  - 5 headshots"
echo "  - 30 weapon_fire"
echo "  - 20 movement events (jump + distance)"
echo "  - 10 grenade events"
echo "  - 5 chat messages"
echo "  - 1 match_end"
echo "  - 4 disconnects"
echo ""
echo "Total: ~120 events"
