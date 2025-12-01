package performance

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/github/github-mcp-server/pkg/sessionstate"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// InstrumentedToolHandlerWrapper wraps a tool handler with performance measurement and optional stateful auth.
// It records the latency of each tool call for performance analysis.
func InstrumentedToolHandlerWrapper(
	session *sessionstate.Session,
	originalHandler server.ToolHandlerFunc,
	metrics *Metrics,
) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		startTime := time.Now()
		isStatefulAuth := session != nil

		// If session is nil, stateful auth is disabled - skip validation
		if session == nil {
			result, err := originalHandler(ctx, request)
			duration := time.Since(startTime)

			if metrics != nil {
				metrics.RecordToolCall(duration, isStatefulAuth)
			}

			fmt.Fprintf(os.Stderr, "[PERF] Tool=%s Duration=%v StatefulAuth=false\n",
				request.Params.Name, duration)

			return result, err
		}

		// Extract repository context from tool arguments
		args, ok := request.Params.Arguments.(map[string]interface{})
		if !ok {
			// If we can't parse arguments, let the original handler deal with it
			result, err := originalHandler(ctx, request)
			duration := time.Since(startTime)

			if metrics != nil {
				metrics.RecordToolCall(duration, isStatefulAuth)
			}

			fmt.Fprintf(os.Stderr, "[PERF] Tool=%s Duration=%v StatefulAuth=true (skipped)\n",
				request.Params.Name, duration)

			return result, err
		}

		repoContext := sessionstate.ExtractRepositoryFromArgs(args)

		// Validate and lock the session
		authStartTime := time.Now()
		if err := session.ValidateAndLock(repoContext); err != nil {
			authDuration := time.Since(authStartTime)
			totalDuration := time.Since(startTime)

			if metrics != nil {
				metrics.RecordToolCall(totalDuration, isStatefulAuth)
			}

			fmt.Fprintf(os.Stderr, "[PERF] Tool=%s TotalDuration=%v AuthDuration=%v StatefulAuth=true Result=policy_violation\n",
				request.Params.Name, totalDuration, authDuration)

			// Return an error response to the client
			return mcp.NewToolResultErrorFromErr("session policy violation", err), nil
		}
		authDuration := time.Since(authStartTime)

		// Policy check passed, call the original handler
		result, err := originalHandler(ctx, request)
		totalDuration := time.Since(startTime)
		handlerDuration := totalDuration - authDuration

		if metrics != nil {
			metrics.RecordToolCall(totalDuration, isStatefulAuth)
		}

		fmt.Fprintf(os.Stderr, "[PERF] Tool=%s TotalDuration=%v AuthDuration=%v HandlerDuration=%v StatefulAuth=true Result=success\n",
			request.Params.Name, totalDuration, authDuration, handlerDuration)

		return result, err
	}
}

// AddInstrumentedTool adds a tool to the server with performance measurement and optional stateful authorization.
func AddInstrumentedTool(
	mcpServer *server.MCPServer,
	session *sessionstate.Session,
	tool mcp.Tool,
	handler server.ToolHandlerFunc,
	metrics *Metrics,
) {
	wrappedHandler := InstrumentedToolHandlerWrapper(session, handler, metrics)
	mcpServer.AddTool(tool, wrappedHandler)
}
