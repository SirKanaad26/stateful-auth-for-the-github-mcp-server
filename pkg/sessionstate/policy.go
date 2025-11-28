package sessionstate

import (
	"fmt"
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
// Returns nil if validation passes, otherwise returns a PolicyViolationError.
func (s *Session) ValidateAndLock(repoContext *RepositoryContext) error {
	if repoContext == nil {
		// No repository context, nothing to validate or lock
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// If not locked yet, lock to this repository
	if !s.isRepositoryLocked {
		s.lockedOwner = repoContext.Owner
		s.lockedRepository = repoContext.Repo
		s.isRepositoryLocked = true
		return nil
	}

	// Already locked, validate that this call targets the same repository
	if s.lockedOwner == repoContext.Owner && s.lockedRepository == repoContext.Repo {
		return nil
	}

	// Policy violation
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
