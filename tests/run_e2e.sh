#!/bin/bash
# ==============================================================================
# OpenMOHAA Stats System - E2E Test Runner
# ==============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo -e "${CYAN}"
echo "=============================================="
echo " OpenMOHAA Stats E2E Test Suite"
echo "=============================================="
echo -e "${NC}"

# Check prerequisites
echo -e "${YELLOW}Checking prerequisites...${NC}"

# Check API
if curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health | grep -q "200"; then
    echo -e "${GREEN}[✓]${NC} Stats API is healthy"
else
    echo -e "${RED}[✗]${NC} Stats API is not running!"
    echo "    Run ./start.sh first"
    exit 1
fi

# Check SMF
if curl -s -o /dev/null -w "%{http_code}" http://localhost:8888/ | grep -qE "200|302"; then
    echo -e "${GREEN}[✓]${NC} SMF Forum is accessible"
else
    echo -e "${RED}[✗]${NC} SMF Forum is not accessible!"
    echo "    Check Apache: sudo systemctl status apache2"
    exit 1
fi

# Run Go tests
echo ""
echo -e "${CYAN}Running E2E Tests...${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

cd "$PROJECT_DIR"

# Run specific test groups
echo ""
echo -e "${BLUE}=== Infrastructure Tests ===${NC}"
go test -v ./tests/... -run "TestAPIHealth|TestSMFHealth" 2>&1 | grep -E "^(---|===|PASS|FAIL|✓|⚠)" || true

echo ""
echo -e "${BLUE}=== Combat Event Tests ===${NC}"
go test -v ./tests/... -run "TestEvent_Player|TestEvent_Weapon|TestEvent_Headshot|TestEvent_Grenade" 2>&1 | grep -E "^(---|===|PASS|FAIL|✓|⚠)" || true

echo ""
echo -e "${BLUE}=== Movement Event Tests ===${NC}"
go test -v ./tests/... -run "TestEvent_PlayerJump|TestEvent_PlayerLand|TestEvent_PlayerCrouch|TestEvent_PlayerProne|TestEvent_PlayerDistance" 2>&1 | grep -E "^(---|===|PASS|FAIL|✓|⚠)" || true

echo ""
echo -e "${BLUE}=== Interaction Event Tests ===${NC}"
go test -v ./tests/... -run "TestEvent_Ladder|TestEvent_Item|TestEvent_PlayerUse" 2>&1 | grep -E "^(---|===|PASS|FAIL|✓|⚠)" || true

echo ""
echo -e "${BLUE}=== Session Event Tests ===${NC}"
go test -v ./tests/... -run "TestEvent_Client|TestEvent_Team|TestEvent_PlayerSay" 2>&1 | grep -E "^(---|===|PASS|FAIL|✓|⚠)" || true

echo ""
echo -e "${BLUE}=== Game Flow Event Tests ===${NC}"
go test -v ./tests/... -run "TestEvent_Match|TestEvent_Round|TestEvent_Heartbeat" 2>&1 | grep -E "^(---|===|PASS|FAIL|✓|⚠)" || true

echo ""
echo -e "${BLUE}=== API Endpoint Tests ===${NC}"
go test -v ./tests/... -run "TestAPI_" 2>&1 | grep -E "^(---|===|PASS|FAIL|✓|⚠)" || true

echo ""
echo -e "${BLUE}=== SMF Frontend Tests ===${NC}"
go test -v ./tests/... -run "TestSMF_" 2>&1 | grep -E "^(---|===|PASS|FAIL|✓|⚠)" || true

echo ""
echo -e "${BLUE}=== Full Simulation ===${NC}"
go test -v ./tests/... -run "TestFullGameSimulation" 2>&1 | grep -E "^(---|===|PASS|FAIL|✓|⚠|Simulation)" || true

echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}E2E Test Suite Complete${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# Summary
echo ""
echo -e "${YELLOW}To run all tests with full output:${NC}"
echo "  go test -v ./tests/..."
echo ""
echo -e "${YELLOW}To run a specific test:${NC}"
echo "  go test -v ./tests/... -run TestEvent_PlayerKill"
