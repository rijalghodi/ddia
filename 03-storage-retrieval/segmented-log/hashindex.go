package segmentedlog

import "io"

// RecordLocation stores where a record is located across segments
type RecordLocation struct {
	SegmentID int
	Offset    int64
}

type HashIndex struct {
	items map[string]RecordLocation
}

func NewHashIndex() *HashIndex {
	return &HashIndex{
		items: make(map[string]RecordLocation),
	}
}

func (h *HashIndex) Put(key string, loc RecordLocation) {
	h.items[key] = loc
}

func (h *HashIndex) Get(key string) (RecordLocation, bool) {
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

func (h *HashIndex) PopulateHashIndex(seg *Segment) error {
	offset := int64(0)
	for offset < seg.Size {
		_, err := seg.File.Seek(offset, io.SeekStart)
		if err != nil {
			return err
		}

		op, key, _, recordSize, err := readRecord(seg.File)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		switch op {
		case opPut:
			h.Put(key, RecordLocation{SegmentID: seg.ID, Offset: offset})
		case opDelete:
			h.Delete(key)
		}
		offset += recordSize
	}
	return nil
}
