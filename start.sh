#!/bin/bash
# ==============================================================================
# OpenMOHAA Stats System - Start All Services
# ==============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

clear
echo -e "${CYAN}"
cat << 'LOGO'
   ___                   __  __       _   _    _    _    
  / _ \ _ __   ___ _ __ |  \/  | ___ | | | |  / \  / \   
 | | | | '_ \ / _ \ '_ \| |\/| |/ _ \| |_| | / _ \/ _ \  
 | |_| | |_) |  __/ | | | |  | | (_) |  _  |/ ___ \ ___ \ 
  \___/| .__/ \___|_| |_|_|  |_|\___/|_| |_/_/   \_\_/  \_\
       |_|                                                  
                    STATS SYSTEM v2.0
LOGO
echo -e "${NC}"

# ==============================================================================
# Docker Services (ClickHouse, PostgreSQL, Redis, Stats API)
# ==============================================================================
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}Starting Docker Services (API, Databases)...${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

cd "$SCRIPT_DIR"
docker compose down 2>/dev/null || true
docker compose up -d --build

echo -e "${GREEN}[✓]${NC} Stats API (port 8080)"
echo -e "${GREEN}[✓]${NC} PostgreSQL (port 5432)"
echo -e "${GREEN}[✓]${NC} ClickHouse (port 8123)"
echo -e "${GREEN}[✓]${NC} Redis (port 6379)"

# ==============================================================================
# Native SMF Forum (Apache + MariaDB)
# ==============================================================================
echo ""
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}Checking Native SMF Services (Apache + MariaDB)...${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# Check Apache
if systemctl is-active --quiet apache2; then
    echo -e "${GREEN}[✓]${NC} Apache2 is running"
else
    echo -e "${YELLOW}[!]${NC} Apache2 not running, starting..."
    sudo systemctl start apache2 && echo -e "${GREEN}[✓]${NC} Apache2 started" || echo -e "${RED}[✗]${NC} Failed"
fi

# Check MariaDB
if systemctl is-active --quiet mariadb; then
    echo -e "${GREEN}[✓]${NC} MariaDB is running"
else
    echo -e "${YELLOW}[!]${NC} MariaDB not running, starting..."
    sudo systemctl start mariadb && echo -e "${GREEN}[✓]${NC} MariaDB started" || echo -e "${RED}[✗]${NC} Failed"
fi

# Wait and test
echo ""
echo -e "${YELLOW}Waiting for services...${NC}"
sleep 3

if curl -s -o /dev/null -w "%{http_code}" http://localhost:8888/ | grep -qE "200|302"; then
    echo -e "${GREEN}[✓]${NC} SMF Forum accessible"
else
    echo -e "${YELLOW}[!]${NC} SMF may not be ready"
fi

if curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health | grep -q "200"; then
    echo -e "${GREEN}[✓]${NC} Stats API healthy"
else
    echo -e "${YELLOW}[!]${NC} Stats API may not be ready"
fi

# Summary
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}                    ALL SERVICES STARTED${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "  ${BLUE}Stats API:${NC}      http://localhost:8080"
echo -e "  ${BLUE}SMF Forum:${NC}      http://localhost:8888"
echo -e "  ${BLUE}ClickHouse:${NC}     http://localhost:8123"
echo ""
echo -e "${CYAN}Docker:${NC}"
docker ps --format "  {{.Names}}: {{.Status}}" | grep -E "opm-stats" || echo "  (none)"
echo ""
echo -e "${YELLOW}Run ./stop.sh to stop all services${NC}"
