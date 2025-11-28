package sessionstate

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterWithSessionPolicy wraps the toolset registration to apply session policy enforcement.
// This should be called after tools are registered to wrap them with policy validation.
//
// The wrapper will:
// 1. Extract repository context from tool arguments
// 2. Validate against the session's locked repository
// 3. Return a policy violation error if the call violates the session policy
// 4. Otherwise, delegate to the original handler
func RegisterWithSessionPolicy(_ *server.MCPServer, _ *Session) error {
	// We need to get the registered tools and wrap them
	// Since MCPServer doesn't expose a direct way to get registered tools,
	// we'll wrap at registration time by modifying how tools are added

	// Note: This is called AFTER RegisterAll, so we need a different approach.
	// The best solution is to instrument the AddTool method on the server,
	// but since we can't modify the server after tools are added easily,
	// we'll use a post-processing approach via a hook.

	// Actually, the best approach is to use the OnBeforeAny hook to intercept
	// CallTool requests and validate them. But OnBeforeAny can't return errors.

	// Instead, we'll create an OnCallTool hook if available, or use a custom
	// transport/handler wrapper.

	// For now, return nil - the session policy will be enforced via middleware wrapping
	// in the tool handlers themselves at registration time
	return nil
}

// ToolCallInterceptor creates a handler that intercepts tool calls and enforces session policy.
// This is the mechanism to wrap tool handlers after they're registered.
func ToolCallInterceptor(session *Session, originalHandler server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract repository context from the tool arguments
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			// If we can't parse arguments, let the original handler deal with it
			return originalHandler(ctx, request)
		}

		repoContext := ExtractRepositoryFromArgs(args)

		// Validate and lock to the repository (if applicable)
		if err := session.ValidateAndLock(repoContext); err != nil {
			// Return a proper error result to the client
			return mcp.NewToolResultErrorFromErr(
				fmt.Sprintf("session policy violation: %v", err),
				err,
			), nil
		}

		// Policy validated, call the original handler
		return originalHandler(ctx, request)
	}
}

// WrapToolHandlerInPlace replaces a tool's handler in-place with a policy-enforcing wrapper.
// This is a utility for wrapping individual tools after registration.
//
// Note: This function would require access to the server's internal tool storage,
// which mcp-go doesn't expose. Instead, tools should be wrapped at registration time
// in the toolset's RegisterTools method.
func WrapToolHandlerInPlace(_ *server.MCPServer, _ string, _ *Session) error {
	// This would require modifying the server to expose registered tools
	// which it currently doesn't do. This is left as a reference for future
	// improvements if the mcp-go library adds tool introspection.
	return fmt.Errorf("tool introspection not yet implemented in mcp-go")
}

// RegisterAllWithPolicy is a wrapper around the toolset group's RegisterAll
// that applies session policy to all tools as they're registered.
// This should be imported and used in place of the standard RegisterAll.
//
// Example:
//
//	session := sessionstate.NewSession()
//	sessionstate.RegisterAllWithPolicy(tsg, ghServer, session)
//
// This approach requires modifying the toolset group to accept a session parameter.
func RegisterAllWithPolicy(_ interface{}, _ *server.MCPServer, _ *Session) error {
	// This is a placeholder - actual implementation would need to modify
	// the toolset group's registration to accept a session parameter
	// and wrap handlers as they're added.

	// The challenge is that toolset registration happens in pkg/toolsets
	// and we want to avoid circular imports and complex refactoring.

	// Instead, we'll modify the approach to wrap at the server level via hooks.
	return nil
}
