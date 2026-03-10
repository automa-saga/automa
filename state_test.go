package automa

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeFromState_PointerDereference(t *testing.T) {
	v := 123
	p := &v
	n, err := normalizeFromState(p)
	require.NoError(t, err)
	assert.Equal(t, 123, n)
}

func TestNormalizeFromState_JSONNumber(t *testing.T) {
	// json.Number should convert to int64, uint64 or float64
	jn := json.Number("42")
	n, err := normalizeFromState(jn)
	require.NoError(t, err)
	assert.Equal(t, int64(42), n)

	jnUnsigned := json.Number("18446744073709551615")
	nUnsigned, err := normalizeFromState(jnUnsigned)
	require.NoError(t, err)
	assert.Equal(t, uint64(^uint64(0)), nUnsigned)

	jn2 := json.Number("3.14")
	n2, err := normalizeFromState(jn2)
	require.NoError(t, err)
	assert.Equal(t, 3.14, n2)
}

func TestNormalizeFromState_YAMLNodeAndMapInterface(t *testing.T) {
	// create YAML node from a map[string]interface{}
	src := map[string]interface{}{"a": 1, "b": []interface{}{2, 3}}
	b, err := yaml.Marshal(src)
	require.NoError(t, err)

	var node yaml.Node
	require.NoError(t, yaml.Unmarshal(b, &node))

	n, err := normalizeFromState(&node)
	require.NoError(t, err)
	// accept either map[string]interface{} or map[interface{}]interface{}
	switch m := n.(type) {
	case map[string]interface{}:
		// numbers may decode as ints or floats depending on decoder; accept numeric
		v := m["a"]
		rv := reflect.ValueOf(v)
		assert.True(t, rv.Kind() == reflect.Int || rv.Kind() == reflect.Int64 || rv.Kind() == reflect.Float64)
	case map[interface{}]interface{}:
		// if still interface-map, check keys and values after normalization
		v := m["a"]
		rv := reflect.ValueOf(v)
		assert.True(t, rv.Kind() == reflect.Int || rv.Kind() == reflect.Int64 || rv.Kind() == reflect.Float64)
	default:
		require.Fail(t, "unexpected normalized type", "got %T", n)
	}

	// test map[interface{}]interface{} conversion
	mi := map[interface{}]interface{}{"x": 7, 8: "y"}
	mout, err := normalizeFromState(mi)
	require.NoError(t, err)
	mm, ok := mout.(map[string]interface{})
	require.True(t, ok)
	// values normalized; numeric 7 may be int or float
	rv := reflect.ValueOf(mm["x"])
	assert.True(t, rv.Kind() == reflect.Int || rv.Kind() == reflect.Int64 || rv.Kind() == reflect.Float64)
	assert.Equal(t, "y", mm["8"]) // non-string key converted to string
}

func TestNormalizeFromState_Slice(t *testing.T) {
	in := []interface{}{json.Number("1"), 2, "3"}
	n, err := normalizeFromState(in)
	require.NoError(t, err)
	arr, ok := n.([]interface{})
	require.True(t, ok)
	// elements should be normalized: first -> int64(1) or numeric, second -> numeric, third -> "3"
	// Accept either int64 or float64 for numeric normalized values
	switch arr[0].(type) {
	case int64, int, float64:
		// ok
	default:
		require.Fail(t, "first element not numeric after normalization")
	}
	// arr[2] should remain the string "3"
	assert.Equal(t, "3", arr[2])
}

func TestStringify_UnsupportedType_ReturnsError(t *testing.T) {
	ch := make(chan int)
	defer close(ch)
	_, err := stringify(ch)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot stringify value of type")
}

func TestNormalizeFromState_MapWithUnstringableKey_PropagatesError(t *testing.T) {
	// map with a channel key; stringify should fail for the channel key and return an error
	mi := map[interface{}]interface{}{}
	ch := make(chan int)
	// channels are comparable and can be map keys
	mi[ch] = 1

	_, err := normalizeFromState(mi)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot stringify value of type")
}

func TestSyncStateBag_PrimitiveOperations(t *testing.T) {
	t.Run("zero value is usable", func(t *testing.T) {
		var s SyncStateBag
		s.Set("k", "v")
		val, ok := s.Get("k")
		require.True(t, ok)
		assert.Equal(t, "v", val)
		assert.Equal(t, 1, s.Size())
	})

	t.Run("delete removes key", func(t *testing.T) {
		s := &SyncStateBag{}
		s.Set("k1", "v1").Set("k2", "v2")
		s.Delete("k1")

		_, ok := s.Get("k1")
		assert.False(t, ok)
		val, ok := s.Get("k2")
		require.True(t, ok)
		assert.Equal(t, "v2", val)
		assert.Equal(t, 1, s.Size())
	})

	t.Run("clear empties bag and bag remains reusable", func(t *testing.T) {
		s := &SyncStateBag{}
		s.Set("k1", 1).Set("k2", 2)
		assert.Equal(t, 2, s.Size())

		s.Clear()
		assert.Equal(t, 0, s.Size())
		assert.Empty(t, s.Keys())
		assert.Empty(t, s.Items())

		// still reusable after clear
		s.Set("k3", 3)
		assert.Equal(t, 1, s.Size())
		assert.Equal(t, 3, s.Int("k3"))
	})

	t.Run("keys and items return snapshots", func(t *testing.T) {
		s := &SyncStateBag{}
		s.Set("b", 2).Set("a", 1).Set("c", 3)

		keys := s.Keys()
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		assert.Equal(t, []Key{"a", "b", "c"}, keys)

		items := s.Items()
		assert.Equal(t, map[Key]interface{}{"a": 1, "b": 2, "c": 3}, items)

		// modifying the returned snapshot must not affect the bag
		items["a"] = 999
		assert.Equal(t, 1, s.Int("a"))
	})
}

func TestSyncStateBag_Merge(t *testing.T) {
	t.Run("overwrites existing keys and keeps existing unrelated keys", func(t *testing.T) {
		dst := &SyncStateBag{}
		dst.Set("shared", "old")
		dst.Set("dst-only", 1)

		src := &SyncStateBag{}
		src.Set("shared", "new")
		src.Set("src-only", 2)

		merged, err := dst.Merge(src)
		require.NoError(t, err)
		require.Same(t, dst, merged)

		assert.Equal(t, "new", dst.String("shared"))
		assert.Equal(t, 1, dst.Int("dst-only"))
		assert.Equal(t, 2, dst.Int("src-only"))
	})

	t.Run("nil other returns same receiver unchanged", func(t *testing.T) {
		s := &SyncStateBag{}
		s.Set("k", "v")

		merged, err := s.Merge(nil)
		require.NoError(t, err)
		require.Same(t, s, merged)
		assert.Equal(t, "v", s.String("k"))
	})

	t.Run("self merge is safe and non-destructive", func(t *testing.T) {
		s := &SyncStateBag{}
		s.Set("k1", "v1")
		s.Set("k2", 2)

		merged, err := s.Merge(s)
		require.NoError(t, err)
		require.Same(t, s, merged)

		assert.Equal(t, "v1", s.String("k1"))
		assert.Equal(t, 2, s.Int("k2"))
		assert.Equal(t, 2, s.Size())
	})

	t.Run("post merge source updates do not change receiver key set or overwritten values", func(t *testing.T) {
		dst := &SyncStateBag{}
		dst.Set("shared", "old")

		src := &SyncStateBag{}
		src.Set("shared", "new")
		src.Set("src-only", 1)

		_, err := dst.Merge(src)
		require.NoError(t, err)

		// mutate source after merge
		src.Set("shared", "changed-again")
		src.Set("new-after-merge", 99)
		src.Delete("src-only")

		// merged result should stay as it was at merge time
		assert.Equal(t, "new", dst.String("shared"))
		assert.Equal(t, 1, dst.Int("src-only"))
		_, ok := dst.Get("new-after-merge")
		assert.False(t, ok)
	})
}
