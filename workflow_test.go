package automa

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNewWorkflow_ExecutesAllSteps(t *testing.T) {
	s1 := &defaultStep{id: "s1"}
	s2 := &defaultStep{id: "s2"}
	wf := NewWorkflow("wf", []Step{s1, s2})

	report, err := wf.Execute(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
}

func TestNewWorkflow_StopsOnStepError(t *testing.T) {
	s1 := &defaultStep{id: "s1"}
	s2 := &defaultStep{id: "s2", onExecute: func(ctx context.Context) (*Report, error) {
		return nil, errors.New("some error")
	}}
	wf := NewWorkflow("wf", []Step{s1, s2})

	report, err := wf.Execute(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
}

func TestNewWorkflow_RollbackModeStopOnError(t *testing.T) {
	s := &defaultStep{id: "s", onExecute: func(ctx context.Context) (*Report, error) {
		return nil, errors.New("some error")
	}}
	wf := NewWorkflow("wf", []Step{s}, WithRollbackMode(RollbackModeStopOnError))

	report, err := wf.Execute(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
}

func TestNewWorkflow_WithLogger(t *testing.T) {
	logger := zerolog.Nop()
	wf := NewWorkflow("wf", nil, WithWorkflowLogger(logger))
	assert.Equal(t, logger, wf.(*workflow).logger)
}

func TestNewWorkflow_Id(t *testing.T) {
	wf := NewWorkflow("mywf", nil)
	assert.Equal(t, "mywf", wf.Id())
}

func TestNewWorkflow_EmptySteps(t *testing.T) {
	wf := NewWorkflow("wf", nil)
	report, err := wf.Execute(context.Background())
	assert.Error(t, err)
	assert.Nil(t, report)
}

func TestWorkflow_OnRollback(t *testing.T) {
	ctx := context.Background()
	rollbackCalled := make(map[string]bool)

	// Custom defaultStep with Rollback tracking
	step1 := &defaultStep{
		id: "step1",
		onRollback: func(ctx context.Context) (*Report, error) {
			rollbackCalled["step1"] = true
			return StepSuccessReport("step1"), nil
		},
	}
	step2 := &defaultStep{
		id: "step2",
		onRollback: func(ctx context.Context) (*Report, error) {
			rollbackCalled["step2"] = true
			return StepSuccessReport("step2"), nil
		},
	}

	wf := NewWorkflow("wf", []Step{step1, step2}).(*workflow)
	report, err := wf.Rollback(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, ActionRollback, report.Action)
	assert.Len(t, report.StepReports, 2)
	assert.True(t, rollbackCalled["step1"])
	assert.True(t, rollbackCalled["step2"])
}

func TestWorkflow_Execute_StatusSuccess(t *testing.T) {
	ctx := context.Background()
	step1 := &defaultStep{
		id: "step1",
		onExecute: func(ctx context.Context) (*Report, error) {
			return StepSuccessReport("step1"), nil
		},
	}
	step2 := &defaultStep{
		id: "step2",
		onExecute: func(ctx context.Context) (*Report, error) {
			return StepSuccessReport("step2"), nil
		},
	}
	wf := NewWorkflow("wf-success", []Step{step1, step2})

	report, err := wf.Execute(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, ActionExecute, report.Action)
	assert.Len(t, report.StepReports, 2)
	for _, sr := range report.StepReports {
		assert.Equal(t, StatusSuccess, sr.Status)
	}
}

func TestWorkflow_RollbackFrom_FailedRollback(t *testing.T) {
	ctx := context.Background()
	failErr := errors.New("rollback failed")
	step1 := &defaultStep{
		id: "step1",
		onRollback: func(ctx context.Context) (*Report, error) {
			return nil, failErr
		},
	}
	step2 := &defaultStep{
		id: "step2",
		onRollback: func(ctx context.Context) (*Report, error) {
			return StepSuccessReport("step2"), nil
		},
	}
	wf := &workflow{
		id:    "wf",
		steps: []Step{step1, step2},
	}

	reports := wf.rollbackFrom(ctx, 1)
	assert.Len(t, reports, 2)
	assert.Equal(t, StatusFailed, reports["step1"].Status)
	assert.Equal(t, failErr, reports["step1"].Error)
	assert.Equal(t, StatusSuccess, reports["step2"].Status)
}

func TestWorkflow_RollbackFrom_ContinueOnError(t *testing.T) {
	ctx := context.Background()
	failErr := errors.New("rollback failed")
	step1 := &defaultStep{
		id: "step1",
		onRollback: func(ctx context.Context) (*Report, error) {
			return nil, failErr
		},
	}
	step2 := &defaultStep{
		id: "step2",
		onRollback: func(ctx context.Context) (*Report, error) {
			return StepSuccessReport("step2"), nil
		},
	}
	wf := &workflow{
		id:           "wf",
		steps:        []Step{step1, step2},
		rollbackMode: RollbackModeContinueOnError,
	}

	reports := wf.rollbackFrom(ctx, 1)
	assert.Len(t, reports, 2)
	assert.Equal(t, StatusFailed, reports["step1"].Status)
	assert.Equal(t, failErr, reports["step1"].Error)
	assert.Equal(t, StatusSuccess, reports["step2"].Status)
}

func TestWorkflow_RollbackFrom_StopOnError(t *testing.T) {
	ctx := context.Background()
	failErr := errors.New("rollback failed")
	step1 := &defaultStep{
		id: "step1",
		onRollback: func(ctx context.Context) (*Report, error) {
			return StepSuccessReport("step2"), nil
		},
	}
	step2 := &defaultStep{
		id: "step2",
		onRollback: func(ctx context.Context) (*Report, error) {
			return nil, failErr
		},
	}
	wf := &workflow{
		id:           "wf",
		steps:        []Step{step1, step2},
		rollbackMode: RollbackModeStopOnError,
	}

	reports := wf.rollbackFrom(ctx, 1)
	assert.Len(t, reports, 1)
	assert.Equal(t, StatusFailed, reports["step2"].Status)
	assert.Equal(t, failErr, reports["step2"].Error)
	_, ok := reports["step1"]
	assert.False(t, ok, "step1 should not be rolled back after failure in stop-on-error mode")
}

func TestWorkflow_RollbackFrom_SkippedStatus(t *testing.T) {
	ctx := context.Background()
	step1 := &defaultStep{
		id: "step1",
		onRollback: func(ctx context.Context) (*Report, error) {
			return &Report{Status: StatusSkipped}, nil
		},
	}
	step2 := &defaultStep{
		id: "step2",
		onRollback: func(ctx context.Context) (*Report, error) {
			return StepSuccessReport("step2"), nil
		},
	}
	wf := &workflow{
		id:           "wf",
		steps:        []Step{step1, step2},
		rollbackMode: RollbackModeContinueOnError,
	}

	reports := wf.rollbackFrom(ctx, 1)
	assert.Len(t, reports, 2)
	assert.Equal(t, StatusSkipped, reports["step1"].Status)
	assert.Equal(t, StatusSuccess, reports["step2"].Status)
}
