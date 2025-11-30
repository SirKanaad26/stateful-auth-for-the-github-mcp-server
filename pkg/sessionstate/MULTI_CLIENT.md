# Multi-Client Session Management

This document describes the per-client session architecture for stateful authorization in multi-tenant deployments.

## Problem Statement

The original implementation used a **single shared session** per server instance:

```go
var session *sessionstate.Session  // Shared across ALL clients!
```

This works fine for **stdio mode** (one process per client) but breaks in **HTTP/multi-tenant mode**:

- Multiple clients share the same session state
- One client's actions affect other clients
- Session resets impact all connected users
- **Security vulnerability**: Cross-client state leakage

## Solution: SessionManager

The `SessionManager` provides **per-client session isolation** while maintaining backward compatibility with stdio mode.

### Architecture

```
┌─────────────────────────────────────────────────┐
│ Server Process                                  │
│                                                 │
│  SessionManager                                 │
│  ├─ Client A (token-abc) → Session A           │
│  │   └─ Locked to: user-a/repo-a               │
│  │                                              │
│  ├─ Client B (token-def) → Session B           │
│  │   └─ Locked to: user-b/repo-b               │
│  │                                              │
│  └─ Client C (token-xyz) → Session C           │
│      └─ Locked to: user-c/repo-c               │
│                                                 │
└─────────────────────────────────────────────────┘
```

### Key Components

#### 1. SessionManager (`manager.go`)

Manages multiple sessions, one per client:

```go
type SessionManager struct {
    sessions map[string]*Session
    mu       sync.RWMutex
    // ...
}
```

**Key Methods:**
- `GetOrCreateSession(clientID)` - Get or create session for a client
- `ResetSession(clientID)` - Reset session on MCP initialize
- `RemoveSession(clientID)` - Clean up on client disconnect
- `SessionCount()` - Get number of active sessions

#### 2. Client ID Extraction (`client_id.go`)

Extracts unique client identifiers from different transport types:

**stdio mode:**
```go
clientID := StdioClientIDExtractor(ctx)  // Returns "default"
```

**HTTP mode:**
```go
clientID := HTTPClientIDExtractor(request)  // Extracts from auth header
```

**Priority order for HTTP:**
1. `Authorization` header (hashed for privacy)
2. `X-MCP-Session-ID` header
3. `mcp-session` cookie
4. IP address + User-Agent
5. Default (fallback)

## Usage Examples

### stdio Mode (Backward Compatible)

No changes needed! Works exactly as before:

```go
// In server.go
session := sessionstate.NewSession()

// On initialize
session = sessionstate.NewSession()  // Reset

// On tool call
session.ValidateAndLock(repoContext)
```

### HTTP Mode (Multi-Tenant)

Use SessionManager for proper isolation:

```go
// Create manager once at server startup
sessionManager := sessionstate.NewSessionManager()

// On each HTTP request
func handleRequest(w http.ResponseWriter, r *http.Request) {
    // Extract client ID from request
    clientID := sessionstate.HTTPClientIDExtractor(r)
    
    // Get/create session for this client
    session := sessionManager.GetOrCreateSession(clientID)
    
    // Use session for this request
    if err := session.ValidateAndLock(repoContext); err != nil {
        // Handle policy violation
    }
}

// On MCP initialize request
func handleInitialize(w http.ResponseWriter, r *http.Request) {
    clientID := sessionstate.HTTPClientIDExtractor(r)
    session := sessionManager.ResetSession(clientID)
    // ...
}

// On client disconnect
func handleDisconnect(clientID string) {
    sessionManager.RemoveSession(clientID)
}
```

### Custom Client ID Extraction

For other transport types:

```go
// Define custom extractor
func MyCustomExtractor(ctx interface{}) string {
    // Extract from your context
    if myCtx, ok := ctx.(*MyContext); ok {
        return myCtx.SessionID
    }
    return sessionstate.DefaultClientID
}

// Use it
clientID := MyCustomExtractor(ctx)
session := sessionManager.GetOrCreateSession(clientID)
```

## Integration Guide

### For stdio Server (No Changes Needed)

Current code works as-is. The `SessionManager` is optional for stdio mode.

### For HTTP/Remote Server (Changes Required)

#### Step 1: Create SessionManager

In `server.go`, replace single session with manager:

```go
// OLD:
var session *sessionstate.Session
if cfg.WAsmSessionStatePath != "" {
    session = sessionstate.NewSessionWithWASM(ctx, cfg.WAsmSessionStatePath)
} else {
    session = sessionstate.NewSession()
}

// NEW:
var sessionManager *sessionstate.SessionManager
if cfg.WAsmSessionStatePath != "" {
    sessionManager = sessionstate.NewSessionManagerWithWASM(ctx, cfg.WAsmSessionStatePath)
} else {
    sessionManager = sessionstate.NewSessionManager()
}
```

#### Step 2: Extract Client ID

Add client ID extraction to your request handler:

```go
// Determine client ID based on transport
var clientID string
if httpReq, ok := request.(*http.Request); ok {
    clientID = sessionstate.HTTPClientIDExtractor(httpReq)
} else {
    clientID = sessionstate.StdioClientIDExtractor(nil)
}
```

#### Step 3: Get Session for Client

```go
session := sessionManager.GetOrCreateSession(clientID)
```

#### Step 4: Update OnBeforeInitialize Hook

```go
OnBeforeInitialize: []server.OnBeforeInitializeFunc{
    func(ctx context.Context, _ any, request *mcp.InitializeRequest) {
        // Extract client ID from context
        clientID := extractClientIDFromContext(ctx)
        
        // Reset session for this client
        session := sessionManager.ResetSession(clientID)
        
        // Continue with initialization
    },
}
```

#### Step 5: Pass Session to Tool Handlers

Modify tool registration to use the client-specific session:

```go
// Wrap tool handlers with session extraction
func wrapWithSessionExtraction(sm *sessionstate.SessionManager, handler ToolHandler) ToolHandler {
    return func(ctx context.Context, request Request) Response {
        clientID := extractClientIDFromContext(ctx)
        session := sm.GetOrCreateSession(clientID)
        
        // Use session for this request
        return handler(ctx, request, session)
    }
}
```

## Security Considerations

### Client ID Privacy

Client IDs are **hashed** when derived from tokens:

```go
// Token: "ghp_abc123xyz..."
// Client ID: "auth-a1b2c3d4e5f6..."  (first 16 chars of SHA256)
```

This prevents sensitive tokens from appearing in logs.

### Session Isolation

Each client gets a **completely independent session**:

- Client A locked to `repo-a` does NOT affect Client B
- Session resets are per-client
- No cross-client state leakage

### Memory Management

**Sessions are long-lived** by design:

- Created on first request
- Persist until explicitly removed
- Reset on MCP initialize (not removed)

For long-running servers, implement cleanup:

```go
// Periodic cleanup of idle sessions (optional)
go func() {
    ticker := time.NewTicker(1 * time.Hour)
    for range ticker.C {
        sessionManager.CleanupIdleSessions()
    }
}()
```

## Testing

Comprehensive tests ensure correctness:

- `manager_test.go` - SessionManager functionality
- `client_id_test.go` - Client ID extraction
- Multi-tenant scenarios
- Concurrent access
- Session isolation

Run tests:

```bash
go test ./pkg/sessionstate -v
```

## Monitoring

Track session metrics:

```go
// Number of active sessions
count := sessionManager.SessionCount()

// List all client IDs (for debugging)
clientIDs := sessionManager.ListClientIDs()

// Log session activity
fmt.Printf("Active sessions: %d\n", count)
for _, id := range clientIDs {
    session := sessionManager.GetSession(id)
    if session.IsRepositoryLocked() {
        owner, repo := session.GetLockedRepository()
        fmt.Printf("  %s: locked to %s/%s\n", id, owner, repo)
    }
}
```

## Migration Checklist

For existing deployments:

- [ ] Determine deployment mode (stdio vs HTTP)
- [ ] If stdio: No changes needed
- [ ] If HTTP: Follow integration guide
- [ ] Add SessionManager to server initialization
- [ ] Implement client ID extraction
- [ ] Update OnBeforeInitialize hook
- [ ] Modify tool handler registration
- [ ] Test with multiple concurrent clients
- [ ] Add monitoring/logging
- [ ] Deploy and verify session isolation

## Performance Impact

**Minimal overhead:**

- Client ID extraction: ~1-2 microseconds (SHA256 hash)
- Session lookup: O(1) map lookup
- Mutex overhead: RWMutex for concurrent access
- Memory: ~100 bytes per session

**Compared to original:**
- Single session: Same performance per client
- Multiple clients: Proper isolation (priceless)

## Future Enhancements

Possible improvements:

1. **Session persistence** - Store sessions in Redis/database
2. **Idle timeout** - Auto-remove inactive sessions
3. **Session analytics** - Track lock patterns, violations
4. **Admin API** - View/manage sessions
5. **Cross-instance sessions** - Share state across server instances
