package appendonlylog

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/rijalghodi/ddia/03-storage-retrieval/hashindex"
)

const (
	opPut    byte = 0
	opDelete byte = 1

	opSize       = 1
	keyLenSize   = 4
	valueLenSize = 4
	headerSize   = opSize + keyLenSize + valueLenSize // 9 bytes
)

type LogDB struct {
	file  *os.File
	index *hashindex.HashIndex
}

func Open(path string) (*LogDB, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	index := hashindex.New()

	if err := populateHashIndex(file, index); err != nil {
		return nil, fmt.Errorf("populate hash index error = %v", err)
	}

	return &LogDB{
		file:  file,
		index: index,
	}, nil
}

func populateHashIndex(file *os.File, index *hashindex.HashIndex) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}

	size := info.Size()
	offset := int64(0)

	for offset < size {
		_, err := file.Seek(offset, io.SeekStart)
		if err != nil {
			return err
		}

		op, key, _, recordSize, err := readRecord(file)
		if err != nil {
			return err
		}

		switch op {
		case opPut:
			index.Put(key, offset)
		case opDelete:
			index.Delete(key)
		}

		offset += recordSize
	}

	return nil
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

	_, err := db.file.Seek(offset, io.SeekStart)
	if err != nil {
		return "", false, err
	}

	_, readKey, value, _, err := readRecord(db.file)
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

// readRecord reads a single record (op byte, header, key, value) from the reader.
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

	buf[0] = op
	binary.BigEndian.PutUint32(buf[1:5], keyLen)
	binary.BigEndian.PutUint32(buf[5:9], valueLen)

	copy(buf[headerSize:], key)
	copy(buf[headerSize+int(keyLen):], value)

	return buf, nil
}
