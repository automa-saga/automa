package automa

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSyncStateBag_SetAndGet(t *testing.T) {
	bag := &SyncStateBag{}
	val := bag.Set("foo", 42)
	assert.Equal(t, 42, val)

	got, ok := bag.Get("foo")
	assert.True(t, ok)
	assert.Equal(t, 42, got)
}

func TestSyncStateBag_Delete(t *testing.T) {
	bag := &SyncStateBag{}
	bag.Set("bar", "baz")
	bag.Delete("bar")
	_, ok := bag.Get("bar")
	assert.False(t, ok)
}

func TestSyncStateBag_Clear(t *testing.T) {
	bag := &SyncStateBag{}
	bag.Set("a", 1)
	bag.Set("b", 2)
	bag.Clear()
	assert.Equal(t, 0, bag.Size())
	assert.Empty(t, bag.Keys())
}

func TestSyncStateBag_Keys(t *testing.T) {
	bag := &SyncStateBag{}
	bag.Set("x", 100)
	bag.Set("y", 200)
	keys := bag.Keys()
	assert.ElementsMatch(t, []string{"x", "y"}, keys)
}

func TestSyncStateBag_Size(t *testing.T) {
	bag := &SyncStateBag{}
	assert.Equal(t, 0, bag.Size())
	bag.Set("one", 1)
	bag.Set("two", 2)
	assert.Equal(t, 2, bag.Size())
}
