# Soul Storage Documentation

## Overview

Souls in Mavis are now persisted in a SQLite database located at `~/.mavis/souls.db`. This provides faster access, better performance, and centralized management of all souls.

## Storage Location

- **Database Path**: `~/.mavis/souls.db`
- **Pause State**: `~/.config/mavis/souls_pause_state`

## Database Schema

Souls are stored in a single table with the following structure:

```sql
CREATE TABLE souls (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    project_path TEXT NOT NULL UNIQUE,
    objectives TEXT NOT NULL,      -- JSON array
    requirements TEXT NOT NULL,    -- JSON array
    status TEXT NOT NULL,
    feedback TEXT NOT NULL,        -- JSON object
    iterations TEXT NOT NULL,      -- JSON array
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
```

## Soul Data Structure

Each soul contains:

```json
{
  "id": "20240115123456-abc123",
  "name": "MyProject",
  "project_path": "/path/to/project",
  "objectives": [
    "Build a REST API",
    "Implement authentication"
  ],
  "requirements": [
    "Use Go 1.21+",
    "PostgreSQL database"
  ],
  "status": "standby",
  "feedback": {
    "implemented_features": [],
    "known_bugs": [],
    "test_results": [],
    "last_updated": "2024-01-15T12:34:56Z"
  },
  "iterations": [],
  "created_at": "2024-01-15T12:34:56Z",
  "updated_at": "2024-01-15T12:34:56Z"
}
```

## Migration from Project-Based Storage

When Mavis starts for the first time with the new SQLite storage:

1. It automatically scans for any existing `.mavis.soul` files in project directories
2. Imports them into the SQLite database
3. Removes the old `.mavis.soul` files after successful migration

## Benefits of SQLite Storage

- **Performance**: Instant loading and querying of souls
- **Reliability**: ACID-compliant database transactions
- **Simplicity**: No need for file system scanning
- **Portability**: Single database file contains all souls
- **Backup**: Easy to backup/restore the entire soul database

## Soul Status Values

- `standby`: Soul is ready for work but not currently active
- `working`: Soul is actively being worked on by an agent

## Creating Souls

Souls are created through the Mavis UI and automatically stored in the SQLite database. Each soul is uniquely identified by:
- **ID**: Auto-generated timestamp-based ID
- **Project Path**: Must be unique across all souls