package automa

import (
	"context"
	"errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
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
			return StepFailureReport("step1", WithError(failErr))
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
	state := StateFromContext(newCtx)
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
	assert.Equal(t, ActionPrepare, report.Action)
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

func TestWorkflow_HandleCompletion_Async(t *testing.T) {
	done := make(chan bool, 1)
	wf := &workflow{
		onCompletion: func(ctx context.Context, report *Report) {
			done <- true
		},
		enableAsyncCallbacks: true,
	}
	wf.handleCompletion(context.Background(), StepSuccessReport("wf"))
	select {
	case <-done:
		// success
	case <-time.After(100 * time.Millisecond):
		t.Error("onCompletion async callback not called")
	}
}

func TestWorkflow_HandleFailure_Async(t *testing.T) {
	done := make(chan bool, 1)
	wf := &workflow{
		onFailure: func(ctx context.Context, report *Report) {
			done <- true
		},
		enableAsyncCallbacks: true,
	}
	wf.handleFailure(context.Background(), StepFailureReport("wf"))
	select {
	case <-done:
		// success
	case <-time.After(100 * time.Millisecond):
		t.Error("onFailure async callback not called")
	}
}

func TestWorkflow_Prepare_MergesState(t *testing.T) {
	wf := &workflow{id: "wf"}
	ctx := context.Background()
	state := &SyncStateBag{}
	state.Set("foo", "bar")
	ctx = context.WithValue(ctx, KeyState, state)
	newCtx, err := wf.Prepare(ctx)
	assert.NoError(t, err)
	merged := StateFromContext(newCtx)
	assert.Equal(t, "bar", StringFromState(merged, "foo"))
}

func TestWorkflow_Execute_NilState(t *testing.T) {
	wf := &workflow{id: "wf", steps: []Step{
		&defaultStep{id: "step", execute: func(ctx context.Context) *Report {
			return StepSuccessReport("step")
		}},
	}}
	wf.state = nil
	report := wf.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, ActionExecute, report.Action)
}

func TestWorkflow_Rollback_NilState(t *testing.T) {
	wf := &workflow{id: "wf", steps: []Step{newDefaultStep()}}
	wf.state = nil
	report := wf.Rollback(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, ActionRollback, report.Action)
}

func TestWorkflow_StepStatePropagation(t *testing.T) {
	stepStateKey := Key("custom")
	stepStateValue := "value"
	step := &defaultStep{
		id: "step",
		execute: func(ctx context.Context) *Report {
			state := StateFromContext(ctx)
			val := StringFromState(state, stepStateKey)
			assert.Equal(t, stepStateValue, val)
			return StepSuccessReport("step")
		},
	}
	wf := &workflow{id: "wf", steps: []Step{step}}
	wf.State().Set(stepStateKey, stepStateValue)
	report := wf.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
}

func TestWorkflow_Execute_NilReportFromStep(t *testing.T) {
	step := &defaultStep{
		id: "step",
		execute: func(ctx context.Context) *Report {
			return nil // Simulate buggy step
		},
	}
	wf := &workflow{id: "wf", steps: []Step{step}}
	report := wf.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
	assert.Contains(t, report.Error.Error(), `workflow "wf" failed at step "step"`)
}

func TestWorkflow_Rollback_NilReportFromStep(t *testing.T) {
	step := &defaultStep{
		id: "step",
		rollback: func(ctx context.Context) *Report {
			return nil // Simulate buggy rollback
		},
	}
	wf := &workflow{id: "wf", steps: []Step{step}}
	report := wf.Rollback(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, ActionRollback, report.Action)
	assert.Len(t, report.StepReports, 1)
	assert.Equal(t, StatusFailed, report.StepReports[0].Status)
	assert.Contains(t, report.StepReports[0].Error.Error(), "rollback returned nil report")
}
