package sessionstate

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewSession(t *testing.T) {
	session := NewSession()

	if session.IsRepositoryLocked() {
		t.Error("new session should not be locked")
	}

	owner, repo := session.GetLockedRepository()
	if owner != "" || repo != "" {
		t.Errorf("new session should have empty owner/repo, got %s/%s", owner, repo)
	}
}

func TestLockRepository(t *testing.T) {
	session := NewSession()

	session.LockRepository("octocat", "Hello-World")

	if !session.IsRepositoryLocked() {
		t.Error("session should be locked after LockRepository")
	}

	owner, repo := session.GetLockedRepository()
	if owner != "octocat" || repo != "Hello-World" {
		t.Errorf("expected octocat/Hello-World, got %s/%s", owner, repo)
	}
}

func TestMatchesLockedRepository(t *testing.T) {
	session := NewSession()

	// Unlocked session should match any repository
	if !session.MatchesLockedRepository("octocat", "Hello-World") {
		t.Error("unlocked session should match any repository")
	}

	// Lock to a specific repository
	session.LockRepository("octocat", "Hello-World")

	// Should match the locked repository
	if !session.MatchesLockedRepository("octocat", "Hello-World") {
		t.Error("should match locked repository")
	}

	// Should not match a different repository
	if session.MatchesLockedRepository("github", "gitignore") {
		t.Error("should not match different repository")
	}
}

func TestValidateAndLockFirstCall(t *testing.T) {
	session := NewSession()
	repoContext := &RepositoryContext{
		Owner: "octocat",
		Repo:  "Hello-World",
	}

	// First call should lock the session
	err := session.ValidateAndLock(repoContext)
	if err != nil {
		t.Errorf("first call should not error, got: %v", err)
	}

	if !session.IsRepositoryLocked() {
		t.Error("session should be locked after first call")
	}

	owner, repo := session.GetLockedRepository()
	if owner != "octocat" || repo != "Hello-World" {
		t.Errorf("expected octocat/Hello-World, got %s/%s", owner, repo)
	}
}

func TestValidateAndLockSameRepository(t *testing.T) {
	session := NewSession()
	repoContext := &RepositoryContext{
		Owner: "octocat",
		Repo:  "Hello-World",
	}

	// Lock to a repository
	session.LockRepository("octocat", "Hello-World")

	// Subsequent call to same repository should succeed
	err := session.ValidateAndLock(repoContext)
	if err != nil {
		t.Errorf("call to same repository should not error, got: %v", err)
	}
}

func TestValidateAndLockDifferentRepository(t *testing.T) {
	session := NewSession()

	// Lock to first repository
	repoContext1 := &RepositoryContext{
		Owner: "octocat",
		Repo:  "Hello-World",
	}
	err := session.ValidateAndLock(repoContext1)
	require.NoError(t, err)

	// Try to access different repository
	repoContext2 := &RepositoryContext{
		Owner: "github",
		Repo:  "gitignore",
	}
	err = session.ValidateAndLock(repoContext2)

	if err == nil {
		t.Error("call to different repository should error")
	}


	policyErr, ok := err.(*PolicyViolationError)
	if !ok {
		t.Errorf("error should be PolicyViolationError, got %T", err)
	}

	if policyErr.LockedOwner != "octocat" || policyErr.LockedRepo != "Hello-World" {
		t.Errorf("error should report locked repo as octocat/Hello-World, got %s/%s",
			policyErr.LockedOwner, policyErr.LockedRepo)
	}

	if policyErr.RequestedOwner != "github" || policyErr.RequestedRepo != "gitignore" {
		t.Errorf("error should report requested repo as github/gitignore, got %s/%s",
			policyErr.RequestedOwner, policyErr.RequestedRepo)
	}
}

func TestValidateAndLockNilRepository(t *testing.T) {
	session := NewSession()

	// Call with nil repository context should not error
	err := session.ValidateAndLock(nil)
	if err != nil {
		t.Errorf("nil repository context should not error, got: %v", err)
	}

	// Session should still be unlocked
	if session.IsRepositoryLocked() {
		t.Error("session should still be unlocked after nil repository context")
	}
}

func TestEnforcePolicy(t *testing.T) {
	session := NewSession()

	// Unlocked session should allow any repository
	repoContext := &RepositoryContext{
		Owner: "octocat",
		Repo:  "Hello-World",
	}
	err := session.EnforcePolicy(repoContext)
	if err != nil {
		t.Errorf("unlocked session should allow any repository, got: %v", err)
	}

	// Lock to a repository
	session.LockRepository("octocat", "Hello-World")

	// Same repository should be allowed
	err = session.EnforcePolicy(repoContext)
	if err != nil {
		t.Errorf("same repository should be allowed, got: %v", err)
	}

	// Different repository should be denied
	differentRepo := &RepositoryContext{
		Owner: "github",
		Repo:  "gitignore",
	}
	err = session.EnforcePolicy(differentRepo)
	if err == nil {
		t.Error("different repository should be denied")
	}
}

func TestExtractRepositoryFromArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		expected *RepositoryContext
	}{
		{
			name: "valid owner and repo",
			args: map[string]interface{}{
				"owner": "octocat",
				"repo":  "Hello-World",
			},
			expected: &RepositoryContext{
				Owner: "octocat",
				Repo:  "Hello-World",
			},
		},
		{
			name: "missing owner",
			args: map[string]interface{}{
				"repo": "Hello-World",
			},
			expected: nil,
		},
		{
			name: "missing repo",
			args: map[string]interface{}{
				"owner": "octocat",
			},
			expected: nil,
		},
		{
			name: "empty owner",
			args: map[string]interface{}{
				"owner": "",
				"repo":  "Hello-World",
			},
			expected: nil,
		},
		{
			name: "empty repo",
			args: map[string]interface{}{
				"owner": "octocat",
				"repo":  "",
			},
			expected: nil,
		},
		{
			name: "wrong type for owner",
			args: map[string]interface{}{
				"owner": 123,
				"repo":  "Hello-World",
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractRepositoryFromArgs(tt.args)

			if tt.expected == nil && result != nil {
				t.Errorf("expected nil, got %+v", result)
			}

			if tt.expected != nil && result == nil {
				t.Errorf("expected %+v, got nil", tt.expected)
			}

			if tt.expected != nil && result != nil {
				if result.Owner != tt.expected.Owner || result.Repo != tt.expected.Repo {
					t.Errorf("expected %+v, got %+v", tt.expected, result)
				}
			}
		})
	}
}
