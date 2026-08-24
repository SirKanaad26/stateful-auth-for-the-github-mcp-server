FROM golang:1.27.0-alpine AS build
ARG VERSION="dev"

# Set the working directory
WORKDIR /build

# Install git
RUN --mount=type=cache,target=/var/cache/apk \
    apk add git

# Build the server
# go build automatically download required module dependencies to /go/pkg/mod
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION} -X main.commit=$(git rev-parse HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /bin/github-mcp-server cmd/github-mcp-server/main.go

# Build WASM module
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -o /bin/sessionstate.wasm cmd/wasm/sessionstate/main.go

# Copy WASM runtime helper
RUN cp $(go env GOROOT)/lib/wasm/wasm_exec.js /bin/

# Make a stage to run the app
FROM gcr.io/distroless/base-debian12

# Add required MCP server annotation
LABEL io.modelcontextprotocol.server.name="io.github.github/github-mcp-server"

# Set the working directory
WORKDIR /server
# Copy the binary from the build stage
COPY --from=build /bin/github-mcp-server .
# Copy WASM module and runtime helper
COPY --from=build /bin/sessionstate.wasm .
COPY --from=build /bin/wasm_exec.js .
# Set the entrypoint to the server binary
ENTRYPOINT ["/server/github-mcp-server"]
# Default arguments for ENTRYPOINT
CMD ["stdio"]