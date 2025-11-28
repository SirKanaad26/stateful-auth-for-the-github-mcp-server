package sessionstate

import (
	"testing"
)

// TestAttackScenario simulates the cross-repository data exfiltration attack
// described in the stateful auth paper.
func TestAttackScenario(t *testing.T) {
	// Create a new session
	session := NewSession()

	// Step 1: Agent lists issues in public-repo (legitimate)
	publicRepoArgs := map[string]interface{}{
		"owner": "octocat",
		"repo":  "public-repo",
	}
	publicRepoContext := ExtractRepositoryFromArgs(publicRepoArgs)
	err := session.ValidateAndLock(publicRepoContext)
	if err != nil {
		t.Fatalf("first tool call to public-repo should succeed, got: %v", err)
	}

	t.Log("✓ Step 1: Agent listed issues in public-repo (session locked)")

	// Step 2: Attacker's malicious prompt tries to access private-repo
	// This should be BLOCKED by the policy
	privateRepoArgs := map[string]interface{}{
		"owner": "octocat",
		"repo":  "private-repo",
	}
	privateRepoContext := ExtractRepositoryFromArgs(privateRepoArgs)
	err = session.ValidateAndLock(privateRepoContext)
	if err == nil {
		t.Error("second tool call to different repo should be blocked")
	}

	policyErr, ok := err.(*PolicyViolationError)
	if !ok {
		t.Fatalf("error should be PolicyViolationError, got %T", err)
	}

	t.Logf("✓ Step 2: Cross-repo access BLOCKED - %s", policyErr.Error())

	// Verify the error contains the correct details
	if policyErr.LockedOwner != "octocat" || policyErr.LockedRepo != "public-repo" {
		t.Errorf("error should report locked repo as octocat/public-repo, got %s/%s",
			policyErr.LockedOwner, policyErr.LockedRepo)
	}

	if policyErr.RequestedOwner != "octocat" || policyErr.RequestedRepo != "private-repo" {
		t.Errorf("error should report requested repo as octocat/private-repo, got %s/%s",
			policyErr.RequestedOwner, policyErr.RequestedRepo)
	}

	t.Log("✓ Policy enforcement working correctly")
}

// TestMultipleSessions verifies that different sessions have independent locks
func TestMultipleSessions(t *testing.T) {
	// Session 1: locked to repo-a
	session1 := NewSession()
	repoA := &RepositoryContext{Owner: "user1", Repo: "repo-a"}
	err := session1.ValidateAndLock(repoA)
	if err != nil {
		t.Fatalf("session1 lock to repo-a failed: %v", err)
	}

	// Session 2: locked to repo-b
	session2 := NewSession()
	repoB := &RepositoryContext{Owner: "user2", Repo: "repo-b"}
	err = session2.ValidateAndLock(repoB)
	if err != nil {
		t.Fatalf("session2 lock to repo-b failed: %v", err)
	}

	// Session 1 can still access repo-a
	err = session1.ValidateAndLock(repoA)
	if err != nil {
		t.Errorf("session1 should still be able to access repo-a, got: %v", err)
	}

	// Session 2 can still access repo-b
	err = session2.ValidateAndLock(repoB)
	if err != nil {
		t.Errorf("session2 should still be able to access repo-b, got: %v", err)
	}

	// Session 1 cannot access repo-b
	err = session1.ValidateAndLock(repoB)
	if err == nil {
		t.Error("session1 should not be able to access repo-b")
	}

	// Session 2 cannot access repo-a
	err = session2.ValidateAndLock(repoA)
	if err == nil {
		t.Error("session2 should not be able to access repo-a")
	}

	t.Log("✓ Multiple sessions maintain independent locks")
}

// TestToolsWithoutRepositoryContext verifies that tools without repo context
// (like get_me, get_notifications) don't affect the lock
func TestToolsWithoutRepositoryContext(t *testing.T) {
	session := NewSession()

	// Tool without repository context (e.g., get_me)
	noRepoArgs := map[string]interface{}{
		"user": "octocat",
	}
	noRepoContext := ExtractRepositoryFromArgs(noRepoArgs)
	if noRepoContext != nil {
		t.Error("tool without repo context should return nil")
	}

	// ValidateAndLock with nil context should not lock the session
	err := session.ValidateAndLock(noRepoContext)
	if err != nil {
		t.Errorf("nil context should not error, got: %v", err)
	}

	if session.IsRepositoryLocked() {
		t.Error("session should not be locked after nil context")
	}

	// Now lock to a specific repo
	repoArgs := map[string]interface{}{
		"owner": "octocat",
		"repo":  "repo-a",
	}
	repoContext := ExtractRepositoryFromArgs(repoArgs)
	err = session.ValidateAndLock(repoContext)
	if err != nil {
		t.Fatalf("lock failed: %v", err)
	}

	// Tools without repository context should still work while locked
	err = session.ValidateAndLock(nil)
	if err != nil {
		t.Errorf("nil context should work while locked, got: %v", err)
	}

	t.Log("✓ Tools without repo context don't affect session lock")
}

// TestConcurrentAccess verifies thread safety of the session
func TestConcurrentAccess(t *testing.T) {
	session := NewSession()
	session.LockRepository("octocat", "public-repo")

	// Channel to collect errors from goroutines
	errChan := make(chan error, 10)

	// Launch multiple goroutines trying to validate different repos
	for i := 0; i < 5; i++ {
		go func(idx int) {
			repoContext := &RepositoryContext{
				Owner: "octocat",
				Repo:  "public-repo",
			}
			err := session.ValidateAndLock(repoContext)
			errChan <- err
		}(i)
	}

	// Launch goroutines trying to access a different repo
	for i := 0; i < 5; i++ {
		go func(idx int) {
			repoContext := &RepositoryContext{
				Owner: "octocat",
				Repo:  "different-repo",
			}
			err := session.ValidateAndLock(repoContext)
			errChan <- err
		}(i)
	}

	// Collect results
	successCount := 0
	violationCount := 0
	for i := 0; i < 10; i++ {
		err := <-errChan
		if err == nil {
			successCount++
		} else {
			_, isPolicyErr := err.(*PolicyViolationError)
			if isPolicyErr {
				violationCount++
			}
		}
	}

	if successCount != 5 {
		t.Errorf("expected 5 successful accesses to same repo, got %d", successCount)
	}

	if violationCount != 5 {
		t.Errorf("expected 5 policy violations for different repo, got %d", violationCount)
	}

	t.Log("✓ Concurrent access is thread-safe")
}
