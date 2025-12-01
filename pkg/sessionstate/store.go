package sessionstate

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SessionStore provides persistent storage for session state using SQLite.
// It supports multiple concurrent sessions identified by clientID.
type SessionStore struct {
	db         *sql.DB
	mu         sync.RWMutex
	dbPath     string
	inMemory   bool
	statements *preparedStatements
}

// preparedStatements holds all prepared SQL statements for performance
type preparedStatements struct {
	getLock       *sql.Stmt
	setLock       *sql.Stmt
	clearLock     *sql.Stmt
	deleteSession *sql.Stmt
	sessionCount  *sql.Stmt
	cleanup       *sql.Stmt
}

// NewSessionStore creates a new SQLite-based session store.
// If dbPath is empty or ":memory:", creates an in-memory database.
func NewSessionStore(dbPath string) (*SessionStore, error) {
	inMemory := dbPath == "" || dbPath == ":memory:"
	if inMemory {
		dbPath = ":memory:"
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	store := &SessionStore{
		db:       db,
		dbPath:   dbPath,
		inMemory: inMemory,
	}

	if err := store.initialize(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := store.prepareStatements(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[SESSION STORE] Initialized (path=%s, in-memory=%v)\n", dbPath, inMemory)
	return store, nil
}

// initialize creates the database schema
func (s *SessionStore) initialize() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		client_id TEXT PRIMARY KEY,
		locked_repository TEXT,
		is_locked INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		last_access_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_last_access ON sessions(last_access_at);
	CREATE INDEX IF NOT EXISTS idx_sessions_locked ON sessions(is_locked, locked_repository);
	`

	_, err := s.db.Exec(schema)
	return err
}

// prepareStatements creates prepared statements for common operations
func (s *SessionStore) prepareStatements() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	s.statements = &preparedStatements{}

	s.statements.getLock, err = s.db.Prepare(`
		SELECT locked_repository, is_locked 
		FROM sessions 
		WHERE client_id = ?
	`)
	if err != nil {
		return fmt.Errorf("prepare getLock: %w", err)
	}

	s.statements.setLock, err = s.db.Prepare(`
		INSERT INTO sessions (client_id, locked_repository, is_locked, created_at, updated_at, last_access_at)
		VALUES (?, ?, 1, ?, ?, ?)
		ON CONFLICT(client_id) DO UPDATE SET
			locked_repository = excluded.locked_repository,
			is_locked = 1,
			updated_at = excluded.updated_at,
			last_access_at = excluded.last_access_at
	`)
	if err != nil {
		return fmt.Errorf("prepare setLock: %w", err)
	}

	s.statements.clearLock, err = s.db.Prepare(`
		UPDATE sessions 
		SET is_locked = 0, locked_repository = NULL, updated_at = ?, last_access_at = ?
		WHERE client_id = ?
	`)
	if err != nil {
		return fmt.Errorf("prepare clearLock: %w", err)
	}

	s.statements.deleteSession, err = s.db.Prepare(`
		DELETE FROM sessions WHERE client_id = ?
	`)
	if err != nil {
		return fmt.Errorf("prepare deleteSession: %w", err)
	}

	s.statements.sessionCount, err = s.db.Prepare(`
		SELECT COUNT(*) FROM sessions
	`)
	if err != nil {
		return fmt.Errorf("prepare sessionCount: %w", err)
	}

	s.statements.cleanup, err = s.db.Prepare(`
		DELETE FROM sessions WHERE last_access_at < ?
	`)
	if err != nil {
		return fmt.Errorf("prepare cleanup: %w", err)
	}

	return nil
}

// GetLock retrieves the lock state for a given client session.
// Returns (repository, isLocked, error).
func (s *SessionStore) GetLock(clientID string) (string, bool, error) {
	s.mu.RLock()
	stmt := s.statements.getLock
	db := s.db
	s.mu.RUnlock()

	if stmt == nil || db == nil {
		return "", false, fmt.Errorf("store is closed")
	}

	var repository sql.NullString
	var isLocked int

	err := stmt.QueryRow(clientID).Scan(&repository, &isLocked)
	if err == sql.ErrNoRows {
		// No session exists yet - return unlocked state
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("query failed: %w", err)
	}

	// Update last access time asynchronously
	go s.touchSession(clientID)

	return repository.String, isLocked == 1, nil
}

// SetLock locks a client session to a specific repository.
func (s *SessionStore) SetLock(clientID, repository string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	_, err := s.statements.setLock.Exec(clientID, repository, now, now, now)
	if err != nil {
		return fmt.Errorf("failed to set lock: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[SESSION STORE] Locked session %s to repository %s\n", clientID, repository)
	return nil
}

// ClearLock unlocks a client session, allowing access to any repository.
func (s *SessionStore) ClearLock(clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	_, err := s.statements.clearLock.Exec(now, now, clientID)
	if err != nil {
		return fmt.Errorf("failed to clear lock: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[SESSION STORE] Unlocked session %s\n", clientID)
	return nil
}

// DeleteSession removes a session from the store.
func (s *SessionStore) DeleteSession(clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.statements.deleteSession.Exec(clientID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[SESSION STORE] Deleted session %s\n", clientID)
	return nil
}

// SessionCount returns the total number of sessions in the store.
func (s *SessionStore) SessionCount() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.statements.sessionCount.QueryRow().Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions: %w", err)
	}

	return count, nil
}

// CleanupStale removes sessions that haven't been accessed for the specified duration.
func (s *SessionStore) CleanupStale(maxAge time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge).Unix()
	result, err := s.statements.cleanup.Exec(cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup stale sessions: %w", err)
	}

	deleted, _ := result.RowsAffected()
	if deleted > 0 {
		fmt.Fprintf(os.Stderr, "[SESSION STORE] Cleaned up %d stale sessions\n", deleted)
	}

	return int(deleted), nil
}

// touchSession updates the last_access_at timestamp for a session
func (s *SessionStore) touchSession(clientID string) {
	s.mu.RLock()
	db := s.db
	s.mu.RUnlock()

	if db == nil {
		return // Store is closed, silently ignore
	}

	now := time.Now().Unix()
	_, err := db.Exec(`UPDATE sessions SET last_access_at = ? WHERE client_id = ?`, now, clientID)
	if err != nil && err != sql.ErrConnDone {
		fmt.Fprintf(os.Stderr, "[SESSION STORE] Warning: failed to update last_access_at for %s: %v\n", clientID, err)
	}
}

// Close closes the database connection and cleans up prepared statements.
func (s *SessionStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close prepared statements first
	if s.statements != nil {
		if s.statements.getLock != nil {
			s.statements.getLock.Close()
		}
		if s.statements.setLock != nil {
			s.statements.setLock.Close()
		}
		if s.statements.clearLock != nil {
			s.statements.clearLock.Close()
		}
		if s.statements.deleteSession != nil {
			s.statements.deleteSession.Close()
		}
		if s.statements.sessionCount != nil {
			s.statements.sessionCount.Close()
		}
		if s.statements.cleanup != nil {
			s.statements.cleanup.Close()
		}
		s.statements = nil
	}

	// Close database connection
	if s.db != nil {
		err := s.db.Close()
		if err != nil {
			return fmt.Errorf("failed to close database: %w", err)
		}
		s.db = nil
	}

	fmt.Fprintf(os.Stderr, "[SESSION STORE] Closed\n")
	return nil
}

// Stats returns statistics about the session store.
func (s *SessionStore) Stats() (map[string]interface{}, error) {
	count, err := s.SessionCount()
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_sessions": count,
		"db_path":        s.dbPath,
		"in_memory":      s.inMemory,
	}

	// Get SQLite stats
	var dbStats sql.DBStats
	if s.db != nil {
		dbStats = s.db.Stats()
		stats["open_connections"] = dbStats.OpenConnections
		stats["in_use"] = dbStats.InUse
		stats["idle"] = dbStats.Idle
	}

	return stats, nil
}
