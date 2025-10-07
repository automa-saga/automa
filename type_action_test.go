package automa

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestTypeAction_String(t *testing.T) {
	assert.Equal(t, "execute", ActionExecute.String())
	assert.Equal(t, "rollback", ActionRollback.String())
	assert.Equal(t, "unknown", TypeAction(99).String())
}

func TestTypeAction_MarshalJSON_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		action   TypeAction
		expected string
	}{
		{ActionExecute, `"execute"`},
		{ActionRollback, `"rollback"`},
	}

	for _, tt := range tests {
		b, err := json.Marshal(tt.action)
		assert.NoError(t, err)
		assert.Equal(t, tt.expected, string(b))

		var a TypeAction
		err = json.Unmarshal(b, &a)
		assert.NoError(t, err)
		assert.Equal(t, tt.action, a)
	}

	// Test unknown value
	var a TypeAction
	err := json.Unmarshal([]byte(`"unknown"`), &a)
	assert.NoError(t, err)
	assert.Equal(t, TypeAction(0), a)
}

func TestTypeAction_MarshalYAML_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		action   TypeAction
		expected string
	}{
		{ActionExecute, "execute"},
		{ActionRollback, "rollback"},
	}

	for _, tt := range tests {
		b, err := yaml.Marshal(tt.action)
		assert.NoError(t, err)
		assert.Contains(t, string(b), tt.expected)

		var a TypeAction
		err = yaml.Unmarshal(b, &a)
		assert.NoError(t, err)
		assert.Equal(t, tt.action, a)
	}

	// Test unknown value
	var a TypeAction
	err := yaml.Unmarshal([]byte("unknown\n"), &a)
	assert.NoError(t, err)
	assert.Equal(t, TypeAction(0), a)
}

func TestTypeAction_String_Prepare(t *testing.T) {
	assert.Equal(t, "prepare", ActionPrepare.String())
}

func TestTypeAction_UnmarshalJSON_Invalid(t *testing.T) {
	var a TypeAction
	err := json.Unmarshal([]byte(`"invalid"`), &a)
	assert.NoError(t, err)
	assert.Equal(t, TypeAction(0), a)
}

func TestTypeAction_UnmarshalYAML_Invalid(t *testing.T) {
	var a TypeAction
	err := yaml.Unmarshal([]byte("invalid\n"), &a)
	assert.NoError(t, err)
	assert.Equal(t, TypeAction(0), a)
}

func TestTypeAction_MarshalYAML_Prepare(t *testing.T) {
	b, err := yaml.Marshal(ActionPrepare)
	assert.NoError(t, err)
	assert.Contains(t, string(b), "prepare")
}

func TestTypeAction_MarshalJSON_Prepare(t *testing.T) {
	b, err := json.Marshal(ActionPrepare)
	assert.NoError(t, err)
	assert.Equal(t, `"prepare"`, string(b))
}
