package hashindex

import "testing"
func TestGet(t *testing.T) {
    index := NewHashIndex()

    index.Put("name", 0)
    index.Put("age", 11)

    offset, ok := index.Get("name")

    if !ok {
        t.Fatal("expected key to exist")
    }

    if offset != 0 {
        t.Fatalf("expected offset 0, got %d", offset)
    }
}

func TestGetMissingKey(t *testing.T) {
    index := NewHashIndex()

    _, ok := index.Get("unknown")

    if ok {
        t.Fatal("expected key to not exist")
    }
}

func TestDelete(t *testing.T) {
    index := NewHashIndex()

    index.Put("name", 0)

    deleted := index.Delete("name")

    if !deleted {
        t.Fatal("expected key to be deleted")
    }

    _, ok := index.Get("name")

    if ok {
        t.Fatal("expected key to no longer exist")
    }
}

func TestDeleteMissingKey(t *testing.T) {
    index := NewHashIndex()

    deleted := index.Delete("unknown")

    if deleted {
        t.Fatal("expected false when deleting missing key")
    }
}