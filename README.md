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
