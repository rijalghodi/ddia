package hashindex

type HashIndex struct {
    items map[string]int32
}

func NewHashIndex() *HashIndex {
	return &HashIndex{
		items: map[string]int32{},
	}
}

func (h *HashIndex) Put(key string, offset int32) {
	h.items[key] = offset
}

func (h *HashIndex) Get(key string) (int32, bool) {
	value, ok := h.items[key]
	return value, ok
}

func (h *HashIndex) Delete(key string) bool {
	_, ok := h.items[key]
	if (!ok) {
		return false
	}

	delete(h.items, key)
	return true
}