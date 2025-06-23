# Migration Plan: Unified Agent Tracking

## Overview

This document outlines how to migrate from the current multi-map tracking system to the unified TrackedAgent system.

## Current State

Currently, agent information is tracked in multiple places:
1. `agentManager.agents` - Main agent storage
2. `agentUserMap` - Agent-to-user mapping (in agent_monitor.go)
3. `runningPerFolder` - Folder-to-agent mapping (in manager.go)
4. `queueTracker.queuedAgents` - Queued agents waiting to start

## Target State

All agent information will be consolidated into a single `UnifiedAgentTracker` that maintains `TrackedAgent` structs with all necessary metadata.

## Migration Steps

### Phase 1: Add Unified Tracker Alongside Existing System

1. Initialize `UnifiedAgentTracker` in main.go
2. Update agent creation to also register in unified tracker
3. Run both systems in parallel to ensure compatibility

### Phase 2: Migrate Monitoring

1. Update `MonitorAgentsProcess` to use unified tracker
2. Update `RecoveryCheck` to use unified tracker
3. Remove `agentUserMap` and related functions

### Phase 3: Migrate Manager

1. Update manager to use unified tracker for folder tracking
2. Remove `runningPerFolder` map
3. Update queue processing to use unified tracker

### Phase 4: Consolidate Queue Tracking

1. Merge queue tracking into unified system
2. Remove separate `queueTracker`
3. Update all queue-related functions

### Phase 5: Cleanup

1. Remove all old tracking maps
2. Update tests to use unified tracker
3. Document new architecture

## Benefits

1. **Single Source of Truth**: All agent data in one place
2. **Reduced Complexity**: No more synchronization between multiple maps
3. **Better Error Recovery**: Easier to detect and fix inconsistencies
4. **Improved Performance**: Single lock instead of multiple
5. **Easier Debugging**: All state visible in one structure

## Risk Mitigation

1. Run both systems in parallel initially
2. Add comprehensive logging during migration
3. Create rollback plan for each phase
4. Test thoroughly at each phase

## Example Usage After Migration

```go
// Register a new agent
tracker.RegisterAgent(agentID, agent, userID, queueID)

// Get agent info
if tracked, exists := tracker.GetAgent(agentID); exists {
    fmt.Printf("Agent %s for user %d\n", tracked.Agent.ID, tracked.UserID)
}

// Find orphaned agents
orphaned := tracker.GetOrphanedAgents()
for _, tracked := range orphaned {
    // Handle orphaned agent
}
```

## Timeline

- Phase 1: 1 day
- Phase 2: 2 days  
- Phase 3: 2 days
- Phase 4: 1 day
- Phase 5: 1 day

Total: ~1 week for complete migration with testing