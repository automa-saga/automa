package automa

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// minimal StepBuilder for testing
type testStepBuilder struct {
	id string
}

func (b *testStepBuilder) Id() string           { return b.id }
func (b *testStepBuilder) Validate() error      { return nil }
func (b *testStepBuilder) Build() (Step, error) { return nil, nil }

func TestRegistry_Add_And_Has_And_Of(t *testing.T) {
	reg := NewRegistry()
	b1 := &testStepBuilder{id: "step1"}
	b2 := &testStepBuilder{id: "step2"}

	// Add steps
	err := reg.Add(b1, b2)
	assert.NoError(t, err)

	// Has
	assert.True(t, reg.Has("step1"))
	assert.True(t, reg.Has("step2"))
	assert.False(t, reg.Has("step3"))

	// Of
	assert.Equal(t, b1, reg.Of("step1"))
	assert.Equal(t, b2, reg.Of("step2"))
	assert.Nil(t, reg.Of("step3"))
}

func TestRegistry_Add_Duplicate(t *testing.T) {
	reg := NewRegistry()
	b1 := &testStepBuilder{id: "dup"}
	b2 := &testStepBuilder{id: "dup"}

	err := reg.Add(b1)
	assert.NoError(t, err)

	err = reg.Add(b2)
	assert.Error(t, err)
}

func TestRegistry_Remove(t *testing.T) {
	reg := NewRegistry()
	b1 := &testStepBuilder{id: "step1"}
	reg.Add(b1)

	// Remove existing
	removed := reg.Remove("step1")
	assert.True(t, removed)
	assert.False(t, reg.Has("step1"))

	// Remove non-existing
	removed = reg.Remove("step1")
	assert.False(t, removed)
}
