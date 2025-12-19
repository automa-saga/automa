package automa

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestTypeMode_String(t *testing.T) {
	assert.Equal(t, "continue", ContinueOnError.String())
	assert.Equal(t, "stop", StopOnError.String())
	assert.Equal(t, "rollback", RollbackOnError.String())
	assert.Equal(t, "unknown", TypeMode(99).String())
}

func TestTypeMode_MarshalJSON_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		mode     TypeMode
		expected string
	}{
		{ContinueOnError, `"continue"`},
		{StopOnError, `"stop"`},
		{RollbackOnError, `"rollback"`},
	}

	for _, tt := range tests {
		b, err := json.Marshal(tt.mode)
		assert.NoError(t, err)
		assert.Equal(t, tt.expected, string(b))

		var m TypeMode
		err = json.Unmarshal(b, &m)
		assert.NoError(t, err)
		assert.Equal(t, tt.mode, m)
	}

	// Unknown value
	var mu TypeMode
	err := json.Unmarshal([]byte(`"unknown"`), &mu)
	assert.Error(t, err)
}

func TestTypeMode_MarshalYAML_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		mode     TypeMode
		expected string
	}{
		{ContinueOnError, "continue"},
		{StopOnError, "stop"},
		{RollbackOnError, "rollback"},
	}

	for _, tt := range tests {
		b, err := yaml.Marshal(tt.mode)
		assert.NoError(t, err)
		assert.Contains(t, string(b), tt.expected)

		var m TypeMode
		err = yaml.Unmarshal(b, &m)
		assert.NoError(t, err)
		assert.Equal(t, tt.mode, m)
	}

	// Unknown value
	var mu TypeMode
	err := yaml.Unmarshal([]byte("unknown\n"), &mu)
	assert.Error(t, err)
}
