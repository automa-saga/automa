package automa

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestTypeRollbackMode_String(t *testing.T) {
	assert.Equal(t, "continue", RollbackModeContinueOnError.String())
	assert.Equal(t, "stop", RollbackModeStopOnError.String())
	assert.Equal(t, "unknown", TypeRollbackMode(99).String())
}

func TestTypeRollbackMode_MarshalJSON_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		mode     TypeRollbackMode
		expected string
	}{
		{RollbackModeContinueOnError, `"continue"`},
		{RollbackModeStopOnError, `"stop"`},
	}

	for _, tt := range tests {
		b, err := json.Marshal(tt.mode)
		assert.NoError(t, err)
		assert.Equal(t, tt.expected, string(b))

		var m TypeRollbackMode
		err = json.Unmarshal(b, &m)
		assert.NoError(t, err)
		assert.Equal(t, tt.mode, m)
	}

	// Unknown value
	var m TypeRollbackMode
	err := json.Unmarshal([]byte(`"unknown"`), &m)
	assert.Error(t, err)
}

func TestTypeRollbackMode_MarshalYAML_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		mode     TypeRollbackMode
		expected string
	}{
		{RollbackModeContinueOnError, "continue"},
		{RollbackModeStopOnError, "stop"},
	}

	for _, tt := range tests {
		b, err := yaml.Marshal(tt.mode)
		assert.NoError(t, err)
		assert.Contains(t, string(b), tt.expected)

		var m TypeRollbackMode
		err = yaml.Unmarshal(b, &m)
		assert.NoError(t, err)
		assert.Equal(t, tt.mode, m)
	}

	// Unknown value
	var m TypeRollbackMode
	err := yaml.Unmarshal([]byte("unknown\n"), &m)
	assert.Error(t, err)
}
