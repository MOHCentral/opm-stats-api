#!/bin/bash
# ==============================================================================
# OpenMOHAA Stats System - Stop All Services
# ==============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${YELLOW}Stopping OpenMOHAA Stats System...${NC}"
echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# Stop Docker containers (Stats API, ClickHouse, PostgreSQL, Redis)
echo ""
echo -e "${YELLOW}Stopping Docker Services...${NC}"
cd "$SCRIPT_DIR"
if [ -f "docker-compose.yml" ]; then
    docker compose down
    echo -e "${GREEN}[✓]${NC} Docker services stopped"
fi

# Note: We don't stop Apache/MariaDB as they may serve other sites
echo ""
echo -e "${YELLOW}Note:${NC} Apache2 and MariaDB are not stopped (native system services)"
echo -e "      To stop them manually: sudo systemctl stop apache2 mariadb"

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}Docker services stopped.${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${YELLOW}Run ./start.sh to restart all services${NC}"
