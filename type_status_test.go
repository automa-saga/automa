package automa

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestTypeStatus_String(t *testing.T) {
	assert.Equal(t, "success", StatusSuccess.String())
	assert.Equal(t, "failed", StatusFailed.String())
	assert.Equal(t, "skipped", StatusSkipped.String())
	assert.Equal(t, "unknown", TypeStatus(99).String())
}

func TestTypeStatus_MarshalJSON_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		status   TypeStatus
		expected string
	}{
		{StatusSuccess, `"success"`},
		{StatusFailed, `"failed"`},
		{StatusSkipped, `"skipped"`},
	}

	for _, tt := range tests {
		b, err := json.Marshal(tt.status)
		assert.NoError(t, err)
		assert.Equal(t, tt.expected, string(b))

		var s TypeStatus
		err = json.Unmarshal(b, &s)
		assert.NoError(t, err)
		assert.Equal(t, tt.status, s)
	}

	// Unknown value
	var s TypeStatus
	err := json.Unmarshal([]byte(`"unknown"`), &s)
	assert.NoError(t, err)
	assert.Equal(t, TypeStatus(0), s)
}

func TestTypeStatus_MarshalYAML_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		status   TypeStatus
		expected string
	}{
		{StatusSuccess, "success"},
		{StatusFailed, "failed"},
		{StatusSkipped, "skipped"},
	}

	for _, tt := range tests {
		b, err := yaml.Marshal(tt.status)
		assert.NoError(t, err)
		assert.Contains(t, string(b), tt.expected)

		var s TypeStatus
		err = yaml.Unmarshal(b, &s)
		assert.NoError(t, err)
		assert.Equal(t, tt.status, s)
	}

	// Unknown value
	var s TypeStatus
	err := yaml.Unmarshal([]byte("unknown\n"), &s)
	assert.NoError(t, err)
	assert.Equal(t, TypeStatus(0), s)
}
