# Mavis LaunchD Service Setup

This guide explains how to run Mavis as a macOS launchd service to prevent SIGTERM issues and ensure reliable background operation.

## Overview

The launchd configuration includes:
- Automatic startup at login
- Automatic restart on crashes
- Proper environment variable handling
- Resource limits to prevent system termination
- Nice priority and I/O throttling to reduce system impact
- Comprehensive logging

## Files Created

1. `com.mavis.bot.plist` - LaunchD property list file
2. `launchd_setup.sh` - Installation and management script
3. `LAUNCHD_README.md` - This documentation

## Installation

1. Ensure the Mavis binary is built:
   ```bash
   go build -o mavis .
   ```

2. Install the service:
   ```bash
   ./launchd_setup.sh install
   ```

3. Start the service:
   ```bash
   ./launchd_setup.sh start
   ```

## Management Commands

```bash
# Check service status
./launchd_setup.sh status

# View logs
./launchd_setup.sh logs

# Stop service
./launchd_setup.sh stop

# Restart service
./launchd_setup.sh restart

# Uninstall service
./launchd_setup.sh uninstall
```

## Configuration Details

### Resource Management
- **Nice Priority**: 10 (lower priority to reduce system impact)
- **Low Priority I/O**: Enabled
- **Throttle Interval**: 10 seconds (prevents rapid restarts)
- **Memory Limits**: 
  - Soft: 2GB
  - Hard: 4GB
- **Process Limits**: 
  - Soft: 512
  - Hard: 1024
- **File Descriptor Limits**:
  - Soft: 4096
  - Hard: 8192

### Environment Variables
The service loads all required environment variables:
- `TELEGRAM_BOT_TOKEN`
- `ADMIN_USER_ID`
- `ONLINE_SERVER_URL`
- `WEB_PORT`
- `WEB_PASSWORD`

### Logging
- **Stdout**: `~/Library/Logs/mavis/stdout.log`
- **Stderr**: `~/Library/Logs/mavis/stderr.log`

## Troubleshooting

### Service Won't Start
1. Check logs: `./launchd_setup.sh logs`
2. Verify binary exists: `ls -la /Users/jairo/mavis/mavis`
3. Check permissions: Binary should be executable

### Still Getting SIGTERM
1. Check system logs: `log show --predicate 'subsystem == "com.apple.xpc.launchd"' --info --last 1h`
2. Reduce resource usage in your code
3. Consider adjusting resource limits in the plist

### Port 80 Permission Issues
If you get permission errors for port 80:
1. Change to a higher port (e.g., 8080) in the plist
2. Or use port forwarding: `sudo pfctl -e && echo "rdr pass on lo0 inet proto tcp from any to any port 80 -> 127.0.0.1 port 8080" | sudo pfctl -f -`

## Manual LaunchD Commands

If you prefer manual control:

```bash
# Load service
launchctl load -w ~/Library/LaunchAgents/com.mavis.bot.plist

# Unload service
launchctl unload -w ~/Library/LaunchAgents/com.mavis.bot.plist

# Start service
launchctl start com.mavis.bot

# Stop service
launchctl stop com.mavis.bot

# Check if loaded
launchctl list | grep com.mavis.bot
```

## Security Notes

1. The plist contains sensitive environment variables. Keep it secure.
2. Consider using macOS Keychain for storing secrets instead of plain text.
3. Ensure proper file permissions on the plist and binary.

## Updating the Service

To update configuration:
1. Stop the service: `./launchd_setup.sh stop`
2. Edit `com.mavis.bot.plist`
3. Reload: `./launchd_setup.sh uninstall && ./launchd_setup.sh install`
4. Start: `./launchd_setup.sh start`