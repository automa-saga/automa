package automa

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeValue_PointerAndJSONNumber(t *testing.T) {
	v := 10
	p := &v
	r := NormalizeValue(p)
	assert.Equal(t, 10, r)

	jn := json.Number("42")
	r2 := NormalizeValue(jn)
	assert.Equal(t, int64(42), r2)

	jnf := json.Number("3.14")
	r3 := NormalizeValue(jnf)
	assert.Equal(t, 3.14, r3)
}

func TestNormalizeValue_YAMLNodeAndMapInterface(t *testing.T) {
	src := map[string]interface{}{"a": 1, "b": []interface{}{2, 3}}
	b, err := yaml.Marshal(src)
	require.NoError(t, err)

	var node yaml.Node
	require.NoError(t, yaml.Unmarshal(b, &node))

	n := NormalizeValue(&node)
	m, ok := n.(map[string]interface{})
	require.True(t, ok)
	// numeric may be int or float64 depending on yaml decoding; accept numeric
	val := m["a"]
	switch val.(type) {
	case int, int64, float64:
		// ok
	default:
		require.Fail(t, "unexpected numeric type for 'a'")
	}
}

func TestToInt64_ToFloat64_ToBool_ToStringHelpers(t *testing.T) {
	// numeric types
	i64, ok := ToInt64(123)
	assert.True(t, ok)
	assert.Equal(t, int64(123), i64)

	f, ok := ToFloat64("3.5")
	assert.True(t, ok)
	assert.Equal(t, 3.5, f)

	b, ok := ToBool("true")
	assert.True(t, ok)
	assert.Equal(t, true, b)

	// json.Number
	jn := json.Number("7")
	i2, ok := ToInt64(jn)
	assert.True(t, ok)
	assert.Equal(t, int64(7), i2)

	// float truncation
	i3, ok := ToInt64(3.9)
	assert.True(t, ok)
	assert.Equal(t, int64(3), i3)
}

func TestToStringSlice_ToStringMap(t *testing.T) {
	arr := []interface{}{"a", 2}
	ss, ok := ToStringSlice(arr)
	assert.True(t, ok)
	assert.Equal(t, []string{"a", "2"}, ss)

	m := map[string]interface{}{"x": 1, "y": "two"}
	mm, ok := ToStringMap(m)
	assert.True(t, ok)
	assert.Equal(t, map[string]string{"x": "1", "y": "two"}, mm)
}
