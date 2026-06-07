package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDB(t *testing.T) *Registry {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	reg, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { reg.Close() })
	return reg
}

func seedInstance(id string, port int) Instance {
	return Instance{
		ID:          id,
		ImageDigest: "sha256:abc123",
		Port:        port,
		DataDir:     "/data/" + id,
		Status:      "running",
		CreatedAt:   "2026-01-01T00:00:00Z",
	}
}

func TestCreateAndGet(t *testing.T) {
	reg := tempDB(t)
	inst := seedInstance("fox-1", 9001)

	if err := reg.Create(inst); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := reg.Get("fox-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != inst.ID || got.Port != inst.Port || got.ImageDigest != inst.ImageDigest {
		t.Errorf("Get returned %+v, want %+v", got, inst)
	}
}

func TestCreateIdempotent(t *testing.T) {
	reg := tempDB(t)
	inst := seedInstance("fox-1", 9001)
	if err := reg.Create(inst); err != nil {
		t.Fatalf("Create: %v", err)
	}

	inst.ImageDigest = "sha256:updated"
	inst.Status = "stopped"
	if err := reg.Create(inst); err != nil {
		t.Fatalf("Create (upsert): %v", err)
	}

	got, err := reg.Get("fox-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ImageDigest != "sha256:updated" {
		t.Errorf("ImageDigest = %q, want %q", got.ImageDigest, "sha256:updated")
	}
	if got.Status != "stopped" {
		t.Errorf("Status = %q, want %q", got.Status, "stopped")
	}
}

func TestCreateDefaultsCreatedAt(t *testing.T) {
	reg := tempDB(t)
	inst := seedInstance("fox-1", 9001)
	inst.CreatedAt = ""

	if err := reg.Create(inst); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := reg.Get("fox-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CreatedAt == "" {
		t.Error("CreatedAt should be auto-populated when empty")
	}
}

func TestGetNotFound(t *testing.T) {
	reg := tempDB(t)
	_, err := reg.Get("nonexistent")
	if err != ErrNotFound {
		t.Errorf("Get(nonexistent) = %v, want ErrNotFound", err)
	}
}

func TestList(t *testing.T) {
	reg := tempDB(t)
	a := seedInstance("fox-a", 9001)
	a.CreatedAt = "2026-01-01T00:00:00Z"
	b := seedInstance("fox-b", 9002)
	b.CreatedAt = "2026-01-02T00:00:00Z"

	if err := reg.Create(a); err != nil {
		t.Fatal(err)
	}
	if err := reg.Create(b); err != nil {
		t.Fatal(err)
	}

	list, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List returned %d items, want 2", len(list))
	}
	if list[0].ID != "fox-a" || list[1].ID != "fox-b" {
		t.Errorf("List order: got [%s, %s], want [fox-a, fox-b]", list[0].ID, list[1].ID)
	}
}

func TestListEmpty(t *testing.T) {
	reg := tempDB(t)
	list, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List returned %d items, want 0", len(list))
	}
}

func TestUpdateStatus(t *testing.T) {
	reg := tempDB(t)
	if err := reg.Create(seedInstance("fox-1", 9001)); err != nil {
		t.Fatal(err)
	}

	if err := reg.UpdateStatus("fox-1", "stopped"); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	got, _ := reg.Get("fox-1")
	if got.Status != "stopped" {
		t.Errorf("Status = %q, want %q", got.Status, "stopped")
	}
}

func TestUpdateStatusNotFound(t *testing.T) {
	reg := tempDB(t)
	err := reg.UpdateStatus("nonexistent", "stopped")
	if err != ErrNotFound {
		t.Errorf("UpdateStatus(nonexistent) = %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	reg := tempDB(t)
	if err := reg.Create(seedInstance("fox-1", 9001)); err != nil {
		t.Fatal(err)
	}

	if err := reg.Delete("fox-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := reg.Get("fox-1")
	if err != ErrNotFound {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	reg := tempDB(t)
	err := reg.Delete("nonexistent")
	if err != ErrNotFound {
		t.Errorf("Delete(nonexistent) = %v, want ErrNotFound", err)
	}
}

func TestUsedPorts(t *testing.T) {
	reg := tempDB(t)
	if err := reg.Create(seedInstance("fox-1", 9001)); err != nil {
		t.Fatal(err)
	}
	if err := reg.Create(seedInstance("fox-2", 9002)); err != nil {
		t.Fatal(err)
	}

	ports, err := reg.UsedPorts()
	if err != nil {
		t.Fatalf("UsedPorts: %v", err)
	}
	if len(ports) != 2 {
		t.Fatalf("UsedPorts returned %d ports, want 2", len(ports))
	}
	if !ports[9001] || !ports[9002] {
		t.Errorf("UsedPorts = %v, want {9001: true, 9002: true}", ports)
	}
}

func TestUsedPortsEmpty(t *testing.T) {
	reg := tempDB(t)
	ports, err := reg.UsedPorts()
	if err != nil {
		t.Fatalf("UsedPorts: %v", err)
	}
	if len(ports) != 0 {
		t.Errorf("UsedPorts returned %d ports, want 0", len(ports))
	}
}

func TestOpenInvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/dir/test.db")
	if err == nil {
		t.Error("Open with invalid path should fail")
	}
}

func TestPortUniqueConstraint(t *testing.T) {
	reg := tempDB(t)
	if err := reg.Create(seedInstance("fox-1", 9001)); err != nil {
		t.Fatal(err)
	}
	err := reg.Create(seedInstance("fox-2", 9001))
	if err == nil {
		t.Error("Create with duplicate port should fail")
	}
}

func TestWALEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal-test.db")
	reg, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer reg.Close()

	_, err = os.Stat(path + "-wal")
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("stat WAL file: %v", err)
	}
}
