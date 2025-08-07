package automa

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistry_AddAndGetSteps(t *testing.T) {
	reg := NewRegistry()
	s1 := newSimpleTask("step1")
	s2 := newSimpleTask("step2")

	reg.AddSteps(s1, s2)
	got1 := reg.(*stepRegistry).GetStep("step1")
	got2 := reg.(*stepRegistry).GetStep("step2")
	gotInvalid := reg.(*stepRegistry).GetStep("invalid")

	assert.Equal(t, s1, got1)
	assert.Equal(t, s2, got2)
	assert.Nil(t, gotInvalid)
}

func TestRegistry_RemoveSteps(t *testing.T) {
	reg := NewRegistry()
	s1 := newSimpleTask("step1")
	s2 := newSimpleTask("step2")
	reg.AddSteps(s1, s2)

	reg.RemoveSteps("step1")
	assert.Nil(t, reg.(*stepRegistry).GetStep("step1"))
	assert.NotNil(t, reg.(*stepRegistry).GetStep("step2"))

	reg.RemoveSteps("step2")
	assert.Nil(t, reg.(*stepRegistry).GetStep("step2"))
}

func TestRegistry_GetSteps(t *testing.T) {
	reg := NewRegistry()
	s1 := newSimpleTask("step1")
	s2 := newSimpleTask("step2")
	reg.AddSteps(s1, s2)

	steps := reg.(*stepRegistry).GetSteps()
	assert.Len(t, steps, 2)
	assert.Contains(t, steps, s1)
	assert.Contains(t, steps, s2)
}

func TestRegistry_BuildWorkflow_Success(t *testing.T) {
	reg := NewRegistry()
	s1 := newSimpleTask("step1")
	s2 := newSimpleTask("step2")
	reg.AddSteps(s1, s2)

	wf, err := reg.(*stepRegistry).BuildWorkflow("wf1", []string{"step1", "step2"})
	assert.NoError(t, err)
	assert.NotNil(t, wf)
}

func TestRegistry_BuildWorkflow_InvalidStep(t *testing.T) {
	reg := NewRegistry()
	s1 := newSimpleTask("step1")
	reg.AddSteps(s1)

	wf, err := reg.(*stepRegistry).BuildWorkflow("wf1", []string{"step1", "missing"})
	assert.Error(t, err)
	assert.Nil(t, wf)
}
