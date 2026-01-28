#!/bin/bash
# Install swaggo if not present
if ! command -v swag &> /dev/null; then
    echo "Installing swaggo/swag..."
    go install github.com/swaggo/swag/cmd/swag@latest
fi

# Add GOPATH/bin to PATH
export PATH=$PATH:$(go env GOPATH)/bin

# Generate docs
echo "Generating OpenAPI spec..."
swag init -g cmd/api/main.go --output web/static

echo "Done. Spec generated at web/static/swagger.yaml"
