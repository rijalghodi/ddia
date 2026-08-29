package appendonlylog

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

const (
	opPut    byte = 0
	opDelete byte = 1

	opOffset    = 0
	keyOffset   = 1
	valueOffset = 5
	headerSize  = 9
)

type LogDB struct {
	file  *os.File
	index *HashIndex
}

func Open(path string) (*LogDB, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	index := NewHashIndex(file)

	return &LogDB{
		file:  file,
		index: index,
	}, nil
}

func (db *LogDB) Put(key string, value string) error {
	offset, err := db.appendRecord(opPut, key, value)
	if err != nil {
		return err
	}

	db.index.Put(key, offset)
	return nil
}

func (db *LogDB) Get(key string) (string, bool, error) {
	offset, ok := db.index.Get(key)
	if !ok {
		return "", false, nil
	}

	readKey, value, _, err := db.readRecordAt(offset)
	if err != nil {
		return "", false, err
	}

	if readKey != key {
		return "", false, fmt.Errorf("index points to wrong key")
	}

	return value, true, nil
}

func (db *LogDB) Delete(key string) error {
	_, err := db.appendRecord(opDelete, key, "")
	if err != nil {
		return err
	}

	db.index.Delete(key)
	return nil
}

func (db *LogDB) Close() error {
	return db.file.Close()
}

// appendRecord writes a new record to the end of the log and returns its offset.
func (db *LogDB) appendRecord(op byte, key, value string) (int64, error) {
	info, err := db.file.Stat()
	if err != nil {
		return 0, err
	}

	offset := info.Size()

	keyValueBytes, err := keyValueToBytes(op, key, value)
	if err != nil {
		return 0, err
	}

	if _, err := db.file.Write(keyValueBytes); err != nil {
		return 0, errors.New("failed to write file")
	}

	return offset, nil
}

func (db *LogDB) readRecordAt(offset int64) (key, value string, size int64, err error) {
	_, err = db.file.Seek(offset, io.SeekStart)
	if err != nil {
		return "", "", 0, err
	}

	op, key, value, size, err := readRecord(db.file)
	if err != nil {
		return "", "", 0, err
	}

	if op == opDelete {
		return "", "", size, nil
	}

	return key, value, size, nil
}

// readRecord reads a single record (op byte, header, key, value) from the reader.
func readRecord(r io.Reader) (op byte, key, value string, size int64, err error) {
	header := make([]byte, headerSize)
	if _, err = io.ReadFull(r, header); err != nil {
		return 0, "", "", 0, err
	}

	op = header[opOffset]
	keyLen := binary.BigEndian.Uint32(header[keyOffset:valueOffset])
	valueLen := binary.BigEndian.Uint32(header[valueOffset:headerSize])

	keyBytes := make([]byte, keyLen)
	if _, err = io.ReadFull(r, keyBytes); err != nil {
		return 0, "", "", 0, err
	}

	valueBytes := make([]byte, valueLen)
	if _, err = io.ReadFull(r, valueBytes); err != nil {
		return 0, "", "", 0, err
	}

	key = string(keyBytes)
	value = string(valueBytes)
	size = int64(headerSize) + int64(keyLen) + int64(valueLen)

	return op, key, value, size, nil
}

func keyValueToBytes(op byte, key, value string) ([]byte, error) {
	if len(key) > math.MaxUint32 {
		return nil, errors.New("key too large")
	}
	if len(value) > math.MaxUint32 {
		return nil, errors.New("value too large")
	}

	keyLen := uint32(len(key))
	valueLen := uint32(len(value))

	buf := make([]byte, headerSize+int(keyLen)+int(valueLen))

	buf[opOffset] = op
	binary.BigEndian.PutUint32(buf[keyOffset:valueOffset], keyLen)
	binary.BigEndian.PutUint32(buf[valueOffset:headerSize], valueLen)

	copy(buf[headerSize:], key)
	copy(buf[headerSize+int(keyLen):], value)

	return buf, nil
}
