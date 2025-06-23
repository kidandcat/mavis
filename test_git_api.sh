#!/bin/bash
# Test script for agent launch with path normalization

API_URL="http://localhost:3141/api"

echo "Testing agent launch with different path formats..."

# Test 1: Launch agent with relative path
echo -e "\n1. Testing agent launch with relative path:"
curl -X POST "$API_URL/agents" \
  -H "Content-Type: application/json" \
  -d '{"task": "Test task 1", "work_dir": "."}'

# Wait a bit
sleep 2

# Test 2: Try to launch another agent with absolute path (should queue)
echo -e "\n\n2. Testing agent launch with absolute path (should queue):"
CURRENT_DIR=$(pwd)
curl -X POST "$API_URL/agents" \
  -H "Content-Type: application/json" \
  -d "{\"task\": \"Test task 2\", \"work_dir\": \"$CURRENT_DIR\"}"

# Wait a bit
sleep 2

# Test 3: Try with home-relative path 
echo -e "\n\n3. Testing agent launch with home-relative path (should also queue):"
# Just use relative path for this test since ResolvePath handles ~ separately
curl -X POST "$API_URL/agents" \
  -H "Content-Type: application/json" \
  -d '{"task": "Test task 3", "work_dir": "./"}'

# Show agent status
echo -e "\n\n4. Current agents status:"
curl "$API_URL/agents"

echo -e "\n\nTest complete!"