package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/github/github-mcp-server/pkg/performance"
	"github.com/github/github-mcp-server/pkg/sessionstate"
)

var (
	iterations  = flag.Int("iterations", 100, "Number of test iterations")
	owner       = flag.String("owner", "github", "Repository owner")
	repo        = flag.String("repo", "github-mcp-server", "Repository name")
	withAuth    = flag.Bool("with-auth", true, "Run tests with stateful auth")
	withoutAuth = flag.Bool("without-auth", true, "Run tests without stateful auth")
	output      = flag.String("output", "", "Output file for JSON results (optional)")
)

func main() {
	flag.Parse()

	fmt.Println("GitHub MCP Server - Performance Test")
	fmt.Println("=====================================\n")

	// Create a mock tool handler for testing
	// In real usage, this would be your actual MCP tool handler
	mockHandler := createMockHandler()

	// Configure the test
	config := performance.TestConfig{
		Iterations:      *iterations,
		ToolName:        "get_file_contents",
		ToolArgs:        map[string]interface{}{"path": "README.md"},
		TestWithAuth:    *withAuth,
		TestWithoutAuth: *withoutAuth,
		Owner:           *owner,
		Repo:            *repo,
	}

	// Create and run the test
	test := performance.NewPerformanceTest(config, mockHandler)

	fmt.Printf("Configuration:\n")
	fmt.Printf("  Iterations: %d\n", *iterations)
	fmt.Printf("  Repository: %s/%s\n", *owner, *repo)
	fmt.Printf("  Test with auth: %v\n", *withAuth)
	fmt.Printf("  Test without auth: %v\n\n", *withoutAuth)

	result, err := test.Run(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running test: %v\n", err)
		os.Exit(1)
	}

	// Print results
	result.PrintResults()

	// Save to file if requested
	if *output != "" {
		if err := result.SaveToFile(*output); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving results: %v\n", err)
			os.Exit(1)
		}
	}
}

// createMockHandler creates a mock tool handler for testing
// This simulates a real MCP tool with optional stateful auth checking
func createMockHandler() performance.HandlerFunc {
	session := sessionstate.NewSession()

	return func(ctx context.Context, args map[string]interface{}) error {
		// Simulate some work (e.g., API call)
		time.Sleep(time.Millisecond * 5)

		// Check if we should validate auth
		// In real usage, this would be handled by middleware
		// Here we demonstrate the overhead of session checking
		repoContext := &sessionstate.RepositoryContext{
			Owner: "github",
			Repo:  "github-mcp-server",
		}

		// This is where the stateful auth overhead occurs
		if err := session.ValidateAndLock(repoContext); err != nil {
			// For testing, we'll reset session on policy violation
			// In production, this would return an error
			session = sessionstate.NewSession()
			_ = session.ValidateAndLock(repoContext)
		}

		// Simulate additional work
		time.Sleep(time.Millisecond * 5)

		return nil
	}
}
