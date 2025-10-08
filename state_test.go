package automa

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSyncStateBag_SetAndGet(t *testing.T) {
	bag := &SyncStateBag{}
	bag.Set("foo", 42)

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
	assert.ElementsMatch(t, []Key{"x", "y"}, keys)
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
	ctxWithBag := ContextWithState(ctx, bag)

	retrieved := StateFromContext(ctxWithBag)
	val, ok := retrieved.Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", val)

	// Test fallback to empty SyncStateBag if not present
	emptyCtx := context.Background()
	fallback := StateFromContext(emptyCtx)
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

	assert.Equal(t, "hello", StringFromState(bag, "str"))
	assert.Equal(t, 123, IntFromState(bag, "int"))
	assert.Equal(t, true, BoolFromState(bag, "bool"))
	assert.Equal(t, 3.14, FloatFromState(bag, "float"))

	assert.ElementsMatch(t, []string{"a", "b"}, StringSliceFromState(bag, "strSlice"))
	assert.ElementsMatch(t, []int{1, 2}, IntSliceFromState(bag, "intSlice"))
	assert.ElementsMatch(t, []bool{true, false}, BoolSliceFromState(bag, "boolSlice"))
	assert.ElementsMatch(t, []float64{1.1, 2.2}, FloatSliceFromState(bag, "floatSlice"))

	assert.Equal(t, map[string]string{"x": "y"}, StringMapFromState(bag, "strMap"))
	assert.Equal(t, map[string]int{"a": 1}, IntMapFromState(bag, "intMap"))
	assert.Equal(t, map[string]bool{"t": true}, BoolMapFromState(bag, "boolMap"))
	assert.Equal(t, map[string]float64{"pi": 3.14}, FloatMapFromState(bag, "floatMap"))

	// Test default values for missing keys
	assert.Equal(t, "", StringFromState(bag, "missingStr"))
	assert.Equal(t, 0, IntFromState(bag, "missingInt"))
	assert.Equal(t, false, BoolFromState(bag, "missingBool"))
	assert.Equal(t, 0.0, FloatFromState(bag, "missingFloat"))
	assert.Empty(t, StringSliceFromState(bag, "missingStrSlice"))
	assert.Empty(t, IntSliceFromState(bag, "missingIntSlice"))
	assert.Empty(t, BoolSliceFromState(bag, "missingBoolSlice"))
	assert.Empty(t, FloatSliceFromState(bag, "missingFloatSlice"))
	assert.Empty(t, StringMapFromState(bag, "missingStrMap"))
	assert.Empty(t, IntMapFromState(bag, "missingIntMap"))
	assert.Empty(t, BoolMapFromState(bag, "missingBoolMap"))
	assert.Empty(t, FloatMapFromState(bag, "missingFloatMap"))
}

func TestSyncStateBag_Merge(t *testing.T) {
	bag1 := &SyncStateBag{}
	bag1.Set("a", 1)
	bag1.Set("b", 2)

	bag2 := &SyncStateBag{}
	bag2.Set("b", 20)
	bag2.Set("c", 30)

	bag1.Merge(bag2)
	assert.Equal(t, 1, IntFromState(bag1, "a"))
	assert.Equal(t, 20, IntFromState(bag1, "b"))
	assert.Equal(t, 30, IntFromState(bag1, "c"))
}

func TestSyncStateBag_Merge_NilOther(t *testing.T) {
	bag := &SyncStateBag{}
	bag.Set("x", 100)
	result := bag.Merge(nil)
	assert.Equal(t, bag, result)
	assert.Equal(t, 100, IntFromState(result, "x"))
}

func TestGetFromState_TypeSafety(t *testing.T) {
	bag := &SyncStateBag{}
	bag.Set("num", "not-an-int")
	assert.Equal(t, 0, IntFromState(bag, "num"))
	bag.Set("slice", "not-a-slice")
	assert.Empty(t, StringSliceFromState(bag, "slice"))
	bag.Set("map", "not-a-map")
	assert.Empty(t, StringMapFromState(bag, "map"))
}

func TestSyncStateBag_Items(t *testing.T) {
	bag := &SyncStateBag{}
	bag.Set("foo", 123)
	bag.Set("bar", "baz")
	items := bag.Items()
	assert.Equal(t, 2, len(items))
	assert.Equal(t, 123, items["foo"])
	assert.Equal(t, "baz", items["bar"])
}

func TestSyncStateBag_Clone(t *testing.T) {
	orig := &SyncStateBag{}
	orig.Set("foo", 42)
	orig.Set("bar", "baz")

	clone := orig.Clone()
	assert.Equal(t, 2, clone.Size())
	assert.Equal(t, 42, IntFromState(clone, "foo"))
	assert.Equal(t, "baz", StringFromState(clone, "bar"))

	// Mutate clone and check original is unchanged
	clone.Set("foo", 100)
	assert.Equal(t, 42, IntFromState(orig, "foo"))
	assert.Equal(t, 100, IntFromState(clone, "foo"))
}

func TestSyncStateBag_HelperMethods(t *testing.T) {
	bag := &SyncStateBag{}
	
	// Test String
	bag.Set("str", "hello")
	assert.Equal(t, "hello", bag.String("str"))
	assert.Equal(t, "", bag.String("missingStr"))
	
	// Test Bool
	bag.Set("bool", true)
	assert.Equal(t, true, bag.Bool("bool"))
	assert.Equal(t, false, bag.Bool("missingBool"))
	
	// Test Int
	bag.Set("int", 123)
	assert.Equal(t, 123, bag.Int("int"))
	assert.Equal(t, 0, bag.Int("missingInt"))
	
	// Test Int8
	bag.Set("int8", int8(8))
	assert.Equal(t, int8(8), bag.Int8("int8"))
	assert.Equal(t, int8(0), bag.Int8("missingInt8"))
	
	// Test Int16
	bag.Set("int16", int16(16))
	assert.Equal(t, int16(16), bag.Int16("int16"))
	assert.Equal(t, int16(0), bag.Int16("missingInt16"))
	
	// Test Int32
	bag.Set("int32", int32(32))
	assert.Equal(t, int32(32), bag.Int32("int32"))
	assert.Equal(t, int32(0), bag.Int32("missingInt32"))
	
	// Test Int64
	bag.Set("int64", int64(64))
	assert.Equal(t, int64(64), bag.Int64("int64"))
	assert.Equal(t, int64(0), bag.Int64("missingInt64"))
	
	// Test Float
	bag.Set("float", 3.14)
	assert.Equal(t, 3.14, bag.Float("float"))
	assert.Equal(t, 0.0, bag.Float("missingFloat"))
	
	// Test Float32
	bag.Set("float32", float32(3.14))
	assert.InDelta(t, float32(3.14), bag.Float32("float32"), 0.001)
	assert.Equal(t, float32(0.0), bag.Float32("missingFloat32"))
	
	// Test Float64
	bag.Set("float64", float64(3.14159))
	assert.Equal(t, 3.14159, bag.Float64("float64"))
	assert.Equal(t, 0.0, bag.Float64("missingFloat64"))
}

func TestSyncStateBag_HelperMethods_TypeSafety(t *testing.T) {
	bag := &SyncStateBag{}
	
	// Test that wrong types return zero values
	bag.Set("notAString", 123)
	assert.Equal(t, "", bag.String("notAString"))
	
	bag.Set("notAnInt", "hello")
	assert.Equal(t, 0, bag.Int("notAnInt"))
	
	bag.Set("notABool", "true")
	assert.Equal(t, false, bag.Bool("notABool"))
	
	bag.Set("notAFloat", "3.14")
	assert.Equal(t, 0.0, bag.Float("notAFloat"))
}

