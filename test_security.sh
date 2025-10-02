#!/bin/bash

# Test WebSocket and CORS Security Configurations
# Run this after starting the backend server

BASE_URL="http://localhost:8080"
API_KEY="your-api-key-here"  # Update with actual key from secrets.cfg
WS_TOKEN="your-ws-token-here"  # Update with actual token from secrets.cfg

echo "Testing CORS and WebSocket Security..."
echo "======================================="

# Test 1: CORS from unauthorized origin
echo -e "\n1. Testing CORS from unauthorized origin (should be blocked):"
curl -i -X OPTIONS $BASE_URL/api/v1/search \
  -H "Origin: http://evil.com" \
  -H "Access-Control-Request-Method: GET" \
  2>/dev/null | grep -E "Access-Control-Allow-Origin|HTTP"

# Test 2: CORS from authorized origin
echo -e "\n2. Testing CORS from authorized origin (should be allowed):"
curl -i -X OPTIONS $BASE_URL/api/v1/search \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: GET" \
  2>/dev/null | grep -E "Access-Control-Allow-Origin|HTTP"

# Test 3: WebSocket without authentication
echo -e "\n3. Testing WebSocket without authentication (should be blocked):"
echo "Attempting connection without token..."
wscat -c ws://localhost:8080/api/v1/ws 2>&1 | head -1 || echo "Connection rejected (expected)"

# Test 4: WebSocket with authentication
echo -e "\n4. Testing WebSocket with authentication (should be allowed):"
echo "Attempting connection with token..."
wscat -c "ws://localhost:8080/api/v1/ws?token=$WS_TOKEN" 2>&1 | head -1 || echo "Check if token is correct"

# Test 5: API endpoint without key (if required)
echo -e "\n5. Testing API endpoint:"
curl -i $BASE_URL/api/v1/health 2>/dev/null | head -1

echo -e "\n======================================="
echo "Security tests completed."
echo ""
echo "Expected results:"
echo "- Unauthorized origins should not receive Access-Control-Allow-Origin header"
echo "- Authorized origins should receive matching Access-Control-Allow-Origin"
echo "- WebSocket without token should fail (401 or connection refused)"
echo "- WebSocket with valid token should connect successfully"
echo ""
echo "Note: Install wscat with: npm install -g wscat"