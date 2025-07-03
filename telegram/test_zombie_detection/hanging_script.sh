#!/bin/bash

# This is a test script that simulates a hanging process
echo "Starting hanging script..."
echo "This script will run indefinitely until interrupted (Ctrl+C)"

# Infinite loop to simulate hanging
while true; do
    sleep 1
    echo -n "."
done