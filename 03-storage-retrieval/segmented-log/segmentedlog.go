package segmentedlog

import (
	"fmt"
	"os"
)

type SegmentedLog struct {
	dir                 string
	maxSegmentSize      int64
	compactionThreshold int64

	activeSegment *Segment
	segments      map[int]*Segment // Maps Segment ID to Segment
	index         *HashIndex
}

// Open initializes the segmented log database.
// It should load existing segments from the directory and rebuild the index.
// For the boilerplate, we provide a skeleton.
func Open(dir string, maxSegmentSize, compactionThreshold int64) (*SegmentedLog, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db := &SegmentedLog{
		dir:            dir,
		maxSegmentSize: maxSegmentSize,
		compactionThreshold: compactionThreshold,
		segments:       make(map[int]*Segment),
		index:          NewHashIndex(),
	}

	// Helper to initialize. You will need to implement loading existing segments
	// and rebuilding the index as part of your tasks, or we can start fresh for now.
	// To keep it simple, let's just create segment 0 if no segments exist.

	err := db.loadSegments()
	if err != nil {
		return nil, err
	}

	if len(db.segments) == 0 {
		if err := db.createAndActivateSegment(0); err != nil {
			return nil, err
		}
	} else {
		if err := db.activateLatestSegment(); err != nil {
			return nil, err
		}
	}

	return db, nil
}

// activateLatestSegment finds the segment with the highest ID and sets it as active.
func (db *SegmentedLog) activateLatestSegment() error {
	var maxID int
	for id := range db.segments {
		if id > maxID {
			maxID = id
		}
	}
	return db.activateSegment(maxID)
}

// Close gracefully closes all segment files.
func (db *SegmentedLog) Close() error {
	for _, seg := range db.segments {
		seg.File.Close()
	}
	return nil
}

// ---------------------------------------------------------
// TASKS TO IMPLEMENT
// ---------------------------------------------------------

// Task 1: Implement Put with Segment Rotation
// 1. Convert key and value to bytes.
// 2. Check if activeSegment.Size + len(bytes) > db.maxSegmentSize.
// 3. If so, create a new segment (ID = activeSegment.ID + 1), add it to db.segments, and make it active.
// 4. Write bytes to the active segment.
// 5. Update db.index with the new RecordLocation.
// 6. Update activeSegment.Size.
func (db *SegmentedLog) Put(key string, value string) error {
	// TODO: Implement me
	// convert key and value to bytes
	keyValueBytes, err := keyValueToBytes(opPut, key, value)
	if err != nil {
		return err
	}

	if db.activeSegment.Size+int64(len(keyValueBytes)) > db.maxSegmentSize {
		if err := db.createAndActivateSegment(db.activeSegment.ID + 1); err != nil {
			return err
		}
	}

	offset, err := db.activeSegment.AppendBytes(keyValueBytes)
	if err != nil {
		return err
	}

	db.index.Put(key, RecordLocation{
		SegmentID: db.activeSegment.ID,
		Offset:    offset,
	})

	return nil
}

// Task 2: Implement Get across segments
// 1. Look up the key in db.index.
// 2. If not found, return ("", false, nil).
// 3. Find the correct segment from db.segments using the SegmentID.
// 4. Seek to the Offset and read the record using readRecord().
// 5. Return the value.
func (db *SegmentedLog) Get(key string) (string, bool, error) {
	recordLocation, ok := db.index.Get(key)
	if !ok {
		return "", false, nil
	}

	seg, ok := db.segments[recordLocation.SegmentID]
	if !ok {
		return "", false, nil
	}

	readKey, value, _, err := seg.ReadRecordAt(recordLocation.Offset)
	if err != nil {
		return "", false, err
	}

	if readKey != key {
		return "", false, fmt.Errorf("index points to wrong key")
	}

	return value, true, nil
}

// Task 3: Implement Delete
// Hint: Similar to Put, but operation is opDelete, and you remove from index.
func (db *SegmentedLog) Delete(key string) error {
	keyValueBytes, err := keyValueToBytes(opDelete, key, "")
	if err != nil {
		return err
	}

	if db.activeSegment.Size+int64(len(keyValueBytes)) > db.maxSegmentSize {
		if err := db.createAndActivateSegment(db.activeSegment.ID + 1); err != nil {
			return err
		}
	}

	_, err = db.activeSegment.AppendBytes(keyValueBytes)
	if err != nil {
		return err
	}

	db.index.Delete(key)

	return nil
}

// Task 4: Implement Compaction
//  1. Skip the active segment. We only compact older, read-only segments.
//  2. Find all unique keys that currently point to older segments.
//  3. Create a new "compacted" segment (e.g., ID can be something unique, or just write a new file).
//     (In real DDIA, we merge multiple segments. Here, let's just write all valid, non-deleted keys
//     from older segments into a single new segment file, and delete the old files).
//  4. Update the index to point to the new compacted segment.
//  5. Remove old segments from db.segments and delete their files from disk.
func (db *SegmentedLog) Compact() error {
	// TODO: Implement me
	return nil
}

// createAndActivateSegment is a helper that creates a new segment and sets it as active
func (db *SegmentedLog) createAndActivateSegment(id int) error {
	seg, err := db.createSegment(id)
	if err != nil {
		return err
	}
	db.activeSegment = seg
	db.segments[id] = seg
	return nil
}

func (db *SegmentedLog) activateSegment(id int) error {
	seg, ok := db.segments[id]
	if !ok {
		return fmt.Errorf("segment %d not found", id)
	}
	db.activeSegment = seg
	return nil
}
