package sessionstate

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionStore(t *testing.T) {
	// Test in-memory store
	store, err := NewSessionStore(":memory:")
	require.NoError(t, err)
	require.NotNil(t, store)
	defer store.Close()

	assert.True(t, store.inMemory)
	assert.Equal(t, ":memory:", store.dbPath)
}

func TestNewSessionStoreFile(t *testing.T) {
	// Test file-based store
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "sessions.db")

	store, err := NewSessionStore(dbPath)
	require.NoError(t, err)
	require.NotNil(t, store)
	defer store.Close()

	assert.False(t, store.inMemory)

	// Verify file exists
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func TestSessionStoreLocking(t *testing.T) {
	store, err := NewSessionStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	clientID := "test-client-123"

	// Initially should be unlocked
	repo, isLocked, err := store.GetLock(clientID)
	require.NoError(t, err)
	assert.False(t, isLocked)
	assert.Empty(t, repo)

	// Lock to a repository
	err = store.SetLock(clientID, "test-repo")
	require.NoError(t, err)

	// Should now be locked
	repo, isLocked, err = store.GetLock(clientID)
	require.NoError(t, err)
	assert.True(t, isLocked)
	assert.Equal(t, "test-repo", repo)

	// Clear lock
	err = store.ClearLock(clientID)
	require.NoError(t, err)

	// Should be unlocked again
	repo, isLocked, err = store.GetLock(clientID)
	require.NoError(t, err)
	assert.False(t, isLocked)
	assert.Empty(t, repo)
}

func TestSessionStoreMultipleSessions(t *testing.T) {
	store, err := NewSessionStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	// Lock multiple sessions to different repos
	clients := map[string]string{
		"client-1": "repo-alpha",
		"client-2": "repo-beta",
		"client-3": "repo-gamma",
	}

	for clientID, repo := range clients {
		err := store.SetLock(clientID, repo)
		require.NoError(t, err)
	}

	// Verify each session has correct lock
	for clientID, expectedRepo := range clients {
		repo, isLocked, err := store.GetLock(clientID)
		require.NoError(t, err)
		assert.True(t, isLocked)
		assert.Equal(t, expectedRepo, repo)
	}

	// Verify session count
	count, err := store.SessionCount()
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestSessionStoreUpdateLock(t *testing.T) {
	store, err := NewSessionStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	clientID := "test-client"

	// Lock to first repo
	err = store.SetLock(clientID, "repo-1")
	require.NoError(t, err)

	repo, isLocked, err := store.GetLock(clientID)
	require.NoError(t, err)
	assert.True(t, isLocked)
	assert.Equal(t, "repo-1", repo)

	// Update to second repo
	err = store.SetLock(clientID, "repo-2")
	require.NoError(t, err)

	repo, isLocked, err = store.GetLock(clientID)
	require.NoError(t, err)
	assert.True(t, isLocked)
	assert.Equal(t, "repo-2", repo)

	// Session count should still be 1
	count, err := store.SessionCount()
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestSessionStoreDelete(t *testing.T) {
	store, err := NewSessionStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	clientID := "test-client"

	// Create a session
	err = store.SetLock(clientID, "test-repo")
	require.NoError(t, err)

	count, err := store.SessionCount()
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Delete the session
	err = store.DeleteSession(clientID)
	require.NoError(t, err)

	// Should be gone
	count, err = store.SessionCount()
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Getting lock should return unlocked state
	repo, isLocked, err := store.GetLock(clientID)
	require.NoError(t, err)
	assert.False(t, isLocked)
	assert.Empty(t, repo)
}

func TestSessionStoreCleanupStale(t *testing.T) {
	store, err := NewSessionStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	// Create several sessions
	for i := 1; i <= 5; i++ {
		clientID := fmt.Sprintf("client-%d", i)
		err := store.SetLock(clientID, "test-repo")
		require.NoError(t, err)
	}

	count, err := store.SessionCount()
	require.NoError(t, err)
	assert.Equal(t, 5, count)

	// Wait longer than the cleanup threshold
	time.Sleep(200 * time.Millisecond)

	// Cleanup sessions older than 100ms (should delete all)
	deleted, err := store.CleanupStale(100 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 5, deleted)

	// Should have no sessions left
	count, err = store.SessionCount()
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestSessionStoreCleanupStalePreserveRecent(t *testing.T) {
	store, err := NewSessionStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	// Create old session
	err = store.SetLock("old-client", "test-repo")
	require.NoError(t, err)

	// Wait to make it old
	time.Sleep(200 * time.Millisecond)

	// Create new session
	err = store.SetLock("new-client", "test-repo")
	require.NoError(t, err)

	// Cleanup sessions older than 100ms (should only delete old one)
	deleted, err := store.CleanupStale(100 * time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)

	// Should have 1 session left
	count, err := store.SessionCount()
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// New session should still exist
	_, isLocked, err := store.GetLock("new-client")
	require.NoError(t, err)
	assert.True(t, isLocked)

	// Old session should be gone
	_, isLocked, err = store.GetLock("old-client")
	require.NoError(t, err)
	assert.False(t, isLocked)
}

func TestSessionStoreStats(t *testing.T) {
	store, err := NewSessionStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	// Create a few sessions
	for i := 1; i <= 3; i++ {
		clientID := fmt.Sprintf("client-%d", i)
		err := store.SetLock(clientID, "test-repo")
		require.NoError(t, err)
	}

	stats, err := store.Stats()
	require.NoError(t, err)
	assert.Equal(t, 3, stats["total_sessions"])
	assert.Equal(t, ":memory:", stats["db_path"])
	assert.Equal(t, true, stats["in_memory"])
	assert.NotNil(t, stats["open_connections"])
}

func TestSessionStoreConcurrency(t *testing.T) {
	// Use a file-based DB for concurrency test to avoid SQLite's in-memory limitations
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-concurrent.db")

	store, err := NewSessionStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	// Run concurrent operations
	const numGoroutines = 10
	const numOps = 10

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			clientID := fmt.Sprintf("client-%d", id)

			for j := 0; j < numOps; j++ {
				// Lock
				err := store.SetLock(clientID, "test-repo")
				if err != nil {
					t.Errorf("SetLock failed: %v", err)
				}

				// Get lock
				_, _, err = store.GetLock(clientID)
				if err != nil {
					t.Errorf("GetLock failed: %v", err)
				}

				// Clear lock
				err = store.ClearLock(clientID)
				if err != nil {
					t.Errorf("ClearLock failed: %v", err)
				}
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestPersistentSession(t *testing.T) {
	store, err := NewSessionStore(":memory:")
	require.NoError(t, err)
	defer func() {
		time.Sleep(50 * time.Millisecond) // Let async updates finish
		store.Close()
	}()

	clientID := "test-client"
	session := NewPersistentSession(store, clientID)

	// Initially unlocked
	assert.False(t, session.IsRepositoryLocked())
	assert.Empty(t, session.GetLockedRepository())

	// Lock to a repository
	session.LockRepository("owner", "test-repo")

	// Should be locked
	assert.True(t, session.IsRepositoryLocked())
	assert.Equal(t, "test-repo", session.GetLockedRepository())

	// Should match
	assert.True(t, session.MatchesLockedRepository("owner", "test-repo"))
	assert.True(t, session.MatchesLockedRepository("different-owner", "test-repo"))
	assert.False(t, session.MatchesLockedRepository("owner", "different-repo"))

	// Unlock
	err = session.UnlockRepository()
	require.NoError(t, err)
	assert.False(t, session.IsRepositoryLocked())
}

func TestPersistentSessionValidateAndLock(t *testing.T) {
	store, err := NewSessionStore(":memory:")
	require.NoError(t, err)
	defer func() {
		time.Sleep(50 * time.Millisecond) // Let async updates finish
		store.Close()
	}()

	session := NewPersistentSession(store, "test-client")

	// First call should lock
	ctx1 := &RepositoryContext{Owner: "octocat", Repo: "Hello-World"}
	err = session.ValidateAndLock(ctx1)
	require.NoError(t, err)
	assert.True(t, session.IsRepositoryLocked())
	assert.Equal(t, "Hello-World", session.GetLockedRepository())

	// Same repo should succeed
	err = session.ValidateAndLock(ctx1)
	require.NoError(t, err)

	// Different owner, same repo should succeed (owner ignored)
	ctx2 := &RepositoryContext{Owner: "different-owner", Repo: "Hello-World"}
	err = session.ValidateAndLock(ctx2)
	require.NoError(t, err)

	// Different repo should fail and unlock
	ctx3 := &RepositoryContext{Owner: "github", Repo: "gitignore"}
	err = session.ValidateAndLock(ctx3)
	require.Error(t, err)

	policyErr, ok := err.(*PolicyViolationError)
	require.True(t, ok)
	assert.Equal(t, "Hello-World", policyErr.LockedRepo)
	assert.Equal(t, "gitignore", policyErr.RequestedRepo)

	// Session should be unlocked after violation
	assert.False(t, session.IsRepositoryLocked())
}
