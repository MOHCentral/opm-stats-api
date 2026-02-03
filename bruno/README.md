# OpenMOHAA Stats API - Bruno Collection

Complete API testing collection for the OpenMOHAA Stats API with 65+ requests organized by category.

## 📁 Collection Structure

```
bruno/
├── bruno.json              # Collection metadata
├── environments/           # Environment configurations
│   ├── Local.bru          # localhost:8084
│   ├── Development.bru    # localhost:8080  
│   └── Production.bru     # api.moh-central.net
├── Achievements/          # 4 requests
├── Advanced Stats/        # 6 requests (War Room, Drilldown, Combos)
├── AI/                    # 2 requests (Predictions)
├── Auth/                  # 10 requests (Device auth, IP management)
├── Ingestion/             # 3 requests (Events, Match results, Event mega-test)
│   └── Events/            # 106 individual event requests
├── Leaderboards/          # 5 requests
├── Match/                 # 1 request
├── Player/                # 3 requests
├── Server/                # 25 requests (Registration, Live, History)
├── Stats/                 # 1 request (Global stats)
├── System/                # 2 requests (Install, Reset)
├── Teams/                 # 1 request
└── Tournaments/           # 3 requests
```

## 🚀 Quick Start

### 1. Install Bruno

**Desktop App:**
```bash
# Download from https://www.usebruno.com/downloads
```

**CLI:**
```bash
npm install -g @usebruno/cli
```

### 2. Import Collection

**In Bruno Desktop:**
1. Open Bruno
2. Click **Open Collection**
3. Navigate to `/path/to/opm-stats-api/bruno`
4. Select the folder

**Via CLI:**
```bash
cd opm-stats-api/bruno
bru run --env Local
```

### 3. Select Environment

Choose one:
- **Local** - Port 8084 (external access)
- **Development** - Port 8080 (internal container)
- **Production** - api.moh-central.net

### 4. Configure Variables

Update `environments/Local.bru`:

```
vars {
  base_url: http://localhost:8084/api/v1
  server_token: 4d170d00-8b08-4619-93d0-cec2ad7883e2
  bearer_token: YOUR_JWT_TOKEN
  guid: YOUR_PLAYER_GUID
  smf_id: 1
  server_id: YOUR_SERVER_ID
  match_id: YOUR_MATCH_ID
}
```

## 🔐 Authentication

### Server Token (X-Server-Token)

Required for:
- `/ingest/*` - Event ingestion
- `/servers/register` - Server registration
- `/system/*` - System operations

**Header:** `X-Server-Token: {{var:server_token}}`

### Bearer Auth (Authorization)

Required for:
- Player-specific endpoints
- Administrative operations

**Header:** `Authorization: Bearer {{var:bearer_token}}`

## 📝 Usage Examples

### Run All Requests
```bash
bru run --env Local --recursive
```

### Run Specific Folder
```bash
bru run --env Local Server/
```

### Run Single Request
```bash
bru run --env Local "Server/POST Register Server.bru"
```

### Test Authenticated Endpoints
```bash
# Set bearer token first
export BEARER_TOKEN="your-jwt-token-here"

# Run auth requests
bru run --env Development Auth/
```

### Test Event Ingestion
```bash
bru run --env Local "Ingestion/POST Ingest Events.bru"
```

## 🔄 Syncing with Swagger

The collection is auto-generated from `web/static/swagger.yaml`.

### Regenerate Collection

After updating API endpoints:

```bash
# 1. Regenerate swagger from Go annotations
make docs

# 2. Regenerate Bruno collection
make bruno

# Or both at once
make docs bruno
```

### Manual Sync
```bash
python3 tools/generate_bruno.py
```

### Watch Mode
```bash
make bruno-watch
```

## 🎯 Event Ingestion Testing

### Ingest All 106 Event Types

**Quick method (bash script):**
```bash
./tools/ingest_all_events.sh Local
# or
make bruno-ingest-all
# or
npm run bruno:ingest-all
```

Posts all 105 event types individually with sample payloads. Perfect for:
- Comprehensive API testing
- Event pipeline validation
- Database schema verification

**Bruno request method:**
- Run `Ingestion/POST ingest-all-event-types-mega.bru`
- Sends ~30 representative events in one request
- Faster, good for smoke testing

See [EVENT_INGESTION_GUIDE.md](EVENT_INGESTION_GUIDE.md) for complete documentation.

## 🧪 Testing Workflows

### Complete Server Registration Flow
1. `Server/POST Register Server.bru`
2. Copy `server_id` from response
3. Update `environments/Local.bru` with server_id
4. `Ingestion/POST Ingest Events.bru`
5. `Server/GET Server Details.bru`

### Player Stats Workflow
1. `Auth/POST Verify Player.bru` - Get player GUID
2. Update `guid` in environment
3. `Player/GET Player Stats.bru`
4. `Advanced Stats/GET Player War Room.bru`
5. `Advanced Stats/GET Player Drilldown.bru`

### Leaderboard Analysis
1. `Leaderboards/GET Global Leaderboard.bru`
2. `Leaderboards/GET Combo Leaderboard.bru`
3. `Leaderboards/GET Peak Performance Leaderboard.bru`

## 🎯 Request Features

Each `.bru` file includes:

✅ **Pre-configured headers** (auth tokens from vars)  
✅ **Query parameters** with defaults  
✅ **Path parameter substitution** (e.g., `{{var:guid}}`)  
✅ **Request body templates** for POST/PUT/PATCH  
✅ **Response documentation** with status codes  
✅ **Automated tests** (200-level status check)  

## 🛠️ Advanced Features

### Environment Variables

Use `{{var:name}}` to reference:
- `{{var:base_url}}` - API base URL
- `{{var:server_token}}` - Server authentication
- `{{var:guid}}` - Player GUID
- `{{var:smf_id}}` - SMF member ID

### Scripting

Add custom scripts to `.bru` files:

```javascript
tests {
  test("Has valid GUID", function() {
    const data = res.body;
    expect(data.guid).to.match(/^[a-f0-9]+$/);
  });
  
  test("Response time < 500ms", function() {
    expect(res.responseTime).to.be.lessThan(500);
  });
}
```

### Chaining Requests

Extract data from one request to another:

```javascript
# In response script
bru.setVar("extracted_guid", res.body.guid);

# In next request URL
GET {{base_url}}/stats/player/{{var:extracted_guid}}
```

## 📊 Collection Metrics

- **Total Requests:** 65
- **GET:** 52
- **POST:** 11
- **DELETE:** 2
- **Categories:** 13
- **Authenticated:** 15
- **Public:** 50

## 🔍 Key Endpoints

### High Traffic
- `POST /ingest/events` - Telemetry ingestion (10k+ req/min)
- `GET /servers/{id}/live` - Real-time server status
- `GET /stats/leaderboard` - Global rankings

### Critical
- `POST /servers/register` - Server registration
- `POST /system/install` - DB schema installation
- `POST /auth/verify` - Player authentication

### Analytics
- `GET /stats/player/{guid}/war-room` - Comprehensive player analytics
- `GET /stats/player/{guid}/drilldown` - Multi-dimensional insights
- `GET /stats/leaderboard/combos` - Composite metrics

## 🐛 Troubleshooting

### 401 Unauthorized
- Verify `server_token` in environment
- Check `X-Server-Token` header is set

### 404 Not Found
- Ensure variables are set (guid, server_id, etc.)
- Check base_url includes `/api/v1`

### Connection Refused
- Verify API is running: `docker ps | grep opm-stats-api`
- Check port mapping (8084 external, 8080 internal)

### Empty Responses
- Some endpoints require data to exist first
- Run seed data: `cd opm-stats-game-scripts && /path/to/openmohaa +exec server.cfg`

## 📚 Resources

- [Bruno Documentation](https://docs.usebruno.com/)
- [OpenAPI Spec](../web/static/swagger.yaml)
- [API Visual Guide](../docs/api_visual_guide.md)
- [Scalar Docs](http://localhost:8084/docs) (when API running)

## 🤝 Contributing

### Adding New Endpoints

1. Add Swagger annotations to Go handler
2. Run `make docs` to regenerate swagger.yaml
3. Run `make bruno` to sync collection
4. Test new request in Bruno
5. Commit both swagger.yaml and .bru files

### Modifying Environments

Edit `environments/*.bru` files directly. These are not auto-generated.

---

**Last Updated:** 2026-02-02  
**API Version:** 1.0.0  
**Collection Version:** 1.0.0
