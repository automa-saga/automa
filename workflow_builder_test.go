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
	wb := NewWorkflowBuilder().WithId("workflow")
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
	wb := NewWorkflowBuilder().WithId("workflow")
	b1 := &mockStepBuilder{id: "bad", valid: false}
	wb.Steps(b1)
	err := wb.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation errors")
}

func TestWorkflowBuilder_Build_MissingStep(t *testing.T) {
	wb := NewWorkflowBuilder().WithId("workflow")
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
	wb.WithOnCompletion(func(ctx context.Context, stp Step, r *Report) {
		called = true
	})
	assert.NotNil(t, wb.workflow.onCompletion)

	st := &defaultStep{id: "step1"}
	wb.workflow.onCompletion(context.Background(), st, &Report{})
	assert.True(t, called)

	failCalled := false
	wb.WithOnFailure(func(ctx context.Context, stp Step, r *Report) { failCalled = true })
	assert.NotNil(t, wb.workflow.onFailure)
	wb.workflow.onFailure(context.Background(), st, &Report{})
	assert.True(t, failCalled)
}

func TestWorkflowBuilder_WithAsyncCallbacks(t *testing.T) {
	wb := NewWorkflowBuilder()
	wb.WithAsyncCallbacks(true)
	assert.True(t, wb.workflow.enableAsyncCallbacks)
	wb.WithAsyncCallbacks(false)
	assert.False(t, wb.workflow.enableAsyncCallbacks)
}

func TestWorkflowBuilder_WithPrepare_SetsFunc(t *testing.T) {
	state := &SyncStateBag{}
	state.Set("test", 123)
	wb := NewWorkflowBuilder().WithId("wf").
		WithState(state).
		Steps(&mockStepBuilder{id: "step", valid: true, buildStep: &defaultStep{id: "step"}})
	called := false
	wb.WithPrepare(func(ctx context.Context, stp Step) (context.Context, error) {
		// check state
		if stp.State() == nil {
			return ctx, errors.New("state bag missing")
		}

		val, ok := stp.State().Get("test")
		if !ok || val != 123 {
			return ctx, errors.New("state bag value incorrect")
		}

		stp.State().Set("test", 456) // modify state

		called = true
		return ctx, nil
	})

	wf, err := wb.Build()
	assert.NoError(t, err)
	assert.NotNil(t, wf)

	ctx, err := wf.Prepare(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, ctx)
	assert.NotNil(t, wf.State())
	val, ok := wf.State().Get("test")
	assert.True(t, ok)
	assert.Equal(t, 456, val)
	assert.True(t, called)
}

func TestWorkflowBuilder_NamedSteps_RegistryNilOrMissingId(t *testing.T) {
	wb := NewWorkflowBuilder()
	wb.NamedSteps("missing") // registry is nil, should do nothing
	assert.Empty(t, wb.stepSequence)

	reg := NewRegistry()
	wb.WithRegistry(reg)
	wb.NamedSteps("notfound") // id not in registry, should do nothing
	assert.Empty(t, wb.stepSequence)
}

func TestWorkflowBuilder_Build_ResetsBuilder(t *testing.T) {
	wb := NewWorkflowBuilder().WithId("workflow")
	b := &mockStepBuilder{id: "step", valid: true, buildStep: &defaultStep{id: "step"}}
	wb.Steps(b)
	step, err := wb.Build()
	assert.NoError(t, err)
	assert.NotNil(t, step)
	// After Build, workflow is reset
	assert.NotEqual(t, "workflow", wb.workflow.id)
	assert.Empty(t, wb.workflow.steps)
}

func TestWorkflowBuilder_Steps_DuplicateStepId(t *testing.T) {
	wb := NewWorkflowBuilder().WithId("workflow")
	b1 := &mockStepBuilder{id: "step", valid: true}
	b2 := &mockStepBuilder{id: "step", valid: true}
	wb.Steps(b1, b2)
	// Only one step with the same id should be added
	assert.Equal(t, []string{"step"}, wb.stepSequence)
}

func TestWorkflowBuilder_Validate_EmptyWorkflowId(t *testing.T) {
	wb := NewWorkflowBuilder()
	b := &mockStepBuilder{id: "step", valid: true}
	wb.Steps(b)
	err := wb.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workflow id cannot be empty")
}

func TestWorkflowBuilder_Validate_NoSteps(t *testing.T) {
	wb := NewWorkflowBuilder().WithId("wf")
	err := wb.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no steps provided for workflow")
}

func TestWorkflowBuilder_MethodChaining(t *testing.T) {
	wb := NewWorkflowBuilder().
		WithId("chain").
		WithRollbackMode(RollbackModeStopOnError).
		WithAsyncCallbacks(true).
		WithState(&SyncStateBag{}).
		WithLogger(zerolog.Nop())
	assert.Equal(t, "chain", wb.Id())
	assert.Equal(t, RollbackModeStopOnError, wb.workflow.rollbackMode)
	assert.True(t, wb.workflow.enableAsyncCallbacks)
	assert.NotNil(t, wb.workflow.state)
}

func TestWorkflowBuilder_WithRollback_SetsRollbackFunc(t *testing.T) {
	wb := NewWorkflowBuilder()
	called := false
	wb.WithRollback(func(ctx context.Context, stp Step) *Report {
		called = true
		return StepSuccessReport("wf")
	})
	assert.NotNil(t, wb.workflow.rollback)
	wb.workflow.rollback(context.Background(), &defaultStep{id: "step"})
	assert.True(t, called)
}

func TestWorkflowBuilder_WithRegistry_NilRegistry(t *testing.T) {
	wb := NewWorkflowBuilder()
	wb.WithRegistry(nil)
	assert.Nil(t, wb.registry)
}

func TestWorkflowBuilder_NamedSteps_DuplicateIds(t *testing.T) {
	reg := NewRegistry()
	b := &mockStepBuilder{id: "dup", valid: true}
	err := reg.Add(b)
	assert.NoError(t, err)
	wb := NewWorkflowBuilder().WithRegistry(reg)
	wb.NamedSteps("dup", "dup")
	assert.Equal(t, []string{"dup"}, wb.stepSequence)
}
