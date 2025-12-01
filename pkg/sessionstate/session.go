package sessionstate

import (
	"fmt"
	"os"
	"sync"
)

// Session represents the state of a single LLM agent session.
// It tracks which repository the session is locked to for stateful authorization.
type Session struct {
	mu                 sync.RWMutex
	lockedRepository   string
	isRepositoryLocked bool
}

// NewSession creates a new session with no repository lock.
func NewSession() *Session {
	return &Session{
		isRepositoryLocked: false,
	}
}

// LockRepository locks the session to a specific repository.
// Once locked, all subsequent tool calls must target this repository.
// Note: Only locks by repository name, not owner, to handle username variations.
func (s *Session) LockRepository(owner, repo string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lockedRepository = repo
	s.isRepositoryLocked = true
	fmt.Fprintf(os.Stderr, "[SESSION] LockRepository called: %s (owner %s ignored)\n", repo, owner)
}

// IsRepositoryLocked returns whether the session is locked to a repository.
func (s *Session) IsRepositoryLocked() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.isRepositoryLocked
}

// GetLockedRepository returns the repository name that the session is locked to.
// Returns empty string if the session is not locked.
func (s *Session) GetLockedRepository() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lockedRepository
}

// MatchesLockedRepository checks if the given repo matches the locked repository.
// Returns true if the session is not locked (no restriction) or if the repo matches.
// Note: Ignores owner to handle username variations from LLM hallucinations.
func (s *Session) MatchesLockedRepository(owner, repo string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.isRepositoryLocked {
		return true
	}

	return s.lockedRepository == repo
}

// Close closes the session and releases any resources.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return nil
}
