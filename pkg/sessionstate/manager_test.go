package sessionstate

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewSessionManager(t *testing.T) {
	sm := NewSessionManager()

	require.NotNil(t, sm)
	require.Equal(t, 0, sm.SessionCount())
	require.False(t, sm.useWASM)
}

func TestGetOrCreateSession(t *testing.T) {
	sm := NewSessionManager()

	// First call should create a new session
	session1 := sm.GetOrCreateSession("client-1")
	require.NotNil(t, session1)
	require.Equal(t, 1, sm.SessionCount())

	// Second call with same client ID should return the same session
	session2 := sm.GetOrCreateSession("client-1")
	require.NotNil(t, session2)
	require.Equal(t, 1, sm.SessionCount())

	// Should be the same session instance
	session1.LockRepository("octocat", "Hello-World")
	require.True(t, session2.IsRepositoryLocked())

	// Different client ID should get different session
	session3 := sm.GetOrCreateSession("client-2")
	require.NotNil(t, session3)
	require.Equal(t, 2, sm.SessionCount())
	require.False(t, session3.IsRepositoryLocked())
}

func TestSessionIsolation(t *testing.T) {
	sm := NewSessionManager()

	// Create sessions for two clients
	session1 := sm.GetOrCreateSession("client-1")
	session2 := sm.GetOrCreateSession("client-2")

	// Lock each to different repositories
	session1.LockRepository("user1", "repo-a")
	session2.LockRepository("user2", "repo-b")

	// Verify isolation
	repo1 := session1.GetLockedRepository()
	require.Equal(t, "repo-a", repo1)

	repo2 := session2.GetLockedRepository()
	require.Equal(t, "repo-b", repo2)

	// Verify they don't interfere
	require.True(t, session1.MatchesLockedRepository("user1", "repo-a"))
	require.False(t, session1.MatchesLockedRepository("user2", "repo-b"))

	require.True(t, session2.MatchesLockedRepository("user2", "repo-b"))
	require.False(t, session2.MatchesLockedRepository("user1", "repo-a"))
}

func TestResetSession(t *testing.T) {
	sm := NewSessionManager()

	// Create and lock a session
	session1 := sm.GetOrCreateSession("client-1")
	session1.LockRepository("octocat", "Hello-World")
	require.True(t, session1.IsRepositoryLocked())

	// Reset the session
	session2 := sm.ResetSession("client-1")
	require.NotNil(t, session2)
	require.False(t, session2.IsRepositoryLocked())

	// Should still have 1 session
	require.Equal(t, 1, sm.SessionCount())

	// Getting the session should return the new (reset) one
	session3 := sm.GetOrCreateSession("client-1")
	require.False(t, session3.IsRepositoryLocked())
}

func TestRemoveSession(t *testing.T) {
	sm := NewSessionManager()

	// Create sessions
	sm.GetOrCreateSession("client-1")
	sm.GetOrCreateSession("client-2")
	require.Equal(t, 2, sm.SessionCount())

	// Remove one session
	sm.RemoveSession("client-1")
	require.Equal(t, 1, sm.SessionCount())

	// Verify it was removed
	session := sm.GetSession("client-1")
	require.Nil(t, session)

	// Other session should still exist
	session2 := sm.GetSession("client-2")
	require.NotNil(t, session2)

	// Removing non-existent session should not error
	sm.RemoveSession("client-999")
	require.Equal(t, 1, sm.SessionCount())
}

func TestGetSession(t *testing.T) {
	sm := NewSessionManager()

	// Non-existent session should return nil
	session := sm.GetSession("client-1")
	require.Nil(t, session)

	// Create a session
	sm.GetOrCreateSession("client-1")

	// Now it should exist
	session = sm.GetSession("client-1")
	require.NotNil(t, session)
}

func TestListClientIDs(t *testing.T) {
	sm := NewSessionManager()

	// Empty list initially
	clientIDs := sm.ListClientIDs()
	require.Len(t, clientIDs, 0)

	// Create some sessions
	sm.GetOrCreateSession("client-1")
	sm.GetOrCreateSession("client-2")
	sm.GetOrCreateSession("client-3")

	// Should return all client IDs
	clientIDs = sm.ListClientIDs()
	require.Len(t, clientIDs, 3)
	require.Contains(t, clientIDs, "client-1")
	require.Contains(t, clientIDs, "client-2")
	require.Contains(t, clientIDs, "client-3")
}

func TestClear(t *testing.T) {
	sm := NewSessionManager()

	// Create some sessions
	sm.GetOrCreateSession("client-1")
	sm.GetOrCreateSession("client-2")
	sm.GetOrCreateSession("client-3")
	require.Equal(t, 3, sm.SessionCount())

	// Clear all sessions
	sm.Clear()
	require.Equal(t, 0, sm.SessionCount())

	// Verify all are gone
	require.Nil(t, sm.GetSession("client-1"))
	require.Nil(t, sm.GetSession("client-2"))
	require.Nil(t, sm.GetSession("client-3"))
}

func TestManagerConcurrentAccess(t *testing.T) {
	sm := NewSessionManager()

	// Launch multiple goroutines creating sessions
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			clientID := "client-" + string(rune('0'+id))
			session := sm.GetOrCreateSession(clientID)
			session.LockRepository("user", "repo")
			done <- true
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have 10 sessions
	require.Equal(t, 10, sm.SessionCount())
}

func TestMultiTenantScenario(t *testing.T) {
	// Simulate the attack scenario with proper session isolation
	sm := NewSessionManager()

	// User A's session
	sessionA := sm.GetOrCreateSession("user-a-token")
	repoA := &RepositoryContext{Owner: "user-a", Repo: "private-repo"}
	err := sessionA.ValidateAndLock(repoA)
	require.NoError(t, err)

	// User B's session (different client)
	sessionB := sm.GetOrCreateSession("user-b-token")
	repoB := &RepositoryContext{Owner: "user-b", Repo: "other-repo"}
	err = sessionB.ValidateAndLock(repoB)
	require.NoError(t, err)

	// User A should still be locked to their repo
	repo := sessionA.GetLockedRepository()
	require.Equal(t, "private-repo", repo)

	// User B should be locked to their repo
	repo = sessionB.GetLockedRepository()
	require.Equal(t, "other-repo", repo)

	// User A can still access their repo
	err = sessionA.ValidateAndLock(repoA)
	require.NoError(t, err)

	// User B can still access their repo
	err = sessionB.ValidateAndLock(repoB)
	require.NoError(t, err)

	// Cross-access should fail
	err = sessionA.ValidateAndLock(repoB)
	require.Error(t, err)

	err = sessionB.ValidateAndLock(repoA)
	require.Error(t, err)
}
