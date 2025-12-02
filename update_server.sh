#!/bin/bash

# Add WAsmSessionStatePath to StdioServerConfig
sed -i '' 's/EnableStatefulAuth   bool$/EnableStatefulAuth   bool\n\tWAsmSessionStatePath string/' internal/ghmcp/server.go

# Add WAsmSessionStatePath to MCPServerConfig
sed -i '' 's/EnableStatefulAuth bool$/EnableStatefulAuth bool\n\tWAsmSessionStatePath string/' internal/ghmcp/server.go

# Add WAsmSessionStatePath in RunStdioServer MCPServerConfig initialization
sed -i '' 's/RepoAccessTTL:        cfg.RepoAccessCacheTTL,/RepoAccessTTL:        cfg.RepoAccessCacheTTL,\n\t\t\tWAsmSessionStatePath: cfg.WAsmSessionStatePath,/' internal/ghmcp/server.go

# Update session creation to use WASM
sed -i '' 's/session = sessionstate.NewSession()/if cfg.WAsmSessionStatePath != "" {\n\t\t\tsession = sessionstate.NewSessionWithWASM(context.Background(), cfg.WAsmSessionStatePath)\n\t\t} else {\n\t\t\tsession = sessionstate.NewSession()\n\t\t}/' internal/ghmcp/server.go

echo "Updated internal/ghmcp/server.go with WASM support"
