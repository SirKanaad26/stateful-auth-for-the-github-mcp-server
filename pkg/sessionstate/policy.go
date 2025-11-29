package sessionstate

import (
	"fmt"
	"os"
)

// PolicyViolationError indicates that a tool call violates the session's access policy.
type PolicyViolationError struct {
	Message        string
	LockedOwner    string
	LockedRepo     string
	RequestedOwner string
	RequestedRepo  string
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
	if s.lockedOwner == repoContext.Owner && s.lockedRepository == repoContext.Repo {
		return nil
	}

	// Policy violation: attempting to access a different repository
	return &PolicyViolationError{
		Message: fmt.Sprintf(
			"repository access denied: session is locked to %s/%s, but tool attempted to access %s/%s",
			s.lockedOwner,
			s.lockedRepository,
			repoContext.Owner,
			repoContext.Repo,
		),
		LockedOwner:    s.lockedOwner,
		LockedRepo:     s.lockedRepository,
		RequestedOwner: repoContext.Owner,
		RequestedRepo:  repoContext.Repo,
	}
}

// ValidateAndLock validates a tool call's repository context and locks the session if unlocked.
// If the session is unlocked, it locks to the repository in the tool call.
// If the session is locked, it validates that the tool call targets the same repository.
// If WASM bridge is available, delegates to WASM for validation.
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

	fmt.Fprintf(os.Stderr, "[SESSION] ValidateAndLock: repo=%s/%s, locked=%v, lockedRepo=%s/%s\n",
		repoContext.Owner, repoContext.Repo, s.isRepositoryLocked, s.lockedOwner, s.lockedRepository)

	// If not locked yet, lock to this repository
	if !s.isRepositoryLocked {
		s.lockedOwner = repoContext.Owner
		s.lockedRepository = repoContext.Repo
		s.isRepositoryLocked = true
		fmt.Fprintf(os.Stderr, "[SESSION] LOCKED to %s/%s\n", repoContext.Owner, repoContext.Repo)
		return nil
	}

	// Already locked, validate that this call targets the same repository
	if s.lockedOwner == repoContext.Owner && s.lockedRepository == repoContext.Repo {
		fmt.Fprintf(os.Stderr, "[SESSION] Repository matches, allowing\n")
		return nil
	}

	// Policy violation - unlock the session to allow legitimate cross-repo access after user intervention
	fmt.Fprintf(os.Stderr, "[SESSION] POLICY VIOLATION: locked=%s/%s, requested=%s/%s\n",
		s.lockedOwner, s.lockedRepository, repoContext.Owner, repoContext.Repo)

	// Capture locked repo info for error message before unlocking
	lockedOwner := s.lockedOwner
	lockedRepo := s.lockedRepository

	// Unlock the session since execution will stop anyway
	s.lockedOwner = ""
	s.lockedRepository = ""
	s.isRepositoryLocked = false
	fmt.Fprintf(os.Stderr, "[SESSION] UNLOCKED after policy violation\n")

	return &PolicyViolationError{
		Message: fmt.Sprintf(
			"repository access denied: session is locked to %s/%s, but tool attempted to access %s/%s",
			lockedOwner,
			lockedRepo,
			repoContext.Owner,
			repoContext.Repo,
		),
		LockedOwner:    lockedOwner,
		LockedRepo:     lockedRepo,
		RequestedOwner: repoContext.Owner,
		RequestedRepo:  repoContext.Repo,
	}
}
