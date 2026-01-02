package automa

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEffectiveStrategy_JSONAndString(t *testing.T) {
	cases := []struct {
		s        EffectiveStrategy
		expected string
	}{
		{StrategyDefault, "default"},
		{StrategyUserInput, "userInput"},
		{StrategyCustom, "custom"},
	}

	for _, c := range cases {
		// String
		assert.Equal(t, c.expected, c.s.String())

		// Marshal JSON -> quoted string
		b, err := json.Marshal(c.s)
		require.NoError(t, err)
		assert.Equal(t, "\""+c.expected+"\"", string(b))

		// Unmarshal back
		var got EffectiveStrategy
		require.NoError(t, json.Unmarshal(b, &got))
		assert.Equal(t, c.s, got)
	}
}

func TestEffectiveValue_BasicBehavior(t *testing.T) {
	// create a Value via existing constructor to avoid depending on internal interfaces
	v, err := NewValue[string]("hello")
	require.NoError(t, err)
	require.NotNil(t, v)

	ev, err := NewEffectiveValue(v, StrategyUserInput)
	require.NoError(t, err)
	require.NotNil(t, ev)

	// accessors
	assert.Equal(t, StrategyUserInput, ev.Strategy())
	assert.Equal(t, v, ev.Get())

	// Clone should succeed and preserve strategy
	clone, err := ev.Clone()
	require.NoError(t, err)
	require.NotNil(t, clone)
	assert.Equal(t, ev.Strategy(), clone.Strategy())
	// underlying value must be present on clone
	assert.NotNil(t, clone.Get())
}

// Test that unknown/invalid strategy strings decode to StrategyDefault.
// This documents the current decision to treat unknown values as StrategyDefault.
func TestEffectiveStrategy_UnmarshalUnknownDefaultsToDefault(t *testing.T) {
	var es EffectiveStrategy
	err := json.Unmarshal([]byte(`"invalid"`), &es)
	require.NoError(t, err)
	assert.Equal(t, StrategyDefault, es)

	// also test another unknown token
	err = json.Unmarshal([]byte(`"unknown"`), &es)
	require.NoError(t, err)
	assert.Equal(t, StrategyDefault, es)
}

func TestNewEffectiveValue_NilValueBehavior(t *testing.T) {
	var nilVal Value[string] = nil

	ev, err := NewEffectiveValue[string](nilVal, StrategyDefault)
	require.Error(t, err)
	require.Nil(t, ev)
}
