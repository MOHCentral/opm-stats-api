#!/bin/bash
# Comprehensive Data Seeder for elgan with ALL event types and complete stats
# This seeds realistic game data including wins, grenades, vehicles, achievements, etc.

set -e

API_URL="http://localhost:8080/api/v1/ingest/events"
GUID="72750883-29ae-4377-85c4-9367f1f89d1a"
NAME="elgan"
SMF_ID=1

# Arrays for variety
declare -a WEAPONS=("Thompson" "M1 Garand" "Kar98k" "MP40" "STG44" "BAR" "Springfield" "Colt .45" "Walther P38" "Shotgun" "Panzerschreck" "Bazooka" "Fists")
declare -a MAPS=("dm/mohdm1" "dm/mohdm2" "dm/mohdm3" "dm/mohdm4" "dm/mohdm5" "dm/mohdm6" "dm/mohdm7" "obj/obj_team1" "obj/obj_team2" "obj/obj_team3" "obj/obj_team4" "lib/stalingrad" "tdm/bridge")
declare -a SERVERS=("Main DM Server" "EU Objective" "US FFA" "Asia TDM" "Pro League")
declare -a OTHER_PLAYERS=("ShadowSniper" "IronWolf" "DeadlyEagle" "SilentKnight" "RapidFire" "GhostHunter" "NightStalker" "BulletStorm" "VenomStrike" "ThunderBolt")
declare -a OTHER_GUIDS=("guid-001" "guid-002" "guid-003" "guid-004" "guid-005" "guid-006" "guid-007" "guid-008" "guid-009" "guid-010")
declare -a HITLOCS=("head" "torso" "left_arm" "right_arm" "left_leg" "right_leg" "neck" "pelvis")
declare -a MODS=("rifle" "smg" "pistol" "sniper" "shotgun" "rocket" "grenade" "bash" "falling" "crushed")
declare -a ITEMS=("health_large" "health_small" "ammo_rifle" "ammo_smg" "armor" "helmet" "medkit")
declare -a VEHICLES=("tank" "jeep" "truck" "halftrack")

TOTAL_MATCHES=100
echo "🚀 Starting Comprehensive Seeder for $NAME ($GUID)"
echo "   Target: $TOTAL_MATCHES matches with complete event coverage"

EVENTS_SENT=0
MATCHES_WON=0

# Timestamp helper - generate timestamps relative to now
ts() {
    # Just use current timestamp, offset by random amount for variety
    local offset=${1:-0}
    date -u -d "@$(($(date +%s) - offset))" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || \
    date -u -v-${offset}S +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || \
    date -u +"%Y-%m-%dT%H:%M:%SZ"
}

# Send event helper
send_event() {
    local payload="$1"
    curl -s -X POST "$API_URL" \
        -H "Content-Type: application/json" \
        -H "X-Server-Token: seeder_test_token_2026" \
        -d "$payload" > /dev/null
    ((EVENTS_SENT++))
}

# Generate a full match
generate_match() {
    local match_num=$1
    local match_id="match_$(printf '%05d' $match_num)"
    local map=${MAPS[$RANDOM % ${#MAPS[@]}]}
    local server=${SERVERS[$RANDOM % ${#SERVERS[@]}]}
    local base_offset=$((86400 * (100 - match_num) / 10)) # Spread over ~10 days ago
    local duration=$((600 + RANDOM % 1800)) # 10-40 min matches
    
    # Decide if elgan wins (70% win rate)
    local won=0
    if [ $((RANDOM % 100)) -lt 70 ]; then
        won=1
        ((MATCHES_WON++))
    fi
    
    # Match start events
    send_event "{\"event_type\":\"connect\",\"timestamp\":\"$(ts $base_offset)\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID}"
    send_event "{\"event_type\":\"spawn\",\"timestamp\":\"$(ts $((base_offset - 2)))\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID,\"team\":\"allies\"}"
    
    # Generate realistic match events
    local kills=$((5 + RANDOM % 20))
    local deaths=$((2 + RANDOM % 10))
    local shots_fired=$((kills * 8 + RANDOM % 50))
    local hits=$((kills * 5 + RANDOM % 20))
    local grenades=$((1 + RANDOM % 5))
    local jumps=$((10 + RANDOM % 30))
    local distance=$((500 + RANDOM % 3000))
    
    # Generate events throughout match
    for ((i=0; i<shots_fired; i++)); do
        local offset=$((RANDOM % duration))
        local weapon=${WEAPONS[$RANDOM % ${#WEAPONS[@]}]}
        send_event "{\"event_type\":\"weapon_fire\",\"timestamp\":\"$(ts "$start_time + $offset seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID,\"weapon\":\"$weapon\",\"ammo_remaining\":$((RANDOM % 30))}"
    done
    
    for ((i=0; i<hits; i++)); do
        local offset=$((RANDOM % duration))
        local weapon=${WEAPONS[$RANDOM % ${#WEAPONS[@]}]}
        local victim=${OTHER_PLAYERS[$RANDOM % ${#OTHER_PLAYERS[@]}]}
        local victim_guid=${OTHER_GUIDS[$RANDOM % ${#OTHER_GUIDS[@]}]}
        local hitloc=${HITLOCS[$RANDOM % ${#HITLOCS[@]}]}
        send_event "{\"event_type\":\"weapon_hit\",\"timestamp\":\"$(ts "$start_time + $offset seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"attacker_guid\":\"$GUID\",\"attacker_name\":\"$NAME\",\"attacker_smf_id\":$SMF_ID,\"target_id\":\"$victim_guid\",\"target_name\":\"$victim\",\"weapon\":\"$weapon\",\"hit_location\":\"$hitloc\",\"damage\":$((10 + RANDOM % 50))}"
    done
    
    for ((i=0; i<kills; i++)); do
        local offset=$((RANDOM % duration))
        local weapon=${WEAPONS[$RANDOM % ${#WEAPONS[@]}]}
        local victim=${OTHER_PLAYERS[$RANDOM % ${#OTHER_PLAYERS[@]}]}
        local victim_guid=${OTHER_GUIDS[$RANDOM % ${#OTHER_GUIDS[@]}]}
        local hitloc=${HITLOCS[$RANDOM % ${#HITLOCS[@]}]}
        local mod=${MODS[$RANDOM % ${#MODS[@]}]}
        
        # Damage event before kill
        send_event "{\"event_type\":\"damage\",\"timestamp\":\"$(ts "$start_time + $offset seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"attacker_guid\":\"$GUID\",\"attacker_name\":\"$NAME\",\"attacker_smf_id\":$SMF_ID,\"target_id\":\"$victim_guid\",\"target_name\":\"$victim\",\"damage\":$((50 + RANDOM % 100)),\"means_of_death\":\"$mod\"}"
        
        # Kill event
        send_event "{\"event_type\":\"kill\",\"timestamp\":\"$(ts "$start_time + $offset seconds + 1 second")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"attacker_guid\":\"$GUID\",\"attacker_name\":\"$NAME\",\"attacker_smf_id\":$SMF_ID,\"target_id\":\"$victim_guid\",\"target_name\":\"$victim\",\"weapon\":\"$weapon\",\"hit_location\":\"$hitloc\",\"means_of_death\":\"$mod\",\"damage\":100}"
        
        # Headshot (30% of kills)
        if [ $((RANDOM % 100)) -lt 30 ]; then
            send_event "{\"event_type\":\"headshot\",\"timestamp\":\"$(ts "$start_time + $offset seconds + 1 second")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"attacker_guid\":\"$GUID\",\"attacker_name\":\"$NAME\",\"attacker_smf_id\":$SMF_ID,\"target_id\":\"$victim_guid\",\"target_name\":\"$victim\",\"weapon\":\"$weapon\"}"
        fi
    done
    
    for ((i=0; i<deaths; i++)); do
        local offset=$((RANDOM % duration))
        local killer=${OTHER_PLAYERS[$RANDOM % ${#OTHER_PLAYERS[@]}]}
        local killer_guid=${OTHER_GUIDS[$RANDOM % ${#OTHER_GUIDS[@]}]}
        local weapon=${WEAPONS[$RANDOM % ${#WEAPONS[@]}]}
        send_event "{\"event_type\":\"death\",\"timestamp\":\"$(ts "$start_time + $offset seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID,\"attacker_id\":\"$killer_guid\",\"attacker_name\":\"$killer\",\"weapon\":\"$weapon\"}"
        
        # Respawn after death
        send_event "{\"event_type\":\"spawn\",\"timestamp\":\"$(ts "$start_time + $offset seconds + 5 seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID,\"team\":\"allies\"}"
    done
    
    # Grenade events
    for ((i=0; i<grenades; i++)); do
        local offset=$((RANDOM % duration))
        send_event "{\"event_type\":\"grenade_throw\",\"timestamp\":\"$(ts "$start_time + $offset seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID,\"projectile_type\":\"grenade\"}"
        
        # Grenade kill (50% success rate)
        if [ $((RANDOM % 2)) -eq 0 ]; then
            local victim=${OTHER_PLAYERS[$RANDOM % ${#OTHER_PLAYERS[@]}]}
            local victim_guid=${OTHER_GUIDS[$RANDOM % ${#OTHER_GUIDS[@]}]}
            send_event "{\"event_type\":\"grenade_kill\",\"timestamp\":\"$(ts "$start_time + $offset seconds + 2 seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"attacker_guid\":\"$GUID\",\"attacker_name\":\"$NAME\",\"attacker_smf_id\":$SMF_ID,\"target_id\":\"$victim_guid\",\"target_name\":\"$victim\"}"
        fi
    done
    
    # Movement events
    for ((i=0; i<jumps; i++)); do
        local offset=$((RANDOM % duration))
        send_event "{\"event_type\":\"jump\",\"timestamp\":\"$(ts "$start_time + $offset seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID}"
    done
    
    send_event "{\"event_type\":\"distance\",\"timestamp\":\"$(ts "$start_time + $duration seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID,\"walked\":$((distance / 2)),\"sprinted\":$((distance / 2)),\"jumped\":$jumps}"
    
    # Stance changes
    send_event "{\"event_type\":\"crouch\",\"timestamp\":\"$(ts "$start_time + $((RANDOM % duration)) seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID}"
    send_event "{\"event_type\":\"prone\",\"timestamp\":\"$(ts "$start_time + $((RANDOM % duration)) seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID}"
    
    # Vehicle (10% of matches)
    if [ $((RANDOM % 10)) -eq 0 ]; then
        local vehicle=${VEHICLES[$RANDOM % ${#VEHICLES[@]}]}
        local offset=$((RANDOM % duration))
        send_event "{\"event_type\":\"vehicle_enter\",\"timestamp\":\"$(ts "$start_time + $offset seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID,\"vehicle\":\"$vehicle\"}"
        send_event "{\"event_type\":\"vehicle_kill\",\"timestamp\":\"$(ts "$start_time + $offset seconds + 30 seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"attacker_guid\":\"$GUID\",\"attacker_name\":\"$NAME\",\"attacker_smf_id\":$SMF_ID,\"target_id\":\"${OTHER_GUIDS[$RANDOM % ${#OTHER_GUIDS[@]}]}\",\"target_name\":\"${OTHER_PLAYERS[$RANDOM % ${#OTHER_PLAYERS[@]}]}\",\"vehicle\":\"$vehicle\"}"
    fi
    
    # Item pickups
    for ((i=0; i<$((3 + RANDOM % 5)); i++)); do
        local offset=$((RANDOM % duration))
        local item=${ITEMS[$RANDOM % ${#ITEMS[@]}]}
        send_event "{\"event_type\":\"item_pickup\",\"timestamp\":\"$(ts "$start_time + $offset seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID,\"item_type\":\"$item\",\"quantity\":1}"
    done
    
    # Bot kill (20% of matches)
    if [ $((RANDOM % 5)) -eq 0 ]; then
        local offset=$((RANDOM % duration))
        send_event "{\"event_type\":\"bot_killed\",\"timestamp\":\"$(ts "$start_time + $offset seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"attacker_guid\":\"$GUID\",\"attacker_name\":\"$NAME\",\"attacker_smf_id\":$SMF_ID,\"bot_name\":\"AI_Soldier_$((RANDOM % 10))\"}"
    fi
    
    # Objective events (for obj/ maps)
    if [[ $map == obj/* ]]; then
        local offset=$((RANDOM % duration))
        send_event "{\"event_type\":\"objective_capture\",\"timestamp\":\"$(ts "$start_time + $offset seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID,\"objective_index\":$((RANDOM % 5))}"
    fi
    
    # Match end
    if [ $won -eq 1 ]; then
        send_event "{\"event_type\":\"team_win\",\"timestamp\":\"$(ts "$start_time + $duration seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"team\":\"allies\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID}"
    fi
    
    send_event "{\"event_type\":\"player_spectate\",\"timestamp\":\"$(ts "$start_time + $duration seconds + 5 seconds")\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"player_smf_id\":$SMF_ID}"
    
    if [ $((match_num % 10)) -eq 0 ]; then
        echo "   ✓ Match $match_num/$TOTAL_MATCHES complete ($EVENTS_SENT events sent, $MATCHES_WON wins)"
    fi
}

# Generate all matches
for ((i=1; i<=TOTAL_MATCHES; i++)); do
    generate_match $i
done

echo ""
echo "✅ Seeding Complete!"
echo "   📊 Matches: $TOTAL_MATCHES"
echo "   🏆 Wins: $MATCHES_WON ($(( (MATCHES_WON * 100) / TOTAL_MATCHES ))%)"
echo "   ⚡ Events: $EVENTS_SENT"
echo ""
echo "Refresh your dashboard to see comprehensive stats!"
