#!/bin/bash
# Comprehensive test data seeder for OpenMOHAA Stats API
# Seeds realistic player stats for testing the War Room display

set -e

API_URL="http://localhost:8080/api/v1/ingest/events"
SERVER_ID="test-server-001"
MATCH_ID="550e8400-e29b-41d4-a716-446655440001"

# Test player GUIDs
PLAYER1_GUID="war-room-test-001"
PLAYER1_NAME="TestWarrior"
PLAYER2_GUID="war-room-test-002" 
PLAYER2_NAME="BotEnemy"

# Maps
MAPS=("dm_stalingrad" "obj_hunt" "dm_normandy" "obj_berlin" "dm_pacific")

# Weapons
WEAPONS=("Thompson" "MP40" "Kar98k" "M1_Garand" "Springfield" "BAR" "StG44" "Grenade" "Colt45" "Knife")

# Hit locations
HITLOCS=("head" "torso" "left_arm" "right_arm" "left_leg" "right_leg")
STANCES=("stand" "crouch" "prone")

TIMESTAMP=$(date +%s)

echo "🎮 Starting comprehensive data seeding..."
echo "   Server: $SERVER_ID"
echo "   Match: $MATCH_ID"
echo ""

# Function to send event
send_event() {
    local payload=$1
    # Compact JSON to single line as API expects NDJSON
    local clean_payload=$(echo "$payload" | tr -d '\n' | tr -s ' ')
    curl -s -X POST "$API_URL" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer test-token" \
        -d "$clean_payload" > /dev/null
}

# Start a match
echo "📍 Starting match on ${MAPS[0]}..."
send_event "{
    \"type\": \"match_start\",
    \"match_id\": \"$MATCH_ID\",
    \"server_id\": \"$SERVER_ID\",
    \"map_name\": \"${MAPS[0]}\",
    \"gametype\": \"tdm\",
    \"timestamp\": $TIMESTAMP
}"

# Connect players
echo "👥 Connecting players..."
send_event "{
    \"type\": \"connect\",
    \"match_id\": \"$MATCH_ID\",
    \"server_id\": \"$SERVER_ID\",
    \"map_name\": \"${MAPS[0]}\",
    \"player_guid\": \"$PLAYER1_GUID\",
    \"player_name\": \"$PLAYER1_NAME\",
    \"player_team\": \"allies\",
    \"timestamp\": $TIMESTAMP
}"

send_event "{
    \"type\": \"connect\",
    \"match_id\": \"$MATCH_ID\",
    \"server_id\": \"$SERVER_ID\",
    \"map_name\": \"${MAPS[0]}\",
    \"player_guid\": \"$PLAYER2_GUID\",
    \"player_name\": \"$PLAYER2_NAME\",
    \"player_team\": \"axis\",
    \"timestamp\": $TIMESTAMP
}"

echo "🔫 Generating 150 kills for $PLAYER1_NAME..."
for i in $(seq 1 150); do
    weapon=${WEAPONS[$((RANDOM % ${#WEAPONS[@]}))]}
    hitloc=${HITLOCS[$((RANDOM % ${#HITLOCS[@]}))]}
    map=${MAPS[$((RANDOM % ${#MAPS[@]}))]}
    damage=$((50 + RANDOM % 100))
    stance=${STANCES[$((RANDOM % ${#STANCES[@]}))]}
    
    send_event "{
        \"type\": \"kill\",
        \"match_id\": \"$MATCH_ID\",
        \"server_id\": \"$SERVER_ID\",
        \"map_name\": \"$map\",
        \"attacker_guid\": \"$PLAYER1_GUID\",
        \"attacker_name\": \"$PLAYER1_NAME\",
        \"attacker_team\": \"allies\",
        \"attacker_smf_id\": 1,
        \"weapon\": \"$weapon\",
        \"pos_x\": $((RANDOM % 5000)),
        \"pos_y\": $((RANDOM % 5000)),
        \"victim_guid\": \"$PLAYER2_GUID\",
        \"victim_name\": \"$PLAYER2_NAME\",
        \"victim_team\": \"axis\",
        \"victim_smf_id\": 0,
        \"target_pos_x\": $((RANDOM % 5000)),
        \"target_pos_y\": $((RANDOM % 5000)),
        \"hitloc\": \"$hitloc\",
        \"damage\": $damage,
        \"attacker_stance\": \"$stance\",
        \"timestamp\": $((TIMESTAMP + i))
    }"
    
    # Headshot event if head
    if [ "$hitloc" == "head" ]; then
        send_event "{
            \"type\": \"headshot\",
            \"match_id\": \"$MATCH_ID\",
            \"server_id\": \"$SERVER_ID\",
            \"map_name\": \"$map\",
            \"attacker_guid\": \"$PLAYER1_GUID\",
            \"attacker_name\": \"$PLAYER1_NAME\",
            \"victim_guid\": \"$PLAYER2_GUID\",
            \"timestamp\": $((TIMESTAMP + i))
        }"
    fi
    
    # Progress indicator
    if [ $((i % 30)) -eq 0 ]; then
        echo "   Kills: $i/150"
    fi
done

echo "💀 Generating 45 deaths for $PLAYER1_NAME..."
for i in $(seq 1 45); do
    weapon=${WEAPONS[$((RANDOM % ${#WEAPONS[@]}))]}
    hitloc=${HITLOCS[$((RANDOM % ${#HITLOCS[@]}))]}
    
    send_event "{
        \"type\": \"kill\",
        \"match_id\": \"$MATCH_ID\",
        \"server_id\": \"$SERVER_ID\",
        \"map_name\": \"${MAPS[0]}\",
        \"attacker_guid\": \"$PLAYER2_GUID\",
        \"attacker_name\": \"$PLAYER2_NAME\",
        \"attacker_team\": \"axis\",
        \"weapon\": \"$weapon\",
        \"victim_guid\": \"$PLAYER1_GUID\",
        \"victim_name\": \"$PLAYER1_NAME\",
        \"victim_team\": \"allies\",
        \"hitloc\": \"$hitloc\",
        \"damage\": 100,
        \"timestamp\": $((TIMESTAMP + 200 + i))
    }"
done

echo "🎯 Generating 500 shots (35% accuracy)..."
for i in $(seq 1 500); do
    weapon=${WEAPONS[$((RANDOM % ${#WEAPONS[@]}))]}
    
    send_event "{
        \"type\": \"weapon_fire\",
        \"match_id\": \"$MATCH_ID\",
        \"server_id\": \"$SERVER_ID\",
        \"map_name\": \"${MAPS[0]}\",
        \"player_guid\": \"$PLAYER1_GUID\",
        \"player_name\": \"$PLAYER1_NAME\",
        \"weapon\": \"$weapon\",
        \"timestamp\": $((TIMESTAMP + 300 + i))
    }"
    
    # ~35% hit rate
    if [ $((RANDOM % 100)) -lt 35 ]; then
        send_event "{
            \"type\": \"weapon_hit\",
            \"match_id\": \"$MATCH_ID\",
            \"server_id\": \"$SERVER_ID\",
            \"map_name\": \"${MAPS[0]}\",
            \"player_guid\": \"$PLAYER1_GUID\",
            \"player_name\": \"$PLAYER1_NAME\",
            \"weapon\": \"$weapon\",
            \"hitloc\": \"${HITLOCS[$((RANDOM % ${#HITLOCS[@]}))]}\",
            \"timestamp\": $((TIMESTAMP + 300 + i))
        }"
    fi
    
    if [ $((i % 100)) -eq 0 ]; then
        echo "   Shots: $i/500"
    fi
done

echo "🏃 Generating 100 jump events..."
for i in $(seq 1 100); do
    send_event "{
        \"type\": \"jump\",
        \"match_id\": \"$MATCH_ID\",
        \"server_id\": \"$SERVER_ID\",
        \"map_name\": \"${MAPS[0]}\",
        \"player_guid\": \"$PLAYER1_GUID\",
        \"player_name\": \"$PLAYER1_NAME\",
        \"pos_x\": $((RANDOM % 5000)),
        \"pos_y\": $((RANDOM % 5000)),
        \"timestamp\": $((TIMESTAMP + 900 + i))
    }"
done

echo "📏 Generating distance traveled..."
send_event "{
    \"type\": \"distance\",
    \"match_id\": \"$MATCH_ID\",
    \"server_id\": \"$SERVER_ID\",
    \"map_name\": \"${MAPS[0]}\",
    \"player_guid\": \"$PLAYER1_GUID\",
    \"player_name\": \"$PLAYER1_NAME\",
    \"distance\": 15000,
    \"timestamp\": $((TIMESTAMP + 1000))
}"

echo "💣 Generating 20 grenade throws..."
for i in $(seq 1 20); do
    send_event "{
        \"type\": \"grenade_throw\",
        \"match_id\": \"$MATCH_ID\",
        \"server_id\": \"$SERVER_ID\",
        \"map_name\": \"${MAPS[0]}\",
        \"player_guid\": \"$PLAYER1_GUID\",
        \"player_name\": \"$PLAYER1_NAME\",
        \"projectile\": \"frag\",
        \"timestamp\": $((TIMESTAMP + 1100 + i))
    }"
done

echo "📦 Generating 30 item pickups..."
ITEMS=("health_small" "health_large" "ammo_thompson" "ammo_kar98" "helmet")
for i in $(seq 1 30); do
    item=${ITEMS[$((RANDOM % ${#ITEMS[@]}))]}
    send_event "{
        \"type\": \"item_pickup\",
        \"match_id\": \"$MATCH_ID\",
        \"server_id\": \"$SERVER_ID\",
        \"map_name\": \"${MAPS[0]}\",
        \"player_guid\": \"$PLAYER1_GUID\",
        \"player_name\": \"$PLAYER1_NAME\",
        \"item\": \"$item\",
        \"timestamp\": $((TIMESTAMP + 1200 + i))
    }"
done

echo "💬 Generating 15 chat messages..."
for i in $(seq 1 15); do
    send_event "{
        \"type\": \"chat\",
        \"match_id\": \"$MATCH_ID\",
        \"server_id\": \"$SERVER_ID\",
        \"map_name\": \"${MAPS[0]}\",
        \"player_guid\": \"$PLAYER1_GUID\",
        \"player_name\": \"$PLAYER1_NAME\",
        \"message\": \"Test message $i\",
        \"timestamp\": $((TIMESTAMP + 1300 + i))
    }"
done

echo "🏁 Ending match..."
send_event "{
    \"type\": \"match_end\",
    \"match_id\": \"$MATCH_ID\",
    \"server_id\": \"$SERVER_ID\",
    \"map_name\": \"${MAPS[0]}\",
    \"winning_team\": \"allies\",
    \"duration\": 1800,
    \"timestamp\": $((TIMESTAMP + 1800))
}"

echo ""
echo "✅ Data seeding complete!"
echo ""
echo "📊 Summary for $PLAYER1_NAME ($PLAYER1_GUID):"
echo "   - 150 kills"
echo "   - 45 deaths (K/D ~3.3)"
echo "   - ~25 headshots"
echo "   - 500 shots fired, ~175 hits (~35% accuracy)"
echo "   - 100 jumps"
echo "   - 15km distance traveled"
echo "   - 20 grenades thrown"
echo "   - 30 item pickups"
echo "   - 15 chat messages"
echo ""
echo "🔗 Check the War Room at: http://localhost:8888/index.php?action=mohaawarroom"
