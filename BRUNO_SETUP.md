# Bruno API Collection Setup - Complete

## ✅ What Was Created

### 1. Collection Structure
```
bruno/
├── bruno.json                    # Collection metadata
├── .gitignore                    # Ignore secrets
├── README.md                     # Complete documentation
├── CI_EXAMPLES.md                # CI/CD integration examples
├── secrets.bru.example           # Template for secrets
├── environments/
│   ├── Local.bru                # localhost:8084
│   ├── Development.bru          # localhost:8080
│   └── Production.bru           # api.moh-central.net
├── Achievements/                 # 4 requests
├── Advanced Stats/               # 6 requests (War Room, Drilldown)
├── AI/                          # 2 requests (Predictions)
├── Auth/                        # 10 requests (Device auth, IP mgmt)
├── Ingestion/                   # 2 requests (Events, Match)
├── Leaderboards/                # 5 requests
├── Match/                       # 1 request
├── Player/                      # 3 requests
├── Server/                      # 25 requests (Registration, Live)
├── Stats/                       # 1 request (Global)
├── System/                      # 2 requests (Install, Reset)
├── Teams/                       # 1 request
└── Tournaments/                 # 3 requests
```

**Total:** 65 API requests organized into 13 categories

### 2. Tools & Scripts

#### Python Generator
- **File:** `tools/generate_bruno.py`
- **Purpose:** Parse swagger.yaml and generate .bru files
- **Usage:** `python3 tools/generate_bruno.py`

#### Test Runner
- **File:** `tools/test_bruno.sh`
- **Purpose:** Run Bruno tests for CI/CD
- **Usage:** `./tools/test_bruno.sh [Local|Development|Production]`

#### Event Ingestion Script
- **File:** `tools/ingest_all_events.sh`
- **Purpose:** Post all 105 event types individually to test the API
- **Usage:** `./tools/ingest_all_events.sh [Local|Development|Production]`
- **What it does:**
  - Posts each of the 105 event types with sample payloads
  - Real-time progress with color-coded output
  - Success/failure tracking
  - Perfect for comprehensive API testing

### 3. Makefile Targets

```bash
make bruno              # Generate Bruno collection from swagger.yaml
make bruno-watch        # Watch swagger.yaml and auto-regenerate
make bruno-test         # Run all Bruno tests
make bruno-ingest-all   # Post all 105 event types to API
make test               # Run Go tests + Bruno tests
make docs bruno         # Regenerate swagger + Bruno
```

### 4. NPM Scripts

```bash
npm run bruno:sync          # Sync collection from swagger
npm run bruno:watch         # Watch for swagger changes
npm run bruno:test          # Test against Local env
npm run bruno:test:dev      # Test against Development
npm run bruno:ingest-all    # Post all 105 event types
npm run bruno:test:prod     # Test against Production
npm run docs:bruno          # Generate docs + sync Bruno
```

### 5. Environment Variables

Each environment file (`.bru`) includes:

```
vars {
  base_url: <API_URL>
  server_token: <SERVER_AUTH_TOKEN>
  bearer_token: <JWT_TOKEN>
  guid: <PLAYER_GUID>
  smf_id: <SMF_MEMBER_ID>
  server_id: <SERVER_ID>
  match_id: <MATCH_ID>
}
```

**Usage in requests:** `{{var:server_token}}`

### 6. Request Features

Every `.bru` file includes:

✅ **HTTP method & URL** with path param substitution  
✅ **Query parameters** (with defaults from swagger)  
✅ **Authentication headers** (X-Server-Token, Bearer)  
✅ **Request body templates** (for POST/PUT/PATCH)  
✅ **Response documentation** (status codes + descriptions)  
✅ **Automated tests** (basic 2xx status check)  

---

## 🚀 Quick Start

### Option 1: Bruno Desktop App

1. **Download:** https://www.usebruno.com/downloads
2. **Open Bruno** → Click "Open Collection"
3. **Navigate** to `opm-stats-api/bruno/`
4. **Select environment:** Local (recommended for dev)
5. **Update variables** if needed (server_token, etc.)
6. **Start testing!** 🎯

### Option 2: Bruno CLI

```bash
# Already installed globally
bru --version  # Should show v3.0.3

# Run all tests
cd opm-stats-api/bruno
bru run --env Local --recursive

# Run specific folder
bru run --env Local Server/

# Run single request
bru run --env Local "Server/POST register-server.bru"
```

---

## 🔄 Workflow Integration

### After Updating API Endpoints

```bash
# 1. Add swagger annotations to Go handler
# 2. Regenerate swagger from Go code
make docs

# 3. Sync Bruno collection from swagger
make bruno

# 4. Test new endpoints
make bruno-test

# 5. Commit both swagger.yaml AND .bru files
git add web/static/swagger.yaml bruno/
git commit -m "Add new endpoint + Bruno tests"
```

### CI/CD Integration

See `bruno/CI_EXAMPLES.md` for:
- GitHub Actions workflow
- GitLab CI pipeline
- Jenkins pipeline
- Docker Compose test setup
- Pre-commit hooks

---

## 📊 Collection Statistics

| Category | Requests | Key Endpoints |
|----------|----------|---------------|
| Server | 25 | Register, Live Status, History |
| Auth | 10 | Device Flow, IP Management, Verify |
| Advanced Stats | 6 | War Room, Drilldown, Combos |
| Leaderboards | 5 | Global, Peak, Contextual |
| Achievements | 4 | Progress, Stats, Match/Tournament |
| Player | 3 | Stats, Maps, Gametypes |
| Tournaments | 3 | List, Details, Stats |
| AI | 2 | Match Predictions |
| Ingestion | 2 | Events, Match Results |
| System | 2 | Install Schema, Reset DB |
| Match | 1 | Match Stats |
| Teams | 1 | Faction Performance |
| Stats | 1 | Global Stats |

**Total: 65 requests**

---

## 🎯 Example Workflows

### 1. Server Registration + Event Ingestion

```bash
# Step 1: Register server
bru run --env Local "Server/POST register-server.bru"
# Copy server_id from response

# Step 2: Update environment
# Edit environments/Local.bru and set server_id

# Step 3: Send events
bru run --env Local "Ingestion/POST ingest-game-events.bru"

# Step 4: Verify
bru run --env Local "Server/GET get-server-details.bru"
```
Comprehensive Event Pipeline Test

```bash
# Post all 105 event types individually
./tools/ingest_all_events.sh Local

# Output shows real-time progress:
# [1] Posting game_init... ✓
# [2] Posting game_start... ✓
# ...
# [106] Posting player_movement... ✓
# 
# Total events: 106
# Success: 106
# Failed: 0
```

### 3. Player Stats Analysis

```bash
# Get player data
bru run --env Local "Player/GET get-player-stats.bru"

# Deep dive
bru run --env Local "Advanced Stats/GET get-war-room-data.bru"
bru run --env Local "Advanced Stats/GET get-player-drilldown.bru"
bru run --env Local "Advanced Stats/GET get-player-combo-metrics.bru"
```

### 4
### 3. Leaderboard Analysis

```bash
# Global rankings
bru run --env Local "Leaderboards/GET get-global-leaderboard.bru"

# Combo metrics
bru run --env Local "Leaderboards/GET get-combo-leaderboard.bru"

# Peak performance
bru run --env Local "Leaderboards/GET get-peak-performance-leaderboard.bru"
```

---

## 🔐 Security

### Secrets Management

1. **Copy template:**
   ```bash
   cp bruno/secrets.bru.example bruno/secrets.bru
   ```

2. **Fill in values** (NEVER commit this file!)

3. **Reference in environments:**
   ```
   bearer_token: {{var:prod_bearer_token}}
   ```

### Git Ignore

The following are automatically ignored:
- `secrets.bru`
- `*.secret.bru`
- `.env.bruno`

---

## 🐛 Troubleshooting

### API Not Running
```bash
docker ps | grep opm-stats-api
# If not running:
cd opm-stats-api && docker-compose up -d
```

### 401 Unauthorized
- Check `server_token` in environment file
- Verify `X-Server-Token` header is set in request

### Connection Refused
- Verify port (8084 for Local, 8080 for Development)
- Check base_url in environment

### Bruno CLI Not Found
```bash
npm install -g @usebruno/cli
```

---

## 📚 Resources

- **Bruno Docs:** https://docs.usebruno.com/
- **Collection README:** [bruno/README.md](bruno/README.md)
- **CI Examples:** [bruno/CI_EXAMPLES.md](bruno/CI_EXAMPLES.md)
- **OpenAPI Spec:** [web/static/swagger.yaml](web/static/swagger.yaml)
- **Scalar Docs:** http://localhost:8084/docs (when running)

---

## 🎉 Summary

You now have a complete Bruno API testing suite with:

✅ **65 pre-configured requests** organized by category  
✅ **3 environments** (Local, Dev, Prod)  
✅ **Auto-sync from Swagger** (make bruno)  
✅ **CI/CD integration** examples  
✅ **NPM scripts** for easy workflows  
✅ **Comprehensive documentation**  
✅ **Security best practices** (secrets management)  

**Next Steps:**
1. Open Bruno Desktop or use CLI
2. Select **Local** environment
3. Update `server_token` if needed
4. Run a test request!
5. See [bruno/README.md](bruno/README.md) for advanced usage

---

**Generated:** 2026-02-02  
**API Version:** 1.0.0  
**Bruno Collection Version:** 1.0.0  
**Total Requests:** 65  
**Environments:** 3
