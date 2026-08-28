package appendonlylog

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) (*LogDB, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "db.log")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	return db, path
}

func TestOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.log")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected log file to exist: %v", err)
	}
}

func TestPutAndGet(t *testing.T) {
	db, _ := newTestDB(t)
	defer db.Close()

	if err := db.Put("name", "Max"); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	value, ok, err := db.Get("name")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if !ok {
		t.Fatal("expected key to exist")
	}

	if value != "Max" {
		t.Fatalf("expected Max, got %q", value)
	}
}

func TestUpdate(t *testing.T) {
	db, _ := newTestDB(t)
	defer db.Close()

	if err := db.Put("name", "Max"); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if err := db.Put("name", "John"); err != nil {
		t.Fatalf("Put() update error = %v", err)
	}

	value, ok, err := db.Get("name")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if !ok {
		t.Fatal("expected key to exist")
	}

	if value != "John" {
		t.Fatalf("expected John, got %q", value)
	}
}

func TestGetMissingKey(t *testing.T) {
	db, _ := newTestDB(t)
	defer db.Close()

	value, ok, err := db.Get("unknown")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if ok {
		t.Fatal("expected key not to exist")
	}

	if value != "" {
		t.Fatalf("expected empty value, got %q", value)
	}
}

func TestDelete(t *testing.T) {
	db, _ := newTestDB(t)
	defer db.Close()

	if err := db.Put("name", "Max"); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if err := db.Delete("name"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, ok, err := db.Get("name")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestDeleteMissingKey(t *testing.T) {
	db, _ := newTestDB(t)
	defer db.Close()

	if err := db.Delete("unknown"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, ok, err := db.Get("unknown")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if ok {
		t.Fatal("expected key not to exist")
	}
}

func TestMultipleKeys(t *testing.T) {
	db, _ := newTestDB(t)
	defer db.Close()

	data := map[string]string{
		"name": "Max",
		"age":  "25",
		"city": "Jakarta",
	}

	for key, value := range data {
		if err := db.Put(key, value); err != nil {
			t.Fatalf("Put(%q) error = %v", key, err)
		}
	}

	for key, expected := range data {
		value, ok, err := db.Get(key)
		if err != nil {
			t.Fatalf("Get(%q) error = %v", key, err)
		}

		if !ok {
			t.Fatalf("expected %q to exist", key)
		}

		if value != expected {
			t.Fatalf("Get(%q) = %q, want %q", key, value, expected)
		}
	}
}

func TestPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.log")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if err := db.Put("name", "Max"); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	db, err = Open(path)
	if err != nil {
		t.Fatalf("Open() after close error = %v", err)
	}
	defer db.Close()

	value, ok, err := db.Get("name")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if !ok {
		t.Fatal("expected key to exist after reopening")
	}

	if value != "Max" {
		t.Fatalf("expected Max, got %q", value)
	}
}

func TestDeletePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.log")

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if err := db.Put("name", "Max"); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if err := db.Delete("name"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	db, err = Open(path)
	if err != nil {
		t.Fatalf("Open() after close error = %v", err)
	}
	defer db.Close()

	_, ok, err := db.Get("name")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if ok {
		t.Fatal("expected deleted key not to exist after reopening")
	}
}

func TestClose(t *testing.T) {
	db, _ := newTestDB(t)

	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestOpenInvalidPath(t *testing.T) {
	path := filepath.Join(
		t.TempDir(),
		"nonexistent",
		"directory",
		"db.log",
	)

	_, err := Open(path)
	if err == nil {
		t.Fatal("expected Open() to return an error")
	}
}
