package automa

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncNamespacedStateBag_JSONRoundTrip(t *testing.T) {
	n := NewNamespacedStateBag(nil, nil)
	n.Global().Set("g", "gv")
	n.Local().Set("l", "lv")
	n.WithNamespace("ns").Set("k", "v")

	b, err := json.Marshal(n)
	require.NoError(t, err)

	var n2 SyncNamespacedStateBag
	require.NoError(t, json.Unmarshal(b, &n2))

	// Verify values
	assert.Equal(t, "gv", n2.Global().String("g"))
	assert.Equal(t, "lv", n2.Local().String("l"))
	assert.Equal(t, "v", n2.WithNamespace("ns").String("k"))
}

func TestSyncNamespacedStateBag_YAMLRoundTrip(t *testing.T) {
	n := NewNamespacedStateBag(nil, nil)
	n.Global().Set("g", "gv")
	n.Local().Set("l", "lv")
	n.WithNamespace("ns").Set("k", "v")

	b, err := yaml.Marshal(n)
	require.NoError(t, err)

	var n2 SyncNamespacedStateBag
	require.NoError(t, yaml.Unmarshal(b, &n2))

	// Verify values
	assert.Equal(t, "gv", n2.Global().String("g"))
	assert.Equal(t, "lv", n2.Local().String("l"))
	assert.Equal(t, "v", n2.WithNamespace("ns").String("k"))
}
