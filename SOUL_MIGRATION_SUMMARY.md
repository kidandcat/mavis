# Soul Migration to SQLite - Summary

## Changes Made

### 1. New SQLite-based Storage System
- **Created `soul/sqlite_store.go`**: Implements SQLite storage for souls with full CRUD operations
- **Created `soul/manager_sqlite.go`**: New manager using SQLite instead of file-based storage
- **Created `soul/migration.go`**: Handles migration of existing .mavis.soul files to SQLite
- **Database location**: `~/.mavis/souls.db`

### 2. Updated Application Code
- **Modified `main.go`**: Uses `ManagerSQLite` instead of `ManagerV2`
- **Modified `web/init.go`**: Updated type references to use `ManagerSQLite`
- **Modified `web/souls_handlers.go`**: 
  - Removed async discovery functionality
  - Removed scan button from UI
  - Updated scan handler to inform users that scanning is no longer needed

### 3. Removed Old Persistence Code
- **Deleted `soul/project_store.go`**: Project-based storage implementation
- **Deleted `soul/manager_v2.go`**: Old manager with file-based storage
- **Deleted `soul/store.go`**: Legacy centralized storage
- **Deleted `soul/manager.go`**: Original soul manager
- **Deleted `soul/project_store_test.go`**: Tests for project store
- **Deleted `soul/manager_v2_test.go`**: Tests for v2 manager

### 4. Updated Documentation
- **Updated `docs/SOUL_FILES.md`**: Now documents SQLite storage instead of file-based storage

## Benefits of SQLite Storage

1. **Performance**: Instant loading and querying of souls (no file system scanning)
2. **Reliability**: ACID-compliant database transactions
3. **Simplicity**: No need for async discovery or file system walking
4. **Centralized**: All souls in one database file in user home directory
5. **Easy Backup**: Single database file to backup/restore

## Migration Process

When Mavis starts with the new SQLite storage:
1. Automatically scans for existing `.mavis.soul` files in project directories
2. Imports them into the SQLite database
3. Removes the old `.mavis.soul` files after successful migration

## Storage Locations

- **Database**: `~/.mavis/souls.db`
- **Pause State**: `~/.config/mavis/souls_pause_state` (unchanged)