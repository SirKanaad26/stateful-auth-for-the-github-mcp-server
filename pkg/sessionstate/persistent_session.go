package sessionstate

import (
	"fmt"
	"os"
)

// PersistentSession wraps the SessionStore to provide the same interface as the in-memory Session.
// This allows drop-in replacement while maintaining backwards compatibility.
type PersistentSession struct {
	store    *SessionStore
	clientID string
}

// NewPersistentSession creates a new session backed by a SessionStore.
func NewPersistentSession(store *SessionStore, clientID string) *PersistentSession {
	return &PersistentSession{
		store:    store,
		clientID: clientID,
	}
}

// LockRepository locks the session to a specific repository.
// Once locked, all subsequent tool calls must target this repository.
// Note: Only locks by repository name, not owner, to handle username variations.
func (s *PersistentSession) LockRepository(owner, repo string) {
	err := s.store.SetLock(s.clientID, repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[SESSION] Error locking repository: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "[SESSION] LockRepository called: %s (owner %s ignored)\n", repo, owner)
}

// IsRepositoryLocked returns whether the session is locked to a repository.
func (s *PersistentSession) IsRepositoryLocked() bool {
	_, isLocked, err := s.store.GetLock(s.clientID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[SESSION] Error checking lock status: %v\n", err)
		return false
	}
	return isLocked
}

// GetLockedRepository returns the repository name that the session is locked to.
// Returns empty string if the session is not locked.
func (s *PersistentSession) GetLockedRepository() string {
	repo, _, err := s.store.GetLock(s.clientID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[SESSION] Error getting locked repository: %v\n", err)
		return ""
	}
	return repo
}

// MatchesLockedRepository checks if the given repo matches the locked repository.
// Returns true if the session is not locked (no restriction) or if the repo matches.
// Note: Ignores owner to handle username variations from LLM hallucinations.
func (s *PersistentSession) MatchesLockedRepository(owner, repo string) bool {
	lockedRepo, isLocked, err := s.store.GetLock(s.clientID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[SESSION] Error checking repository match: %v\n", err)
		return false
	}

	if !isLocked {
		return true
	}

	return lockedRepo == repo
}

// UnlockRepository unlocks the session, allowing access to any repository.
func (s *PersistentSession) UnlockRepository() error {
	return s.store.ClearLock(s.clientID)
}

// ValidateAndLock validates a tool call's repository context and locks the session if unlocked.
// If the session is unlocked, it locks to the repository in the tool call.
// If the session is locked, it validates that the tool call targets the same repository.
// Returns nil if validation passes, otherwise returns a PolicyViolationError.
func (s *PersistentSession) ValidateAndLock(repoContext *RepositoryContext) error {
	if repoContext == nil {
		// No repository context, nothing to validate or lock
		fmt.Fprintf(os.Stderr, "[SESSION] ValidateAndLock: no repo context\n")
		return nil
	}

	lockedRepo, isLocked, err := s.store.GetLock(s.clientID)
	if err != nil {
		return fmt.Errorf("failed to get lock state: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[SESSION] ValidateAndLock: repo=%s, locked=%v, lockedRepo=%s\n",
		repoContext.Repo, isLocked, lockedRepo)

	// If not locked yet, lock to this repository
	if !isLocked {
		if err := s.store.SetLock(s.clientID, repoContext.Repo); err != nil {
			return fmt.Errorf("failed to lock repository: %w", err)
		}
		fmt.Fprintf(os.Stderr, "[SESSION] LOCKED to %s (owner %s ignored)\n", repoContext.Repo, repoContext.Owner)
		return nil
	}

	// Already locked, validate that this call targets the same repository
	// Note: Only checking repo name, not owner, to handle username variations
	if lockedRepo == repoContext.Repo {
		fmt.Fprintf(os.Stderr, "[SESSION] Repository matches, allowing\n")
		return nil
	}

	// Policy violation - unlock the session to allow legitimate cross-repo access after user intervention
	fmt.Fprintf(os.Stderr, "[SESSION] POLICY VIOLATION: locked=%s, requested=%s\n",
		lockedRepo, repoContext.Repo)

	// Unlock the session since execution will stop anyway
	if err := s.store.ClearLock(s.clientID); err != nil {
		fmt.Fprintf(os.Stderr, "[SESSION] Warning: failed to unlock after policy violation: %v\n", err)
	}
	fmt.Fprintf(os.Stderr, "[SESSION] UNLOCKED after policy violation\n")

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

// EnforcePolicy validates a tool call against the session's repository lock policy.
// If the session is locked to a repository and the tool call targets a different repository,
// it returns a PolicyViolationError. Otherwise, it returns nil.
func (s *PersistentSession) EnforcePolicy(repoContext *RepositoryContext) error {
	if repoContext == nil {
		// Tool call has no repository context, so it's not subject to the lock
		return nil
	}

	lockedRepo, isLocked, err := s.store.GetLock(s.clientID)
	if err != nil {
		return fmt.Errorf("failed to get lock state: %w", err)
	}

	// If session is not locked, any repository access is allowed
	if !isLocked {
		return nil
	}

	// Check if the requested repository matches the locked repository
	// Note: Only checking repo name, not owner, to handle username variations
	if lockedRepo == repoContext.Repo {
		return nil
	}

	// Policy violation: attempting to access a different repository
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

// Close cleans up the session (no-op for persistent sessions).
func (s *PersistentSession) Close() error {
	// The session itself doesn't own the store, so nothing to close
	return nil
}
