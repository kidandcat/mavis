#!/bin/bash

# Kill any existing mavis processes before starting
echo "Checking for existing mavis processes..."
pkill -f "mavis|go-build.*exe/main" 2>/dev/null
if [ $? -eq 0 ]; then
    echo "Killed existing processes. Waiting for cleanup..."
    sleep 2
fi

# Function to cleanup on exit
cleanup() {
    echo "Shutting down..."
    # Kill the go process and any children
    if [ ! -z "$GO_PID" ]; then
        kill -TERM -$GO_PID 2>/dev/null
        wait $GO_PID 2>/dev/null
    fi
    exit 0
}

# Set up signal handlers
trap cleanup SIGINT SIGTERM

echo "Starting mavis..."

# Main loop
n=1
while true; do
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Starting run #$n"
    
    # Run in background so we can capture PID
    go run . &
    GO_PID=$!
    
    # Wait for the process to finish
    wait $GO_PID
    EXIT_CODE=$?
    
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] Process exited with code $EXIT_CODE"
    
    # Brief pause before restart
    sleep 1
    n=$((n + 1))
done