package sessionstate

import (
	"sync"
)

// Session represents the state of a single LLM agent session.
// It tracks which repository the session is locked to for stateful authorization.
type Session struct {
	mu                 sync.RWMutex
	lockedOwner        string
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
func (s *Session) LockRepository(owner, repo string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lockedOwner = owner
	s.lockedRepository = repo
	s.isRepositoryLocked = true
}

// IsRepositoryLocked returns whether the session is locked to a repository.
func (s *Session) IsRepositoryLocked() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.isRepositoryLocked
}

// GetLockedRepository returns the owner and repository name that the session is locked to.
// Returns empty strings if the session is not locked.
func (s *Session) GetLockedRepository() (owner, repo string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lockedOwner, s.lockedRepository
}

// MatchesLockedRepository checks if the given owner/repo matches the locked repository.
// Returns true if the session is not locked (no restriction) or if the owner/repo matches.
func (s *Session) MatchesLockedRepository(owner, repo string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.isRepositoryLocked {
		return true
	}

	return s.lockedOwner == owner && s.lockedRepository == repo
}
