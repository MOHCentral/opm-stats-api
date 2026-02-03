# OpenMOHAA Stats API

High-performance Go backend for accumulating and processing game events.

## 🚀 Production Deployment

We use Docker Compose for easy production deployment.

### 1. Setup Environment
Copy the example environment file and configure your secrets:

```bash
cp .env.example .env
nano .env
```

**Critical Settings to Change:**
- `JWT_SECRET`: Generate a long random string.
- `POSTGRES_PASSWORD`: Set a secure database password.

### 2. Run with Docker Compose

```bash
docker compose up -d
```

This will include:
- **API Server** (Port 8080)
- **PostgreSQL** (Port 5432) - User Data
- **ClickHouse** (Port 8123/9000) - Event Analytics
- **Redis** (Port 6379) - Caching

### 3. Verify Health

Check if the API is running:
```bash
curl http://localhost:8080/health
```

## 🛠️ Development

For local development without Docker containers for the API itself (running DBs in Docker):

```bash
# Start DBs only
docker compose up -d postgres clickhouse redis

# Run API locally
go run ./cmd/api
```

## 📁 Structure

- `cmd/api`: Entry point.
- `internal/`: Application logic.
- `migrations/`: SQL migration files.
- `tools/`: Utility scripts.
- `bruno/`: API testing collection (65+ requests + 105 event tests).

## 🧪 API Testing with Bruno

Complete API testing suite with 65+ organized requests + 106 individual event test files.

### Quick Start

```bash
# Generate/update Bruno collection from Swagger
make bruno

# Generate individual event test files (105 events)
make bruno-events

# Run all API tests
make bruno-test

# Post all 105 event types for comprehensive testing
make bruno-ingest-all

# Or use Bruno CLI directly
cd bruno && bru run --env Local --recursive
```

### Using Bruno Desktop

1. Download from [usebruno.com](https://www.usebruno.com/downloads)
2. Open Bruno → **Open Collection**
3. Select `opm-stats-api/bruno/`
4. Choose environment: **Local**, **Development**, or **Production**
5. Navigate to `Ingestion/Events/` to test individual events (e.g., "Player Kill")
6. Start testing! 🚀

**See [bruno/README.md](bruno/README.md) for complete documentation.**

## 📚 API Documentation

- **Scalar Docs (Interactive)**: http://localhost:8084/docs
- **OpenAPI Spec**: [web/static/swagger.yaml](web/static/swagger.yaml)
- **Bruno Collection**: [bruno/](bruno/) - 65+ tested requests
- **Architecture Guide**: [docs/api_visual_guide.md](docs/api_visual_guide.md)
