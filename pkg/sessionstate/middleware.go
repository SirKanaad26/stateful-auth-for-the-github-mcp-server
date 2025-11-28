package sessionstate

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ToolHandlerWrapper creates a wrapper around a tool handler that enforces session policy.
// If the tool call violates the policy, it returns an error to the client.
func ToolHandlerWrapper(session *Session, originalHandler server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract repository context from tool arguments
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			// If we can't parse arguments, let the original handler deal with it
			return originalHandler(ctx, request)
		}

		repoContext := ExtractRepositoryFromArgs(args)

		// Validate and lock the session
		if err := session.ValidateAndLock(repoContext); err != nil {
			// Return an error response to the client
			return mcp.NewToolResultErrorFromErr("session policy violation", err), nil
		}

		// Policy check passed, call the original handler
		return originalHandler(ctx, request)
	}
}

// WrapToolHandlers wraps a tool handler with policy enforcement.
func WrapToolHandlers(session *Session, handler server.ToolHandlerFunc) server.ToolHandlerFunc {
	return ToolHandlerWrapper(session, handler)
}

// AddWrappedTool adds a tool to the server with stateful authorization enforcement.
func AddWrappedTool(mcpServer *server.MCPServer, session *Session, tool mcp.Tool, handler server.ToolHandlerFunc) {
	wrappedHandler := WrapToolHandlers(session, handler)
	mcpServer.AddTool(tool, wrappedHandler)
}
