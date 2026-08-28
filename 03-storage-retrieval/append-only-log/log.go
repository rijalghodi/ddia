package appendonlylog

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/rijalghodi/ddia/03-storage-retrieval/hashindex"
)

const (
	opPut    byte = 0
	opDelete byte = 1
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
		// Reset the cursor
		_, err := file.Seek(offset, io.SeekStart)
		if err != nil {
			return err
		}

		// Read op
		op := []byte{0x01}
		_, err = io.ReadFull(file, op)
		if err != nil {
			return err
		}

		// Read header
		header := make([]byte, 8)

		_, err = io.ReadFull(file, header)
		if err != nil {
			return err
		}

		// Read key and value
		keyLen := binary.BigEndian.Uint32(header[0:4])
		valueLen := binary.BigEndian.Uint32(header[4:8])

		keyBytes := make([]byte, keyLen)
		_, err = io.ReadFull(file, keyBytes)
		if err != nil {
			return err
		}

		switch op[0] {
		case opPut:
			index.Put(string(keyBytes), offset)
		case opDelete:
			index.Delete(string(keyBytes))
		}

		offset = offset + 1 + 8 + int64(keyLen) + int64(valueLen)
	}

	return nil
}

func (db *LogDB) Put(key string, value string) error {

	info, err := db.file.Stat()
	if err != nil {
		return err
	}

	size := info.Size()

	// Append file
	keyValueBytes := keyValueToBytes(key, value, false)
	if _, err := db.file.Write(keyValueBytes); err != nil {
		return errors.New("failed to write file")
	}

	db.index.Put(key, size)

	return nil
}

func (db *LogDB) Get(key string) (string, bool, error) {
	// Get offset from index
	offset, ok := db.index.Get(key)
	if !ok {
		return "", false, nil
	}

	// Move cursor to record
	_, err := db.file.Seek(offset, io.SeekStart)
	if err != nil {
		return "", false, err
	}

	// Read op
	op := []byte{0x01}
	_, err = io.ReadFull(db.file, op)
	if err != nil {
		return "", false, err
	}

	// Read header
	header := make([]byte, 8)
	_, err = io.ReadFull(db.file, header)
	if err != nil {
		return "", false, err
	}

	keyLen := binary.BigEndian.Uint32(header[0:4])
	valueLen := binary.BigEndian.Uint32(header[4:8])

	// Read key
	keyBytes := make([]byte, keyLen)
	_, err = io.ReadFull(db.file, keyBytes)
	if err != nil {
		return "", false, err
	}

	// Verify key
	if string(keyBytes) != key {
		return "", false, fmt.Errorf("index points to wrong key")
	}

	// Read value
	valueBytes := make([]byte, valueLen)
	_, err = io.ReadFull(db.file, valueBytes)
	if err != nil {
		return "", false, err
	}

	return string(valueBytes), true, nil
}

func (db *LogDB) Delete(key string) error {
	_, err := db.file.Stat()
	if err != nil {
		return err
	}

	// append file
	if _, err := db.file.Write(keyValueToBytes(key, "", true)); err != nil {
		return errors.New("failed to write file")
	}

	db.index.Delete(key)

	return nil
}

func (db *LogDB) Close() error {
	return db.file.Close()
}

func keyValueToBytes(key, value string, delete bool) []byte {
	keyLen := uint32(len(key))
	valueLen := uint32(len(value))

	buf := make([]byte, 1+8+len(key)+len(value))

	opByte := opPut
	if delete {
		opByte = opDelete
	}

	buf[0] = opByte

	binary.BigEndian.PutUint32(buf[1:5], keyLen)
	binary.BigEndian.PutUint32(buf[5:9], valueLen)

	copy(buf[9:], key)
	copy(buf[9+len(key):], value)

	return buf
}
