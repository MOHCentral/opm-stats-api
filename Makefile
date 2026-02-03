.PHONY: build docs run test clean generate-types bruno bruno-events bruno-watch

GO_BIN ?= api
GOPATH ?= $(shell go env GOPATH)
SWAG_BIN := $(GOPATH)/bin/swag

all: build docs bruno

build:
	@echo "Building API..."
	go build -o $(GO_BIN) ./cmd/api

generate-types:
	@echo "Generating type-safe event constants from OpenAPI spec..."
	@python3 ../tools/generate_types.py
	@echo "Type generation complete. Run 'go build ./...' to verify."

docs:
	@echo "Checking for swag..."
	@if [ ! -x $(SWAG_BIN) ]; then \
		echo "Installing swaggo/swag..."; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
	fi
	@echo "Generating OpenAPI spec..."
	@$(SWAG_BIN) init -g cmd/api/main.go --output web/static
	@echo "Done. Spec generated at web/static/swagger.yaml"

run: build
	./$(GO_BIN)

diagrams:
	@echo "Generating architecture diagrams from Mermaid..."
	@mkdir -p web/static/img
	@npx -y @mermaid-js/mermaid-cli -i docs/api_visual_guide.md -o web/static/img/guide.png -p puppeteer-config.json
	@echo "Diagrams generated in web/static/img/"

charts:
	@echo "Generating data charts from ClickHouse..."
	@go run tools/chartgen/main.go
	@echo "Charts generated in web/static/img/"

bruno-test:
	@./tools/test_bruno.sh Local

bruno-ingest-all:
	@./tools/ingest_all_events.sh Local

test:
	go test ./...
	@echo ""
	@echo "Running Bruno API tests..."
	@./tools/test_bruno.sh Local || echo "⚠️  Bruno tests failed"

bruno:
	@echo "Generating Bruno API collection from swagger.yaml..."
	@python3 tools/generate_bruno.py
	@echo "✓ Bruno collection ready at bruno/"

bruno-events:
	@echo "🎯 Generating individual event .bru files..."
	@python3 tools/generate_bruno_events.py
	@echo "✅ Individual event files generated at bruno/Ingestion/Events/"

bruno-watch:
	@echo "Watching swagger.yaml for changes..."
	@echo "Press Ctrl+C to stop"
	@while true; do \
		inotifywait -e modify web/static/swagger.yaml 2>/dev/null && \
		echo "\n🔄 Detected swagger.yaml change, regenerating Bruno collection..." && \
		python3 tools/generate_bruno.py; \
	done || echo "Note: Install inotify-tools for watch mode"

clean:
	rm -f $(GO_BIN)
	rm -rf web/static/img
	rm -f web/static/swagger.json web/static/swagger.yaml web/static/docs.go
	rm -rf bruno/*/
	@echo "Note: bruno/environments/ and bruno/*.json preserved"
