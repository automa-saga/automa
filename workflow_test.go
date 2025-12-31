package automa

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func newTestWorkflow(id string, steps []Step) *workflow {
	return &workflow{
		id:            id,
		steps:         steps,
		executionMode: StopOnError,
		rollbackMode:  ContinueOnError,
	}
}

func TestWorkflow_ExecutesAllSteps(t *testing.T) {
	s1 := &defaultStep{id: "s1", execute: func(ctx context.Context, stp Step) *Report {
		return StepSuccessReport("s1")
	}}
	s2 := &defaultStep{id: "s2", execute: func(ctx context.Context, stp Step) *Report {
		return StepSuccessReport("s2")
	}}
	wf := newTestWorkflow("wf", []Step{s1, s2})

	report := wf.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Len(t, report.StepReports, 2)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, StopOnError, report.ExecutionMode)
	assert.Equal(t, ContinueOnError, report.RollbackMode)
}

func TestWorkflow_StopsOnStepError(t *testing.T) {
	s1 := &defaultStep{id: "s1", execute: func(ctx context.Context, stp Step) *Report {
		return StepSuccessReport("s1")
	}}
	s2 := &defaultStep{id: "s2", execute: func(ctx context.Context, stp Step) *Report {
		return StepFailureReport("s2", WithError(StepExecutionError.New("some error")))
	}}

	wf := newTestWorkflow("wf", []Step{s1, s2})

	report := wf.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
}

func TestWorkflow_ContinueOnSkippedStep(t *testing.T) {
	s1 := &defaultStep{id: "s1", execute: func(ctx context.Context, stp Step) *Report {
		return StepSkippedReport("s1")
	}}
	s2 := &defaultStep{id: "s2", execute: func(ctx context.Context, stp Step) *Report {
		return StepSuccessReport("s2")
	}}
	wf := &workflow{id: "wf", steps: []Step{s1, s2}}

	report := wf.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, StatusSkipped, report.StepReports[0].Status)
	assert.Equal(t, StatusSuccess, report.StepReports[1].Status)
}

func TestWorkflow_RollbackModeStopOnError(t *testing.T) {
	s := &defaultStep{id: "s", execute: func(ctx context.Context, stp Step) *Report {
		return StepFailureReport("s", WithError(StepExecutionError.New("some error")))
	}}

	wf := newTestWorkflow("wf", []Step{s})
	wf.rollbackMode = StopOnError

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
		rollback: func(ctx context.Context, stp Step) *Report {
			rollbackCalled["step1"] = true
			return StepSuccessReport("step1")
		},
	}
	step2 := &defaultStep{
		id: "step2",
		rollback: func(ctx context.Context, stp Step) *Report {
			rollbackCalled["step2"] = true
			return StepSuccessReport("step2")
		},
	}

	wf := newTestWorkflow("wf", []Step{step1, step2})
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
		execute: func(ctx context.Context, stp Step) *Report {
			return StepSuccessReport("step1")
		},
	}
	step2 := &defaultStep{
		id: "step2",
		execute: func(ctx context.Context, stp Step) *Report {
			return StepSuccessReport("step2")
		},
	}

	wf := newTestWorkflow("wf-success", []Step{step1, step2})

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
		rollback: func(ctx context.Context, stp Step) *Report {
			return StepFailureReport("step1", WithError(failErr))
		},
	}
	step2 := &defaultStep{
		id: "step2",
		rollback: func(ctx context.Context, stp Step) *Report {
			return StepSuccessReport("step2")
		},
	}

	wf := newTestWorkflow("wf", []Step{step1, step2})

	reports := wf.rollbackFrom(ctx, 1, nil)
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
		rollback: func(ctx context.Context, stp Step) *Report {
			return StepFailureReport("step1", WithError(failErr))
		},
	}
	step2 := &defaultStep{
		id: "step2",
		rollback: func(ctx context.Context, stp Step) *Report {
			return StepSuccessReport("step2")
		},
	}

	wf := newTestWorkflow("wf", []Step{step1, step2})
	wf.rollbackMode = ContinueOnError

	reports := wf.rollbackFrom(ctx, 1, nil)
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
		rollback: func(ctx context.Context, stp Step) *Report {
			return StepSuccessReport("step1")
		},
	}
	step2 := &defaultStep{
		id: "step2",
		rollback: func(ctx context.Context, stp Step) *Report {
			return StepFailureReport("step2", WithError(failErr))
		},
	}

	wf := newTestWorkflow("wf", []Step{step1, step2})
	wf.rollbackMode = StopOnError

	reports := wf.rollbackFrom(ctx, 1, nil)
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
		rollback: func(ctx context.Context, stp Step) *Report {
			return StepSkippedReport("step1")
		},
	}
	step2 := &defaultStep{
		id: "step2",
		rollback: func(ctx context.Context, stp Step) *Report {
			return StepFailureReport("step2", WithError(errors.New("rollback failed")))
		},
	}

	wf := newTestWorkflow("wf", []Step{step1, step2})
	wf.rollbackMode = ContinueOnError

	reports := wf.rollbackFrom(ctx, 1, nil)
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
	assert.NotNil(t, newCtx)
	assert.NotNil(t, wf.State())
}

func TestRunWorkflow_Success(t *testing.T) {
	wb := &WorkflowBuilder{
		workflow: &workflow{
			id: "wf",
		},
		stepSequence: []string{"s1"},
		stepBuilders: map[string]Builder{
			"s1": NewStepBuilder().WithId("s1").WithExecute(func(ctx context.Context, stp Step) *Report {
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
			prepare: func(ctx context.Context, stp Step) (context.Context, error) {
				return nil, errors.New("prepare failed")
			},
		},
		stepSequence: []string{"s1"},
		stepBuilders: map[string]Builder{
			"s1": NewStepBuilder().WithId("s1").WithExecute(func(ctx context.Context, stp Step) *Report {
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
		onCompletion: func(ctx context.Context, stp Step, report *Report) {
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
		onFailure: func(ctx context.Context, stp Step, report *Report) {
			done <- true
		},
		enableAsyncCallbacks: true,
		executionMode:        StopOnError,
	}
	wf.handleFailure(context.Background(), StepFailureReport("wf"))
	select {
	case <-done:
		// success
	case <-time.After(100 * time.Millisecond):
		t.Error("onFailure async callback not called")
	}
}

func TestWorkflow_Execute_NilState(t *testing.T) {
	wf := newTestWorkflow("wf", []Step{
		&defaultStep{id: "step", execute: func(ctx context.Context, stp Step) *Report {
			return StepSuccessReport("step")
		}},
	})
	wf.state = nil
	report := wf.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, ActionExecute, report.Action)
}

func TestWorkflow_Rollback_NilState(t *testing.T) {
	wf := newTestWorkflow("wf", []Step{newDefaultStep()})
	wf.state = nil
	report := wf.Rollback(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, ActionRollback, report.Action)
}

func TestWorkflow_StepStatePropagation(t *testing.T) {
	stepStateKey := Key("custom")
	stepStateValue := "value"
	workflowStateKey := Key("wf-custom")
	workflowStateValue := "wf-value"
	step := &defaultStep{
		id: "step",
		prepare: func(ctx context.Context, stp Step) (context.Context, error) {
			stp.State().Set(stepStateKey, stepStateValue) // set state
			return ctx, nil
		},
		execute: func(ctx context.Context, stp Step) *Report {
			state := stp.State()
			assert.NotNil(t, state)

			// retrieve state item and verify
			_, ok := state.Get(stepStateKey)
			assert.True(t, ok, "step state should contain the key set in prepare")
			assert.Equal(t, stepStateValue, StringFromState(state, stepStateKey))

			// also verify workflow state item is accessible
			_, ok = state.Get(workflowStateKey)
			assert.True(t, ok, "workflow state should not be accessible from step")

			return StepSuccessReport("step")
		},
	}

	wf := newTestWorkflow("wf", []Step{step})
	wf.State().Set(workflowStateKey, workflowStateValue)
	report := wf.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
}

func TestWorkflow_Execute_NilReportFromStep(t *testing.T) {
	step := &defaultStep{
		id: "step-id1",
		execute: func(ctx context.Context, stp Step) *Report {
			return nil // Simulate buggy step
		},
	}
	wf := newTestWorkflow("wf", []Step{step})
	report := wf.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
	assert.Contains(t, report.Error.Error(), `workflow "wf" completed with 1 step failures: [step-id1]`)
}

func TestWorkflow_Rollback_NilReportFromStep(t *testing.T) {
	step := &defaultStep{
		id: "step",
		rollback: func(ctx context.Context, stp Step) *Report {
			return nil // Simulate buggy rollback
		},
	}
	wf := newTestWorkflow("wf", []Step{step})
	report := wf.Rollback(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, ActionRollback, report.Action)
	assert.Len(t, report.StepReports, 1)
	assert.Equal(t, StatusFailed, report.StepReports[0].Status)
	assert.Contains(t, report.StepReports[0].Error.Error(), "rollback returned nil report")
}

func TestWorkflow_InvokeRollbackFunc_UserDefinedRollback(t *testing.T) {
	called := false
	wf := newTestWorkflow("wf", []Step{})
	wf.rollback = func(ctx context.Context, stp Step) *Report {
		called = true
		return StepSuccessReport("wf")
	}
	report := wf.Rollback(context.Background())
	assert.True(t, called, "user-defined rollback should be called")
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, ActionRollback, report.Action)
}

func TestWorkflow_InvokeRollbackFunc_UserDefinedRollbackFails(t *testing.T) {
	wf := newTestWorkflow("wf", []Step{})
	wf.rollback = func(ctx context.Context, stp Step) *Report {
		return StepFailureReport("wf", WithError(errors.New("rollback failed")))
	}
	report := wf.Rollback(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
	assert.Equal(t, ActionRollback, report.Action)
	assert.Contains(t, report.Error.Error(), "rollback failed")
}

func TestWorkflow_InvokeRollbackFunc_NilReport(t *testing.T) {
	wf := newTestWorkflow("wf", []Step{})
	wf.rollback = func(ctx context.Context, stp Step) *Report {
		return nil
	}
	report := wf.Rollback(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
	assert.Equal(t, ActionRollback, report.Action)
	assert.Contains(t, report.Error.Error(), "returned nil report")
}

func TestWorkflow_HandleCompletion_Sync(t *testing.T) {
	called := false
	wf := newTestWorkflow("wf", []Step{})
	wf.onCompletion = func(ctx context.Context, stp Step, report *Report) {
		called = true
	}
	wf.enableAsyncCallbacks = false
	wf.handleCompletion(context.Background(), StepSuccessReport("wf"))
	assert.True(t, called, "onCompletion should be called synchronously")
}

func TestWorkflow_HandleFailure_Sync(t *testing.T) {
	called := false
	wf := newTestWorkflow("wf", []Step{})
	wf.onFailure = func(ctx context.Context, stp Step, report *Report) {
		called = true
	}
	wf.enableAsyncCallbacks = false

	wf.handleFailure(context.Background(), StepFailureReport("wf"))
	assert.True(t, called, "onFailure should be called synchronously")
}

func TestWorkflow_Execute_ContinueOnError(t *testing.T) {
	ctx := context.Background()

	s1 := &defaultStep{id: "s1", execute: func(ctx context.Context, stp Step) *Report {
		return StepSuccessReport("s1")
	}}
	s2 := &defaultStep{id: "s2", execute: func(ctx context.Context, stp Step) *Report {
		return StepFailureReport("s2", WithError(StepExecutionError.New("failure in s2")))
	}}
	s3 := &defaultStep{id: "s3", execute: func(ctx context.Context, stp Step) *Report {
		return StepSuccessReport("s3")
	}}

	wf := newTestWorkflow("wf", []Step{s1, s2, s3})
	wf.executionMode = ContinueOnError

	report := wf.Execute(ctx)
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
	assert.Equal(t, ActionExecute, report.Action)
	assert.Len(t, report.StepReports, 3)

	// verify individual step statuses and ids
	assert.Equal(t, StatusSuccess, report.StepReports[0].Status)
	assert.Equal(t, "s1", report.StepReports[0].Id)

	assert.Equal(t, StatusFailed, report.StepReports[1].Status)
	assert.Equal(t, "s2", report.StepReports[1].Id)

	assert.Equal(t, StatusSuccess, report.StepReports[2].Status)
	assert.Equal(t, "s3", report.StepReports[2].Id)
}

func TestWorkflow_Execute_RollbackOnError(t *testing.T) {
	ctx := context.Background()

	s1RollbackCalled := false
	s2RollbackCalled := false

	s1 := &defaultStep{
		id: "s1",
		execute: func(ctx context.Context, stp Step) *Report {
			return StepSuccessReport("s1")
		},
		rollback: func(ctx context.Context, stp Step) *Report {
			s1RollbackCalled = true
			return StepSuccessReport("s1")
		},
	}

	s2 := &defaultStep{
		id: "s2",
		execute: func(ctx context.Context, stp Step) *Report {
			return StepFailureReport("s2", WithError(StepExecutionError.New("s2 failed")))
		},
		rollback: func(ctx context.Context, stp Step) *Report {
			s2RollbackCalled = true
			return StepFailureReport("s2", WithError(errors.New("rollback failed")))
		},
	}

	// this step should not be executed when RollbackOnError is used
	s3 := &defaultStep{
		id: "s3",
		execute: func(ctx context.Context, stp Step) *Report {
			t.Fatalf("s3 should not be executed in RollbackOnError mode")
			return nil
		},
	}

	wf := newTestWorkflow("wf", []Step{s1, s2, s3})
	wf.executionMode = RollbackOnError

	report := wf.Execute(ctx)
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
	assert.Equal(t, ActionExecute, report.Action)

	// only s1 and s2 should have been executed
	assert.Len(t, report.StepReports, 2)
	assert.Equal(t, "s1", report.StepReports[0].Id)
	assert.Equal(t, "s2", report.StepReports[1].Id)

	// rollback reports should be attached to executed step reports
	assert.NotNil(t, report.StepReports[0].Rollback)
	assert.NotNil(t, report.StepReports[1].Rollback)

	// verify rollback callbacks were invoked
	assert.True(t, s1RollbackCalled)
	assert.True(t, s2RollbackCalled)

	// verify rollback statuses match the step rollback implementations
	assert.Equal(t, StatusSuccess, report.StepReports[0].Rollback.Status)
	assert.Equal(t, StatusFailed, report.StepReports[1].Rollback.Status)
}

func TestWorkflow_Execute_StopOnError(t *testing.T) {
	ctx := context.Background()

	s1RollbackCalled := false
	s2RollbackCalled := false

	s1 := &defaultStep{
		id: "s1",
		execute: func(ctx context.Context, stp Step) *Report {
			return StepSuccessReport("s1")
		},
		rollback: func(ctx context.Context, stp Step) *Report {
			s1RollbackCalled = true
			return StepSuccessReport("s1")
		},
	}

	s2 := &defaultStep{
		id: "s2",
		execute: func(ctx context.Context, stp Step) *Report {
			return StepFailureReport("s2", WithError(StepExecutionError.New("s2 failed")))
		},
		rollback: func(ctx context.Context, stp Step) *Report {
			s2RollbackCalled = true
			return StepSuccessReport("s2")
		},
	}

	// this step should not be executed when StopOnError is used
	s3 := &defaultStep{
		id: "s3",
		execute: func(ctx context.Context, stp Step) *Report {
			t.Fatalf("s3 should not be executed in StopOnError mode")
			return nil
		},
	}

	wf := newTestWorkflow("wf", []Step{s1, s2, s3})
	// newTestWorkflow defaults to StopOnError, but set explicitly to be clear
	wf.executionMode = StopOnError

	report := wf.Execute(ctx)
	assert.NotNil(t, report)
	assert.Equal(t, StatusFailed, report.Status)
	assert.Equal(t, ActionExecute, report.Action)

	// only s1 and s2 should have been executed
	assert.Len(t, report.StepReports, 2)
	assert.Equal(t, "s1", report.StepReports[0].Id)
	assert.Equal(t, "s2", report.StepReports[1].Id)

	// in StopOnError mode no rollback should be attached to step reports
	assert.Nil(t, report.StepReports[0].Rollback)
	assert.Nil(t, report.StepReports[1].Rollback)

	// verify rollback callbacks were NOT invoked
	assert.False(t, s1RollbackCalled)
	assert.False(t, s2RollbackCalled)

	// verify individual statuses
	assert.Equal(t, StatusSuccess, report.StepReports[0].Status)
	assert.Equal(t, StatusFailed, report.StepReports[1].Status)
}

func TestWorkflow_SubWorkflow_Isolation(t *testing.T) {
	// prepare sub-workflow that reads parent's key and writes its own key
	var subSawParent string
	subStep := &defaultStep{
		id: "sub-step",
		execute: func(ctx context.Context, stp Step) *Report {
			subSawParent = StringFromState(stp.State(), Key("parentKey"))
			// mutate sub-workflow state (should not affect parent)
			stp.State().Set(Key("subKey"), "sub-value")
			return StepSuccessReport("sub-step")
		},
	}
	subWF := newTestWorkflow("subwf", []Step{subStep})

	// check step in parent that verifies parent was not mutated by sub-workflow
	var parentHasSubKey bool
	checkStep := &defaultStep{
		id: "check",
		execute: func(ctx context.Context, stp Step) *Report {
			_, parentHasSubKey = stp.State().Get(Key("subKey"))
			return StepSuccessReport("check")
		},
	}

	parent := newTestWorkflow("parent", []Step{subWF, checkStep})
	parent.State().Set(Key("parentKey"), "pval")

	report := parent.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, "pval", subSawParent, "sub-workflow should see parent's key value at start")
	assert.False(t, parentHasSubKey, "parent state must not be modified by sub-workflow")
}

func TestWorkflow_NonWorkflow_SharedState(t *testing.T) {
	// first step mutates shared state, second step should observe it
	setStep := &defaultStep{
		id: "set",
		prepare: func(ctx context.Context, stp Step) (context.Context, error) {
			stp.State().Set(Key("shared"), "val")
			return ctx, nil
		},
		execute: func(ctx context.Context, stp Step) *Report {
			return StepSuccessReport("set")
		},
	}

	var saw string
	checkStep := &defaultStep{
		id: "check",
		execute: func(ctx context.Context, stp Step) *Report {
			saw = StringFromState(stp.State(), Key("shared"))
			return StepSuccessReport("check")
		},
	}

	wf := newTestWorkflow("wf-shared", []Step{setStep, checkStep})

	report := wf.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, "val", saw, "subsequent non-workflow step should observe shared mutation")
	// workflow state should also contain the key
	assert.Equal(t, "val", StringFromState(wf.State(), Key("shared")))
}

func TestWorkflow_ParentMutates_BetweenSubWorkflows(t *testing.T) {
	// sub-workflow 1 records version seen
	var seen1 string
	sub1 := newTestWorkflow("sub1", []Step{
		&defaultStep{
			id: "s1",
			execute: func(ctx context.Context, stp Step) *Report {
				seen1 = StringFromState(stp.State(), Key("version"))
				return StepSuccessReport("s1")
			},
		},
	})

	// sub-workflow 2 records version seen
	var seen2 string
	sub2 := newTestWorkflow("sub2", []Step{
		&defaultStep{
			id: "s2",
			execute: func(ctx context.Context, stp Step) *Report {
				seen2 = StringFromState(stp.State(), Key("version"))
				return StepSuccessReport("s2")
			},
		},
	})

	// parent steps: set v1 -> sub1 -> set v2 -> sub2
	setV1 := &defaultStep{
		id: "set-v1",
		prepare: func(ctx context.Context, stp Step) (context.Context, error) {
			// mutate parent state visible to next sub-workflow (which will clone it)
			stp.State().Set(Key("version"), "v1")
			return ctx, nil
		},
		execute: func(ctx context.Context, stp Step) *Report { return StepSuccessReport("set-v1") },
	}
	setV2 := &defaultStep{
		id: "set-v2",
		prepare: func(ctx context.Context, stp Step) (context.Context, error) {
			stp.State().Set(Key("version"), "v2")
			return ctx, nil
		},
		execute: func(ctx context.Context, stp Step) *Report { return StepSuccessReport("set-v2") },
	}

	parent := newTestWorkflow("parent-versions", []Step{setV1, sub1, setV2, sub2})

	report := parent.Execute(context.Background())
	assert.NotNil(t, report)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.Equal(t, "v1", seen1, "first sub-workflow should see v1")
	assert.Equal(t, "v2", seen2, "second sub-workflow should see v2")
}
