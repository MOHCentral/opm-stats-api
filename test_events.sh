#!/bin/bash
# ==============================================================================
# MOHAA Stats API - Event Simulator
# ==============================================================================
# Simulates events as if they were sent from the tracker.scr script in-game.
# This is useful for testing the API without running MOHAA.
# ==============================================================================

API_BASE="http://localhost:8080"
EVENTS_ENDPOINT="${API_BASE}/api/v1/ingest/events"
SERVER_TOKEN="dev-server-token-replace-in-production"
SERVER_ID="dev-server-01"
MATCH_ID="match_test_$(date +%s)"
SESSION_ID="sess_test_$(date +%s)"
MAP_NAME="dm/mohdm1"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Test player GUIDs
PLAYER1_GUID="ABCD1234567890EF"
PLAYER1_NAME="SoldierOne"
PLAYER2_GUID="1234567890ABCDEF"
PLAYER2_NAME="SniperTwo"
PLAYER3_GUID="FEDCBA0987654321"
PLAYER3_NAME="TankThree"
PLAYER4_GUID="9876543210FEDCBA"
PLAYER4_NAME="MedicFour"

# Weapons
WEAPONS=("m1_garand" "thompson" "kar98" "mp40" "springfield" "bar" "stg44" "colt45" "p38" "grenade")
HITLOCS=("head" "torso" "left_arm" "right_arm" "left_leg" "right_leg" "pelvis" "neck")

# Utility function to send event
send_event() {
    local payload="$1"
    local full_payload="${payload}&server_token=${SERVER_TOKEN}&server_id=${SERVER_ID}"
    
    response=$(curl -s -w "\n%{http_code}" -X POST "${EVENTS_ENDPOINT}" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "${full_payload}")
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" = "202" ] || [ "$http_code" = "200" ]; then
        echo -e "${GREEN}✓${NC} Event sent (HTTP $http_code)"
        return 0
    else
        echo -e "${RED}✗${NC} Event failed (HTTP $http_code): $body"
        return 1
    fi
}

# Random helpers
random_weapon() {
    echo "${WEAPONS[$RANDOM % ${#WEAPONS[@]}]}"
}

random_hitloc() {
    echo "${HITLOCS[$RANDOM % ${#HITLOCS[@]}]}"
}

random_pos() {
    echo "$((RANDOM % 2000 - 1000))"
}

timestamp() {
    date +%s.%N
}

# ==============================================================================
# EVENT GENERATORS
# ==============================================================================

send_match_start() {
    echo -e "\n${CYAN}▶ Starting Match${NC}"
    local payload="type=match_start"
    payload+="&match_id=${MATCH_ID}"
    payload+="&session_id=${SESSION_ID}"
    payload+="&map_name=${MAP_NAME}"
    payload+="&timestamp=$(timestamp)"
    payload+="&gametype=deathmatch"
    payload+="&timelimit=20"
    payload+="&fraglimit=50"
    payload+="&maxclients=20"
    
    send_event "$payload"
}

send_player_connect() {
    local name="$1"
    local guid="$2"
    echo -e "${BLUE}→ ${name} connected${NC}"
    
    local payload="type=connect"
    payload+="&match_id=${MATCH_ID}"
    payload+="&session_id=${SESSION_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&player_name=${name}"
    payload+="&player_guid=${guid}"
    
    send_event "$payload"
}

send_player_spawn() {
    local name="$1"
    local guid="$2"
    local team="$3"
    echo -e "${BLUE}→ ${name} spawned (${team})${NC}"
    
    local payload="type=spawn"
    payload+="&match_id=${MATCH_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&player_name=${name}"
    payload+="&player_guid=${guid}"
    payload+="&team=${team}"
    payload+="&pos_x=$(random_pos)"
    payload+="&pos_y=$(random_pos)"
    payload+="&pos_z=0"
    
    send_event "$payload"
}

send_kill() {
    local attacker_name="$1"
    local attacker_guid="$2"
    local victim_name="$3"
    local victim_guid="$4"
    local weapon="${5:-$(random_weapon)}"
    local hitloc="${6:-$(random_hitloc)}"
    
    echo -e "${YELLOW}☠ ${attacker_name} killed ${victim_name} with ${weapon} (${hitloc})${NC}"
    
    local payload="type=kill"
    payload+="&match_id=${MATCH_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&attacker_name=${attacker_name}"
    payload+="&attacker_guid=${attacker_guid}"
    payload+="&attacker_team=allies"
    payload+="&attacker_x=$(random_pos)"
    payload+="&attacker_y=$(random_pos)"
    payload+="&attacker_z=50"
    payload+="&attacker_pitch=$((RANDOM % 90 - 45))"
    payload+="&attacker_yaw=$((RANDOM % 360))"
    payload+="&victim_name=${victim_name}"
    payload+="&victim_guid=${victim_guid}"
    payload+="&victim_team=axis"
    payload+="&victim_x=$(random_pos)"
    payload+="&victim_y=$(random_pos)"
    payload+="&victim_z=0"
    payload+="&weapon=${weapon}"
    payload+="&hitloc=${hitloc}"
    
    send_event "$payload"
}

send_headshot() {
    local shooter_name="$1"
    local shooter_guid="$2"
    local victim_name="$3"
    local victim_guid="$4"
    local weapon="${5:-springfield}"
    
    echo -e "${RED}💀 HEADSHOT! ${shooter_name} → ${victim_name} with ${weapon}${NC}"
    
    local payload="type=headshot"
    payload+="&match_id=${MATCH_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&player_name=${shooter_name}"
    payload+="&player_guid=${shooter_guid}"
    payload+="&victim_name=${victim_name}"
    payload+="&victim_guid=${victim_guid}"
    payload+="&weapon=${weapon}"
    
    send_event "$payload"
}

send_weapon_fire() {
    local name="$1"
    local guid="$2"
    local weapon="$3"
    
    local payload="type=weapon_fire"
    payload+="&match_id=${MATCH_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&player_name=${name}"
    payload+="&player_guid=${guid}"
    payload+="&weapon=${weapon}"
    payload+="&ammo_remaining=$((RANDOM % 30 + 1))"
    payload+="&pos_x=$(random_pos)"
    payload+="&pos_y=$(random_pos)"
    payload+="&pos_z=50"
    payload+="&aim_pitch=$((RANDOM % 90 - 45))"
    payload+="&aim_yaw=$((RANDOM % 360))"
    
    send_event "$payload"
}

send_damage() {
    local attacker_name="$1"
    local attacker_guid="$2"
    local victim_name="$3"
    local victim_guid="$4"
    local damage="${5:-25}"
    local weapon="${6:-$(random_weapon)}"
    
    local payload="type=damage"
    payload+="&match_id=${MATCH_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&attacker_name=${attacker_name}"
    payload+="&attacker_guid=${attacker_guid}"
    payload+="&victim_name=${victim_name}"
    payload+="&victim_guid=${victim_guid}"
    payload+="&damage=${damage}"
    payload+="&weapon=${weapon}"
    
    send_event "$payload"
}

send_death() {
    local name="$1"
    local guid="$2"
    
    local payload="type=death"
    payload+="&match_id=${MATCH_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&player_name=${name}"
    payload+="&player_guid=${guid}"
    payload+="&pos_x=$(random_pos)"
    payload+="&pos_y=$(random_pos)"
    payload+="&pos_z=0"
    
    send_event "$payload"
}

send_grenade_throw() {
    local name="$1"
    local guid="$2"
    echo -e "${YELLOW}💣 ${name} threw a grenade${NC}"
    
    local payload="type=grenade_throw"
    payload+="&match_id=${MATCH_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&player_name=${name}"
    payload+="&player_guid=${guid}"
    payload+="&projectile=grenade"
    payload+="&pos_x=$(random_pos)"
    payload+="&pos_y=$(random_pos)"
    payload+="&pos_z=50"
    
    
    send_event "$payload"
}

send_reload() {
    local name="$1"
    local guid="$2"
    local weapon="${3:-$(random_weapon)}"
    
    local payload="type=reload"
    payload+="&match_id=${MATCH_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&player_name=${name}"
    payload+="&player_guid=${guid}"
    payload+="&weapon=${weapon}"
    payload+="&ammo_before=$((RANDOM % 10))"
    payload+="&ammo_after=30"
    
    send_event "$payload"
}

send_weapon_drop() {
    local name="$1"
    local guid="$2"
    local weapon="${3:-$(random_weapon)}"
    
    local payload="type=weapon_drop"
    payload+="&match_id=${MATCH_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&player_name=${name}"
    payload+="&player_guid=${guid}"
    payload+="&weapon=${weapon}"
    
    send_event "$payload"
}

send_ladder_mount() {
    local name="$1"
    local guid="$2"
    
    local payload="type=ladder_mount"
    payload+="&match_id=${MATCH_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&player_name=${name}"
    payload+="&player_guid=${guid}"
    payload+="&pos_x=$(random_pos)"
    payload+="&pos_y=$(random_pos)"
    payload+="&pos_z=100"
    
    send_event "$payload"
}

send_crouch() {
    local name="$1"
    local guid="$2"
    
    local payload="type=player_crouch"
    payload+="&match_id=${MATCH_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&player_name=${name}"
    payload+="&player_guid=${guid}"
    payload+="&pos_x=$(random_pos)"
    payload+="&pos_y=$(random_pos)"
    payload+="&pos_z=0"
    
    send_event "$payload"
}

send_vehicle_collision() {
    local name="$1"
    local guid="$2"
    
    local payload="type=vehicle_collision"
    payload+="&match_id=${MATCH_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&player_name=${name}"
    payload+="&player_guid=${guid}"
    payload+="&velocity=$((RANDOM % 100))"
    
    send_event "$payload"
}

send_jump() {
    local name="$1"
    local guid="$2"
    
    local payload="type=jump"
    payload+="&match_id=${MATCH_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&player_name=${name}"
    payload+="&player_guid=${guid}"
    payload+="&pos_x=$(random_pos)"
    payload+="&pos_y=$(random_pos)"
    payload+="&pos_z=50"
    
    send_event "$payload"
}

send_heartbeat() {
    local player_count="$1"
    local allies_score="$2"
    local axis_score="$3"
    echo -e "${CYAN}♥ Heartbeat (${player_count} players, Allies: ${allies_score}, Axis: ${axis_score})${NC}"
    
    local payload="type=heartbeat"
    payload+="&match_id=${MATCH_ID}"
    payload+="&session_id=${SESSION_ID}"
    payload+="&map_name=${MAP_NAME}"
    payload+="&timestamp=$(timestamp)"
    payload+="&round_number=1"
    payload+="&allies_score=${allies_score}"
    payload+="&axis_score=${axis_score}"
    payload+="&player_count=${player_count}"
    
    send_event "$payload"
}

send_match_end() {
    local winning_team="$1"
    local allies_score="$2"
    local axis_score="$3"
    echo -e "\n${CYAN}■ Match Ended - Winner: ${winning_team}${NC}"
    
    local payload="type=match_end"
    payload+="&match_id=${MATCH_ID}"
    payload+="&session_id=${SESSION_ID}"
    payload+="&map_name=${MAP_NAME}"
    payload+="&timestamp=$(timestamp)"
    payload+="&duration=1200"
    payload+="&winning_team=${winning_team}"
    payload+="&allies_score=${allies_score}"
    payload+="&axis_score=${axis_score}"
    payload+="&total_rounds=1"
    
    send_event "$payload"
}

send_chat() {
    local name="$1"
    local guid="$2"
    local message="$3"
    echo -e "${BLUE}💬 ${name}: ${message}${NC}"
    
    local payload="type=chat"
    payload+="&match_id=${MATCH_ID}"
    payload+="&timestamp=$(timestamp)"
    payload+="&player_name=${name}"
    payload+="&player_guid=${guid}"
    payload+="&message=${message}"
    
    send_event "$payload"
}

# ==============================================================================
# TEST SCENARIOS
# ==============================================================================

test_quick() {
    echo -e "\n${GREEN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  MOHAA Event Simulator - Quick Test${NC}"
    echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
    
    # Start match
    send_match_start
    sleep 0.1
    
    # Players connect
    send_player_connect "$PLAYER1_NAME" "$PLAYER1_GUID"
    send_player_connect "$PLAYER2_NAME" "$PLAYER2_GUID"
    sleep 0.1
    
    # Players spawn
    send_player_spawn "$PLAYER1_NAME" "$PLAYER1_GUID" "allies"
    send_player_spawn "$PLAYER2_NAME" "$PLAYER2_GUID" "axis"
    sleep 0.1
    
    # Some combat
    echo -e "\n${YELLOW}── Combat Sequence ──${NC}"
    send_weapon_fire "$PLAYER1_NAME" "$PLAYER1_GUID" "m1_garand"
    send_damage "$PLAYER1_NAME" "$PLAYER1_GUID" "$PLAYER2_NAME" "$PLAYER2_GUID" 50 "m1_garand"
    send_kill "$PLAYER1_NAME" "$PLAYER1_GUID" "$PLAYER2_NAME" "$PLAYER2_GUID" "m1_garand" "torso"
    send_death "$PLAYER2_NAME" "$PLAYER2_GUID"
    sleep 0.1
    
    # Headshot!
    send_headshot "$PLAYER1_NAME" "$PLAYER1_GUID" "$PLAYER2_NAME" "$PLAYER2_GUID" "springfield"
    sleep 0.1
    
    # Grenade
    send_grenade_throw "$PLAYER2_NAME" "$PLAYER2_GUID"
    sleep 0.1
    
    # Chat
    send_chat "$PLAYER1_NAME" "$PLAYER1_GUID" "gg nice shot!"
    sleep 0.1
    
    # Heartbeat
    send_heartbeat 2 5 3
    
    echo -e "\n${GREEN}Quick test complete!${NC}"
}

test_full_match() {
    echo -e "\n${GREEN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  MOHAA Event Simulator - Full Match Simulation${NC}"
    echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
    
    local allies_score=0
    local axis_score=0
    
    # Start match
    send_match_start
    sleep 0.2
    
    # All 4 players connect and spawn
    echo -e "\n${CYAN}── Players Joining ──${NC}"
    send_player_connect "$PLAYER1_NAME" "$PLAYER1_GUID"
    send_player_spawn "$PLAYER1_NAME" "$PLAYER1_GUID" "allies"
    send_player_connect "$PLAYER2_NAME" "$PLAYER2_GUID"
    send_player_spawn "$PLAYER2_NAME" "$PLAYER2_GUID" "allies"
    send_player_connect "$PLAYER3_NAME" "$PLAYER3_GUID"
    send_player_spawn "$PLAYER3_NAME" "$PLAYER3_GUID" "axis"
    send_player_connect "$PLAYER4_NAME" "$PLAYER4_GUID"
    send_player_spawn "$PLAYER4_NAME" "$PLAYER4_GUID" "axis"
    sleep 0.2
    
    # Simulate 20 combat engagements
    echo -e "\n${YELLOW}── Combat Phase ──${NC}"
    for i in {1..20}; do
        # Random combat events
        local attackers=("$PLAYER1_NAME:$PLAYER1_GUID:allies" "$PLAYER2_NAME:$PLAYER2_GUID:allies" "$PLAYER3_NAME:$PLAYER3_GUID:axis" "$PLAYER4_NAME:$PLAYER4_GUID:axis")
        local victims=("$PLAYER3_NAME:$PLAYER3_GUID:axis" "$PLAYER4_NAME:$PLAYER4_GUID:axis" "$PLAYER1_NAME:$PLAYER1_GUID:allies" "$PLAYER2_NAME:$PLAYER2_GUID:allies")
        
        local attacker_idx=$((RANDOM % 4))
        local victim_idx=$(( (attacker_idx + 2 + RANDOM % 2) % 4 ))  # Pick enemy
        
        IFS=':' read -r att_name att_guid att_team <<< "${attackers[$attacker_idx]}"
        IFS=':' read -r vic_name vic_guid vic_team <<< "${victims[$victim_idx]}"
        
        local weapon=$(random_weapon)
        local hitloc=$(random_hitloc)
        
        # Fire shots
        for shot in {1..3}; do
            send_weapon_fire "$att_name" "$att_guid" "$weapon" > /dev/null 2>&1
        done
        
        # Damage and kill
        send_damage "$att_name" "$att_guid" "$vic_name" "$vic_guid" 75 "$weapon"
        
        if [ $((RANDOM % 3)) -eq 0 ]; then
            # Headshot!
            send_headshot "$att_name" "$att_guid" "$vic_name" "$vic_guid" "$weapon"
            hitloc="head"
        fi
        
        send_kill "$att_name" "$att_guid" "$vic_name" "$vic_guid" "$weapon" "$hitloc"
        send_death "$vic_name" "$vic_guid"
        
        # Update scores
        if [ "$att_team" = "allies" ]; then
            ((allies_score++))
        else
            ((axis_score++))
        fi
        
        # Occasional grenade
        if [ $((RANDOM % 5)) -eq 0 ]; then
            send_grenade_throw "$att_name" "$att_guid"
        fi
        
        # Occasional chat
        if [ $((RANDOM % 10)) -eq 0 ]; then
            local messages=("nice shot!" "gg" "lol" "wow" "noob" "ez")
            send_chat "$vic_name" "$vic_guid" "${messages[$RANDOM % ${#messages[@]}]}"
        fi
        
        # Creative Stats Events
        # Random Reload (OCD Reloading)
        if [ $((RANDOM % 3)) -eq 0 ]; then
             send_reload "$att_name" "$att_guid" "$weapon" > /dev/null 2>&1
        fi
        
        # Random Weapon Drop (Butterfingers)
        if [ $((RANDOM % 10)) -eq 0 ]; then
             send_weapon_drop "$vic_name" "$vic_guid" > /dev/null 2>&1
        fi
        
        # Random Movement (Jump/Crouch/Ladder)
        if [ $((RANDOM % 2)) -eq 0 ]; then
             send_jump "$att_name" "$att_guid" > /dev/null 2>&1
        fi
        if [ $((RANDOM % 5)) -eq 0 ]; then
             send_crouch "$att_name" "$att_guid" > /dev/null 2>&1
        fi
        if [ $((RANDOM % 15)) -eq 0 ]; then
             send_ladder_mount "$att_name" "$att_guid" > /dev/null 2>&1
        fi
        
        # Random Vehicle Collision (Road Rage)
        if [ $((RANDOM % 20)) -eq 0 ]; then
             send_vehicle_collision "$att_name" "$att_guid" > /dev/null 2>&1
        fi
        
        sleep 0.1
    done
    
    # Heartbeat mid-match
    echo -e "\n${CYAN}── Mid-Match Status ──${NC}"
    send_heartbeat 4 $allies_score $axis_score
    
    # End match
    sleep 0.2
    if [ $allies_score -gt $axis_score ]; then
        send_match_end "allies" $allies_score $axis_score
    else
        send_match_end "axis" $allies_score $axis_score
    fi
    
    echo -e "\n${GREEN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  Match Complete!${NC}"
    echo -e "${GREEN}  Allies: ${allies_score} | Axis: ${axis_score}${NC}"
    echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
}

test_stress() {
    echo -e "\n${GREEN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  MOHAA Event Simulator - Stress Test (100 rapid events)${NC}"
    echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
    
    send_match_start
    
    local success=0
    local failed=0
    
    echo -e "\n${YELLOW}Sending 100 rapid kill events...${NC}"
    for i in {1..100}; do
        local payload="type=kill"
        payload+="&match_id=${MATCH_ID}"
        payload+="&timestamp=$(timestamp)"
        payload+="&attacker_name=Stress_${i}"
        payload+="&attacker_guid=STRESS$(printf '%012d' $i)"
        payload+="&victim_name=Target_${i}"
        payload+="&victim_guid=TARGET$(printf '%012d' $i)"
        payload+="&weapon=$(random_weapon)"
        payload+="&hitloc=$(random_hitloc)"
        payload+="&server_token=${SERVER_TOKEN}"
        payload+="&server_id=${SERVER_ID}"
        
        response=$(curl -s -w "%{http_code}" -o /dev/null -X POST "${EVENTS_ENDPOINT}" \
            -H "Content-Type: application/x-www-form-urlencoded" \
            -d "${payload}")
        
        if [ "$response" = "202" ] || [ "$response" = "200" ]; then
            ((success++))
        else
            ((failed++))
        fi
        
        # Progress indicator
        if [ $((i % 10)) -eq 0 ]; then
            echo -e "${CYAN}Progress: ${i}/100 (Success: ${success}, Failed: ${failed})${NC}"
        fi
    done
    
    echo -e "\n${GREEN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  Stress Test Complete!${NC}"
    echo -e "${GREEN}  Success: ${success} | Failed: ${failed}${NC}"
    echo -e "${GREEN}════════════════════════════════════════════════════════════${NC}"
}

check_api() {
    echo -e "\n${CYAN}Checking API health...${NC}"
    response=$(curl -s -w "\n%{http_code}" "${API_BASE}/health")
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" = "200" ]; then
        echo -e "${GREEN}✓ API is healthy${NC}"
        echo -e "  Response: $body"
        return 0
    else
        echo -e "${RED}✗ API is not responding (HTTP $http_code)${NC}"
        echo -e "  Make sure the API server is running at ${API_BASE}"
        return 1
    fi
}

# ==============================================================================
# MAIN
# ==============================================================================

print_help() {
    echo "MOHAA Stats API - Event Simulator"
    echo ""
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  quick    - Run a quick test with basic events"
    echo "  full     - Simulate a full match with 20 kills"
    echo "  stress   - Stress test with 100 rapid events"
    echo "  check    - Check if API is healthy"
    echo "  help     - Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 check     # Check API health first"
    echo "  $0 quick     # Run quick test"
    echo "  $0 full      # Simulate full match"
    echo "  $0 stress    # Stress test"
}

case "${1:-quick}" in
    quick)
        check_api && test_quick
        ;;
    full)
        check_api && test_full_match
        ;;
    stress)
        check_api && test_stress
        ;;
    check)
        check_api
        ;;
    help|--help|-h)
        print_help
        ;;
    *)
        echo "Unknown command: $1"
        print_help
        exit 1
        ;;
esac
