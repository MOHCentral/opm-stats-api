# Event Ingestion Tools

## Quick Reference

### Option 1: Individual Event Requests (Bruno UI)

**Generate individual .bru files for all 105 events:**

```bash
# Using Make
make bruno-events

# Or NPM
npm run bruno:events

# Or directly
python3 tools/generate_bruno_events.py
```

**Then open Bruno Desktop:**
1. Open the collection
2. Navigate to `Ingestion` → `Events`
3. Select any event (e.g., "Player Kill")
4. Customize the JSON payload as needed
5. Click "Send"

**Features:**
- ✅ 106 individual, editable event requests
- ✅ Pre-filled with realistic sample data
- ✅ Environment variable support ({{guid}}, {{server_id}})
- ✅ Manual testing and debugging
- ✅ Built-in response tests

---

### Option 2: Bash Script (Complete Automation)

**Ingest all 105 event types individually:**

```bash
# Local environment (default)
./tools/ingest_all_events.sh

# Or specify environment
./tools/ingest_all_events.sh Local
./tools/ingest_all_events.sh Development
./tools/ingest_all_events.sh Production
```

**Via Make:**
```bash
make bruno-ingest-all
```

**Via NPM:**
```bash
npm run bruno:ingest-all
```

**Features:**
- ✅ Posts all 105 event types individually
- ✅ Color-coded real-time progress
- ✅ Success/failure tracking per event
- ✅ Comprehensive payload examples
- ✅ Environment variable support

**Output:**
```
╔══════════════════════════════════════════════════╗
║  OpenMOHAA Stats - Ingest All Event Types       ║
╔══════════════════════════════════════════════════╗
Environment: Local
Base URL: http://localhost:8084/api/v1

[1] Posting game_init... ✓
[2] Posting game_start... ✓
[3] Posting game_end... ✓
...
[106] Posting player_movement... ✓

Total events: 106
Success: 106
Failed: 0

✓ All event types successfully ingested!
```

---

### Option 3: Bruno Mega Request (Quick Multi-Event Test)

**In Bruno Desktop:**
1. Open collection
2. Navigate to `Ingestion/`
3. Run **"Ingest All Event Types (Mega Payload)"**

**Via CLI:**
```bash
cd bruno
bru run --env Local "Ingestion/POST ingest-all-event-types-mega.bru"
```

**Features:**
- ✅ Single request with ~30 representative events
- ✅ Newline-separated JSON format
- ✅ Covers all major categories
- ✅ Fast execution (~1 second)

**Use when:**
- Quick smoke test
- Testing multi-event payload parsing
- Verifying event pipeline is operational

---

## Event Categories Covered

| Category | Count | Examples |
|----------|-------|----------|
| Game Flow | 11 | game_init, match_start, round_end |
| Combat | 22 | player_kill, damage, weapon_fire |
| Movement | 10 | jump, land, crouch, spawn |
| Interaction | 6 | use, chat, player_freeze |
| Items | 6 | item_pickup, health_pickup, ammo_pickup |
| Vehicles | 7 | vehicle_enter, turret_enter, vehicle_crash |
| Server | 6 | server_init, heartbeat, console_command |
| Map | 8 | map_init, map_change_start, map_ready |
| Team | 6 | team_join, vote_start, team_win |
| Client | 5 | connect, disconnect, client_begin |
| World | 3 | door_open, door_close, explosion |
| Bots/Actors | 7 | bot_spawn, bot_killed, actor_spawn |
| Objectives | 2 | objective_update, objective_capture |
| Score | 2 | score_change, teamkill_kick |
| Auth/Meta | 3 | player_auth, accuracy_summary, identity_claim |

**Total: 105 event types**

---

## Environment Configuration

### Local (Default)
```bash
BASE_URL="http://localhost:8084/api/v1"
SERVER_TOKEN="4d170d00-8b08-4619-93d0-cec2ad7883e2"
```

### Development
```bash
BASE_URL="http://localhost:8080/api/v1"
SERVER_TOKEN="${SERVER_TOKEN}"  # From env var
```

### Production
```bash
BASE_URL="https://api.moh-central.net/api/v1"
SERVER_TOKEN="${SERVER_TOKEN}"  # From env var
```

**Override server token:**
```bash
SERVER_TOKEN="your-token-here" ./tools/ingest_all_events.sh Local
```

---

## Troubleshooting

### Script Reports Failures

**Check API is running:**
```bash
curl http://localhost:8084/api/v1/stats/global
```

**Check server token:**
```bash
# View in environment file
cat bruno/environments/Local.bru | grep server_token
```

**Check logs:**
```bash
docker logs opm-stats-api
```

### Event Not Ingesting

**Verify JSON format:**
```bash
# Test single event
curl -X POST http://localhost:8084/api/v1/ingest/events \
  -H "X-Server-Token: 4d170d00-8b08-4619-93d0-cec2ad7883e2" \
  -H "Content-Type: application/json" \
  -d '{"type":"test","timestamp":"2026-02-02T17:30:00Z","server_id":"test"}'
```

**Check event type exists:**
```bash
# List all valid event types
grep 'EventType.*=' opm-stats-api/internal/models/event_types_generated.go
```

### Rate Limiting

If ingesting rapidly, the API may apply backpressure:

```bash
# Add delay between events (modify script)
post_event() {
    # ... existing code ...
    sleep 0.1  # 100ms delay
}
```

---

## Integration Examples

### CI/CD Pipeline

```yaml
# .github/workflows/test.yml
- name: Test Event Ingestion
  run: |
    make bruno-ingest-all
    # Verify events in database
    make test
```

### Pre-Deployment Smoke Test

```bash
#!/bin/bash
# deploy.sh
echo "Running smoke tests..."
./tools/ingest_all_events.sh Development

if [ $? -eq 0 ]; then
    echo "✓ All events accepted, proceeding with deployment"
    ./deploy.sh
else
    echo "✗ Event ingestion failed, aborting deployment"
    exit 1
fi
```

### Load Testing

```bash
# Ingest 100 full event cycles
for i in {1..100}; do
    ./tools/ingest_all_events.sh Local &
done
wait
echo "Load test complete"
```

---

## Payload Examples

### Single Event (Bash)
```bash
curl -X POST http://localhost:8084/api/v1/ingest/events \
  -H "X-Server-Token: 4d170d00-8b08-4619-93d0-cec2ad7883e2" \
  -H "Content-Type: application/json" \
  -d '{"type":"player_kill","timestamp":"2026-02-02T17:30:00Z","server_id":"test","match_id":"match_1","attacker":{"guid":"p1","name":"Player1"},"victim":{"guid":"p2","name":"Player2"},"weapon":"M1 Garand","hitloc":"head","mod":"MOD_RIFLE"}'
```

### Multi-Event (Newline-Separated)
```json
{"type":"game_start","timestamp":"2026-02-02T17:30:00Z","server_id":"test"}
{"type":"player_spawn","timestamp":"2026-02-02T17:30:01Z","server_id":"test","player":{"guid":"p1","name":"Player1"}}
{"type":"player_kill","timestamp":"2026-02-02T17:30:05Z","server_id":"test","attacker":{"guid":"p1"},"victim":{"guid":"p2"},"weapon":"Thompson"}
```

---

## Additional Resources

- **Event Type Definitions:** `internal/models/event_types_generated.go`
- **Ingestion Handler:** `internal/handlers/ingest.go`
- **Event Processor:** `internal/worker/event_processor.go`
- **Bruno Collection:** `bruno/Ingestion/`
- **API Documentation:** http://localhost:8084/docs

---

**Last Updated:** 2026-02-02  
**Script Version:** 1.0.0  
**Total Event Types:** 106
