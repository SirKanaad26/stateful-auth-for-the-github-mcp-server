package sessionstate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// getWASMPath returns the path to the WASM module for testing
func getWASMPath(t *testing.T) string {
	// Get the project root (assuming we're in pkg/sessionstate)
	cwd, err := os.Getwd()
	if err != nil {
		t.Skipf("Skipping WASM test: cannot get working directory: %v", err)
	}

	// Navigate up to project root
	projectRoot := filepath.Join(cwd, "..", "..")
	wasmPath := filepath.Join(projectRoot, "wasm", "sessionstate", "sessionstate.wasm")

	// Check if WASM file exists
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		t.Skipf("Skipping WASM test: WASM module not found at %s (run 'bash script/build-wasm' to build)", wasmPath)
	}

	return wasmPath
}

// TestWASMSessionIsolation verifies that different WASM sessions have completely isolated state
// This is the critical test to ensure Client A cannot lock out Client B
func TestWASMSessionIsolation(t *testing.T) {
	wasmPath := getWASMPath(t)
	ctx := context.Background()

	// Create two separate sessions with WASM
	session1 := NewSessionWithWASM(ctx, wasmPath)
	defer session1.Close()

	session2 := NewSessionWithWASM(ctx, wasmPath)
	defer session2.Close()

	// Verify both sessions have WASM enabled
	if !session1.useWASM {
		t.Fatal("session1 should have WASM enabled")
	}
	if !session2.useWASM {
		t.Fatal("session2 should have WASM enabled")
	}

	// Session 1: lock to repo-a
	repoA := &RepositoryContext{Owner: "user1", Repo: "repo-a"}
	err := session1.ValidateAndLock(repoA)
	if err != nil {
		t.Fatalf("session1 lock to repo-a failed: %v", err)
	}
	t.Log("✓ Session 1 locked to user1/repo-a")

	// Session 2: lock to repo-b (should succeed independently)
	repoB := &RepositoryContext{Owner: "user2", Repo: "repo-b"}
	err = session2.ValidateAndLock(repoB)
	if err != nil {
		t.Fatalf("session2 lock to repo-b failed: %v", err)
	}
	t.Log("✓ Session 2 locked to user2/repo-b")

	// Session 1 can still access repo-a (should not be affected by session2)
	err = session1.ValidateAndLock(repoA)
	if err != nil {
		t.Errorf("session1 should still be able to access repo-a, got: %v", err)
	}
	t.Log("✓ Session 1 can still access repo-a")

	// Session 2 can still access repo-b (should not be affected by session1)
	err = session2.ValidateAndLock(repoB)
	if err != nil {
		t.Errorf("session2 should still be able to access repo-b, got: %v", err)
	}
	t.Log("✓ Session 2 can still access repo-b")

	// Session 1 CANNOT access repo-b (locked to repo-a)
	err = session1.ValidateAndLock(repoB)
	if err == nil {
		t.Error("session1 should not be able to access repo-b (locked to repo-a)")
	} else {
		t.Logf("✓ Session 1 correctly blocked from accessing repo-b: %v", err)
	}

	// Session 2 CANNOT access repo-a (locked to repo-b)
	err = session2.ValidateAndLock(repoA)
	if err == nil {
		t.Error("session2 should not be able to access repo-a (locked to repo-b)")
	} else {
		t.Logf("✓ Session 2 correctly blocked from accessing repo-a: %v", err)
	}

	t.Log("✓ WASM sessions maintain independent locks - session isolation verified!")
}

// TestWASMBasicLocking verifies basic WASM locking functionality
func TestWASMBasicLocking(t *testing.T) {
	wasmPath := getWASMPath(t)
	ctx := context.Background()

	session := NewSessionWithWASM(ctx, wasmPath)
	defer session.Close()

	if !session.useWASM {
		t.Fatal("session should have WASM enabled")
	}

	// First access locks to repo-a
	repoA := &RepositoryContext{Owner: "octocat", Repo: "repo-a"}
	err := session.ValidateAndLock(repoA)
	if err != nil {
		t.Fatalf("first lock should succeed: %v", err)
	}
	t.Log("✓ WASM session locked to octocat/repo-a")

	// Second access to same repo should succeed
	err = session.ValidateAndLock(repoA)
	if err != nil {
		t.Errorf("accessing same repo should succeed: %v", err)
	}
	t.Log("✓ Accessing same repo succeeded")

	// Access to different repo should fail
	repoB := &RepositoryContext{Owner: "octocat", Repo: "repo-b"}
	err = session.ValidateAndLock(repoB)
	if err == nil {
		t.Error("accessing different repo should fail")
	} else {
		t.Logf("✓ Cross-repo access blocked: %v", err)
	}

	// Session should be unlocked after rejection
	if session.IsRepositoryLocked() {
		t.Error("session should be unlocked after rejection")
	}
	t.Log("✓ Session unlocked after rejection")

	// Now repo-b should be accessible
	err = session.ValidateAndLock(repoB)
	if err != nil {
		t.Fatalf("accessing repo-b after unlock should succeed: %v", err)
	}
	t.Log("✓ Legitimate access to repo-b succeeded after unlock")
}

// TestWASMAttackScenario simulates the cross-repository attack with WASM enforcement
func TestWASMAttackScenario(t *testing.T) {
	wasmPath := getWASMPath(t)
	ctx := context.Background()

	session := NewSessionWithWASM(ctx, wasmPath)
	defer session.Close()

	if !session.useWASM {
		t.Fatal("session should have WASM enabled")
	}

	// Step 1: Agent lists issues in public-repo (legitimate)
	publicRepo := &RepositoryContext{Owner: "octocat", Repo: "public-repo"}
	err := session.ValidateAndLock(publicRepo)
	if err != nil {
		t.Fatalf("first access to public-repo should succeed: %v", err)
	}
	t.Log("✓ Step 1: WASM session locked to public-repo")

	// Step 2: Attacker's malicious prompt tries to access private-repo
	privateRepo := &RepositoryContext{Owner: "octocat", Repo: "private-repo"}
	err = session.ValidateAndLock(privateRepo)
	if err == nil {
		t.Fatal("WASM should block cross-repo access")
	}

	policyErr, ok := err.(*PolicyViolationError)
	if !ok {
		t.Fatalf("error should be PolicyViolationError, got %T", err)
	}
	t.Logf("✓ Step 2: WASM blocked cross-repo access - %s", policyErr.Message)

	// Step 3: Verify session is unlocked after rejection
	if session.IsRepositoryLocked() {
		t.Error("session should be unlocked after WASM rejection")
	}
	t.Log("✓ Step 3: Session unlocked after WASM rejection")

	// Step 4: Legitimate access to private-repo should now work
	err = session.ValidateAndLock(privateRepo)
	if err != nil {
		t.Fatalf("legitimate access to private-repo should succeed: %v", err)
	}
	t.Log("✓ Step 4: Legitimate access to private-repo succeeded via WASM")
}

// TestWASMFallbackToNative verifies that invalid WASM path falls back to native
func TestWASMFallbackToNative(t *testing.T) {
	ctx := context.Background()

	// Create session with invalid WASM path
	session := NewSessionWithWASM(ctx, "/nonexistent/path/to/wasm.wasm")
	defer session.Close()

	// Should fall back to native implementation
	if session.useWASM {
		t.Error("session should not have WASM enabled with invalid path")
	}

	// Basic locking should still work with native implementation
	repoA := &RepositoryContext{Owner: "octocat", Repo: "repo-a"}
	err := session.ValidateAndLock(repoA)
	if err != nil {
		t.Fatalf("native fallback should work: %v", err)
	}

	t.Log("✓ WASM fallback to native implementation works")
}

// TestWASMMemoryManagement verifies that WASM bridge properly manages memory
func TestWASMMemoryManagement(t *testing.T) {
	wasmPath := getWASMPath(t)
	ctx := context.Background()

	session := NewSessionWithWASM(ctx, wasmPath)
	defer session.Close()

	if !session.useWASM {
		t.Fatal("session should have WASM enabled")
	}

	// Test with long repository names to verify memory allocation works
	longOwner := "very-long-owner-name-that-tests-memory-allocation-limits"
	longRepo := "very-long-repository-name-that-tests-memory-allocation-and-proper-offset-management"

	repoContext := &RepositoryContext{Owner: longOwner, Repo: longRepo}
	err := session.ValidateAndLock(repoContext)
	if err != nil {
		t.Fatalf("WASM should handle long strings: %v", err)
	}

	t.Log("✓ WASM memory management handles long strings")

	// Access same repo again to verify memory is properly managed across calls
	err = session.ValidateAndLock(repoContext)
	if err != nil {
		t.Fatalf("repeated access should work: %v", err)
	}

	t.Log("✓ WASM memory management works across multiple calls")
}