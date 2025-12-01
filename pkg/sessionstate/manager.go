package sessionstate

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

// SessionManager manages multiple sessions for different clients.
// This enables proper session isolation in multi-tenant/HTTP deployments
// while maintaining backward compatibility with stdio mode.
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex

	// Configuration
	ctx         context.Context
	autoCleanup bool
	maxIdleTime time.Duration
}

// NewSessionManager creates a new session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:    make(map[string]*Session),
		ctx:         context.Background(),
		autoCleanup: false, // Disabled by default, can be enabled for long-running servers
		maxIdleTime: 24 * time.Hour,
	}
}

// GetOrCreateSession returns an existing session for the client ID or creates a new one.
// The clientID should be derived from a stable identifier (e.g., auth token, connection ID).
func (sm *SessionManager) GetOrCreateSession(clientID string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if session already exists
	if session, exists := sm.sessions[clientID]; exists {
		fmt.Fprintf(os.Stderr, "[SESSION-MANAGER] Returning existing session for client %s\n", clientID)
		return session
	}

	// Create new session
	session := NewSession()
	sm.sessions[clientID] = session
	fmt.Fprintf(os.Stderr, "[SESSION-MANAGER] Created new session for client %s (total sessions: %d)\n", clientID, len(sm.sessions))
	return session
}

// GetSession returns an existing session for the client ID, or nil if not found.
func (sm *SessionManager) GetSession(clientID string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[clientID]
}

// ResetSession resets the session for a specific client.
// This is typically called on MCP initialize request.
func (sm *SessionManager) ResetSession(clientID string) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Create new session
	session := NewSession()
	sm.sessions[clientID] = session
	fmt.Fprintf(os.Stderr, "[SESSION-MANAGER] Reset session for client %s\n", clientID)
	return session
}

// RemoveSession removes a session for a specific client.
// This should be called when a client disconnects.
func (sm *SessionManager) RemoveSession(clientID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[clientID]; exists {
		delete(sm.sessions, clientID)
		fmt.Fprintf(os.Stderr, "[SESSION-MANAGER] Removed session for client %s (remaining: %d)\n", clientID, len(sm.sessions))
	}
}

// SessionCount returns the number of active sessions.
func (sm *SessionManager) SessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// ListClientIDs returns a list of all active client IDs.
// Useful for debugging and monitoring.
func (sm *SessionManager) ListClientIDs() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	clientIDs := make([]string, 0, len(sm.sessions))
	for clientID := range sm.sessions {
		clientIDs = append(clientIDs, clientID)
	}
	return clientIDs
}

// CleanupIdleSessions removes sessions that haven't been used recently.
// This is useful for long-running servers to prevent memory leaks.
// Currently, this is a placeholder - you'd need to track last access time.
func (sm *SessionManager) CleanupIdleSessions() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// TODO: Implement idle tracking if needed
	// For now, this is a no-op
	return 0
}

// Clear removes all sessions. Useful for testing.
func (sm *SessionManager) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	count := len(sm.sessions)
	sm.sessions = make(map[string]*Session)
	fmt.Fprintf(os.Stderr, "[SESSION-MANAGER] Cleared all sessions (removed %d sessions)\n", count)
}
