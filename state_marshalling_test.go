package automa

import (
	"encoding/json"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncStateBag_JSONRoundTrip(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("str", "hello")
	s.Set("bool", true)
	s.Set("num", 42)
	s.Set("map", map[string]string{"a": "b"})
	s.Set("slice", []string{"x", "y"})

	b, err := json.Marshal(s)
	require.NoError(t, err)

	var s2 SyncStateBag
	require.NoError(t, json.Unmarshal(b, &s2))

	// strings and bools unmarshaled as expected
	assert.Equal(t, "hello", s2.String("str"))
	assert.Equal(t, true, s2.Bool("bool"))

	// numeric values in JSON decode to float64
	val, ok := s2.Get("num")
	require.True(t, ok)
	assert.Equal(t, reflect.Float64, reflect.TypeOf(val).Kind())
	assert.Equal(t, 42.0, val.(float64))

	// map becomes map[string]interface{}
	mval, ok := s2.Get("map")
	require.True(t, ok)
	m, ok := mval.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "b", m["a"])

	// slice becomes []interface{}
	sliceVal, ok := s2.Get("slice")
	require.True(t, ok)
	sliceIface, ok := sliceVal.([]interface{})
	require.True(t, ok)
	assert.Equal(t, "x", sliceIface[0])
}

func TestSyncStateBag_YAMLRoundTrip(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("str", "hello-yaml")
	s.Set("num", 7)
	s.Set("nested", map[string]interface{}{"k": "v"})

	b, err := yaml.Marshal(s)
	require.NoError(t, err)

	var s2 SyncStateBag
	require.NoError(t, yaml.Unmarshal(b, &s2))

	assert.Equal(t, "hello-yaml", s2.String("str"))

	// YAML unmarshals numbers as int when possible (yaml.v3 preserves types), so we accept float64/int
	val, ok := s2.Get("num")
	require.True(t, ok)
	switch v := val.(type) {
	case int:
		assert.Equal(t, 7, v)
	case int64:
		assert.Equal(t, int64(7), v)
	case float64:
		assert.Equal(t, 7.0, v)
	default:
		t.Fatalf("unexpected numeric type: %T", v)
	}

	nval, ok := s2.Get("nested")
	require.True(t, ok)
	nmap, ok := nval.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "v", nmap["k"])
}

// testClonerDeep implements Clone() (value, error) so SyncStateBag.Clone will deep-copy it
type testClonerDeep struct {
	N int
}

func (t *testClonerDeep) Clone() (*testClonerDeep, error) {
	return &testClonerDeep{N: t.N}, nil
}

func TestSyncStateBag_CloneDeepCopy(t *testing.T) {
	s := &SyncStateBag{}
	orig := &testClonerDeep{N: 1}
	s.Set("c", orig)

	cloneState, err := s.Clone()
	require.NoError(t, err)
	cloneBag, ok := cloneState.(*SyncStateBag)
	require.True(t, ok)

	// modify original value
	orig.N = 2

	// ensure cloned value has preserved old value
	cv, ok := cloneBag.Get("c")
	require.True(t, ok)
	clonedVal, ok := cv.(*testClonerDeep)
	require.True(t, ok)
	assert.Equal(t, 1, clonedVal.N)

	// original state should reflect updated value
	ov, ok := s.Get("c")
	require.True(t, ok)
	origVal, ok := ov.(*testClonerDeep)
	require.True(t, ok)
	assert.Equal(t, 2, origVal.N)
}

func TestFromState_JSONTypedAccessors(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("int", 5)
	s.Set("float", 3.14)
	s.Set("boolStr", "true")
	s.Set("numStr", "123")

	b, err := json.Marshal(s)
	require.NoError(t, err)

	var s2 SyncStateBag
	require.NoError(t, json.Unmarshal(b, &s2))

	// raw decoded shapes
	val, ok := s2.Get("int")
	require.True(t, ok)
	assert.Equal(t, reflect.Float64, reflect.TypeOf(val).Kind()) // JSON numbers -> float64
	assert.Equal(t, 5.0, val.(float64))

	// typed accessors should coerce correctly
	assert.Equal(t, 5, s2.Int("int"))
	assert.Equal(t, int64(5), s2.Int64("int"))
	assert.Equal(t, 5.0, s2.Float64("int"))

	// float -> Int truncates toward zero
	assert.Equal(t, 3, s2.Int("float"))
	assert.InDelta(t, 3.14, s2.Float64("float"), 1e-9)

	// string boolean should coerce to bool
	assert.Equal(t, true, s2.Bool("boolStr"))

	// numeric string coercion
	assert.Equal(t, 123, s2.Int("numStr"))
	assert.Equal(t, 123.0, s2.Float64("numStr"))
	assert.Equal(t, "123", s2.String("numStr"))
}

func TestFromState_YAMLTypedAccessors(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("int", 42)
	s.Set("floatInt", 7.0) // YAML may preserve as float or int depending on node
	s.Set("bool", true)
	s.Set("numStr", "99")

	b, err := yaml.Marshal(s)
	require.NoError(t, err)

	var s2 SyncStateBag
	require.NoError(t, yaml.Unmarshal(b, &s2))

	// numeric value may be int/int64/float64 depending on YAML decoding;
	// typed accessors should return expected numeric values regardless.
	assert.Equal(t, 42, s2.Int("int"))
	assert.Equal(t, int64(42), s2.Int64("int"))
	assert.Equal(t, 42.0, s2.Float64("int"))

	assert.Equal(t, 7, s2.Int("floatInt"))
	assert.InDelta(t, 7.0, s2.Float64("floatInt"), 1e-9)

	// boolean preserved
	assert.Equal(t, true, s2.Bool("bool"))

	// numeric string coercion on YAML
	assert.Equal(t, 99, s2.Int("numStr"))
	assert.Equal(t, "99", s2.String("numStr"))
}

func TestFromState_SliceAndMapAfterRoundTrip(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("slice", []int{1, 2, 3})
	s.Set("map", map[string]int{"a": 1, "b": 2})

	// JSON round-trip
	jb, err := json.Marshal(s)
	require.NoError(t, err)

	var js2 SyncStateBag
	require.NoError(t, json.Unmarshal(jb, &js2))

	// decoded slice becomes []interface{} with float64 numbers
	sliceVal, ok := js2.Get("slice")
	require.True(t, ok)
	sliceIface, ok := sliceVal.([]interface{})
	require.True(t, ok)
	assert.Len(t, sliceIface, 3)
	// accessor for elements: use FromState on each element and coercion helpers
	for i, v := range sliceIface {
		vn, err := normalizeFromState(v)
		require.NoError(t, err)
		iv, ok := toInt64(vn)
		require.True(t, ok)
		assert.Equal(t, int64(i+1), iv)
	}

	// decoded map becomes map[string]interface{}
	mapVal, ok := js2.Get("map")
	require.True(t, ok)
	m, ok := mapVal.(map[string]interface{})
	require.True(t, ok)
	// values are float64 after JSON decode
	assert.Equal(t, 1.0, m["a"])
	assert.Equal(t, 2.0, m["b"])
}
