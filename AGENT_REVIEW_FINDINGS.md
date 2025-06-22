# Agent Implementation Review - Findings Report

## Executive Summary

The agent implementation has several critical issues that can cause agents to "disappear without feedback". The main problems stem from:
1. Complex lifecycle management with multiple tracking systems
2. Race conditions between monitoring and queue processing
3. Blocking agent execution that can cause goroutine buildup
4. Inconsistent error handling and user notification

## Critical Issues

### 1. Agent Lifecycle Management

**Problem**: Agents are tracked in multiple places that can get out of sync:
- `agentManager.agents` - Main agent storage
- `agentUserMap` - Agent-to-user mapping (for notifications)
- `runningPerFolder` - Folder-to-agent mapping (for queue management)
- `queueTracker.queuedAgents` - Queued agents waiting to start

**Impact**: When these get out of sync, agents can:
- Run without user association (no completion notification)
- Complete without triggering queue processing
- Become "orphaned" and removed without notification

**Root Cause**: The association between queued agents and started agents relies on callbacks that can fail or be missed.

### 2. Blocking Agent Execution

**Problem**: `agent.Start()` uses `cmd.CombinedOutput()` which blocks until the process completes.

**Impact**: 
- Goroutines accumulate for long-running agents
- No way to stream output in real-time
- Can't detect if agent process dies unexpectedly

**Code Location**: `/Users/jairo/mavis/codeagent/agent.go:112`

### 3. Race Conditions in Agent Removal

**Problem**: Multiple components try to remove agents:
- Monitor detects completion and calls `RemoveAgent`
- `RemoveAgent` triggers queue processing
- Queue processing might start before notification is sent

**Impact**: 
- User might not receive completion notification
- Queue might process prematurely
- Duplicate removal attempts cause errors

### 4. Queue Processing Dependencies

**Problem**: Queue processing depends on successful agent removal, which depends on monitor detection.

**Impact**: If any step fails:
- Queued agents never start
- Folders get "stuck" with phantom running agents
- Users don't get feedback about queue status

### 5. Error Handling Gaps

**Problem**: Many errors are logged but not communicated to users:
- Agent start failures
- Queue processing failures  
- Monitor detection failures

**Impact**: Users experience "disappearing agents" with no explanation.

## Simplification Opportunities

### 1. Unified Agent Tracking
Instead of multiple maps, use a single source of truth:
```go
type TrackedAgent struct {
    Agent    *Agent
    UserID   int64
    QueueID  string  // If this was queued
    Folder   string
}
```

### 2. Event-Based Architecture
Replace polling monitor with event-driven system:
- Agent completion triggers event
- Event handler sends notification AND processes queue
- No timing dependencies

### 3. Streaming Execution
Replace `CombinedOutput()` with `StdoutPipe/StderrPipe`:
- Stream output in real-time
- Detect process death immediately
- Better user experience

### 4. Simplified Queue Management
- Store queue info with agent from the start
- No complex ID transformations
- Direct association maintained throughout lifecycle

## Immediate Fixes Needed

### Priority 1: Fix Agent-User Association
Ensure every started agent has a user association:
```go
// In createAndStartAgent, before starting:
if queueID != "" {
    if info, exists := queueTracker.GetQueuedAgentInfo(queueID); exists {
        RegisterAgentForUser(id, info.UserID)
    }
}
```

### Priority 2: Fix Race Condition
Add completion callback to agent instead of relying on monitor:
```go
// In agent.Start():
defer func() {
    if a.completionCallback != nil {
        a.completionCallback(a)
    }
}()
```

### Priority 3: Add Recovery Mechanism
Add periodic check for stuck agents/queues:
```go
// Every 30 seconds, check for:
- Agents marked running but process is dead
- Folders with queues but no running agent
- Orphaned agents without user association
```

## Test Coverage Gaps

Components without tests that need coverage:
1. **queue_tracker.go** - Critical for queue functionality
2. **handleMessage.go** - User interaction layer
3. **Integration tests** for:
   - Queue to agent transition
   - Agent failure recovery
   - Concurrent operations

## Recommended Action Plan

1. **Immediate**: Add recovery mechanism to detect stuck agents
2. **Short-term**: Fix agent-user association for queued agents
3. **Medium-term**: Refactor to event-based architecture
4. **Long-term**: Simplify to single tracking system

## Conclusion

The agent "disappearing" issue is caused by a combination of:
- Complex state management across multiple systems
- Race conditions in completion handling
- Lack of recovery mechanisms
- Silent failure modes

The system needs simplification and better error recovery to be reliable.