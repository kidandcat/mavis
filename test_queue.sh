#!/bin/bash

# Test script to verify agent queueing functionality

echo "Testing agent queueing system..."
echo "================================"

# This script demonstrates how the queueing system works:
# 1. Launch an agent in a directory
# 2. While that agent is running, launch another agent in the same directory
# 3. The second agent should be queued and start automatically when the first completes

echo ""
echo "To test the queueing system:"
echo "1. Start the Mavis bot: ./run.sh"
echo "2. In Telegram, send: /code ~/test-dir 'Create a file called test1.txt with content \"First agent\"'"
echo "3. Immediately send: /code ~/test-dir 'Create a file called test2.txt with content \"Second agent\"'"
echo "4. The second command should show that the agent is queued"
echo "5. When the first agent completes, you should get a notification that the queued agent has started"
echo "6. Use /ps to see the queue status at any time"
echo ""
echo "Expected behavior:"
echo "- First agent starts immediately"
echo "- Second agent is queued with position and count info"
echo "- When first agent completes, second agent starts automatically"
echo "- User receives notification when queued agent starts"
echo "- Queue empties after all agents complete"