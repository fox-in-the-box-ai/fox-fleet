package source

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenRegistry(t *testing.T) {
	db := openTestDB(t)
	reg, err := OpenRegistry(db)
	if err != nil {
		t.Fatalf("OpenRegistry: %v", err)
	}
	if reg == nil {
		t.Fatal("registry is nil")
	}
}

func TestCRUD(t *testing.T) {
	db := openTestDB(t)
	reg, err := OpenRegistry(db)
	if err != nil {
		t.Fatal(err)
	}

	src := Source{
		ID:         "src-1",
		Type:       "file",
		Name:       "Test Docs",
		Collection: "docs",
		Config:     map[string]string{"path": "/data/docs"},
	}
	if err := reg.Create(src); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := reg.Get("src-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Test Docs" {
		t.Errorf("Name = %q, want %q", got.Name, "Test Docs")
	}
	if got.Status != "pending" {
		t.Errorf("Status = %q, want pending", got.Status)
	}
	if got.CreatedAt == "" {
		t.Error("CreatedAt should be auto-set")
	}
	if got.Config["path"] != "/data/docs" {
		t.Errorf("Config[path] = %q, want /data/docs", got.Config["path"])
	}

	list, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}
	if list[0].Config["path"] != "/data/docs" {
		t.Errorf("List Config[path] = %q, want /data/docs", list[0].Config["path"])
	}

	if err := reg.UpdateStatus("src-1", "active"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	got, _ = reg.Get("src-1")
	if got.Status != "active" {
		t.Errorf("Status = %q after update, want active", got.Status)
	}

	if err := reg.UpdateCounts("src-1", 10, 50); err != nil {
		t.Fatalf("UpdateCounts: %v", err)
	}
	got, _ = reg.Get("src-1")
	if got.DocCount != 10 || got.ChunkCount != 50 {
		t.Errorf("counts = %d/%d, want 10/50", got.DocCount, got.ChunkCount)
	}

	if err := reg.SetError("src-1", "connection timeout"); err != nil {
		t.Fatalf("SetError: %v", err)
	}
	got, _ = reg.Get("src-1")
	if got.Status != "error" {
		t.Errorf("Status = %q after SetError, want error", got.Status)
	}
	if got.ErrorDetail != "connection timeout" {
		t.Errorf("ErrorDetail = %q", got.ErrorDetail)
	}

	if err := reg.Delete("src-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = reg.Get("src-1")
	if err != ErrNotFound {
		t.Errorf("Get after delete: %v, want ErrNotFound", err)
	}
}

func TestGetNotFound(t *testing.T) {
	db := openTestDB(t)
	reg, _ := OpenRegistry(db)

	_, err := reg.Get("nonexistent")
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateStatusNotFound(t *testing.T) {
	db := openTestDB(t)
	reg, _ := OpenRegistry(db)

	err := reg.UpdateStatus("nonexistent", "active")
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	db := openTestDB(t)
	reg, _ := OpenRegistry(db)

	err := reg.Delete("nonexistent")
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestDuplicateCreate(t *testing.T) {
	db := openTestDB(t)
	reg, _ := OpenRegistry(db)

	src := Source{ID: "src-1", Type: "file", Name: "A", Collection: "docs"}
	if err := reg.Create(src); err != nil {
		t.Fatal(err)
	}
	err := reg.Create(src)
	if err == nil {
		t.Error("expected error on duplicate create")
	}
}
