# Installation Guide - Stateful Auth for GitHub MCP Server

## Prerequisites
- Docker Desktop installed and running
- GitHub Personal Access Token ([create one here](https://github.com/settings/personal-access-tokens/new) with `repo`, `read:org`, `read:user` scopes)

## Installation

### 1. Clone the Repository
```bash
git clone https://github.com/SirKanaad26/stateful-auth-for-the-github-mcp-server.git
cd stateful-auth-for-the-github-mcp-server
```

### 2. Build the WASM Module and Binary

**Build WASM module:**
```bash
./script/build-wasm
```

**Build Go binary (optional - for local testing):**
```bash
go build -o github-mcp-server ./cmd/github-mcp-server
```

**Build Docker image:**
```bash
docker build -t github-mcp-server:wasm .
```

### 3. Configure Claude Desktop

**Edit config file:**
- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`

**Add this configuration:**
```json
{
  "mcpServers": {
    "github": {
      "command": "docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "-e",
        "GITHUB_PERSONAL_ACCESS_TOKEN",
        "github-mcp-server:wasm"
      ],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "YOUR_GITHUB_TOKEN_HERE"
      }
    }
  }
}
```

Replace `YOUR_GITHUB_TOKEN_HERE` with your actual token.

### 4. Restart Claude Desktop
- Completely quit Claude Desktop (`Cmd+Q` on macOS)
- Reopen Claude Desktop

## Verify Installation

Ask Claude: "List my GitHub repositories"

## What's Included
- ✅ WASM-based session isolation (403-byte policy module)
- ✅ SQLite persistent session storage
- ✅ Cross-repository attack prevention
- ✅ Docker containerized deployment

## Troubleshooting

**View logs:**
```bash
# macOS
cat ~/Library/Logs/Claude/mcp-server-github.log

# Windows
type %APPDATA%\Claude\logs\mcp-server-github.log
```

**Test Docker container:**
```bash
docker run -i --rm -e GITHUB_PERSONAL_ACCESS_TOKEN=your_token github-mcp-server:wasm
```

## Local Development & Testing

**Run all tests:**
```bash
go test ./...
```

**Run tests for specific package:**
```bash
go test ./pkg/sessionstate/
```

**Run tests with verbose output:**
```bash
go test -v ./pkg/sessionstate/
```

**Run a specific test:**
```bash
go test -v -run TestWASMSessionIsolation ./pkg/sessionstate/
```

**Run the server locally (without Docker):**
```bash
export GITHUB_PERSONAL_ACCESS_TOKEN=your_token
./github-mcp-server stdio
```

**Run with WASM session state:**
```bash
export GITHUB_PERSONAL_ACCESS_TOKEN=your_token
./github-mcp-server stdio --wasm-sessionstate-path ./wasm/sessionstate/sessionstate.wasm
```