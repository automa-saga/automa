package automa

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// minimal Builder for testing
type mockStepBuilder struct {
	id        string
	valid     bool
	buildStep Step
}

func (b *mockStepBuilder) Id() string { return b.id }
func (b *mockStepBuilder) Validate() error {
	if b.valid {
		return nil
	} else {
		return errors.New("invalid")
	}
}
func (b *mockStepBuilder) Build() (Step, error) { return b.buildStep, nil }

func TestWorkflowBuilder_Steps_AddAndBuild(t *testing.T) {
	wb := NewWorkflowBuilder()
	b1 := &mockStepBuilder{id: "step1", valid: true, buildStep: &defaultStep{id: "step1"}}
	b2 := &mockStepBuilder{id: "step2", valid: true, buildStep: &defaultStep{id: "step2"}}
	wb.Steps(b1, b2)
	assert.Equal(t, []string{"step1", "step2"}, wb.stepSequence)

	step, err := wb.Build()
	assert.NoError(t, err)
	assert.NotNil(t, step)
	assert.Equal(t, "step1", step.(*workflow).steps[0].Id())
	assert.Equal(t, "step2", step.(*workflow).steps[1].Id())
}

func TestWorkflowBuilder_NamedSteps_UsesRegistry(t *testing.T) {
	reg := NewRegistry()
	b1 := &mockStepBuilder{id: "named1", valid: true}
	b2 := &mockStepBuilder{id: "named2", valid: true}
	reg.Add(b1, b2)

	wb := NewWorkflowBuilder().WithRegistry(reg)
	wb.NamedSteps("named1", "named2")
	assert.Equal(t, []string{"named1", "named2"}, wb.stepSequence)
}

func TestWorkflowBuilder_Validate_Errors(t *testing.T) {
	wb := NewWorkflowBuilder()
	b1 := &mockStepBuilder{id: "bad", valid: false}
	wb.Steps(b1)
	err := wb.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation errors")
}

func TestWorkflowBuilder_Build_MissingStep(t *testing.T) {
	wb := NewWorkflowBuilder()
	wb.stepSequence = []string{"missing"}
	_, err := wb.Build()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no steps provided for workflow")
}

func TestWorkflowBuilder_WithId_WithLogger_WithRollbackMode(t *testing.T) {
	wb := NewWorkflowBuilder()
	wb.WithId("wf-id")
	assert.Equal(t, "wf-id", wb.Id())

	logger := zerolog.Nop()
	wb.WithLogger(logger)
	assert.Equal(t, logger, wb.workflow.logger)

	wb.WithRollbackMode(RollbackModeContinueOnError)
	assert.Equal(t, RollbackModeContinueOnError, wb.workflow.rollbackMode)
}

func TestWorkflowBuilder_WithOnCompletion_WithOnFailure(t *testing.T) {
	wb := NewWorkflowBuilder()
	called := false
	wb.WithOnCompletion(func(ctx context.Context, r *Report) {
		called = true
	})
	assert.NotNil(t, wb.workflow.onCompletion)
	wb.workflow.onCompletion(context.Background(), &Report{})
	assert.True(t, called)

	failCalled := false
	wb.WithOnFailure(func(ctx context.Context, r *Report) { failCalled = true })
	assert.NotNil(t, wb.workflow.onFailure)
	wb.workflow.onFailure(context.Background(), &Report{})
	assert.True(t, failCalled)
}
