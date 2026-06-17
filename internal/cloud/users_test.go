package cloud

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("enable WAL: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable FK: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE instances (
		id TEXT PRIMARY KEY,
		image_digest TEXT NOT NULL,
		port INTEGER NOT NULL UNIQUE,
		data_dir TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'provisioning',
		created_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create instances table: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS cloud_users (
		username       TEXT PRIMARY KEY COLLATE NOCASE,
		password_hash  TEXT NOT NULL,
		instance_id    TEXT REFERENCES instances(id) ON DELETE SET NULL,
		created_at     TEXT NOT NULL,
		updated_at     TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create cloud_users table: %v", err)
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_cloud_users_instance ON cloud_users(instance_id) WHERE instance_id IS NOT NULL`); err != nil {
		t.Fatalf("create unique index: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(dir)
	})
	return db
}

func insertInstance(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO instances (id, image_digest, port, data_dir, status, created_at) VALUES (?, 'sha256:abc', ?, '/tmp/test', 'running', '2026-01-01T00:00:00Z')`,
		id, 8787+len(id),
	)
	if err != nil {
		t.Fatalf("insert instance %s: %v", id, err)
	}
}

func TestUserStore_CreateAndGet(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	u, err := store.Create("alice", "password123")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("username = %q, want %q", u.Username, "alice")
	}
	if u.InstanceID != nil {
		t.Errorf("instance_id = %v, want nil", u.InstanceID)
	}

	got, err := store.Get("alice")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Username != "alice" {
		t.Errorf("get username = %q, want %q", got.Username, "alice")
	}
}

func TestUserStore_CaseInsensitiveUsername(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	if _, err := store.Create("Alice", "pass1"); err != nil {
		t.Fatalf("create Alice: %v", err)
	}

	// Duplicate with different case should fail
	_, err := store.Create("alice", "pass2")
	if err != ErrUserExists {
		t.Fatalf("expected ErrUserExists, got: %v", err)
	}

	// Lookup with different case should succeed
	u, err := store.Get("ALICE")
	if err != nil {
		t.Fatalf("get ALICE: %v", err)
	}
	if u.Username != "Alice" {
		t.Errorf("username = %q, want %q (original case)", u.Username, "Alice")
	}
}

func TestUserStore_DuplicateUsername(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	if _, err := store.Create("bob", "pass1"); err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err := store.Create("bob", "pass2")
	if err != ErrUserExists {
		t.Errorf("expected ErrUserExists, got: %v", err)
	}
}

func TestUserStore_GetNotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	_, err := store.Get("nobody")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestUserStore_List(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	if _, err := store.Create("alice", "pass1"); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if _, err := store.Create("bob", "pass2"); err != nil {
		t.Fatalf("create bob: %v", err)
	}

	users, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("len = %d, want 2", len(users))
	}
}

func TestUserStore_Delete(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	if _, err := store.Create("alice", "pass1"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := store.Delete("alice"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err := store.Get("alice")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound after delete, got: %v", err)
	}
}

func TestUserStore_DeleteNotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	err := store.Delete("nobody")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestUserStore_Authenticate(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	if _, err := store.Create("alice", "correct-password"); err != nil {
		t.Fatalf("create: %v", err)
	}

	u, err := store.Authenticate("alice", "correct-password")
	if err != nil {
		t.Fatalf("authenticate with correct password: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("username = %q, want %q", u.Username, "alice")
	}

	_, err = store.Authenticate("alice", "wrong-password")
	if err != ErrInvalidCredentials {
		t.Errorf("wrong password: expected ErrInvalidCredentials, got: %v", err)
	}

	_, err = store.Authenticate("nobody", "any-password")
	if err != ErrInvalidCredentials {
		t.Errorf("nonexistent user: expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestUserStore_UpdatePassword(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	if _, err := store.Create("alice", "old-pass"); err != nil {
		t.Fatalf("create: %v", err)
	}

	newPass := "new-pass"
	_, err := store.Update("alice", &newPass, nil)
	if err != nil {
		t.Fatalf("update password: %v", err)
	}

	_, err = store.Authenticate("alice", "old-pass")
	if err != ErrInvalidCredentials {
		t.Errorf("old password should fail: %v", err)
	}

	_, err = store.Authenticate("alice", "new-pass")
	if err != nil {
		t.Errorf("new password should work: %v", err)
	}
}

func TestUserStore_SetInstanceID(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)
	insertInstance(t, db, "fox-1")

	if _, err := store.Create("alice", "pass"); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := store.SetInstanceID("alice", "fox-1"); err != nil {
		t.Fatalf("set instance_id: %v", err)
	}

	u, err := store.Get("alice")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if u.InstanceID == nil || *u.InstanceID != "fox-1" {
		t.Errorf("instance_id = %v, want fox-1", u.InstanceID)
	}

	// Clear instance_id
	if err := store.SetInstanceID("alice", ""); err != nil {
		t.Fatalf("clear instance_id: %v", err)
	}
	u, err = store.Get("alice")
	if err != nil {
		t.Fatalf("get after clear: %v", err)
	}
	if u.InstanceID != nil {
		t.Errorf("instance_id = %v, want nil after clear", u.InstanceID)
	}
}

func TestUserStore_FKOnInstanceDelete(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)
	insertInstance(t, db, "fox-1")

	if _, err := store.Create("alice", "pass"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := store.SetInstanceID("alice", "fox-1"); err != nil {
		t.Fatalf("set instance_id: %v", err)
	}

	// Delete the instance — FK ON DELETE SET NULL should clear instance_id
	if _, err := db.Exec(`DELETE FROM instances WHERE id = 'fox-1'`); err != nil {
		t.Fatalf("delete instance: %v", err)
	}

	u, err := store.Get("alice")
	if err != nil {
		t.Fatalf("get after instance delete: %v", err)
	}
	if u.InstanceID != nil {
		t.Errorf("instance_id = %v, want nil after instance delete (FK SET NULL)", u.InstanceID)
	}
}

func TestUserStore_EmptyUsernameRejected(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	_, err := store.Create("", "pass")
	if err == nil {
		t.Fatal("expected error for empty username")
	}
}

func TestUserStore_EmptyPasswordRejected(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	_, err := store.Create("alice", "")
	if err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestUserStore_UpdateNotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	pass := "new"
	_, err := store.Update("nobody", &pass, nil)
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestUserStore_UpdateInstanceID(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)
	insertInstance(t, db, "fox-2")

	if _, err := store.Create("bob", "pass"); err != nil {
		t.Fatalf("create: %v", err)
	}

	instID := "fox-2"
	u, err := store.Update("bob", nil, &instID)
	if err != nil {
		t.Fatalf("update instance_id: %v", err)
	}
	if u.InstanceID == nil || *u.InstanceID != "fox-2" {
		t.Errorf("instance_id = %v, want fox-2", u.InstanceID)
	}
}

func TestUserStore_UniqueInstanceConstraint(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)
	insertInstance(t, db, "fox-1")

	if _, err := store.Create("alice", "pass1"); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if _, err := store.Create("bob", "pass2"); err != nil {
		t.Fatalf("create bob: %v", err)
	}

	if err := store.SetInstanceID("alice", "fox-1"); err != nil {
		t.Fatalf("set alice instance: %v", err)
	}

	err := store.SetInstanceID("bob", "fox-1")
	if err == nil {
		t.Fatal("expected error assigning same instance to two users")
	}
}

func TestUserStore_AuthenticateCaseInsensitive(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	if _, err := store.Create("alice", "secret"); err != nil {
		t.Fatalf("create: %v", err)
	}

	u, err := store.Authenticate("ALICE", "secret")
	if err != nil {
		t.Fatalf("authenticate with different case: %v", err)
	}
	if u.Username != "alice" {
		t.Errorf("username = %q, want %q", u.Username, "alice")
	}
}

func TestUserStore_SetInstanceIDNotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	err := store.SetInstanceID("nobody", "fox-1")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestUserStore_UpdateEmptyPasswordRejected(t *testing.T) {
	db := openTestDB(t)
	store := NewUserStore(db)

	if _, err := store.Create("alice", "pass"); err != nil {
		t.Fatalf("create: %v", err)
	}

	empty := ""
	_, err := store.Update("alice", &empty, nil)
	if err == nil {
		t.Fatal("expected error for empty password in update")
	}
}
