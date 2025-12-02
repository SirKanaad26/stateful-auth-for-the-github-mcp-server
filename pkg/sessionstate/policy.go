package sessionstate

import (
	"fmt"
	"os"
)

// PolicyViolationError indicates that a tool call violates the session's access policy.
type PolicyViolationError struct {
	Message       string
	LockedRepo    string
	RequestedRepo string
}

func (e *PolicyViolationError) Error() string {
	return e.Message
}

// EnforcePolicy validates a tool call against the session's repository lock policy.
// If the session is locked to a repository and the tool call targets a different repository,
// it returns a PolicyViolationError. Otherwise, it returns nil.
func (s *Session) EnforcePolicy(repoContext *RepositoryContext) error {
	if repoContext == nil {
		// Tool call has no repository context, so it's not subject to the lock
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// If session is not locked, any repository access is allowed
	if !s.isRepositoryLocked {
		return nil
	}

	// Check if the requested repository matches the locked repository
	// Note: Only checking repo name, not owner, to handle username variations
	if s.lockedRepository == repoContext.Repo {
		return nil
	}

	// Policy violation: attempting to access a different repository
	return &PolicyViolationError{
		Message: fmt.Sprintf(
			"repository access denied: session is locked to %s, but tool attempted to access %s",
			s.lockedRepository,
			repoContext.Repo,
		),
		LockedRepo:    s.lockedRepository,
		RequestedRepo: repoContext.Repo,
	}
}

// ValidateAndLock validates a tool call's repository context and locks the session if unlocked.
// If the session is unlocked, it locks to the repository in the tool call.
// If the session is locked, it validates that the tool call targets the same repository.
// If WASM is enabled, it delegates validation to the WASM module.
// Returns nil if validation passes, otherwise returns a PolicyViolationError.
func (s *Session) ValidateAndLock(repoContext *RepositoryContext) error {
	if repoContext == nil {
		// No repository context, nothing to validate or lock
		fmt.Fprintf(os.Stderr, "[SESSION] ValidateAndLock: no repo context\n")
		return nil
	}

	// If WASM bridge is enabled, delegate to WASM
	if s.useWASM && s.wasmBridge != nil {
		fmt.Fprintf(os.Stderr, "[SESSION] Delegating to WASM bridge for %s/%s\n", repoContext.Owner, repoContext.Repo)
		return s.wasmBridge.ValidateAndLockWASM(repoContext)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	fmt.Fprintf(os.Stderr, "[SESSION] ValidateAndLock: repo=%s, locked=%v, lockedRepo=%s\n",
		repoContext.Repo, s.isRepositoryLocked, s.lockedRepository)

	// If not locked yet, lock to this repository
	if !s.isRepositoryLocked {
		s.lockedRepository = repoContext.Repo
		s.isRepositoryLocked = true
		fmt.Fprintf(os.Stderr, "[SESSION] LOCKED to %s (owner %s ignored)\n", repoContext.Repo, repoContext.Owner)
		return nil
	}

	// Already locked, validate that this call targets the same repository
	// Note: Only checking repo name, not owner, to handle username variations
	if s.lockedRepository == repoContext.Repo {
		fmt.Fprintf(os.Stderr, "[SESSION] Repository matches, allowing\n")
		return nil
	}

	// Policy violation - unlock the session to allow legitimate cross-repo access after user intervention
	fmt.Fprintf(os.Stderr, "[SESSION] POLICY VIOLATION: locked=%s, requested=%s\n",
		s.lockedRepository, repoContext.Repo)

	// Capture locked repo info for error message before unlocking
	lockedRepo := s.lockedRepository

	// Unlock the session since execution will stop anyway
	s.lockedRepository = ""
	s.isRepositoryLocked = false
	fmt.Fprintf(os.Stderr, "[SESSION] UNLOCKED after policy violation\n")

	// Also unlock in WASM if enabled
	if s.useWASM && s.wasmBridge != nil {
		if err := s.wasmBridge.UnlockRepositoryWASM(); err != nil {
			fmt.Fprintf(os.Stderr, "[SESSION] Warning: WASM unlock failed: %v\n", err)
		}
	}

	return &PolicyViolationError{
		Message: fmt.Sprintf(
			"repository access denied: session is locked to %s, but tool attempted to access %s",
			lockedRepo,
			repoContext.Repo,
		),
		LockedRepo:    lockedRepo,
		RequestedRepo: repoContext.Repo,
	}
}
