# WASM Integration for Session State Validation

## Overview

This document describes the WebAssembly (WASM) integration for the GitHub MCP Server's session state management. The WASM module provides an optional sandboxed environment for enforcing repository access policies, preventing cross-repository access attacks.

## Quick Start

### Building the WASM Module

```bash
# Build WASM module
bash script/build-wasm

# Output files:
# - wasm/sessionstate/sessionstate.wasm (2.4MB)
# - wasm/sessionstate/wasm_exec.js (runtime helper)
```

### Running with WASM

```bash
# Native mode (default - no WASM overhead)
./github-mcp-server stdio

# WASM mode (with sandboxed policy enforcement)
./github-mcp-server stdio --wasm-sessionstate-path ./wasm/sessionstate/sessionstate.wasm

# With other flags
./github-mcp-server stdio \
  --wasm-sessionstate-path ./wasm/sessionstate/sessionstate.wasm \
  --stateful-auth \
  --toolsets default
```

## Architecture

### Components

1. **pkg/sessionstate/wasm_bridge.go** (228 lines)
   - Wraps WebAssembly runtime using tetratelabs/wazero
   - Methods: NewWASMBridge(), ValidateAndLockWASM(), LockRepositoryWASM(), UnlockRepositoryWASM()
   - Memory management: writeString() for safe string passing to WASM

2. **cmd/wasm/sessionstate/main.go** (115 lines)
   - Go-to-WASM entry point with syscall/js bindings
   - Exports: validateAndLock, lockRepository, unlockRepository, getLockStatus, isLocked
   - Platform: //go:build wasm

3. **pkg/sessionstate/session.go**
   - Added WASM support: useWASM, wasmBridge fields
   - NewSessionWithWASM() constructor

4. **pkg/sessionstate/policy.go**
   - ValidateAndLock() delegates to WASM when enabled
   - Falls back to native Go if WASM unavailable
   - Unlocks session on policy violation

5. **cmd/github-mcp-server/main.go**
   - New flag: --wasm-sessionstate-path (optional)

## Security Model

### Attack Prevention

**Threat:** Cross-repository access by compromised/malicious LLM

**Solution:** Repository locking per session
- First tool call locks session to repository
- Subsequent calls must access same repository
- Cross-repo attempts trigger PolicyViolationError
- Session unlocks to allow user intervention

### Validation Flow

```
Tool Call
  ↓
Session.ValidateAndLock(repo)
  ↓
  ├─→ [WASM Enabled]
  │    ├─→ WASMBridge.ValidateAndLockWASM()
  │    └─→ [Isolated WASM execution]
  │
  └─→ [Native Mode]
       └─→ Native Go validation
  ↓
Policy Violation?
  ├─→ YES: Unlock, return error, stop execution
  └─→ NO: Continue
```

## Testing

### Unit Tests

```bash
go test ./pkg/sessionstate -v
```

### Test Coverage

- **TestAttackScenario**: Full attack prevention workflow
- **TestMultipleSessions**: Independent session locks
- **TestConcurrentAccess**: Thread-safe locking
- **TestUnlockOnRejection**: Policy enforcement
- **TestToolsWithoutRepositoryContext**: Non-repo tools

### Key Test Result

```
✓ Step 1: Agent listed issues in public-repo (session locked)
✓ Step 2: Cross-repo access BLOCKED (policy violation)
✓ Step 3: Session unlocked after rejection
✓ Step 4: Legitimate cross-repo access works after user intervention
```

## Performance

### Build Times
- Server binary: ~1 second
- WASM module: ~2 seconds
- Total: ~3 seconds

### Module Size
- Server binary: 26MB (arm64)
- WASM module: 2.4MB
- wasm_exec.js: 17KB

### Runtime Overhead
- **Native mode**: Negligible (~microseconds)
- **WASM mode**: ~1-5ms per validation (module loads once per session)

## Files

### Created
- `pkg/sessionstate/wasm_bridge.go` - WASM runtime wrapper
- `cmd/wasm/sessionstate/main.go` - WASM entry point
- `script/build-wasm` - Build script
- `WASM_INTEGRATION.md` - This file

### Modified
- `pkg/sessionstate/session.go` - Added WASM support
- `pkg/sessionstate/policy.go` - Added WASM delegation
- `pkg/sessionstate/manager.go` - Added WASM-aware creation
- `cmd/github-mcp-server/main.go` - Added CLI flag
- `internal/ghmcp/server.go` - Added configuration
- `go.mod` - Added wazero v1.10.1

## Deployment

### Development
```bash
./github-mcp-server stdio  # Native mode (default)
```

### Production (Security-Sensitive)
```bash
bash script/build-wasm
./github-mcp-server stdio --wasm-sessionstate-path ./wasm/sessionstate/sessionstate.wasm
```

### Docker
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY . .
RUN bash script/build-wasm
RUN go build -o server ./cmd/github-mcp-server

FROM alpine:latest
COPY --from=builder /build/server /app/server
COPY --from=builder /build/wasm/sessionstate/sessionstate.wasm /app/wasm/
ENTRYPOINT ["/app/server", "stdio", "--wasm-sessionstate-path", "/app/wasm/sessionstate.wasm"]
```

## Troubleshooting

### WASM Module Not Loading
```bash
# Verify file exists and is executable
ls -lh ./wasm/sessionstate/sessionstate.wasm
chmod +x ./wasm/sessionstate/sessionstate.wasm

# Check logs
./github-mcp-server stdio --wasm-sessionstate-path ... 2>&1 | grep WASM
```

### Policy Not Enforced
1. Verify flag: `--wasm-sessionstate-path` is set
2. Check stderr for `[WASM]` debug messages
3. Confirm WASM module built successfully: `file wasm/sessionstate/sessionstate.wasm`

## References

- [tetratelabs/wazero](https://github.com/tetratelabs/wazero) - WebAssembly runtime
- [Go WASM Support](https://github.com/golang/go/wiki/WebAssembly)
- [WebAssembly Spec](https://webassembly.org/)
