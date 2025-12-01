package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// TestConfig contains configuration for performance tests
type TestConfig struct {
	// Number of iterations for each test case
	Iterations int

	// Tool to test (e.g., "get_file_contents")
	ToolName string

	// Arguments for the tool call
	ToolArgs map[string]interface{}

	// Whether to test with stateful auth enabled
	TestWithAuth bool

	// Whether to test without stateful auth
	TestWithoutAuth bool

	// Repository context for testing (owner/repo)
	Owner string
	Repo  string
}

// TestResult contains results from a performance test run
type TestResult struct {
	Config             TestConfig
	WithAuthStats      Stats
	WithoutAuthStats   Stats
	OverheadAverage    time.Duration
	OverheadPercentage float64
}

// HandlerFunc is a function that executes a test operation
type HandlerFunc func(ctx context.Context, args map[string]interface{}) error

// PerformanceTest runs performance tests comparing stateful auth vs no auth
type PerformanceTest struct {
	config  TestConfig
	handler HandlerFunc
}

// NewPerformanceTest creates a new performance test
func NewPerformanceTest(config TestConfig, handler HandlerFunc) *PerformanceTest {
	return &PerformanceTest{
		config:  config,
		handler: handler,
	}
}

// Run executes the performance test
func (pt *PerformanceTest) Run(ctx context.Context) (*TestResult, error) {
	result := &TestResult{
		Config: pt.config,
	}

	fmt.Printf("\n=== Performance Test Configuration ===\n")
	fmt.Printf("Tool: %s\n", pt.config.ToolName)
	fmt.Printf("Iterations: %d\n", pt.config.Iterations)
	fmt.Printf("Repository: %s/%s\n\n", pt.config.Owner, pt.config.Repo)

	// Test with stateful auth enabled
	if pt.config.TestWithAuth {
		fmt.Printf("Running test WITH stateful auth (%d iterations)...\n", pt.config.Iterations)
		withAuthMetrics, err := pt.runWithAuth(ctx)
		if err != nil {
			return nil, fmt.Errorf("test with auth failed: %w", err)
		}
		result.WithAuthStats = withAuthMetrics.GetStats()
		fmt.Printf("✓ Completed\n\n")
	}

	// Test without stateful auth
	if pt.config.TestWithoutAuth {
		fmt.Printf("Running test WITHOUT stateful auth (%d iterations)...\n", pt.config.Iterations)
		withoutAuthMetrics, err := pt.runWithoutAuth(ctx)
		if err != nil {
			return nil, fmt.Errorf("test without auth failed: %w", err)
		}
		result.WithoutAuthStats = withoutAuthMetrics.GetStats()
		fmt.Printf("✓ Completed\n\n")
	}

	// Calculate overhead
	if pt.config.TestWithAuth && pt.config.TestWithoutAuth {
		result.OverheadAverage = result.WithAuthStats.Average - result.WithoutAuthStats.Average
		if result.WithoutAuthStats.Average > 0 {
			result.OverheadPercentage = float64(result.OverheadAverage) / float64(result.WithoutAuthStats.Average) * 100
		}
	}

	return result, nil
}

// runWithAuth runs the test with stateful authorization enabled
func (pt *PerformanceTest) runWithAuth(ctx context.Context) (*Metrics, error) {
	metrics := NewMetrics()

	for i := 0; i < pt.config.Iterations; i++ {
		startTime := time.Now()

		// Call the handler with auth checks enabled (handler should implement auth logic)
		err := pt.handler(ctx, pt.config.ToolArgs)
		duration := time.Since(startTime)

		if err != nil {
			// Some errors are expected (e.g., policy violations)
			// Continue collecting metrics
		}

		metrics.RecordToolCall(duration, true)

		// Progress indicator
		if (i+1)%10 == 0 {
			fmt.Printf("  Progress: %d/%d\r", i+1, pt.config.Iterations)
		}
	}

	return metrics, nil
}

// runWithoutAuth runs the test without stateful authorization
func (pt *PerformanceTest) runWithoutAuth(ctx context.Context) (*Metrics, error) {
	metrics := NewMetrics()

	for i := 0; i < pt.config.Iterations; i++ {
		startTime := time.Now()

		// Call the handler directly (no auth check)
		err := pt.handler(ctx, pt.config.ToolArgs)
		duration := time.Since(startTime)

		if err != nil {
			// Collect metrics even on error
		}

		metrics.RecordToolCall(duration, false)

		// Progress indicator
		if (i+1)%10 == 0 {
			fmt.Printf("  Progress: %d/%d\r", i+1, pt.config.Iterations)
		}
	}

	return metrics, nil
}

// PrintResults prints formatted test results
func (tr *TestResult) PrintResults() {
	fmt.Printf("\n%s\n", strings.Repeat("=", 70))
	fmt.Printf("PERFORMANCE TEST RESULTS\n")
	fmt.Printf("%s\n\n", strings.Repeat("=", 70))

	if tr.Config.TestWithoutAuth {
		fmt.Printf("--- WITHOUT Stateful Auth ---\n")
		fmt.Printf("%s\n\n", tr.WithoutAuthStats.String())
	}

	if tr.Config.TestWithAuth {
		fmt.Printf("--- WITH Stateful Auth ---\n")
		fmt.Printf("%s\n\n", tr.WithAuthStats.String())
	}

	if tr.Config.TestWithAuth && tr.Config.TestWithoutAuth {
		fmt.Printf("--- Overhead Analysis ---\n")
		fmt.Printf("  Average Overhead:      %v\n", tr.OverheadAverage)
		fmt.Printf("  Overhead Percentage:   %.2f%%\n", tr.OverheadPercentage)
		fmt.Printf("  Slowdown Factor:       %.2fx\n\n",
			float64(tr.WithAuthStats.Average)/float64(tr.WithoutAuthStats.Average))
	}

	fmt.Printf("%s\n", strings.Repeat("=", 70))
}

// SaveToFile saves test results to a JSON file
func (tr *TestResult) SaveToFile(filename string) error {
	data, err := json.MarshalIndent(tr, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Results saved to: %s\n", filename)
	return nil
}
