#!/usr/bin/env bash
# Validation script for Bruno setup
# Checks that everything is configured correctly

set -e

echo "🔍 Validating Bruno API Collection Setup"
echo "========================================"
echo ""

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

ERRORS=0

# Check 1: Bruno CLI installed
echo -n "Checking Bruno CLI... "
if command -v bru &> /dev/null; then
    VERSION=$(bru --version 2>&1 || echo "unknown")
    echo -e "${GREEN}✓${NC} Installed ($VERSION)"
else
    echo -e "${RED}✗${NC} Not found"
    echo -e "  ${YELLOW}Install with: npm install -g @usebruno/cli${NC}"
    ERRORS=$((ERRORS + 1))
fi

# Check 2: Collection structure exists
echo -n "Checking collection structure... "
if [ -d "bruno" ] && [ -f "bruno/bruno.json" ]; then
    echo -e "${GREEN}✓${NC} Found"
else
    echo -e "${RED}✗${NC} bruno/ directory or bruno.json missing"
    ERRORS=$((ERRORS + 1))
fi

# Check 3: Environment files
echo -n "Checking environment files... "
ENV_COUNT=0
for env in bruno/environments/*.bru; do
    [ -e "$env" ] && ENV_COUNT=$((ENV_COUNT + 1))
done
if [ $ENV_COUNT -ge 3 ]; then
    echo -e "${GREEN}✓${NC} Found $ENV_COUNT environments"
else
    echo -e "${RED}✗${NC} Expected 3+ environments, found $ENV_COUNT"
    ERRORS=$((ERRORS + 1))
fi

# Check 4: Request files
echo -n "Checking request files... "
BRU_COUNT=$(find bruno -name "*.bru" -not -path "*/environments/*" | wc -l)
if [ $BRU_COUNT -ge 60 ]; then
    echo -e "${GREEN}✓${NC} Found $BRU_COUNT request files"
else
    echo -e "${YELLOW}⚠${NC}  Found $BRU_COUNT request files (expected 60+)"
    echo -e "  ${YELLOW}Run 'make bruno' to regenerate${NC}"
fi

# Check 5: Swagger spec exists
echo -n "Checking swagger spec... "
if [ -f "web/static/swagger.yaml" ]; then
    ENDPOINTS=$(grep -c "^  /" web/static/swagger.yaml 2>/dev/null || echo 0)
    echo -e "${GREEN}✓${NC} Found ($ENDPOINTS endpoints)"
else
    echo -e "${RED}✗${NC} web/static/swagger.yaml not found"
    echo -e "  ${YELLOW}Run 'make docs' to generate${NC}"
    ERRORS=$((ERRORS + 1))
fi

# Check 6: Python generator script
echo -n "Checking generator script... "
if [ -x "tools/generate_bruno.py" ]; then
    echo -e "${GREEN}✓${NC} Executable"
elif [ -f "tools/generate_bruno.py" ]; then
    echo -e "${YELLOW}⚠${NC}  Found but not executable"
    echo -e "  ${YELLOW}Run: chmod +x tools/generate_bruno.py${NC}"
    ERRORS=$((ERRORS + 1))
else
    echo -e "${RED}✗${NC} Not found"
    ERRORS=$((ERRORS + 1))
fi

# Check 7: Test runner script
echo -n "Checking test runner... "
if [ -x "tools/test_bruno.sh" ]; then
    echo -e "${GREEN}✓${NC} Executable"
elif [ -f "tools/test_bruno.sh" ]; then
    echo -e "${YELLOW}⚠${NC}  Found but not executable"
    echo -e "  ${YELLOW}Run: chmod +x tools/test_bruno.sh${NC}"
    ERRORS=$((ERRORS + 1))
else
    echo -e "${RED}✗${NC} Not found"
    ERRORS=$((ERRORS + 1))
fi

# Check 8: Documentation
echo -n "Checking documentation... "
DOC_COUNT=0
[ -f "bruno/README.md" ] && DOC_COUNT=$((DOC_COUNT + 1))
[ -f "bruno/CI_EXAMPLES.md" ] && DOC_COUNT=$((DOC_COUNT + 1))
[ -f "BRUNO_SETUP.md" ] && DOC_COUNT=$((DOC_COUNT + 1))
if [ $DOC_COUNT -eq 3 ]; then
    echo -e "${GREEN}✓${NC} Complete (3/3 files)"
else
    echo -e "${YELLOW}⚠${NC}  Found $DOC_COUNT/3 documentation files"
fi

# Check 9: Gitignore for secrets
echo -n "Checking gitignore... "
if grep -q "bruno/secrets.bru" .gitignore 2>/dev/null; then
    echo -e "${GREEN}✓${NC} Secrets protected"
else
    echo -e "${YELLOW}⚠${NC}  Secrets not in .gitignore"
    echo -e "  ${YELLOW}Add: bruno/secrets.bru to .gitignore${NC}"
fi

# Check 10: API availability (optional)
echo -n "Checking API availability... "
if curl -sf http://localhost:8084/api/v1/stats/global > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} API responding on :8084"
elif curl -sf http://localhost:8080/api/v1/stats/global > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} API responding on :8080"
else
    echo -e "${YELLOW}⚠${NC}  API not reachable"
    echo -e "  ${YELLOW}Start with: docker-compose up -d${NC}"
fi

echo ""
echo "========================================"
if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}✅ All checks passed!${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Open Bruno Desktop or run: cd bruno && bru run --env Local"
    echo "  2. Update server_token in environments/Local.bru if needed"
    echo "  3. Start testing! 🚀"
    exit 0
else
    echo -e "${RED}❌ Found $ERRORS error(s)${NC}"
    echo ""
    echo "Fix the errors above and run this script again."
    exit 1
fi
