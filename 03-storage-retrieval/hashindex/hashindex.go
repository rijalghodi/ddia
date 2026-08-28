package hashindex

type HashIndex struct {
	items map[string]int64
}

func New() *HashIndex {
	return &HashIndex{
		items: map[string]int64{},
	}
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
