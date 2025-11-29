package sessionstate

import (
	"context"
	"fmt"
	"os"
	"sync"
)

// Session represents the state of a single LLM agent session.
// It tracks which repository the session is locked to for stateful authorization.
// Optionally integrates with WASM module for policy enforcement.
type Session struct {
	mu                 sync.RWMutex
	lockedOwner        string
	lockedRepository   string
	isRepositoryLocked bool
	wasmBridge         *WASMBridge
	useWASM            bool
}

// NewSession creates a new session with no repository lock.
func NewSession() *Session {
	return &Session{
		isRepositoryLocked: false,
		useWASM:            false,
	}
}

// NewSessionWithWASM creates a new session with WASM bridge integration.
// If wasmPath is provided, it will attempt to load the WASM module.
// If loading fails, it falls back to native Go implementation with a warning.
func NewSessionWithWASM(ctx context.Context, wasmPath string) *Session {
	session := NewSession()

	if wasmPath != "" {
		bridge, err := NewWASMBridge(ctx, wasmPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[SESSION] Warning: failed to load WASM bridge: %v, using native implementation\n", err)
			return session
		}
		session.wasmBridge = bridge
		session.useWASM = true
		fmt.Fprintf(os.Stderr, "[SESSION] WASM integration enabled\n")
	}

	return session
}

// LockRepository locks the session to a specific repository.
// Once locked, all subsequent tool calls must target this repository.
func (s *Session) LockRepository(owner, repo string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lockedOwner = owner
	s.lockedRepository = repo
	s.isRepositoryLocked = true
	fmt.Fprintf(os.Stderr, "[SESSION] LockRepository called: %s/%s\n", owner, repo)
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

// Close closes the session and releases any resources (e.g., WASM runtime).
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.wasmBridge != nil {
		if err := s.wasmBridge.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "[SESSION] Error closing WASM bridge: %v\n", err)
			return err
		}
		s.wasmBridge = nil
	}
	return nil
}
