# SessionState WASM Module

The stateful authorization and policy enforcement logic has been compiled to WebAssembly, allowing it to be used independently of the GitHub MCP Server or integrated into other applications.

## Overview

The sessionstate package provides session-based access control that:
- Locks a session to a single repository after the first tool call
- Prevents subsequent tool calls from accessing different repositories
- Resets on new conversation/session

The WASM module exports all the core functionality as JavaScript-accessible functions.

## Building the WASM Module

### Prerequisites
- Go 1.21+ (with GOOS=js and GOARCH=wasm support)

### Build Command

```bash
GOOS=js GOARCH=wasm go build -o wasm/sessionstate/sessionstate.wasm ./cmd/wasm/sessionstate
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" wasm/sessionstate/
```

Or use the build script:

```bash
bash script/build-wasm
```

### Output Files
- `wasm/sessionstate/sessionstate.wasm` - Compiled WASM module (~4.8MB)
- `wasm/sessionstate/wasm_exec.js` - Go runtime helper (required)
- `wasm/sessionstate/sessionstate.ts` - TypeScript wrapper (optional)

## Usage Examples

### Browser (JavaScript/TypeScript)

```typescript
import { 
  initSessionStateWasm, 
  validateToolCall, 
  getSessionLockStatus,
  resetSessionState 
} from './wasm/sessionstate/sessionstate';

// Initialize the module
const wasm = await initSessionStateWasm('./wasm/sessionstate/sessionstate.wasm');

// First tool call - locks to this repo
const result1 = validateToolCall(wasm, 'octocat', 'Hello-World');
if (result1.success) {
  console.log('First call succeeded, session locked to octocat/Hello-World');
}

// Second tool call to different repo - blocked
const result2 = validateToolCall(wasm, 'octocat', 'different-repo');
if (!result2.success) {
  console.error('Access denied:', result2.error);
  // Output: Access denied: repository access denied: session is locked to octocat/Hello-World, 
  //         but tool attempted to access octocat/different-repo
}

// Get current lock status
const status = getSessionLockStatus(wasm);
console.log(`Session locked: ${status.locked}`);
if (status.locked) {
  console.log(`Locked to: ${status.owner}/${status.repo}`);
}

// Reset for new conversation
resetSessionState(wasm);
const status2 = getSessionLockStatus(wasm);
console.log(`After reset - locked: ${status2.locked}`); // false
```

### Node.js

```javascript
const { initSessionStateWasm, validateToolCall } = require('./wasm/sessionstate/sessionstate.ts');

async function testSessionState() {
  const wasm = await initSessionStateWasm('./wasm/sessionstate/sessionstate.wasm');
  
  const result = validateToolCall(wasm, 'octocat', 'Hello-World');
  console.log(result); // { success: true }
}

testSessionState().catch(console.error);
```

### Go Integration (Current Approach)

The Go server continues to use the sessionstate package directly:

```go
import "github.com/github/github-mcp-server/pkg/sessionstate"

session := sessionstate.NewSession()
repoContext := &sessionstate.RepositoryContext{
  Owner: "octocat",
  Repo:  "Hello-World",
}

if err := session.ValidateAndLock(repoContext); err != nil {
  // Policy violation
  fmt.Println(err)
}
```

## API Reference

### WASM Functions

All functions are accessible as global JavaScript functions:

#### `validateAndLock(owner: string, repo: string): ValidationResult`

Validates a tool call against the session policy and locks if unlocked.

**Parameters:**
- `owner` - Repository owner
- `repo` - Repository name

**Returns:**
- `{ success: true }` if validation passes
- `{ success: false, error: "..." }` if validation fails

**Behavior:**
- First call: Locks session to the specified repository
- Subsequent calls: Validates that repository matches the locked repository
- No repository context: Passes without locking (e.g., `get_me` tool)

#### `isRepositoryLocked(): LockStatus`

Checks if the session is locked to a repository.

**Returns:**
- `{ locked: true }` if locked
- `{ locked: false }` if unlocked

#### `getLockedRepository(): RepositoryInfo`

Gets the repository the session is locked to.

**Returns:**
- `{ owner: "...", repo: "..." }` if locked
- `{ owner: "", repo: "" }` if unlocked

#### `lockRepository(owner: string, repo: string): Result`

Manually locks the session to a repository.

**Parameters:**
- `owner` - Repository owner
- `repo` - Repository name

**Returns:**
- `{ success: true }`

#### `resetSession(): Result`

Resets the session (clears lock). Called automatically on new conversation initialization.

**Returns:**
- `{ success: true }`

## Size and Performance

- **WASM Binary Size:** ~4.8MB (uncompressed)
- **Initialization Time:** < 100ms
- **Validation Call Latency:** < 1ms

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ GitHub MCP Server                                           │
│                                                              │
│ ┌──────────────────────────────────────────────────────┐   │
│ │ Tool Handlers (list_issues, etc.)                    │   │
│ └──────────────────────────────────────────────────────┘   │
│                          │                                   │
│                          ▼                                   │
│ ┌──────────────────────────────────────────────────────┐   │
│ │ SessionState Wrapper                                 │   │
│ │ (Intercepts tool calls)                              │   │
│ └──────────────────────────────────────────────────────┘   │
│                          │                                   │
│       ┌──────────────────┴──────────────────┐              │
│       │ (Option 1)          (Option 2)      │              │
│       ▼                     ▼               │              │
│   ┌────────────┐       ┌──────────────┐    │              │
│   │ Go Native  │       │ WASM Module  │    │              │
│   │ SessionAPI │  OR   │ (JavaScript) │    │              │
│   └────────────┘       └──────────────┘    │              │
│                                             │              │
└─────────────────────────────────────────────────────────────┘
```

## Testing

Run the sessionstate tests:

```bash
go test ./pkg/sessionstate -v
```

This includes:
- `TestAttackScenario` - Verifies cross-repo access is blocked
- `TestValidateAndLock` - Tests locking behavior
- `TestConcurrentAccess` - Ensures thread safety
- `TestExtractRepositoryFromArgs` - Tests argument parsing

## Security Considerations

1. **Session Isolation:** Each session is independent (reset on new conversation)
2. **Thread Safety:** All operations are protected by mutexes (Go) or single-threaded execution (WASM/JS)
3. **No Persistence:** Sessions exist only in memory
4. **Repository Context:** Extraction from tool arguments is mandatory for enforcement

## Future Enhancements

- [ ] User-level policies (allow multi-repo access for specific users)
- [ ] Time-based session expiration
- [ ] Audit logging of policy violations
- [ ] Support for repository groups/organizations
- [ ] WebAssembly System Interface (WASI) for standalone use
