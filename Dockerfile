FROM golang:1.25.4-alpine AS build
ARG VERSION="dev"

# Set the working directory
WORKDIR /build

# Install git
RUN --mount=type=cache,target=/var/cache/apk \
    apk add git

# Build WASM module for session state validation
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    GOOS=js GOARCH=wasm CGO_ENABLED=0 go build \
    -o /tmp/sessionstate.wasm \
    cmd/wasm/sessionstate/main.go

# Copy wasm_exec.js runtime helper
RUN --mount=type=bind,target=. \
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" /tmp/wasm_exec.js || \
    cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" /tmp/wasm_exec.js

# Build the server
# go build automatically download required module dependencies to /go/pkg/mod
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION} -X main.commit=$(git rev-parse HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /bin/github-mcp-server cmd/github-mcp-server/main.go

# Make a stage to run the app
FROM gcr.io/distroless/base-debian12

# Add required MCP server annotation
LABEL io.modelcontextprotocol.server.name="io.github.github/github-mcp-server"

# Set the working directory
WORKDIR /server

# Copy the binary from the build stage
COPY --from=build /bin/github-mcp-server .

# Copy WASM module
COPY --from=build /tmp/sessionstate.wasm ./wasm/
COPY --from=build /tmp/wasm_exec.js ./wasm/

# Set the entrypoint to the server binary
ENTRYPOINT ["/server/github-mcp-server"]

# Default arguments for ENTRYPOINT
# To use WASM, override with: ["stdio", "--wasm-sessionstate-path", "/server/wasm/sessionstate.wasm"]
CMD ["stdio"]
