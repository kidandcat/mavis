# Modular Package Update Summary

## Changes Made

### 1. Updated main.go imports
- Added imports for `mavis/web`, `mavis/telegram`, and `mavis/core` packages
- Exported `Bot` variable (was `b`) to make it accessible to other packages

### 2. Created initialization functions
- `telegram.InitializeGlobals()` - Sets bot, agentManager, and AdminUserID
- `web.InitializeGlobals()` - Sets bot, agentManager, AdminUserID, and ProjectDir  
- `core.InitializeGlobals()` - Sets AdminUserID

### 3. Updated function references
- `SendMessage` → `core.SendMessage`
- `SendLongMessage` → `core.SendLongMessage`
- `ResolvePath` → `core.ResolvePath`
- `SendFile` → `core.SendFile`
- `IsPortInUse` → `core.IsPortInUse`
- `FindAvailablePort` → `core.FindAvailablePort`
- `queueTracker` → `core.GetQueueTracker()`
- `handleMessage` → `telegram.HandleMessage`
- `handlePhotoMessage` → `telegram.HandlePhotoMessage`
- `handleDocumentMessage` → `telegram.HandleDocumentMessage`
- `MonitorAgentsProcess` → `telegram.MonitorAgentsProcess`
- `RecoveryCheck` → `telegram.RecoveryCheck`
- `StartWebServer` → `web.StartWebServer`

### 4. Moved functions to appropriate packages
- Image handling functions moved from main.go to telegram/init.go
- Exported `HandleMessage`, `HandlePhotoMessage`, `HandleDocumentMessage` in telegram package

### 5. Added package imports where needed
- Added `mavis/core` import to telegram handlers that use core functions
- Added missing imports (path/filepath, sort, etc.) where needed

## Circular Dependencies and Issues

### 1. Global Variables
Several global variables need to be accessible across packages:
- `AdminUserID` - Currently duplicated in core, telegram, and web packages
- `Bot` - Passed during initialization
- `agentManager` - Passed during initialization
- `ProjectDir` - Only needed by web package

### 2. Missing Functions (TODO)
The following functions were referenced but not implemented:
- `BroadcastSSEEvent` - Used by telegram package to notify web interface
  - Solution: Should be moved to web package and made accessible
- `GetPublicIP` - Used for UPnP functionality
  - Solution: Should be in core/upnp.go
- `StartFileServer` - Used for LAN file serving
  - Solution: Should be in web package

### 3. Temporary Stubs Created
- `upnpManager` stub in telegram/init.go - Should be moved to core/upnp.go
- `StartFileServer` stub in telegram/init.go - Should be moved to web package

### 4. LAN Server Variables
The LAN server tracking variables are currently in telegram/init.go but should probably be in their own package or in core:
- `lanServerProcess`
- `lanHTTPServer`
- `lanServerPort`
- `lanServerWorkDir`
- `lanServerCmd`
- `lanServerMutex`

## Recommendations for Resolution

1. **Create a shared config package** to hold truly global configuration that multiple packages need
2. **Move BroadcastSSEEvent to web package** and export it for telegram package to use
3. **Complete the UPnP implementation** in core/upnp.go
4. **Move LAN server functionality** to its own package or into web package
5. **Consider using dependency injection** instead of global variables where possible

## Files Modified

### Core Package
- `/Users/jairo/mavis/core/init.go` (created)
- `/Users/jairo/mavis/core/user.go` (already exists, references AdminUserID)
- `/Users/jairo/mavis/core/utils.go` (already exists with utility functions)
- `/Users/jairo/mavis/core/queue_tracker.go` (already exists)

### Telegram Package  
- `/Users/jairo/mavis/telegram/init.go` (created)
- `/Users/jairo/mavis/telegram/handleMessage.go` (exported HandleMessage)
- `/Users/jairo/mavis/telegram/handlers_*.go` (updated imports and function calls)
- `/Users/jairo/mavis/telegram/agent_monitor.go` (updated imports and function calls)

### Web Package
- `/Users/jairo/mavis/web/init.go` (created)

### Main Package
- `/Users/jairo/mavis/main.go` (updated imports and initialization)

## Next Steps

1. Test compilation to identify any remaining issues
2. Implement the missing functions in their proper packages
3. Consider refactoring to reduce global state
4. Update tests to work with the new modular structure