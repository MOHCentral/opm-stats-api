#!/bin/bash

# Defaults
SERVER_NAME="${1:-Dev Server $(date +%s)}"
SERVER_IP="${2:-127.0.0.1}"
SERVER_PORT="${3:-12203}"
API_URL="http://localhost:8084/api/v1/servers/register"

echo "Registering server..."
echo "  Name: $SERVER_NAME"
echo "  IP:   $SERVER_IP"
echo "  Port: $SERVER_PORT"
echo "----------------------------------------"

# Register
RESPONSE=$(curl -s -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -d "{
    \"name\": \"$SERVER_NAME\",
    \"ip_address\": \"$SERVER_IP\",
    \"port\": $SERVER_PORT
  }")

# Check if curl failed
if [ -z "$RESPONSE" ]; then
    echo "Error: Empty response from API. Is it running at $API_URL?"
    exit 1
fi

# Parse JSON using Python (since jq might not be installed)
echo "$RESPONSE" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    if 'server_id' in data and 'token' in data:
        print('\nSUCCESS! Copy these lines to your server.cfg:\n')
        print(f'set opm_server_id \"{data[\"server_id\"]}\"')
        print(f'set opm_server_token \"{data[\"token\"]}\"')
        print('')
    else:
        print('\nError: unexpected response format')
        print(data)
except Exception as e:
    print('\nError parsing response:')
    print(sys.stdin.read())
"
