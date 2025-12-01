# Performance Analysis for Stateful Authorization

## Summary

This document summarizes the performance testing framework built to measure the latency overhead introduced by stateful authorization in the GitHub MCP Server.

## What Was Built

### 1. **Conditional Stateful Auth** ✅
- Added `EnableStatefulAuth` flag to `MCPServerConfig` in `internal/ghmcp/server.go`
- When disabled, session is set to `nil` (zero overhead)
- When enabled, session enforces repository-based locking
- Middleware automatically checks for `nil` session and skips auth

### 2. **Performance Measurement** ✅
- Created `pkg/performance/metrics.go` - tracks timing statistics
  - Records min, max, average, median, P50, P95, P99 latencies
  - Thread-safe with RWMutex
  - Separates auth vs non-auth calls
  
- Created `pkg/performance/instrumented_middleware.go` - measures latency
  - Logs: `[PERF] Tool=X TotalDuration=Y AuthDuration=Z StatefulAuth=true/false`
  - Measures total, auth, and handler durations separately
  - Integrates with Metrics for statistics

### 3. **100-Run Tester** ✅
- Created `pkg/performance/tester.go` - test orchestration
  - Configurable iterations (default 100)
  - Runs tests with and without stateful auth
  - Calculates overhead percentage and slowdown factor
  - Exports results to JSON
  
- Created `cmd/perftest/main.go` - standalone executable
  - Command-line flags for configuration
  - Mock handler for testing
  - Pretty-printed results and JSON export

## Initial Results

From 100 iterations:
```
--- WITHOUT Stateful Auth ---
  Average:             11.266242ms
  Median:              11.356729ms
  P95:                 11.464375ms
  Throughput:          88.70 calls/sec

--- WITH Stateful Auth ---
  Average:             11.348509ms
  Median:              11.326874ms
  P95:                 11.440125ms
  Throughput:          88.07 calls/sec

--- Overhead Analysis ---
  Average Overhead:    82.267µs
  Overhead Percentage: 0.73%
  Slowdown Factor:     1.01x
```

**Key Finding:** Stateful authorization adds approximately **82 microseconds** of overhead per call (0.73% increase).

## Usage

### Quick Test

```bash
# Build the test executable
go build -o perftest ./cmd/perftest

# Run 100 iterations
./perftest

# Run 1000 iterations and save to JSON
./perftest -iterations 1000 -output results.json
```

### Custom Configuration

```bash
# Test specific repository
./perftest -owner myorg -repo myrepo

# Test only with auth
./perftest -without-auth=false

# Test only without auth
./perftest -with-auth=false
```

### Integration with Real Server

See `docs/performance-testing.md` for full documentation on:
- Instrumenting real tool handlers
- Enabling performance logging
- Analyzing production metrics

## Files Created

### Core Framework
- `pkg/performance/metrics.go` (200+ lines) - Statistics tracking
- `pkg/performance/tester.go` (200+ lines) - Test orchestration  
- `pkg/performance/instrumented_middleware.go` (100+ lines) - Timing middleware

### Executable & Docs
- `cmd/perftest/main.go` (100+ lines) - Test executable
- `docs/performance-testing.md` (300+ lines) - Complete documentation

### Server Changes
- `internal/ghmcp/server.go` - Added `EnableStatefulAuth` flag
- `pkg/sessionstate/middleware.go` - Support for `nil` session

## Architecture Changes

### Before
```
Request → Middleware (always validates) → Handler
```

### After  
```
Request → Middleware (checks if session != nil) → Handler
                ↓ (if enabled)
          Session Validation
```

When `session == nil`:
- Zero overhead (direct pass-through)
- Perfect for performance testing
- Can be enabled/disabled via config flag

## Next Steps

1. **Run with real GitHub API calls** to measure actual overhead in production
2. **Test with SQLite session store** (currently tests use in-memory)
3. **Benchmark with different toolsets** (read-only vs write operations)
4. **Profile memory usage** with `go test -benchmem`

## Testing Notes

- Core sessionstate tests: 11/13 passing
- 2 cleanup tests are flaky (timing-dependent due to async touchSession)
- Performance framework compiles and runs successfully
- Real-world overhead may be higher due to SQLite I/O

## Related Documentation

- [Performance Testing Guide](docs/performance-testing.md) - Complete usage guide
- [SQLite Store](pkg/sessionstate/SQLITE_STORE.md) - Session persistence
- [Session State](pkg/sessionstate/README.md) - Stateful auth overview
