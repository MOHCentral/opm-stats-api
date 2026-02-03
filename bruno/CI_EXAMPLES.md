# Bruno CI/CD Integration Examples

## GitHub Actions

```yaml
name: API Tests

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
      
      clickhouse:
        image: clickhouse/clickhouse-server:latest
        
      redis:
        image: redis:7-alpine
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.22'
    
    - name: Install Bruno CLI
      run: npm install -g @usebruno/cli
    
    - name: Build API
      run: make build
    
    - name: Start API
      run: ./api &
      env:
        DATABASE_URL: postgres://postgres:postgres@localhost/mohaa_stats
        CLICKHOUSE_URL: http://localhost:8123
        REDIS_URL: redis://localhost:6379
    
    - name: Wait for API
      run: sleep 5
    
    - name: Run Bruno Tests
      run: |
        cd bruno
        bru run --env Development --recursive
      env:
        SERVER_TOKEN: ${{ secrets.TEST_SERVER_TOKEN }}
        BEARER_TOKEN: ${{ secrets.TEST_BEARER_TOKEN }}
    
    - name: Upload Test Results
      if: always()
      uses: actions/upload-artifact@v3
      with:
        name: bruno-results
        path: bruno/results.json
```

## GitLab CI

```yaml
test:api:
  stage: test
  image: golang:1.22
  
  services:
    - postgres:15
    - clickhouse/clickhouse-server:latest
    - redis:7-alpine
  
  before_script:
    - apt-get update && apt-get install -y nodejs npm
    - npm install -g @usebruno/cli
    - make build
  
  script:
    - ./api &
    - sleep 5
    - cd bruno && bru run --env Development --recursive
  
  variables:
    DATABASE_URL: "postgres://postgres:postgres@postgres/mohaa_stats"
    CLICKHOUSE_URL: "http://clickhouse:8123"
    REDIS_URL: "redis://redis:6379"
    SERVER_TOKEN: $TEST_SERVER_TOKEN
    BEARER_TOKEN: $TEST_BEARER_TOKEN
```

## Jenkins Pipeline

```groovy
pipeline {
    agent any
    
    environment {
        SERVER_TOKEN = credentials('test-server-token')
        BEARER_TOKEN = credentials('test-bearer-token')
    }
    
    stages {
        stage('Install Bruno') {
            steps {
                sh 'npm install -g @usebruno/cli'
            }
        }
        
        stage('Build API') {
            steps {
                sh 'make build'
            }
        }
        
        stage('Start Services') {
            steps {
                sh 'docker-compose up -d'
                sh 'sleep 10'
            }
        }
        
        stage('Run Tests') {
            steps {
                dir('bruno') {
                    sh 'bru run --env Development --recursive'
                }
            }
        }
    }
    
    post {
        always {
            sh 'docker-compose down'
            archiveArtifacts artifacts: 'bruno/results.json', allowEmptyArchive: true
        }
    }
}
```

## Docker Compose Test Environment

```yaml
# docker-compose.test.yml
version: '3.8'

services:
  api:
    build: .
    environment:
      - DATABASE_URL=postgres://postgres:postgres@postgres/mohaa_stats
      - CLICKHOUSE_URL=http://clickhouse:8123
      - REDIS_URL=redis://redis:6379
    depends_on:
      - postgres
      - clickhouse
      - redis
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v1/stats/global"]
      interval: 5s
      timeout: 3s
      retries: 10
  
  bruno:
    image: node:18-alpine
    working_dir: /workspace
    volumes:
      - ./bruno:/workspace/bruno
    depends_on:
      api:
        condition: service_healthy
    command: >
      sh -c "
        npm install -g @usebruno/cli &&
        cd bruno &&
        bru run --env Development --recursive
      "
    environment:
      - SERVER_TOKEN=${TEST_SERVER_TOKEN}
      - BEARER_TOKEN=${TEST_BEARER_TOKEN}
  
  postgres:
    image: postgres:15
    environment:
      - POSTGRES_PASSWORD=postgres
  
  clickhouse:
    image: clickhouse/clickhouse-server:latest
  
  redis:
    image: redis:7-alpine
```

Run with:
```bash
docker-compose -f docker-compose.test.yml up --abort-on-container-exit
```

## Pre-commit Hook

```bash
#!/bin/sh
# .git/hooks/pre-commit

echo "Running API tests..."

# Check if swagger changed
if git diff --cached --name-only | grep -q "web/static/swagger.yaml"; then
    echo "Swagger changed, regenerating Bruno collection..."
    make bruno
    
    # Stage regenerated files
    git add bruno/
fi

# Run quick smoke tests
cd bruno
if ! bru run --env Local "Server/GET Health.bru" 2>/dev/null; then
    echo "⚠️  Warning: API health check failed"
    echo "Continue anyway? [y/N]"
    read -r response
    if [[ ! "$response" =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

echo "✓ Pre-commit checks passed"
```

## Makefile Integration

```makefile
.PHONY: test-api test-api-full test-api-ci

test-api:
	@./tools/test_bruno.sh Local

test-api-full:
	@echo "Running comprehensive API tests..."
	@./tools/test_bruno.sh Local
	@./tools/test_bruno.sh Development

test-api-ci:
	@echo "Running CI API tests..."
	@docker-compose -f docker-compose.test.yml up --abort-on-container-exit
	@docker-compose -f docker-compose.test.yml down
```

## Scheduled Health Checks (Cron)

```bash
#!/bin/bash
# /etc/cron.d/api-health-check

# Run health checks every 15 minutes
*/15 * * * * /usr/local/bin/bru run --env Production "Server/GET Health.bru" || \
    echo "API health check failed at $(date)" | \
    mail -s "API Health Alert" admin@moh-central.net
```

## Load Testing with Bruno

```bash
#!/bin/bash
# tools/load_test.sh

# Run 100 concurrent requests
for i in {1..100}; do
    bru run --env Local "Ingestion/POST Ingest Events.bru" &
done

wait
echo "Load test complete"
```
