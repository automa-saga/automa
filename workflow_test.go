package automa

import (
	"context"
	"errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestWorkflow_ExecutesAllSteps(t *testing.T) {
	s1 := &defaultStep{id: "s1", execute: func(ctx context.Context) *Report {
		return StepSuccessReport("s1")
	}}
	s2 := &defaultStep{id: "s2", execute: func(ctx context.Context) *Report {
		return StepSuccessReport("s2")
	}}
	wf := &workflow{id: "wf", steps: []Step{s1, s2}}

	report := wf.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
}

func TestWorkflow_StopsOnStepError(t *testing.T) {
	s1 := &defaultStep{id: "s1", execute: func(ctx context.Context) *Report {
		return StepSuccessReport("s1")
	}}
	s2 := &defaultStep{id: "s2", execute: func(ctx context.Context) *Report {
		return StepFailureReport("s2", WithError(StepExecutionError.New("some error")))
	}}
	wf := &workflow{id: "wf", steps: []Step{s1, s2}}

	report := wf.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
}

func TestWorkflow_RollbackModeStopOnError(t *testing.T) {
	s := &defaultStep{id: "s", execute: func(ctx context.Context) *Report {
		return StepFailureReport("s", WithError(StepExecutionError.New("some error")))
	}}
	wf := &workflow{id: "wf", steps: []Step{s}, rollbackMode: RollbackModeStopOnError}

	report := wf.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
}

func TestWorkflow_WithLogger(t *testing.T) {
	logger := zerolog.Nop()
	wf := &workflow{id: "wf", logger: logger}
	assert.Equal(t, logger, wf.logger)
}

func TestWorkflow_Id(t *testing.T) {
	wf := &workflow{id: "mywf"}
	assert.Equal(t, "mywf", wf.Id())
}

func TestWorkflow_EmptySteps(t *testing.T) {
	wf := &workflow{id: "wf"}
	report := wf.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
}

func TestWorkflow_OnRollback(t *testing.T) {
	ctx := context.Background()
	rollbackCalled := make(map[string]bool)

	step1 := &defaultStep{
		id: "step1",
		rollback: func(ctx context.Context) *Report {
			rollbackCalled["step1"] = true
			return StepSuccessReport("step1")
		},
	}
	step2 := &defaultStep{
		id: "step2",
		rollback: func(ctx context.Context) *Report {
			rollbackCalled["step2"] = true
			return StepSuccessReport("step2")
		},
	}

	wf := &workflow{id: "wf", steps: []Step{step1, step2}}
	report := wf.Rollback(ctx)
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
		execute: func(ctx context.Context) *Report {
			return StepSuccessReport("step1")
		},
	}
	step2 := &defaultStep{
		id: "step2",
		execute: func(ctx context.Context) *Report {
			return StepSuccessReport("step2")
		},
	}
	wf := &workflow{id: "wf-success", steps: []Step{step1, step2}}

	report := wf.Execute(ctx)
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
		rollback: func(ctx context.Context) *Report {
			return StepFailureReport("step1", WithError(failErr))
		},
	}
	step2 := &defaultStep{
		id: "step2",
		rollback: func(ctx context.Context) *Report {
			return StepSuccessReport("step2")
		},
	}
	wf := &workflow{id: "wf", steps: []Step{step1, step2}}

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
		rollback: func(ctx context.Context) *Report {
			return StepFailureReport("step2", WithError(failErr))
		},
	}
	step2 := &defaultStep{
		id: "step2",
		rollback: func(ctx context.Context) *Report {
			return StepSuccessReport("step2")
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
		rollback: func(ctx context.Context) *Report {
			return StepSuccessReport("step1")
		},
	}
	step2 := &defaultStep{
		id: "step2",
		rollback: func(ctx context.Context) *Report {
			return StepFailureReport("step2", WithError(failErr))
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
		rollback: func(ctx context.Context) *Report {
			return StepSkippedReport("step1")
		},
	}
	step2 := &defaultStep{
		id: "step2",
		rollback: func(ctx context.Context) *Report {
			return StepFailureReport("step2", WithError(errors.New("rollback failed")))
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
	assert.Equal(t, StatusFailed, reports["step2"].Status)
}

func TestWorkflow_State_LazyInit(t *testing.T) {
	wf := &workflow{id: "wf"}
	assert.Nil(t, wf.state)
	state := wf.State()
	assert.NotNil(t, state)
	assert.Equal(t, 0, state.Size())
}

func TestWorkflow_Prepare_InjectsState(t *testing.T) {
	wf := &workflow{id: "wf"}
	ctx := context.Background()
	newCtx, err := wf.Prepare(ctx)
	assert.NoError(t, err)
	state := GetStateBagFromContext(newCtx)
	assert.NotNil(t, state)
}

func TestRunWorkflow_Success(t *testing.T) {
	wb := &WorkflowBuilder{
		workflow: &workflow{
			id: "wf",
		},
		stepSequence: []string{"s1"},
		stepBuilders: map[string]Builder{
			"s1": NewStepBuilder().WithId("s1").WithExecute(func(ctx context.Context) *Report {
				return StepSuccessReport("s1")
			}),
		},
	}
	report := RunWorkflow(context.Background(), wb)
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, "wf", report.Id)
}

func TestRunWorkflow_BuildError(t *testing.T) {
	wb := &WorkflowBuilder{
		workflow: &workflow{
			id: "",
		},
	}
	report := RunWorkflow(context.Background(), wb)
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
	assert.Contains(t, report.Error.Error(), "build failed")
}

func TestRunWorkflow_PrepareError(t *testing.T) {
	wb := &WorkflowBuilder{
		workflow: &workflow{
			id: "wf",
			prepare: func(ctx context.Context) (context.Context, error) {
				return nil, errors.New("prepare failed")
			},
		},
		stepSequence: []string{"s1"},
		stepBuilders: map[string]Builder{
			"s1": NewStepBuilder().WithId("s1").WithExecute(func(ctx context.Context) *Report {
				return StepSuccessReport("s1")
			}),
		},
	}
	report := RunWorkflow(context.Background(), wb)
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
	assert.Contains(t, report.Error.Error(), "prepare failed")
}
