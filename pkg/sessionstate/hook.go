package sessionstate

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// ToolCallValidator creates a hook function that validates tool calls against session policy.
// It should be used as a BeforeCallTool hook in the MCP server.
func ToolCallValidator(session *Session) func(ctx context.Context, request *mcp.CallToolRequest) error {
	return func(_ context.Context, request *mcp.CallToolRequest) error {
		// Type assert the arguments to map[string]interface{}
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			// If arguments are not in expected format, skip validation
			return nil
		}

		// Extract repository context from tool arguments
		repoContext := ExtractRepositoryFromArgs(args)

		// Validate and lock the session
		if err := session.ValidateAndLock(repoContext); err != nil {
			return err
		}

		return nil
	}
}
