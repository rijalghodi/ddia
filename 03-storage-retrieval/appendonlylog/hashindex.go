package appendonlylog

import (
	"io"
	"os"
)

type HashIndex struct {
	items map[string]int64
}

func NewHashIndex(file *os.File) *HashIndex {
	index := &HashIndex{
		items: map[string]int64{},
	}
	index.PopulateHashIndex(file)
	return index
}

func (h *HashIndex) Put(key string, offset int64) {
	h.items[key] = offset
}

func (h *HashIndex) Get(key string) (int64, bool) {
	value, ok := h.items[key]
	return value, ok
}

func (h *HashIndex) Delete(key string) bool {
	_, ok := h.items[key]
	if !ok {
		return false
	}

	delete(h.items, key)
	return true
}

func (h *HashIndex) PopulateHashIndex(file *os.File) error {
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
			h.Put(key, offset)
		case opDelete:
			h.Delete(key)
		}

		offset += recordSize
	}

	return nil
}
