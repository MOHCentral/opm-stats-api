#!/bin/bash
# Generate 50,000 varied events for user "elgan" (GUID_ELGAN)

API_URL="http://localhost:8080/api/v1/events"
GUID="GUID_ELGAN"
NAME="elgan"

# Event types with relative weights
declare -a KILL_MODS=("rifle" "smg" "pistol" "sniper" "shotgun" "mg" "rocket" "grenade" "bash" "knife")
declare -a MAPS=("dm/mohdm1" "dm/mohdm2" "dm/mohdm3" "dm/mohdm4" "dm/mohdm5" "dm/mohdm6" "obj/obj1" "obj/obj2")
declare -a SERVERS=("Main Server" "EU Deathmatch" "US Objective" "Asia FFA" "Pro League" "Newbie Friendly")
declare -a WEAPONS=("M1 Garand" "Thompson" "MP40" "Kar98k" "Springfield" "BAR" "StG44" "Colt .45" "Luger" "Shotgun" "Panzerschreck" "Bazooka" "Grenade")
declare -a ITEMS=("health_large" "health_small" "ammo_rifle" "ammo_smg" "ammo_pistol" "armor" "helmet" "medkit")
declare -a HITLOCS=("head" "torso" "left_arm" "right_arm" "left_leg" "right_leg" "neck" "pelvis")
declare -a OTHER_PLAYERS=("ShadowSniper" "IronWolf" "DeadlyEagle" "SilentKnight" "RapidFire" "GhostHunter" "NightStalker" "BulletStorm" "VenomStrike" "ThunderBolt")

TOTAL=50000
BATCH=500
SENT=0

echo "Seeding $TOTAL events for $NAME ($GUID)..."

generate_event() {
    local event_type=$1
    local ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local map=${MAPS[$RANDOM % ${#MAPS[@]}]}
    local server=${SERVERS[$RANDOM % ${#SERVERS[@]}]}
    local match_id="MATCH_ELGAN_$(printf '%05d' $((RANDOM % 500)))"
    
    case $event_type in
        kill)
            local victim=${OTHER_PLAYERS[$RANDOM % ${#OTHER_PLAYERS[@]}]}
            local weapon=${WEAPONS[$RANDOM % ${#WEAPONS[@]}]}
            local mod=${KILL_MODS[$RANDOM % ${#KILL_MODS[@]}]}
            local hitloc=${HITLOCS[$RANDOM % ${#HITLOCS[@]}]}
            echo "{\"event_type\":\"kill\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"attacker_guid\":\"$GUID\",\"attacker_name\":\"$NAME\",\"attacker_smf_id\":42,\"target_id\":\"GUID_${victim}\",\"target_name\":\"$victim\",\"target_smf_id\":0,\"weapon\":\"$weapon\",\"damage\":$((50 + RANDOM % 100)),\"means_of_death\":\"$mod\",\"hit_location\":\"$hitloc\"}"
            ;;
        death)
            local killer=${OTHER_PLAYERS[$RANDOM % ${#OTHER_PLAYERS[@]}]}
            local weapon=${WEAPONS[$RANDOM % ${#WEAPONS[@]}]}
            echo "{\"event_type\":\"death\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"attacker_id\":\"GUID_${killer}\",\"attacker_name\":\"$killer\",\"weapon\":\"$weapon\"}"
            ;;
        headshot)
            local victim=${OTHER_PLAYERS[$RANDOM % ${#OTHER_PLAYERS[@]}]}
            local weapon=${WEAPONS[$RANDOM % ${#WEAPONS[@]}]}
            echo "{\"event_type\":\"headshot\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"attacker_guid\":\"$GUID\",\"attacker_name\":\"$NAME\",\"target_id\":\"GUID_${victim}\",\"target_name\":\"$victim\",\"weapon\":\"$weapon\"}"
            ;;
        weapon_fire)
            local weapon=${WEAPONS[$RANDOM % ${#WEAPONS[@]}]}
            echo "{\"event_type\":\"weapon_fire\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"weapon\":\"$weapon\",\"ammo_remaining\":$((RANDOM % 30))}"
            ;;
        weapon_hit)
            local victim=${OTHER_PLAYERS[$RANDOM % ${#OTHER_PLAYERS[@]}]}
            local weapon=${WEAPONS[$RANDOM % ${#WEAPONS[@]}]}
            local hitloc=${HITLOCS[$RANDOM % ${#HITLOCS[@]}]}
            echo "{\"event_type\":\"weapon_hit\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"attacker_guid\":\"$GUID\",\"attacker_name\":\"$NAME\",\"target_id\":\"GUID_${victim}\",\"target_name\":\"$victim\",\"weapon\":\"$weapon\",\"hit_location\":\"$hitloc\",\"damage\":$((10 + RANDOM % 50))}"
            ;;
        damage)
            local victim=${OTHER_PLAYERS[$RANDOM % ${#OTHER_PLAYERS[@]}]}
            echo "{\"event_type\":\"damage\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"attacker_guid\":\"$GUID\",\"attacker_name\":\"$NAME\",\"target_id\":\"GUID_${victim}\",\"target_name\":\"$victim\",\"damage\":$((5 + RANDOM % 80)),\"means_of_death\":\"${KILL_MODS[$RANDOM % ${#KILL_MODS[@]}]}\"}"
            ;;
        jump)
            echo "{\"event_type\":\"jump\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"position_x\":$((RANDOM % 2000)),\"position_y\":$((RANDOM % 2000)),\"position_z\":$((RANDOM % 500))}"
            ;;
        crouch)
            echo "{\"event_type\":\"crouch\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\"}"
            ;;
        prone)
            echo "{\"event_type\":\"prone\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\"}"
            ;;
        ladder_mount)
            echo "{\"event_type\":\"ladder_mount\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\"}"
            ;;
        item_pickup)
            local item=${ITEMS[$RANDOM % ${#ITEMS[@]}]}
            echo "{\"event_type\":\"item_pickup\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"item_type\":\"$item\",\"quantity\":1}"
            ;;
        chat)
            echo "{\"event_type\":\"chat\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"message\":\"gg\"}"
            ;;
        door_open)
            echo "{\"event_type\":\"door_open\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\"}"
            ;;
        spawn)
            echo "{\"event_type\":\"spawn\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"team\":\"allies\"}"
            ;;
        connect)
            echo "{\"event_type\":\"connect\",\"timestamp\":\"$ts\",\"server_id\":\"$server\",\"map_name\":\"$map\",\"match_id\":\"$match_id\",\"player_guid\":\"$GUID\",\"player_name\":\"$NAME\",\"ip_address\":\"192.168.1.100\"}"
            ;;
    esac
}

# Event type distribution (weighted)
# kills: 15%, deaths: 10%, headshots: 5%, weapon_fire: 25%, weapon_hit: 20%, damage: 10%, 
# jump: 5%, crouch: 2%, prone: 1%, ladder: 1%, item_pickup: 3%, chat: 1%, door: 1%, spawn: 0.5%, connect: 0.5%

get_random_event_type() {
    local r=$((RANDOM % 100))
    if [ $r -lt 15 ]; then echo "kill"
    elif [ $r -lt 25 ]; then echo "death"
    elif [ $r -lt 30 ]; then echo "headshot"
    elif [ $r -lt 55 ]; then echo "weapon_fire"
    elif [ $r -lt 75 ]; then echo "weapon_hit"
    elif [ $r -lt 85 ]; then echo "damage"
    elif [ $r -lt 90 ]; then echo "jump"
    elif [ $r -lt 92 ]; then echo "crouch"
    elif [ $r -lt 93 ]; then echo "prone"
    elif [ $r -lt 94 ]; then echo "ladder_mount"
    elif [ $r -lt 97 ]; then echo "item_pickup"
    elif [ $r -lt 98 ]; then echo "chat"
    elif [ $r -lt 99 ]; then echo "door_open"
    else echo "spawn"
    fi
}

while [ $SENT -lt $TOTAL ]; do
    event_type=$(get_random_event_type)
    payload=$(generate_event "$event_type")
    
    response=$(curl -s -w "%{http_code}" -o /dev/null -X POST "$API_URL" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer server_token_12345" \
        -d "$payload")
    
    if [ "$response" = "200" ] || [ "$response" = "201" ] || [ "$response" = "202" ]; then
        ((SENT++))
        if [ $((SENT % 1000)) -eq 0 ]; then
            echo "Progress: $SENT / $TOTAL events sent"
        fi
    fi
done

echo "Done! Sent $SENT events for $NAME"
