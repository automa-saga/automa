package automa

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExportedNormalizationHelpers_Smoke(t *testing.T) {
	// Minimal smoke tests to ensure exported helpers are wired; detailed behavior
	// is covered by more specific test files to avoid duplication.
	v, err := NormalizeValue(42)
	require.NoError(t, err)
	require.Equal(t, 42, v)

	i, ok := ToInt64(json.Number("7"))
	require.True(t, ok)
	require.Equal(t, int64(7), i)

	f, ok := ToFloat64("3.5")
	require.True(t, ok)
	require.Equal(t, 3.5, f)

	b, ok := ToBool("true")
	require.True(t, ok)
	require.Equal(t, true, b)

	ss, ok := ToStringSlice([]interface{}{"a", 2})
	require.True(t, ok)
	require.Equal(t, []string{"a", "2"}, ss)

	sm, ok := ToStringMap(map[string]interface{}{"x": 1})
	require.True(t, ok)
	require.Equal(t, map[string]string{"x": "1"}, sm)

	// Ensure NormalizeValue handles nil gracefully (was previously an error)
	n, err := NormalizeValue(nil)
	require.NoError(t, err)
	require.Nil(t, n)
}
