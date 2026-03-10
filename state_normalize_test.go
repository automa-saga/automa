package automa

import (
	"encoding/json"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeFromState_PointerDereference(t *testing.T) {
	v := 123
	p := &v
	n := normalizeFromState(p)
	assert.Equal(t, 123, n)
}

func TestNormalizeFromState_JSONNumber(t *testing.T) {
	// json.Number should convert to int64 or float64
	jn := json.Number("42")
	n := normalizeFromState(jn)
	assert.Equal(t, int64(42), n)

	jn2 := json.Number("3.14")
	n2 := normalizeFromState(jn2)
	assert.Equal(t, 3.14, n2)
}

func TestNormalizeFromState_YAMLNodeAndMapInterface(t *testing.T) {
	// create YAML node from a map[string]interface{}
	src := map[string]interface{}{"a": 1, "b": []interface{}{2, 3}}
	b, err := yaml.Marshal(src)
	require.NoError(t, err)

	var node yaml.Node
	require.NoError(t, yaml.Unmarshal(b, &node))

	n := normalizeFromState(&node)
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
	mout := normalizeFromState(mi)
	mm, ok := mout.(map[string]interface{})
	require.True(t, ok)
	// values normalized; numeric 7 may be int or float
	rv := reflect.ValueOf(mm["x"])
	assert.True(t, rv.Kind() == reflect.Int || rv.Kind() == reflect.Int64 || rv.Kind() == reflect.Float64)
	assert.Equal(t, "y", mm["8"]) // non-string key converted to string
}

func TestNormalizeFromState_Slice(t *testing.T) {
	in := []interface{}{json.Number("1"), 2, "3"}
	n := normalizeFromState(in)
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
