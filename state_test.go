package automa

import (
	"context"
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

func TestContextWithStateBagAndGetStateBagFromContext(t *testing.T) {
	bag := &SyncStateBag{}
	bag.Set("foo", "bar")

	ctx := context.Background()
	ctxWithBag := ContextWithStateBag(ctx, bag)

	retrieved := GetStateBagFromContext(ctxWithBag)
	val, ok := retrieved.Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", val)

	// Test fallback to empty SyncStateBag if not present
	emptyCtx := context.Background()
	fallback := GetStateBagFromContext(emptyCtx)
	assert.NotNil(t, fallback)
	assert.Equal(t, 0, fallback.Size())
}

func TestSpecializedGettersFromState(t *testing.T) {
	bag := &SyncStateBag{}
	bag.Set("str", "hello")
	bag.Set("int", 123)
	bag.Set("bool", true)
	bag.Set("float", 3.14)
	bag.Set("strSlice", []string{"a", "b"})
	bag.Set("intSlice", []int{1, 2})
	bag.Set("boolSlice", []bool{true, false})
	bag.Set("floatSlice", []float64{1.1, 2.2})
	bag.Set("strMap", map[string]string{"x": "y"})
	bag.Set("intMap", map[string]int{"a": 1})
	bag.Set("boolMap", map[string]bool{"t": true})
	bag.Set("floatMap", map[string]float64{"pi": 3.14})

	assert.Equal(t, "hello", GetStringFromState(bag, "str"))
	assert.Equal(t, 123, GetIntFromState(bag, "int"))
	assert.Equal(t, true, GetBoolFromState(bag, "bool"))
	assert.Equal(t, 3.14, GetFloatFromState(bag, "float"))

	assert.ElementsMatch(t, []string{"a", "b"}, GetStringSliceFromState(bag, "strSlice"))
	assert.ElementsMatch(t, []int{1, 2}, GetIntSliceFromState(bag, "intSlice"))
	assert.ElementsMatch(t, []bool{true, false}, GetBoolSliceFromState(bag, "boolSlice"))
	assert.ElementsMatch(t, []float64{1.1, 2.2}, GetFloatSliceFromState(bag, "floatSlice"))

	assert.Equal(t, map[string]string{"x": "y"}, GetStringMapFromState(bag, "strMap"))
	assert.Equal(t, map[string]int{"a": 1}, GetIntMapFromState(bag, "intMap"))
	assert.Equal(t, map[string]bool{"t": true}, GetBoolMapFromState(bag, "boolMap"))
	assert.Equal(t, map[string]float64{"pi": 3.14}, GetFloatMapFromState(bag, "floatMap"))

	// Test default values for missing keys
	assert.Equal(t, "", GetStringFromState(bag, "missingStr"))
	assert.Equal(t, 0, GetIntFromState(bag, "missingInt"))
	assert.Equal(t, false, GetBoolFromState(bag, "missingBool"))
	assert.Equal(t, 0.0, GetFloatFromState(bag, "missingFloat"))
	assert.Empty(t, GetStringSliceFromState(bag, "missingStrSlice"))
	assert.Empty(t, GetIntSliceFromState(bag, "missingIntSlice"))
	assert.Empty(t, GetBoolSliceFromState(bag, "missingBoolSlice"))
	assert.Empty(t, GetFloatSliceFromState(bag, "missingFloatSlice"))
	assert.Empty(t, GetStringMapFromState(bag, "missingStrMap"))
	assert.Empty(t, GetIntMapFromState(bag, "missingIntMap"))
	assert.Empty(t, GetBoolMapFromState(bag, "missingBoolMap"))
	assert.Empty(t, GetFloatMapFromState(bag, "missingFloatMap"))
}
