#!/usr/bin/env python3
"""
Generate individual Bruno .bru files for each event type.
This allows manual testing of individual events from the Bruno UI.
"""

import re
from pathlib import Path
from datetime import datetime

# Event categories with sample payloads
EVENT_PAYLOADS = {
    # Game Flow Events
    "game_init": {
        "map_name": "dm/mohdm1",
        "game_type": "1",
        "max_players": 32
    },
    "game_start": {
        "timestamp": "2026-02-02T10:00:00Z"
    },
    "game_end": {
        "duration": 1200,
        "reason": "time_limit"
    },
    "match_start": {
        "match_id": "{{match_id}}",
        "map_name": "dm/mohdm1"
    },
    "match_end": {
        "match_id": "{{match_id}}",
        "duration": 900
    },
    "match_outcome": {
        "match_id": "{{match_id}}",
        "winner": "allies",
        "allied_score": 150,
        "axis_score": 120
    },
    "round_start": {
        "round_number": 1
    },
    "round_end": {
        "round_number": 1,
        "duration": 300
    },
    "warmup_start": {
        "server_id": "{{server_id}}"
    },
    "warmup_end": {
        "duration": 30
    },
    "intermission_start": {
        "server_id": "{{server_id}}"
    },
    
    # Combat Events
    "player_kill": {
        "match_id": "{{match_id}}",
        "timestamp": 1738540800.0,
        "attacker_guid": "{{guid}}",
        "victim_guid": "victim_abc123",
        "weapon": "Colt 45",
        "damage": 100,
        "hitloc": "head",
        "mod": "MOD_PISTOL",
        "distance": 15.5
    },
    "death": {
        "player_guid": "{{guid}}",
        "attacker_guid": "killer_xyz789",
        "means_of_death": "MOD_PISTOL"
    },
    "damage": {
        "victim_guid": "{{guid}}",
        "attacker_guid": "attacker_abc",
        "damage": 35,
        "weapon": "Thompson",
        "hitloc": "torso",
        "mod": "MOD_THOMPSON"
    },
    "player_pain": {
        "victim_guid": "{{guid}}",
        "damage": 25,
        "attacker_guid": "attacker_def",
        "hitloc": "arm",
        "mod": "MOD_RIFLE"
    },
    "player_suicide": {
        "player_guid": "{{guid}}",
        "method": "grenade"
    },
    "player_crushed": {
        "victim_guid": "{{guid}}",
        "attacker_guid": "tank"
    },
    "player_telefragged": {
        "victim_guid": "{{guid}}",
        "attacker_guid": "killer_jkl"
    },
    "player_roadkill": {
        "victim_guid": "{{guid}}",
        "attacker_guid": "driver_mno",
        "weapon": "jeep"
    },
    "player_bash": {
        "attacker_guid": "{{guid}}",
        "victim_guid": "victim_pqr",
        "weapon": "rifle_butt"
    },
    "player_teamkill": {
        "attacker_guid": "{{guid}}",
        "victim_guid": "victim_stu",
        "weapon": "Thompson"
    },
    "weapon_fire": {
        "player_guid": "{{guid}}",
        "weapon": "BAR",
        "ammo_count": 18
    },
    "weapon_hit": {
        "attacker_guid": "{{guid}}",
        "target_guid": "target_vwx",
        "weapon": "KAR98K",
        "hitloc": "head",
        "mod": "MOD_KAR98K",
        "damage": 75
    },
    "weapon_change": {
        "player_guid": "{{guid}}",
        "old_weapon": "Colt 45",
        "new_weapon": "Thompson"
    },
    "reload": {
        "player_guid": "{{guid}}",
        "weapon": "Thompson"
    },
    "weapon_reload_done": {
        "player_guid": "{{guid}}",
        "weapon": "Thompson"
    },
    "weapon_ready": {
        "player_guid": "{{guid}}",
        "weapon": "Springfield"
    },
    "weapon_no_ammo": {
        "player_guid": "{{guid}}",
        "weapon": "BAR"
    },
    "weapon_holster": {
        "player_guid": "{{guid}}",
        "weapon": "KAR98K"
    },
    "weapon_raise": {
        "player_guid": "{{guid}}",
        "weapon": "Thompson"
    },
    "weapon_drop": {
        "player_guid": "{{guid}}",
        "weapon": "Panzerschreck"
    },
    "grenade_throw": {
        "player_guid": "{{guid}}",
        "weapon": "Frag Grenade"
    },
    "grenade_explode": {
        "player_guid": "{{guid}}",
        "weapon": "Frag Grenade"
    },
    
    # Movement Events
    "jump": {
        "player_guid": "{{guid}}"
    },
    "land": {
        "player_guid": "{{guid}}",
        "fall_height": 128
    },
    "crouch": {
        "player_guid": "{{guid}}"
    },
    "prone": {
        "player_guid": "{{guid}}"
    },
    "player_stand": {
        "player_guid": "{{guid}}"
    },
    "player_spawn": {
        "player_guid": "{{guid}}",
        "player_team": "allies"
    },
    "player_respawn": {
        "player_guid": "{{guid}}"
    },
    "distance": {
        "player_guid": "{{guid}}",
        "distance": 1500.75
    },
    "player_movement": {
        "player_guid": "{{guid}}"
    },
    "ladder_mount": {
        "player_guid": "{{guid}}"
    },
    "ladder_dismount": {
        "player_guid": "{{guid}}"
    },
    
    # Interaction Events
    "use": {
        "player_guid": "{{guid}}",
        "entity": "door_main"
    },
    "player_use_object_start": {
        "player_guid": "{{guid}}",
        "object": "mg42_turret_1"
    },
    "player_use_object_finish": {
        "player_guid": "{{guid}}",
        "object": "mg42_turret_1",
        "duration": 15
    },
    "player_spectate": {
        "player_guid": "{{guid}}",
        "target_guid": "target_yz"
    },
    "player_freeze": {
        "player_guid": "{{guid}}",
        "reason": "intermission"
    },
    "chat": {
        "player_guid": "{{guid}}",
        "message": "gg wp",
        "team_only": False
    },
    
    # Item Events
    "item_pickup": {
        "player_guid": "{{guid}}",
        "item": "healthpack",
        "location": "120 180 64"
    },
    "item_drop": {
        "player_guid": "{{guid}}",
        "item": "ammo_clip",
        "location": "150 200 64"
    },
    "item_respawn": {
        "item": "healthpack_large",
        "location": "200 300 72"
    },
    "health_pickup": {
        "player_guid": "{{guid}}",
        "health_restored": 25
    },
    "ammo_pickup": {
        "player_guid": "{{guid}}",
        "ammo_type": "rifle",
        "amount": 50
    },
    "armor_pickup": {
        "player_guid": "{{guid}}",
        "armor_amount": 50
    },
    
    # Vehicle Events
    "vehicle_enter": {
        "player_guid": "{{guid}}",
        "vehicle": "jeep_1",
        "position": "driver"
    },
    "vehicle_exit": {
        "player_guid": "{{guid}}",
        "vehicle": "jeep_1"
    },
    "vehicle_death": {
        "vehicle": "tank_1",
        "destroyer_guid": "destroyer_ab"
    },
    "vehicle_crash": {
        "vehicle": "jeep_2",
        "driver_guid": "{{guid}}",
        "speed": 85,
        "damage": 45
    },
    "vehicle_change": {
        "player_guid": "{{guid}}",
        "from_vehicle": "jeep_1",
        "to_vehicle": "jeep_2"
    },
    "turret_enter": {
        "player_guid": "{{guid}}",
        "turret": "mg42_nest_1"
    },
    "turret_exit": {
        "player_guid": "{{guid}}",
        "turret": "mg42_nest_1"
    },
    
    # Server Events
    "server_init": {
        "version": "2.60",
        "protocol": 17
    },
    "server_start": {
        "timestamp": "2026-02-02T10:00:00Z"
    },
    "server_shutdown": {
        "reason": "restart"
    },
    "server_spawned": {
        "map_name": "dm/mohdm1"
    },
    "server_console_command": {
        "command": "kick player_123",
        "executor": "admin"
    },
    "heartbeat": {
        "players": 12,
        "cpu_usage": 35.5
    },
    
    # Map Events
    "map_init": {
        "map_name": "dm/mohdm1"
    },
    "map_start": {
        "map_name": "dm/mohdm1",
        "timestamp": "2026-02-02T10:05:00Z"
    },
    "map_ready": {
        "map_name": "dm/mohdm1"
    },
    "map_shutdown": {
        "map_name": "dm/mohdm1"
    },
    "map_load_start": {
        "map_name": "obj/obj_team1"
    },
    "map_load_end": {
        "map_name": "obj/obj_team1",
        "load_time": 2.5
    },
    "map_change_start": {
        "from_map": "dm/mohdm1",
        "to_map": "dm/mohdm6"
    },
    "map_restart": {
        "map_name": "dm/mohdm1"
    },
    
    # Team Events
    "team_join": {
        "player_guid": "{{guid}}",
        "team": "allies"
    },
    "team_change": {
        "player_guid": "{{guid}}",
        "from_team": "axis",
        "to_team": "allies"
    },
    "team_win": {
        "winning_team": "allies",
        "score": 200
    },
    "vote_start": {
        "player_guid": "{{guid}}",
        "vote_type": "map_change",
        "vote_target": "dm/mohdm6"
    },
    "vote_passed": {
        "vote_type": "kick_player",
        "yes_votes": 8,
        "no_votes": 2
    },
    "vote_failed": {
        "vote_type": "map_change",
        "yes_votes": 3,
        "no_votes": 7
    },
    
    # Client Events
    "connect": {
        "player_guid": "{{guid}}",
        "ip": "192.168.1.100",
        "name": "Player1"
    },
    "disconnect": {
        "player_guid": "{{guid}}",
        "reason": "quit"
    },
    "client_begin": {
        "player_guid": "{{guid}}"
    },
    "client_userinfo_changed": {
        "player_guid": "{{guid}}",
        "name": "NewPlayerName"
    },
    "player_inactivity_drop": {
        "player_guid": "{{guid}}",
        "idle_time": 180
    },
    
    # World Events
    "door_open": {
        "door": "door_main",
        "opener_guid": "{{guid}}"
    },
    "door_close": {
        "door": "door_main"
    },
    "explosion": {
        "location": "300 400 80",
        "radius": 150,
        "damage": 200
    },
    
    # Bot/Actor Events
    "actor_spawn": {
        "actor_id": "actor_123",
        "actor_type": "soldier",
        "location": "500 600 64"
    },
    "actor_killed": {
        "actor_id": "actor_123",
        "killer_guid": "{{guid}}"
    },
    "bot_spawn": {
        "bot_id": "bot_001",
        "team": "axis"
    },
    "bot_killed": {
        "bot_id": "bot_001",
        "killer_guid": "{{guid}}"
    },
    "bot_roam": {
        "bot_id": "bot_002",
        "location": "100 150 64"
    },
    "bot_curious": {
        "bot_id": "bot_002",
        "target_location": "200 250 64"
    },
    "bot_attack": {
        "bot_id": "bot_003",
        "target_guid": "{{guid}}"
    },
    
    # Objective Events
    "objective_update": {
        "objective_id": "obj_1",
        "status": "in_progress",
        "progress": 65
    },
    "objective_capture": {
        "objective_id": "obj_1",
        "capturing_team": "allies",
        "player_guid": "{{guid}}"
    },
    
    # Score Events
    "score_change": {
        "player_guid": "{{guid}}",
        "score_delta": 10,
        "new_score": 150
    },
    "teamkill_kick": {
        "player_guid": "{{guid}}",
        "teamkill_count": 3
    },
    
    # Auth Events
    "player_auth": {
        "player_guid": "{{guid}}",
        "smf_id": "{{smf_id}}",
        "auth_token": "token_xyz"
    },
    "accuracy_summary": {
        "player_guid": "{{guid}}",
        "shots_fired": 150,
        "shots_hit": 67,
        "accuracy": 44.67
    },
    "identity_claim": {
        "player_guid": "{{guid}}",
        "claimed_id": "{{smf_id}}"
    }
}


def generate_bru_file(event_type: str, payload: dict) -> str:
    """Generate a .bru file for a single event type."""
    
    # Format the JSON payload as compact single-line JSON
    json_parts = [f'"type":"{event_type}"']
    for key, value in payload.items():
        if isinstance(value, str):
            json_parts.append(f'"{key}":"{value}"')
        elif isinstance(value, bool):
            json_parts.append(f'"{key}":{str(value).lower()}')
        elif isinstance(value, (int, float)):
            json_parts.append(f'"{key}":{value}')
    
    # Wrap in array as API expects []models.RawEvent
    json_body = "[{" + ",".join(json_parts) + "}]"
    
    # Create nice display name
    display_name = event_type.replace('_', ' ').title()
    
    return f"""meta {{
  name: {display_name}
  type: http
  seq: 1
}}

post {{
  url: {{{{base_url}}}}/ingest/events
  body: json
  auth: none
}}

headers {{
  Content-Type: application/json
  X-Server-Token: {{{{server_token}}}}
}}

body:json {{
  {json_body}
}}

tests {{
  test("Response status is 202", function() {{
    expect(res.status).to.equal(202);
  }});
  
  test("Response has status property", function() {{
    expect(res.body).to.have.property('status');
    expect(res.body.status).to.equal('accepted');
  }});
  
  test("At least one event processed", function() {{
    expect(res.body).to.have.property('processed');
    expect(res.body.processed).to.be.greaterThan(0);
  }});
}}

docs {{
  ## {display_name} Event
  
  Event Type: `{event_type}`
  
  ### Description
  This event is posted when a {event_type.replace('_', ' ')} occurs in the game.
  
  ### Payload Fields
  {chr(10).join(f"  - `{key}`: {type(value).__name__}" for key, value in payload.items())}
  
  ### Notes
  - Modify the payload values as needed for your test scenario
  - The type field is set to `{event_type}`
  - Uses server_token from environment for authentication
}}
"""


def main():
    script_dir = Path(__file__).parent
    project_root = script_dir.parent
    events_dir = project_root / "bruno" / "Ingestion" / "Events"
    
    # Create events directory
    events_dir.mkdir(parents=True, exist_ok=True)
    
    print(f"🎯 Generating individual event .bru files...")
    print(f"📁 Output directory: {events_dir}")
    
    # Parse event types from generated file
    event_types_file = project_root / "internal" / "models" / "event_types_generated.go"
    with open(event_types_file, 'r') as f:
        content = f.read()
    
    # Extract event types using regex
    pattern = r'Event\w+\s+EventType\s+=\s+"([^"]+)"'
    matches = re.findall(pattern, content)
    
    created_count = 0
    for event_type in matches:
        if event_type in EVENT_PAYLOADS:
            # Add required fields to all payloads if not present
            payload = EVENT_PAYLOADS[event_type].copy()
            if "match_id" not in payload:
                payload["match_id"] = "{{match_id}}"
            if "timestamp" not in payload:
                payload["timestamp"] = 1738540800.0
            
            bru_content = generate_bru_file(event_type, payload)
            
            # Create filename: player_kill -> Player Kill.bru
            filename = event_type.replace('_', ' ').title() + ".bru"
            filepath = events_dir / filename
            
            with open(filepath, 'w') as f:
                f.write(bru_content)
            
            created_count += 1
            print(f"  ✓ {filename}")
        else:
            print(f"  ⚠ Missing payload definition for: {event_type}")
    
    print(f"\n✅ Generated {created_count} event .bru files")
    print(f"📂 Location: bruno/Ingestion/Events/")
    print(f"\n💡 Next steps:")
    print(f"   1. Open Bruno Desktop")
    print(f"   2. Navigate to Ingestion → Events")
    print(f"   3. Select any event (e.g., 'Player Kill')")
    print(f"   4. Modify payload fields as needed")
    print(f"   5. Click 'Send' to test")


if __name__ == "__main__":
    main()
