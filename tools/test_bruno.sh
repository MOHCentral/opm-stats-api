#!/usr/bin/env bash
# Bruno API Test Runner for CI/CD
# Runs all API tests against specified environment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BRUNO_DIR="$SCRIPT_DIR/../bruno"

# Default environment
ENV="${1:-Local}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=================================${NC}"
echo -e "${GREEN}Bruno API Test Suite${NC}"
echo -e "${GREEN}=================================${NC}"
echo ""

# Check if Bruno CLI is installed
if ! command -v bru &> /dev/null; then
    echo -e "${RED}✗ Bruno CLI not found${NC}"
    echo -e "${YELLOW}Install with: npm install -g @usebruno/cli${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Bruno CLI found${NC}"

# Check if collection exists
if [ ! -d "$BRUNO_DIR" ]; then
    echo -e "${RED}✗ Bruno collection not found at $BRUNO_DIR${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Collection found${NC}"
echo ""

# Check if API is running (for Local/Development)
if [[ "$ENV" == "Local" ]] || [[ "$ENV" == "Development" ]]; then
    API_URL="http://localhost:8084/api/v1/stats/global"
    if [[ "$ENV" == "Development" ]]; then
        API_URL="http://localhost:8080/api/v1/stats/global"
    fi
    
    echo -e "${YELLOW}Checking API availability at $API_URL...${NC}"
    if curl -sf "$API_URL" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ API is running${NC}"
    else
        echo -e "${RED}✗ API not reachable${NC}"
        echo -e "${YELLOW}Start API with: make run${NC}"
        exit 1
    fi
    echo ""
fi

# Run tests
echo -e "${GREEN}Running tests against ${YELLOW}$ENV${GREEN} environment...${NC}"
echo ""

cd "$BRUNO_DIR"

# Run all tests recursively
if bru run --env "$ENV" --recursive --reporter json --output results.json; then
    echo ""
    echo -e "${GREEN}=================================${NC}"
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo -e "${GREEN}=================================${NC}"
    
    # Parse results if available
    if [ -f "results.json" ]; then
        total=$(jq -r '.summary.total // 0' results.json 2>/dev/null || echo "0")
        passed=$(jq -r '.summary.passed // 0' results.json 2>/dev/null || echo "0")
        failed=$(jq -r '.summary.failed // 0' results.json 2>/dev/null || echo "0")
        
        echo ""
        echo "📊 Results:"
        echo "   Total:  $total"
        echo "   Passed: $passed"
        echo "   Failed: $failed"
        
        rm -f results.json
    fi
    
    exit 0
else
    echo ""
    echo -e "${RED}=================================${NC}"
    echo -e "${RED}✗ Some tests failed${NC}"
    echo -e "${RED}=================================${NC}"
    
    # Show failed tests if available
    if [ -f "results.json" ]; then
        echo ""
        echo -e "${YELLOW}Failed tests:${NC}"
        jq -r '.results[] | select(.status == "failed") | "  • \(.name): \(.error)"' results.json 2>/dev/null || true
        
        rm -f results.json
    fi
    
    exit 1
fi
