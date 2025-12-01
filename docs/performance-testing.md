# Performance Testing Framework

This document describes the performance testing framework for measuring the latency overhead introduced by stateful authorization.

## Overview

The performance testing framework allows you to:
1. Conditionally enable/disable stateful authorization
2. Measure API call latency with and without authorization
3. Run automated tests with 100+ iterations for statistical analysis
4. Export results in JSON format for further analysis

## Components

### 1. Performance Metrics (`pkg/performance/metrics.go`)

Tracks and analyzes performance metrics:
- Records individual tool call durations
- Calculates statistics (min, max, average, median, P50, P95, P99)
- Thread-safe for concurrent use
- Separates auth vs non-auth calls

**Usage:**
```go
metrics := performance.NewMetrics()
metrics.RecordToolCall(duration, isStatefulAuth)
stats := metrics.GetStats()
fmt.Println(stats.String())
```

### 2. Instrumented Middleware (`pkg/performance/instrumented_middleware.go`)

Wraps tool handlers with timing measurements:
- Measures total duration, auth duration, and handler duration separately
- Logs detailed performance data with `[PERF]` prefix
- Automatically records metrics
- Supports nil session for no-auth mode

**Usage:**
```go
metrics := performance.NewMetrics()
handler := performance.InstrumentedToolHandlerWrapper(
    session,       // nil for no-auth mode
    originalHandler,
    metrics,
)
```

### 3. Performance Tester (`pkg/performance/tester.go`)

Orchestrates performance test runs:
- Configurable iterations (default 100)
- Runs tests with and without auth
- Calculates overhead statistics
- Exports results to JSON

**Usage:**
```go
config := performance.TestConfig{
    Iterations:      100,
    ToolName:        "get_file_contents",
    ToolArgs:        map[string]interface{}{"path": "README.md"},
    TestWithAuth:    true,
    TestWithoutAuth: true,
    Owner:           "github",
    Repo:            "github-mcp-server",
}

test := performance.NewPerformanceTest(config, handlerFunc)
result, err := test.Run(context.Background())
result.PrintResults()
result.SaveToFile("results.json")
```

### 4. Test Executable (`cmd/perftest/main.go`)

Standalone executable for running performance tests:

```bash
./perftest [flags]

Flags:
  -iterations int     Number of test iterations (default 100)
  -owner string       Repository owner (default "github")
  -repo string        Repository name (default "github-mcp-server")
  -with-auth          Run tests with stateful auth (default true)
  -without-auth       Run tests without stateful auth (default true)
  -output string      Output file for JSON results (optional)
```

## Running Performance Tests

### Build the Test Executable

```bash
go build -o perftest ./cmd/perftest
```

### Run Basic Test

```bash
./perftest
```

This runs 100 iterations with both auth modes and displays results.

### Run with Custom Configuration

```bash
./perftest -iterations 500 -output results.json
```

### Run Only Auth Mode

```bash
./perftest -without-auth=false
```

### Run Only No-Auth Mode

```bash
./perftest -with-auth=false
```

## Enabling/Disabling Stateful Auth in Server

### Server Configuration

The `EnableStatefulAuth` flag in `MCPServerConfig` controls whether stateful authorization is enabled:

```go
cfg := &ghmcp.MCPServerConfig{
    EnableStatefulAuth: true,  // Enable stateful auth
    // ... other config
}
```

When `EnableStatefulAuth` is `false`:
- Session is set to `nil`
- Middleware skips auth checks (zero overhead)
- Server logs: "Stateful authorization DISABLED (performance mode)"

When `EnableStatefulAuth` is `true`:
- Session is created normally
- Middleware enforces repository locking
- Server logs: "Stateful authorization ENABLED"

### Environment Variable

You can also control this via environment variable:

```bash
# Enable stateful auth
export GITHUB_STATEFUL_AUTH=true
./github-mcp-server stdio

# Disable for performance testing
export GITHUB_STATEFUL_AUTH=false
./github-mcp-server stdio
```

## Understanding Results

### Example Output

```
======================================================================
PERFORMANCE TEST RESULTS
======================================================================

--- WITHOUT Stateful Auth ---
Performance Statistics:
  Total Calls:           100
  Sample Size:           100
  
  Latency:
    Min:                 10.604125ms
    Max:                 12.501458ms
    Average:             11.266242ms
    Median:              11.356729ms
    P95:                 11.464375ms
    P99:                 12.501458ms
  
  Average Throughput:    88.70 calls/sec

--- WITH Stateful Auth ---
Performance Statistics:
  Total Calls:           100
  Sample Size:           100
  
  Latency:
    Min:                 10.413167ms
    Max:                 22.202792ms
    Average:             11.348509ms
    Median:              11.326874ms
    P95:                 11.440125ms
    P99:                 22.202792ms
  
  Average Throughput:    88.07 calls/sec

--- Overhead Analysis ---
  Average Overhead:      82.267µs
  Overhead Percentage:   0.73%
  Slowdown Factor:       1.01x
======================================================================
```

### Metrics Explained

- **Min/Max**: Fastest and slowest call times
- **Average**: Mean latency across all calls
- **Median**: Middle value (50th percentile)
- **P95**: 95th percentile - 95% of calls were faster than this
- **P99**: 99th percentile - 99% of calls were faster than this
- **Overhead**: Additional time added by stateful auth
- **Overhead Percentage**: Overhead as % of baseline (no-auth) time
- **Slowdown Factor**: How many times slower with auth (1.0x = no slowdown)

### JSON Output

Results are also saved in JSON format for programmatic analysis:

```json
{
  "Config": {
    "Iterations": 100,
    "ToolName": "get_file_contents",
    "ToolArgs": {"path": "README.md"},
    "TestWithAuth": true,
    "TestWithoutAuth": true,
    "Owner": "github",
    "Repo": "github-mcp-server"
  },
  "WithAuthStats": {
    "TotalCalls": 100,
    "StatefulAuthCalls": 100,
    "Min": 10413167,
    "Max": 22202792,
    "Average": 11348509,
    "Median": 11326874,
    "P50": 11327916,
    "P95": 11440125,
    "P99": 22202792,
    "TotalDuration": 1135490250,
    "SampleSize": 100
  },
  "WithoutAuthStats": { ... },
  "OverheadAverage": 82267,
  "OverheadPercentage": 0.73
}
```

All duration values in JSON are in nanoseconds.

## Integration with Real Server

To measure real-world performance with actual GitHub API calls:

### 1. Modify Server Initialization

```go
// In internal/ghmcp/server.go
metrics := performance.NewMetrics()

// Wrap your tool handlers
handler := performance.InstrumentedToolHandlerWrapper(
    session,
    originalHandler,
    metrics,
)
```

### 2. Enable Performance Logging

The instrumented middleware automatically logs:
```
[PERF] Tool=get_file_contents TotalDuration=11.2ms AuthDuration=82µs HandlerDuration=11.1ms StatefulAuth=true
```

### 3. Extract Statistics

After running the server for a while, extract statistics:
```go
stats := metrics.GetStats()
fmt.Println(stats.String())
```

## Performance Optimization Tips

### 1. Session State Caching

The session state is already cached in memory. For better performance with SQLite:
- Connection pooling is enabled (25 max, 5 idle)
- WAL mode reduces write contention
- Prepared statements minimize parsing overhead

### 2. Reduce Auth Overhead

- Use repository-only locking (owner removed)
- Session validation is O(1) after first lock
- Middleware skips auth when `session == nil`

### 3. Monitoring in Production

Enable performance logging to identify slow operations:
```bash
GITHUB_MCP_SERVER_PERF_LOG=true ./github-mcp-server stdio
```

## Troubleshooting

### High Overhead Percentage

If overhead is >10%, check:
1. Session logging - verbose logs add latency
2. SQLite contention - use PRAGMA busy_timeout
3. Network latency - stateful auth is local, shouldn't affect this

### Inconsistent Results

Run with more iterations:
```bash
./perftest -iterations 1000
```

This provides better statistical significance.

### Memory Usage

Monitor memory with:
```bash
go test -bench=. -benchmem ./pkg/performance
```

## Related Documentation

- [Session State Management](../pkg/sessionstate/README.md)
- [SQLite Store](../pkg/sessionstate/SQLITE_STORE.md)
- [Testing Guide](./testing.md)
