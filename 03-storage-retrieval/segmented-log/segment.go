package segmentedlog

import (
	"io"
	"os"
)

// Segment represents a single log file segment
type Segment struct {
	ID   int64
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
