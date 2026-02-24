package automa

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflow_StatePreservation_Enabled(t *testing.T) {
	wb := NewWorkflowBuilder().WithId("test-workflow").
		WithStatePreservation(true) // explicitly enable (default)

	step1 := NewStepBuilder().WithId("step1").
		WithExecute(func(ctx context.Context, stp Step) *Report {
			stp.State().Local().Set("data", "step1-data")
			return SuccessReport(stp)
		})

	step2 := NewStepBuilder().WithId("step2").
		WithExecute(func(ctx context.Context, stp Step) *Report {
			stp.State().Local().Set("data", "step2-data")
			return SuccessReport(stp)
		})

	wb.Steps(step1, step2)
	wf, err := wb.Build()
	require.NoError(t, err)

	// Execute workflow
	report := wf.Execute(context.Background())
	assert.True(t, report.IsSuccess())

	// Verify state snapshots were preserved
	typedWf := wf.(*workflow)
	assert.NotNil(t, typedWf.lastExecutionStates)
	assert.Equal(t, 2, len(typedWf.lastExecutionStates))
	assert.Contains(t, typedWf.lastExecutionStates, "step1")
	assert.Contains(t, typedWf.lastExecutionStates, "step2")
}

func TestWorkflow_StatePreservation_Disabled(t *testing.T) {
	wb := NewWorkflowBuilder().WithId("test-workflow").
		WithStatePreservation(false) // disable state preservation

	step1 := NewStepBuilder().WithId("step1").
		WithExecute(func(ctx context.Context, stp Step) *Report {
			stp.State().Local().Set("data", "step1-data")
			return SuccessReport(stp)
		})

	step2 := NewStepBuilder().WithId("step2").
		WithExecute(func(ctx context.Context, stp Step) *Report {
			stp.State().Local().Set("data", "step2-data")
			return SuccessReport(stp)
		})

	wb.Steps(step1, step2)
	wf, err := wb.Build()
	require.NoError(t, err)

	// Execute workflow
	report := wf.Execute(context.Background())
	assert.True(t, report.IsSuccess())

	// Verify state snapshots were NOT preserved
	typedWf := wf.(*workflow)
	assert.Nil(t, typedWf.lastExecutionStates)
}

func TestWorkflow_StatePreservation_Default(t *testing.T) {
	// Test that default is enabled (backward compatibility)
	wb := NewWorkflowBuilder().WithId("test-workflow")
	// Don't call WithStatePreservation - use default

	step1 := NewStepBuilder().WithId("step1").
		WithExecute(func(ctx context.Context, stp Step) *Report {
			stp.State().Local().Set("data", "step1-data")
			return SuccessReport(stp)
		})

	wb.Steps(step1)
	wf, err := wb.Build()
	require.NoError(t, err)

	// Execute workflow
	report := wf.Execute(context.Background())
	assert.True(t, report.IsSuccess())

	// Verify state snapshots were preserved (default behavior)
	typedWf := wf.(*workflow)
	assert.NotNil(t, typedWf.lastExecutionStates)
	assert.Equal(t, 1, len(typedWf.lastExecutionStates))
}

func TestWorkflow_StatePreservation_DisabledRollback(t *testing.T) {
	wb := NewWorkflowBuilder().WithId("test-workflow").
		WithStatePreservation(false). // disable state preservation
		WithExecutionMode(RollbackOnError)

	var rollbackCalled bool

	step1 := NewStepBuilder().WithId("step1").
		WithExecute(func(ctx context.Context, stp Step) *Report {
			stp.State().Local().Set("data", "step1-data")
			return SuccessReport(stp)
		}).
		WithRollback(func(ctx context.Context, stp Step) *Report {
			rollbackCalled = true
			// With state preservation disabled, rollback uses workflow state (not snapshot)
			return SuccessReport(stp)
		})

	step2 := NewStepBuilder().WithId("step2").
		WithExecute(func(ctx context.Context, stp Step) *Report {
			return FailureReport(stp, WithError(IllegalArgument.New("simulated failure")))
		})

	wb.Steps(step1, step2)
	wf, err := wb.Build()
	require.NoError(t, err)

	// Execute workflow - step2 fails, triggers rollback
	report := wf.Execute(context.Background())
	assert.False(t, report.IsSuccess())

	// Verify rollback was called (even with state preservation disabled)
	assert.True(t, rollbackCalled)

	// Verify no state snapshots were preserved
	typedWf := wf.(*workflow)
	assert.Nil(t, typedWf.lastExecutionStates)
}
