package segmentedlog

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
)

// readRecord reads a single record from the reader.
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

	return op, string(keyBytes), string(valueBytes), int64(headerSize + keyLen + valueLen), nil
}

func keyValueToBytes(op byte, key, value string) ([]byte, error) {
	if len(key) > math.MaxUint32 || len(value) > math.MaxUint32 {
		return nil, errors.New("key or value too large")
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
