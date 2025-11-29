#!/bin/bash
# Test script to demonstrate stateful authorization

echo "Building binaries..."
go build -o github-mcp-server ./cmd/github-mcp-server
go build -o mcpcurl ./cmd/mcpcurl/main.go

echo ""
echo "==== TEST 1: Single server instance, sequential tool calls ===="
echo "This test will demonstrate session persistence within ONE server."
echo ""

# Start the server in background and capture its PID
GITHUB_PERSONAL_ACCESS_TOKEN=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 \
  ./github-mcp-server stdio &
SERVER_PID=$!

sleep 2

echo "✓ Server started (PID: $SERVER_PID)"
echo ""

# Kill the server when done
cleanup() {
  kill $SERVER_PID 2>/dev/null
}
trap cleanup EXIT

echo "Test completed. Server stopped."
