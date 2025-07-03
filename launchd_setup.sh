#!/bin/bash

# Mavis LaunchD Setup Script

PLIST_NAME="com.mavis.bot"
PLIST_FILE="$PLIST_NAME.plist"
LAUNCHD_DIR="$HOME/Library/LaunchAgents"
LOG_DIR="$HOME/Library/Logs/mavis"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

function print_usage() {
    echo "Usage: $0 [install|uninstall|start|stop|restart|status|logs]"
    echo ""
    echo "Commands:"
    echo "  install   - Install the launchd service"
    echo "  uninstall - Remove the launchd service"
    echo "  start     - Start the service"
    echo "  stop      - Stop the service"
    echo "  restart   - Restart the service"
    echo "  status    - Check service status"
    echo "  logs      - Tail the service logs"
}

function create_log_directory() {
    if [ ! -d "$LOG_DIR" ]; then
        echo -e "${YELLOW}Creating log directory...${NC}"
        mkdir -p "$LOG_DIR"
    fi
}

function install_service() {
    echo -e "${GREEN}Installing Mavis launchd service...${NC}"
    
    # Create log directory
    create_log_directory
    
    # Create LaunchAgents directory if it doesn't exist
    mkdir -p "$LAUNCHD_DIR"
    
    # Copy plist file
    cp "$PLIST_FILE" "$LAUNCHD_DIR/"
    
    # Load the service
    launchctl load -w "$LAUNCHD_DIR/$PLIST_FILE"
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}Service installed successfully!${NC}"
        echo -e "${YELLOW}The service will start automatically at login.${NC}"
        echo -e "${YELLOW}To start it now, run: $0 start${NC}"
    else
        echo -e "${RED}Failed to install service.${NC}"
        exit 1
    fi
}

function uninstall_service() {
    echo -e "${YELLOW}Uninstalling Mavis launchd service...${NC}"
    
    # Stop and unload the service
    launchctl unload -w "$LAUNCHD_DIR/$PLIST_FILE" 2>/dev/null
    
    # Remove plist file
    rm -f "$LAUNCHD_DIR/$PLIST_FILE"
    
    echo -e "${GREEN}Service uninstalled successfully!${NC}"
}

function start_service() {
    echo -e "${GREEN}Starting Mavis service...${NC}"
    launchctl start "$PLIST_NAME"
    
    # Wait a moment and check status
    sleep 2
    status_service
}

function stop_service() {
    echo -e "${YELLOW}Stopping Mavis service...${NC}"
    launchctl stop "$PLIST_NAME"
}

function restart_service() {
    echo -e "${YELLOW}Restarting Mavis service...${NC}"
    stop_service
    sleep 2
    start_service
}

function status_service() {
    echo -e "${GREEN}Checking service status...${NC}"
    
    # Check if service is loaded
    if launchctl list | grep -q "$PLIST_NAME"; then
        echo -e "${GREEN}Service is loaded${NC}"
        
        # Get detailed status
        STATUS=$(launchctl list | grep "$PLIST_NAME")
        echo "Status: $STATUS"
        
        # Check if process is running
        PID=$(echo "$STATUS" | awk '{print $1}')
        if [ "$PID" != "-" ] && [ "$PID" != "0" ]; then
            echo -e "${GREEN}Service is running (PID: $PID)${NC}"
            
            # Show process info
            ps aux | grep -E "^[^ ]*[ ]+$PID" | grep -v grep
        else
            echo -e "${RED}Service is not running${NC}"
        fi
    else
        echo -e "${RED}Service is not loaded${NC}"
    fi
}

function show_logs() {
    echo -e "${GREEN}Showing Mavis logs (press Ctrl+C to exit)...${NC}"
    echo ""
    
    # Create log files if they don't exist
    touch "$LOG_DIR/stdout.log" "$LOG_DIR/stderr.log"
    
    # Tail both stdout and stderr
    tail -f "$LOG_DIR/stdout.log" "$LOG_DIR/stderr.log"
}

# Main script logic
case "$1" in
    install)
        install_service
        ;;
    uninstall)
        uninstall_service
        ;;
    start)
        start_service
        ;;
    stop)
        stop_service
        ;;
    restart)
        restart_service
        ;;
    status)
        status_service
        ;;
    logs)
        show_logs
        ;;
    *)
        print_usage
        exit 1
        ;;
esac