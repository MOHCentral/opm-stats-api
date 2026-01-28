.PHONY: build docs run test clean

GO_BIN ?= api
GOPATH ?= $(shell go env GOPATH)
SWAG_BIN := $(GOPATH)/bin/swag

all: build docs

build:
	@echo "Building API..."
	go build -o $(GO_BIN) ./cmd/api

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

test:
	go test ./...

clean:
	rm -f $(GO_BIN)
	rm -rf web/static/img
	rm -f web/static/swagger.json web/static/swagger.yaml web/static/docs.go
