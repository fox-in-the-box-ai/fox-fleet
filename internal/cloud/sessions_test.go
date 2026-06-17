package cloud

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func openTestDBWithSessions(t *testing.T) *SessionStore {
	t.Helper()
	db := openTestDB(t)
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS cloud_sessions (
		token_hash  TEXT PRIMARY KEY,
		user_id     TEXT NOT NULL REFERENCES cloud_users(username) ON DELETE CASCADE,
		created_at  TEXT NOT NULL,
		expires_at  TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create cloud_sessions table: %v", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_cloud_sessions_user ON cloud_sessions(user_id)`); err != nil {
		t.Fatalf("create user index: %v", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_cloud_sessions_expires ON cloud_sessions(expires_at)`); err != nil {
		t.Fatalf("create expires index: %v", err)
	}
	return NewSessionStore(db)
}

func createTestUser(t *testing.T, store *SessionStore, username string) {
	t.Helper()
	us := NewUserStore(store.db)
	if _, err := us.Create(username, "testpass"); err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
}

func TestSessionStore_CreateAndValidate(t *testing.T) {
	ss := openTestDBWithSessions(t)
	createTestUser(t, ss, "alice")

	token, sess, err := ss.Create("alice", time.Hour)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}
	if sess.UserID != "alice" {
		t.Errorf("user_id = %q, want %q", sess.UserID, "alice")
	}

	got, err := ss.Validate(token)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got.UserID != "alice" {
		t.Errorf("validated user_id = %q, want %q", got.UserID, "alice")
	}
	if got.TokenHash != sess.TokenHash {
		t.Errorf("token_hash mismatch")
	}
}

func TestSessionStore_ValidateExpired(t *testing.T) {
	ss := openTestDBWithSessions(t)
	createTestUser(t, ss, "alice")

	token, _, err := ss.Create("alice", -time.Second)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	_, err = ss.Validate(token)
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound for expired session, got: %v", err)
	}
}

func TestSessionStore_ValidateNotFound(t *testing.T) {
	ss := openTestDBWithSessions(t)

	_, err := ss.Validate("nonexistent-token")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestSessionStore_Delete(t *testing.T) {
	ss := openTestDBWithSessions(t)
	createTestUser(t, ss, "alice")

	token, _, err := ss.Create("alice", time.Hour)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := ss.Delete(token); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = ss.Validate(token)
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound after delete, got: %v", err)
	}
}

func TestSessionStore_DeleteNotFound(t *testing.T) {
	ss := openTestDBWithSessions(t)

	err := ss.Delete("nonexistent-token")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestSessionStore_DeleteByUser(t *testing.T) {
	ss := openTestDBWithSessions(t)
	createTestUser(t, ss, "alice")

	for i := 0; i < 3; i++ {
		if _, _, err := ss.Create("alice", time.Hour); err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
	}

	n, err := ss.DeleteByUser("alice")
	if err != nil {
		t.Fatalf("delete by user: %v", err)
	}
	if n != 3 {
		t.Errorf("deleted %d sessions, want 3", n)
	}
}

func TestSessionStore_PurgeExpired(t *testing.T) {
	ss := openTestDBWithSessions(t)
	createTestUser(t, ss, "alice")

	if _, _, err := ss.Create("alice", -time.Second); err != nil {
		t.Fatalf("create expired: %v", err)
	}
	if _, _, err := ss.Create("alice", -time.Minute); err != nil {
		t.Fatalf("create expired 2: %v", err)
	}
	if _, _, err := ss.Create("alice", time.Hour); err != nil {
		t.Fatalf("create valid: %v", err)
	}

	n, err := ss.PurgeExpired()
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 2 {
		t.Errorf("purged %d, want 2", n)
	}
}

func TestSessionStore_FKCascadeOnUserDelete(t *testing.T) {
	ss := openTestDBWithSessions(t)
	createTestUser(t, ss, "alice")

	token, _, err := ss.Create("alice", time.Hour)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	us := NewUserStore(ss.db)
	if err := us.Delete("alice"); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	_, err = ss.Validate(token)
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound after user delete (FK CASCADE), got: %v", err)
	}
}

func TestSessionStore_MultipleSessions(t *testing.T) {
	ss := openTestDBWithSessions(t)
	createTestUser(t, ss, "alice")

	tokens := make([]string, 3)
	for i := range tokens {
		tok, _, err := ss.Create("alice", time.Hour)
		if err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
		tokens[i] = tok
	}

	for i, tok := range tokens {
		if _, err := ss.Validate(tok); err != nil {
			t.Errorf("session %d should be valid: %v", i, err)
		}
	}

	if err := ss.Delete(tokens[1]); err != nil {
		t.Fatalf("delete middle session: %v", err)
	}

	if _, err := ss.Validate(tokens[0]); err != nil {
		t.Error("session 0 should still be valid")
	}
	if _, err := ss.Validate(tokens[1]); err != ErrSessionNotFound {
		t.Error("session 1 should be deleted")
	}
	if _, err := ss.Validate(tokens[2]); err != nil {
		t.Error("session 2 should still be valid")
	}
}

func TestSessionStore_StartPurge(t *testing.T) {
	ss := openTestDBWithSessions(t)
	createTestUser(t, ss, "alice")

	if _, _, err := ss.Create("alice", -time.Second); err != nil {
		t.Fatalf("create expired: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ss.StartPurge(ctx, 50*time.Millisecond, slog.Default())

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("purge goroutine did not clean expired session within timeout")
		default:
			n, err := ss.PurgeExpired()
			if err != nil {
				t.Fatalf("purge check: %v", err)
			}
			if n == 0 {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestSessionStore_CreateEmptyUserIDRejected(t *testing.T) {
	ss := openTestDBWithSessions(t)

	_, _, err := ss.Create("", time.Hour)
	if err == nil {
		t.Fatal("expected error for empty user_id")
	}
}

func TestSessionStore_TokenUniqueness(t *testing.T) {
	ss := openTestDBWithSessions(t)
	createTestUser(t, ss, "alice")

	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		tok, _, err := ss.Create("alice", time.Hour)
		if err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
		if seen[tok] {
			t.Fatalf("duplicate token on iteration %d", i)
		}
		seen[tok] = true
	}
}
