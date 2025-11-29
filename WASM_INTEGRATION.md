# WASM Integration

This document explains how the WASM module is integrated with the GitHub MCP Server.

## Overview

The GitHub MCP Server now supports optional integration with the `sessionstate` WASM module for policy enforcement. This allows the server to use WebAssembly-based session state management as an alternative to the native Go implementation.

## Architecture

### Native Go Implementation (Default)

The server uses the native Go implementation by default:
- **File:** `pkg/sessionstate/session.go`
- **Components:** Session struct with thread-safe repository locking
- **Performance:** Native Go speed without WebAssembly overhead
- **Usage:** Default when no WASM path is specified

### WASM Bridge

When a WASM path is provided, the server creates a WASM runtime bridge:
- **File:** `pkg/sessionstate/wasm_loader.go`
- **Runtime:** Uses `github.com/tetratelabs/wazero` for WASM execution
- **Module:** Loads `sessionstate.wasm` compiled from `cmd/wasm/sessionstate/main.go`
- **Fallback:** Automatically falls back to native Go if WASM loading fails

## Enabling WASM Integration

### Via Command Line

```bash
# Using native Go implementation (default)
./github-mcp-server stdio

# Using WASM module
./github-mcp-server stdio --wasm-sessionstate-path /path/to/sessionstate.wasm
```

### Via Environment Variable

```bash
export GITHUB_WASM_SESSIONSTATE_PATH="/path/to/sessionstate.wasm"
./github-mcp-server stdio
```

### In Docker

The Docker image includes the compiled WASM module at `/server/sessionstate.wasm`:

```bash
docker run -e GITHUB_PERSONAL_ACCESS_TOKEN=your-token \
           -e GITHUB_WASM_SESSIONSTATE_PATH=/server/sessionstate.wasm \
           github-mcp-server:latest stdio
```

## Building WASM Module

The WASM module is automatically built as part of the Docker build process. To manually build:

```bash
GOOS=js GOARCH=wasm go build -o wasm/sessionstate/sessionstate.wasm ./cmd/wasm/sessionstate/main.go
```

## Implementation Details

### Session Creation with WASM

```go
// Native Go (default)
session := sessionstate.NewSession()

// With WASM
session := sessionstate.NewSessionWithWASM(context.Background(), "/path/to/sessionstate.wasm")
```

### Session Reset on New Conversation

The server automatically resets the session when a new conversation initializes:

```go
// In hooks.OnBeforeInitialize
if wasmPath != "" {
    session = sessionstate.NewSessionWithWASM(context.Background(), wasmPath)
} else {
    session = sessionstate.NewSession()
}
```

### Policy Enforcement

Both implementations enforce the same policy:
1. Lock session to repository on first tool call
2. Validate all subsequent calls target the same repository
3. Return `PolicyViolationError` if cross-repo access is attempted

## Wazero Runtime

The WASM integration uses `github.com/tetratelabs/wazero` for runtime management:

- **Thread-Safe:** Each WASM bridge has its own runtime instance
- **Resource Cleanup:** Runtime is properly closed when session ends via `WASMBridge.Close()`
- **WASI Support:** Instantiates WASI snapshot preview 1 for system calls
- **Error Handling:** Graceful fallback to native Go if WASM fails

## Performance Considerations

- **Native Go:** Zero overhead, direct function calls
- **WASM:** Small startup cost for runtime initialization, then near-native speed
- **Recommendation:** Use native Go for performance-critical deployments, WASM for portability

## Debugging

Enable debug logging to see WASM integration details:

```bash
./github-mcp-server stdio --wasm-sessionstate-path /path/to/sessionstate.wasm --log-file debug.log
```

Look for `[SESSION][WASM]` log messages to verify WASM bridge is being used.

## Future Enhancements

- Node.js/Browser integration: Export WASM module for use in JavaScript environments
- Multi-language support: Compile WASM module for use in other programming languages
- Custom policies: Extend WASM module with custom policy functions
