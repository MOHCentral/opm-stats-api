#!/bin/bash
# Audit all SMF stat pages for data display

BASE_URL="http://localhost:8888/index.php"
GUID="72750883-29ae-4377-85c4-9367f1f89d1a"

echo "=== SMF STATS SYSTEM AUDIT ==="
echo ""

# Test Player Profile
echo "1. Player Profile Page"
KILLS=$(curl -s "${BASE_URL}?action=mohaaplayer&guid=${GUID}" | grep -oP '(?<=Total Kills</div>\s+<div[^>]+>)[^<]+' | head -1)
DEATHS=$(curl -s "${BASE_URL}?action=mohaaplayer&guid=${GUID}" | grep -oP '(?<=Deaths</div>\s+<div[^>]+>)[^<]+' | head -1)
echo "   Kills: $KILLS"
echo "   Deaths: $DEATHS"
echo ""

# Test Servers Page
echo "2. Servers Page"
curl -s "${BASE_URL}?action=mohaaservers" > /tmp/servers.html
SIZE=$(wc -c < /tmp/servers.html)
echo "   Page Size: ${SIZE} bytes"
echo ""

# Test Achievements Page
echo "3. Achievements Page"
curl -s "${BASE_URL}?action=mohaaachievements" > /tmp/achievements.html
SIZE=$(wc -c < /tmp/achievements.html)
echo "   Page Size: ${SIZE} bytes"
echo ""

# Test Leaderboards
echo "4. Leaderboards Page"
curl -s "${BASE_URL}?action=mohaastats&sa=leaderboards" > /tmp/leaderboards.html
SIZE=$(wc -c < /tmp/leaderboards.html)
echo "   Page Size: ${SIZE} bytes"
echo ""

# Test Weapons Page
echo "5. Weapons Page"
curl -s "${BASE_URL}?action=mohaaweapons" > /tmp/weapons.html
SIZE=$(wc -c < /tmp/weapons.html)
echo "   Page Size: ${SIZE} bytes"
echo ""

# Test Maps Page
echo "6. Maps Page"
curl -s "${BASE_URL}?action=mohaamaps" > /tmp/maps.html
SIZE=$(wc -c < /tmp/maps.html)
echo "   Page Size: ${SIZE} bytes"
echo ""

echo "=== AUDIT COMPLETE ==="
echo ""
echo "View full pages:"
echo "  Player: ${BASE_URL}?action=mohaaplayer&guid=${GUID}"
echo "  Servers: ${BASE_URL}?action=mohaaservers"
echo "  Achievements: ${BASE_URL}?action=mohaaachievements"
