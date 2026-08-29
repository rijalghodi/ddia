package segmentedlog

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	opPut    byte = 0
	opDelete byte = 1

	opOffset    = 0
	keyOffset   = 1
	valueOffset = 5
	headerSize  = 9
	dbFileExt   = ".log"
)

type SegmentedLog struct {
	dir                 string
	maxSegmentSize      int64
	compactionThreshold int64

	mu            sync.RWMutex
	isCompacting  bool
	activeSegment *Segment
	segments      map[int64]*Segment // Maps Segment ID to Segment
	index         *HashIndex
}

// Open initializes the segmented log database.
func Open(dir string, maxSegmentSize, compactionThreshold int64) (*SegmentedLog, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db := &SegmentedLog{
		dir:                 dir,
		maxSegmentSize:      maxSegmentSize,
		compactionThreshold: compactionThreshold,
		segments:            make(map[int64]*Segment),
		index:               NewHashIndex(),
	}

	err := db.loadSegments()
	if err != nil {
		return nil, err
	}

	if err := db.createAndActivateSegment(); err != nil {
		return nil, err
	}

	return db, nil
}

// Close gracefully closes all segment files.
func (db *SegmentedLog) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	for _, seg := range db.segments {
		seg.File.Close()
	}
	return nil
}

// Put put values to segments
func (db *SegmentedLog) Put(key string, value string) error {
	keyValueBytes, err := keyValueToBytes(opPut, key, value)
	if err != nil {
		return err
	}

	db.mu.Lock()
	if db.activeSegment.Size+int64(len(keyValueBytes)) > db.maxSegmentSize {
		if err := db.createAndActivateSegment(); err != nil {
			db.mu.Unlock()
			return err
		}
	}

	offset, err := db.activeSegment.AppendBytes(keyValueBytes)
	segID := db.activeSegment.ID
	db.mu.Unlock()

	if err != nil {
		return err
	}

	db.index.Put(key, RecordLocation{
		SegmentID: segID,
		Offset:    offset,
	})

	db.triggerCompactionIfNeeded()

	return nil
}

// Get retrieve values from segments
func (db *SegmentedLog) Get(key string) (string, bool, error) {
	recordLocation, ok := db.index.Get(key)
	if !ok {
		return "", false, nil
	}

	db.mu.RLock()
	seg, ok := db.segments[recordLocation.SegmentID]
	db.mu.RUnlock()

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

// Delete mark the value as deleted and remove it from index.
func (db *SegmentedLog) Delete(key string) error {
	keyValueBytes, err := keyValueToBytes(opDelete, key, "")
	if err != nil {
		return err
	}

	db.mu.Lock()
	if db.activeSegment.Size+int64(len(keyValueBytes)) > db.maxSegmentSize {
		if err := db.createAndActivateSegment(); err != nil {
			db.mu.Unlock()
			return err
		}
	}

	_, err = db.activeSegment.AppendBytes(keyValueBytes)
	db.mu.Unlock()

	if err != nil {
		return err
	}

	db.index.Delete(key)

	db.triggerCompactionIfNeeded()

	return nil
}

func (db *SegmentedLog) triggerCompactionIfNeeded() {
	db.mu.Lock()
	shouldCompact := !db.isCompacting && int64(len(db.segments)) > db.compactionThreshold
	if shouldCompact {
		db.isCompacting = true
		go db.compactInternal()
	}
	db.mu.Unlock()
}

// Compact merges inactive segments to reclaim space
func (db *SegmentedLog) Compact() error {
	db.mu.Lock()
	if db.isCompacting {
		db.mu.Unlock()
		return nil
	}
	db.isCompacting = true
	db.mu.Unlock()

	return db.compactInternal()
}

func (db *SegmentedLog) compactInternal() error {
	defer func() {
		db.mu.Lock()
		db.isCompacting = false
		db.mu.Unlock()
	}()

	inactiveIDs, compactID := db.identifyInactiveSegments()
	if len(inactiveIDs) <= 1 {
		return nil // Nothing to compact or only 1 segment
	}

	newLocations, err := db.writeCompactedData(compactID, inactiveIDs)
	if err != nil {
		return err
	}

	if err := db.swapCompactedSegments(compactID, inactiveIDs); err != nil {
		return err
	}

	db.updateIndexAfterCompaction(compactID, inactiveIDs, newLocations)
	return nil
}

// createSegment creates a new segment file and returns the Segment struct.
func (db *SegmentedLog) createSegment(id int64) (*Segment, error) {
	path := filepath.Join(db.dir, fmt.Sprintf("%d%s", id, dbFileExt))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	return &Segment{
		ID:   id,
		File: file,
		Size: info.Size(),
	}, nil
}

// loadSegments reads the directory for existing .log files and populates db.segments and db.index.
func (db *SegmentedLog) loadSegments() error {
	entries, err := os.ReadDir(db.dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), dbFileExt) {
			continue
		}

		idStr := strings.TrimSuffix(entry.Name(), dbFileExt)
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			continue
		}

		seg, err := db.createSegment(id)
		if err != nil {
			return err
		}
		db.segments[id] = seg

		// Rebuild index from this segment
		if err := db.index.PopulateHashIndex(seg); err != nil {
			return err
		}
	}
	return nil
}

func (db *SegmentedLog) identifyInactiveSegments() (inactiveIDs []int64, compactID int64) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	compactID = -1
	for id := range db.segments {
		if id == db.activeSegment.ID {
			continue
		}
		inactiveIDs = append(inactiveIDs, id)
		if id > compactID {
			compactID = id
		}
	}
	return inactiveIDs, compactID
}

func (db *SegmentedLog) writeCompactedData(compactID int64, inactiveIDs []int64) (map[string]int64, error) {
	tmpPath := filepath.Join(db.dir, fmt.Sprintf("%d%s.tmp", compactID, dbFileExt))
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	// We do NOT defer os.Remove(tmpPath) here because we need it to survive
	// for the atomic swap in the next step. If an error occurs, we manually remove it.
	defer tmpFile.Close()

	newLocations := make(map[string]int64)
	var currentOffset int64 = 0

	snapshot := db.index.Snapshot()
	for key, loc := range snapshot {
		if !slices.Contains(inactiveIDs, loc.SegmentID) {
			continue
		}

		db.mu.RLock()
		seg := db.segments[loc.SegmentID]
		db.mu.RUnlock()

		readKey, value, size, err := seg.ReadRecordAt(loc.Offset)
		if err != nil || readKey != key {
			continue
		}

		keyValueBytes, err := keyValueToBytes(opPut, key, value)
		if err != nil {
			os.Remove(tmpPath)
			return nil, err
		}

		if _, err := tmpFile.Write(keyValueBytes); err != nil {
			os.Remove(tmpPath)
			return nil, err
		}

		newLocations[key] = currentOffset
		currentOffset += size
	}

	if err := tmpFile.Sync(); err != nil {
		os.Remove(tmpPath)
		return nil, err
	}

	return newLocations, nil
}

func (db *SegmentedLog) swapCompactedSegments(compactID int64, inactiveIDs []int64) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Close old files to allow rename/deletion
	for _, id := range inactiveIDs {
		db.segments[id].File.Close()
	}

	tmpPath := filepath.Join(db.dir, fmt.Sprintf("%d%s.tmp", compactID, dbFileExt))
	finalPath := filepath.Join(db.dir, fmt.Sprintf("%d%s", compactID, dbFileExt))

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return err
	}

	// Remove old segments (except the one we just renamed over)
	for _, id := range inactiveIDs {
		if id != compactID {
			os.Remove(filepath.Join(db.dir, fmt.Sprintf("%d%s", id, dbFileExt)))
		}
		delete(db.segments, id)
	}

	// Reopen the newly compacted segment
	compactedSeg, err := db.createSegment(compactID)
	if err != nil {
		return err
	}
	db.segments[compactID] = compactedSeg

	return nil
}

func (db *SegmentedLog) updateIndexAfterCompaction(compactID int64, inactiveIDs []int64, newLocations map[string]int64) {
	for key, offset := range newLocations {
		// Only update if it hasn't been overwritten by a newer write
		if loc, ok := db.index.Get(key); ok {
			if slices.Contains(inactiveIDs, loc.SegmentID) {
				db.index.Put(key, RecordLocation{SegmentID: compactID, Offset: offset})
			}
		}
	}
}

func (db *SegmentedLog) createAndActivateSegment() error {
	id := time.Now().UnixNano()
	seg, err := db.createSegment(id)
	if err != nil {
		return err
	}
	db.activeSegment = seg
	db.segments[id] = seg
	return nil
}
