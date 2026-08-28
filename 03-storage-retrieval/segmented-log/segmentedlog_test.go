package segmentedlog

import (
	"os"
	"strconv"
	"testing"
)

func TestSegmentedLog_PutAndGet(t *testing.T) {
	dir, err := os.MkdirTemp("", "segmented_log_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := Open(dir, 1024, 3)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Put data
	err = db.Put("key1", "value1")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Get data
	val, ok, err := db.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !ok {
		t.Fatal("Expected key1 to be found")
	}
	if val != "value1" {
		t.Fatalf("Expected value1, got %s", val)
	}
}

func TestSegmentedLog_SegmentRotation(t *testing.T) {
	dir, err := os.MkdirTemp("", "segmented_log_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Very small max segment size to force rotation quickly
	// header is 9 bytes. key="k", val="v" -> 2 bytes. Total 11 bytes per record.
	// Max size 20 means it should rotate after 1 record.
	db, err := Open(dir, 20, 3)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	for i := 0; i < 5; i++ {
		key := "k" + strconv.Itoa(i)
		val := "v" + strconv.Itoa(i)
		if err := db.Put(key, val); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	// We wrote 5 records, each 11 bytes. Max size 20.
	// Segment 0: k0 (11 bytes), k1 (11 bytes - total 22 bytes, exceeds 20, rotates *after* write? or before?)
	// According to our logic in Task 1, rotation happens if adding the new record exceeds max size.
	// So:
	// Insert k0 (11 bytes) -> seg 0 size = 11.
	// Insert k1 (11 bytes). 11 + 11 = 22 > 20. Rotate to seg 1! seg 1 size = 11.
	// Insert k2 (11 bytes) -> 11 + 11 = 22 > 20. Rotate to seg 2!
	// So we should have multiple segments.
	if len(db.segments) < 3 {
		t.Fatalf("Expected multiple segments due to rotation, got %d", len(db.segments))
	}

	// Ensure we can still read all of them
	for i := 0; i < 5; i++ {
		key := "k" + strconv.Itoa(i)
		expectedVal := "v" + strconv.Itoa(i)
		
		val, ok, err := db.Get(key)
		if err != nil {
			t.Fatalf("Get failed for %s: %v", key, err)
		}
		if !ok {
			t.Fatalf("Expected %s to be found", key)
		}
		if val != expectedVal {
			t.Fatalf("Expected %s, got %s", expectedVal, val)
		}
	}
}

func TestSegmentedLog_Delete(t *testing.T) {
	dir, err := os.MkdirTemp("", "segmented_log_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := Open(dir, 1024, 3)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	db.Put("key1", "value1")
	db.Delete("key1")

	_, ok, err := db.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if ok {
		t.Fatal("Expected key1 to be deleted")
	}
}

func TestSegmentedLog_Compaction(t *testing.T) {
	dir, err := os.MkdirTemp("", "segmented_log_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := Open(dir, 50, 3)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Write key1 multiple times to older segments
	db.Put("key1", "val1")
	db.Put("key1", "val2")
	db.Put("key1", "val3")
	db.Put("key2", "val_a")
	db.Put("key2", "val_b")
	db.Put("key3", "val_c")
	db.Delete("key3")

	// We should have multiple segments now.
	numSegmentsBefore := len(db.segments)
	
	err = db.Compact()
	if err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	numSegmentsAfter := len(db.segments)
	if numSegmentsAfter >= numSegmentsBefore {
		t.Fatalf("Expected number of segments to decrease after compaction. Before: %d, After: %d", numSegmentsBefore, numSegmentsAfter)
	}

	// Verify data is correct after compaction
	val, ok, _ := db.Get("key1")
	if !ok || val != "val3" {
		t.Fatalf("Expected key1=val3, got %s (ok=%v)", val, ok)
	}

	val, ok, _ = db.Get("key2")
	if !ok || val != "val_b" {
		t.Fatalf("Expected key2=val_b, got %s (ok=%v)", val, ok)
	}

	_, ok, _ = db.Get("key3")
	if ok {
		t.Fatal("Expected key3 to remain deleted after compaction")
	}
	
	db.Close()
}
