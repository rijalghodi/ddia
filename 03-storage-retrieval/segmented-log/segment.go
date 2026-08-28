package segmentedlog

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	opPut    byte = 0
	opDelete byte = 1

	headerSize = 9 // 1 byte op + 4 bytes keyLen + 4 bytes valueLen
	dbFileExt  = ".log"
)

// Segment represents a single log file segment
type Segment struct {
	ID   int
	File *os.File
	Size int64
}

func (s *Segment) AppendBytes(data []byte) (int64, error) {
	offset, err := s.File.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	if _, err = s.File.Write(data); err != nil {
		return 0, err
	}

	s.Size += int64(len(data))
	return offset, nil
}

func (s *Segment) ReadRecordAt(offset int64) (key, value string, size int64, err error) {
	_, err = s.File.Seek(offset, io.SeekStart)
	if err != nil {
		return "", "", 0, err
	}

	op, key, value, size, err := readRecord(s.File)
	if err != nil {
		return "", "", 0, err
	}

	if op == opDelete {
		return "", "", size, nil
	}

	return key, value, size, nil
}

// createSegment creates a new segment file and returns the Segment struct.
func (db *SegmentedLog) createSegment(id int) (*Segment, error) {
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
		id, err := strconv.Atoi(idStr)
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

// readRecord reads a single record from the reader.
func readRecord(r io.Reader) (op byte, key, value string, size int64, err error) {
	header := make([]byte, headerSize)
	if _, err = io.ReadFull(r, header); err != nil {
		return 0, "", "", 0, err
	}

	op = header[0]
	keyLen := binary.BigEndian.Uint32(header[1:5])
	valueLen := binary.BigEndian.Uint32(header[5:9])

	keyBytes := make([]byte, keyLen)
	if _, err = io.ReadFull(r, keyBytes); err != nil {
		return 0, "", "", 0, err
	}

	valueBytes := make([]byte, valueLen)
	if _, err = io.ReadFull(r, valueBytes); err != nil {
		return 0, "", "", 0, err
	}

	return op, string(keyBytes), string(valueBytes), int64(headerSize + keyLen + valueLen), nil
}

func keyValueToBytes(op byte, key, value string) ([]byte, error) {
	if len(key) > math.MaxUint32 || len(value) > math.MaxUint32 {
		return nil, errors.New("key or value too large")
	}

	keyLen := uint32(len(key))
	valueLen := uint32(len(value))

	buf := make([]byte, headerSize+int(keyLen)+int(valueLen))
	buf[0] = op
	binary.BigEndian.PutUint32(buf[1:5], keyLen)
	binary.BigEndian.PutUint32(buf[5:9], valueLen)

	copy(buf[headerSize:], key)
	copy(buf[headerSize+int(keyLen):], value)

	return buf, nil
}
