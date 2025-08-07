package automa

import (
	"context"
	"fmt"
	"github.com/joomcode/errorx"
	assert "github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"testing"
)

func TestWorkflow_Forward_empty(t *testing.T) {
	wf := NewWorkflow("empty").(*workflow)
	ctx := NewContext(context.Background())
	result, err := wf.Forward(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, len(result.Report.StepReports))
	assert.Equal(t, StatusSuccess, result.Report.Status)
}

func TestWorkflow_RemoveSteps_empty(t *testing.T) {
	wf := NewWorkflow("empty").(*workflow)
	err := wf.RemoveSteps("nonexistent")
	assert.NoError(t, err)
}

func TestWorkflow_HasStep_nonexistent(t *testing.T) {
	wf := NewWorkflow("wf").(*workflow)
	assert.False(t, wf.HasStep("does_not_exist"))
}

func TestWorkflow_AddSteps_duplicateIDs(t *testing.T) {
	wf := NewWorkflow("wf").(*workflow)
	task1 := newSimpleTask("dup")
	task2 := newSimpleTask("dup")
	err := wf.AddSteps(task1, task2)
	assert.Error(t, err)
	assert.True(t, wf.HasStep("dup"))
}

func TestWorkflow_addStep_and_HasStep(t *testing.T) {
	wf := NewWorkflow("wf1").(*workflow)
	task1 := newSimpleTask("step1")
	task2 := newSimpleTask("step2")

	// Add first step
	err := wf.addStep(task1)
	assert.NoError(t, err)
	assert.True(t, wf.HasStep("step1"))

	// Add second step
	err = wf.addStep(task2)
	assert.NoError(t, err)
	assert.True(t, wf.HasStep("step2"))

	// Add duplicate step
	err = wf.addStep(task1)
	assert.Error(t, err)
}

func TestWorkflow_addStep_selfReference(t *testing.T) {
	wf := NewWorkflow("wf1").(*workflow)
	// Try to add itself as a step
	err := wf.addStep(wf)
	assert.Error(t, err)
}

func TestWorkflow_AddSteps_multiple(t *testing.T) {
	wf := NewWorkflow("wf2").(*workflow)
	task1 := newSimpleTask("a")
	task2 := newSimpleTask("b")
	err := wf.AddSteps(task1, task2)
	assert.NoError(t, err)
	assert.True(t, wf.HasStep("a"))
	assert.True(t, wf.HasStep("b"))
}

func TestWorkflow_RemoveSteps(t *testing.T) {
	wf := NewWorkflow("wf3").(*workflow)
	task1 := newSimpleTask("x")
	task2 := newSimpleTask("y")
	err := wf.addStep(task1)
	assert.NoError(t, err)
	err = wf.addStep(task2)
	assert.NoError(t, err)
	assert.True(t, wf.HasStep("x"))
	assert.True(t, wf.HasStep("y"))

	err = wf.RemoveSteps("x")
	assert.NoError(t, err)
	assert.False(t, wf.HasStep("x"))
	assert.True(t, wf.HasStep("y"))

	err = wf.RemoveSteps("y", "notfound")
	assert.NoError(t, err)
	assert.False(t, wf.HasStep("y"))
}

func TestWorkflow_GetStepSequence_externalModificationHasNoEffect(t *testing.T) {
	wf := NewWorkflow("wf4").(*workflow)
	task1 := newSimpleTask("a")
	task2 := newSimpleTask("b")
	err := wf.addStep(task1)
	assert.NoError(t, err)
	err = wf.addStep(task2)
	assert.NoError(t, err)
	seq := wf.GetStepSequence()
	assert.Equal(t, []string{"a", "b"}, seq)
	seq = append(seq, "c")
	assert.Equal(t, []string{"a", "b"}, wf.GetStepSequence())
}

func TestWorkflow_Reverse_empty(t *testing.T) {
	wf := NewWorkflow("empty").(*workflow)
	ctx := NewContext(context.Background())
	result, err := wf.Reverse(ctx)
	assert.Nil(t, result)
	assert.Error(t, err)
}

func TestWorkflowEngine_Forward(t *testing.T) {
	ctx, cancel := NewContext(context.Background()).WithCancel()
	defer cancel()

	stop := &Task{
		ID: "stop_containers",
		Run: func(ctx *Context) error {
			return nil
		},
		Rollback: func(ctx *Context) error {
			return errorx.IllegalState.New("Cannot rollback stopped container")
		},
	}

	fetch := &Task{
		ID: "fetch_latest_images",
		Run: func(ctx *Context) error {
			return nil
		},
		Rollback: func(ctx *Context) error {
			return nil
		},
	}

	notify := &Task{
		ID: "notify_all_on_slack",
		Skip: func(ctx *Context) bool {
			return true // this step is skipped
		},
		Run: func(ctx *Context) error {
			fmt.Println("notify_all_on_slack:Run this shouldn't be printed as it is meant to be skipped")
			return nil
		},
		Rollback: func(ctx *Context) error {
			fmt.Println("notify_all_on_slack:Rollback this shouldn't be printed as it is meant to be skipped")
			return nil
		},
	}

	restart := &Task{
		ID: "restart_containers",
		Run: func(ctx *Context) error {
			return errorx.IllegalState.New("Mock error on restart")
		},
		Rollback: func(ctx *Context) error {
			return nil
		},
	}

	registry := NewRegistry().AddSteps(fetch, stop, notify, restart)

	_, err := registry.BuildWorkflow("workflow_1", []string{
		"INVALID",
	})
	assert.Error(t, err)

	// a new workflow with notify in the middle
	workflow1, err := registry.BuildWorkflow("workflow_1", []string{
		stop.GetID(),
		fetch.GetID(),
		notify.GetID(),
		restart.GetID(),
	})
	assert.NoError(t, err)
	assert.Equal(t, "workflow_1", workflow1.GetID())
	assert.Equal(t, 4, len(workflow1.GetStepSequence()))
	assert.Equal(t, []string{stop.GetID(), fetch.GetID(), notify.GetID(), restart.GetID()}, workflow1.GetStepSequence())
	assert.True(t, workflow1.HasStep(stop.GetID()))
	assert.False(t, workflow1.HasStep("INVALID"))

	// modifying the step sequence externally should not affect the workflow
	seq := workflow1.GetStepSequence()
	seq = append(seq, "notify_all_on_slack")
	assert.Equal(t, []string{stop.GetID(), fetch.GetID(), notify.GetID(), restart.GetID()}, workflow1.GetStepSequence())

	result, err := workflow1.Forward(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 8, len(result.Report.StepReports)) // it will reach all steps and rollback

	r, err := yaml.Marshal(result)
	assert.NoError(t, err)
	fmt.Println(string(r))

	// a new workflow with notify at the end
	workflow2, err := registry.BuildWorkflow("workflow_2", []string{
		stop.GetID(),
		fetch.GetID(),
		restart.GetID(),
		notify.GetID(),
	})
	assert.NoError(t, err)
	assert.Equal(t, "workflow_2", workflow2.GetID())

	result2, err := workflow2.Forward(ctx.setPrevResult(nil))
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 6, len(result2.Report.StepReports)) // it will not reach notify step
	assert.NotNil(t, result2.Report.StepReports[5].Error)

	// a new workflow with no failure
	workflow3, err := registry.BuildWorkflow("workflow_3", []string{
		stop.GetID(),
		fetch.GetID(),
		notify.GetID(),
	})
	assert.NoError(t, err)
	assert.Equal(t, "workflow_3", workflow3.GetID())

	result3, err := workflow3.Forward(ctx.setPrevResult(nil))
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, len(result3.Report.StepReports))
	assert.Equal(t, StatusSuccess, result3.Report.Status)
	assert.Equal(t, []string{stop.GetID(), fetch.GetID(), notify.GetID()}, result3.Report.StepSequence)

	for _, stepReport := range result3.Report.StepReports {
		if (stepReport.StepID == restart.GetID() && stepReport.Action == RunAction) ||
			(stepReport.StepID == stop.GetID() && stepReport.Action == RollbackAction) {
			assert.Equal(t, StatusFailed, stepReport.Status)
		} else {
			if stepReport.StepID == notify.GetID() && stepReport.Action == RunAction {
				assert.Equal(t, StatusSkipped, stepReport.Status)
			} else {
				assert.Equal(t, StatusSuccess, stepReport.Status)
			}
		}
	}

	// NoOp scenario when first step is null
	noopWorkflow, err := registry.BuildWorkflow("noop_workflow", []string{})
	assert.NoError(t, err)
	report4, err := noopWorkflow.Forward(ctx.setPrevResult(nil))
	assert.Nil(t, err)
	assert.NotNil(t, report4)
	assert.Equal(t, 0, len(report4.Report.StepReports))
	assert.Nil(t, err)
}

func TestComposableWorkflows_Execution(t *testing.T) {
	stop := &Task{
		ID:       "stop",
		Run:      func(ctx *Context) error { return nil },
		Rollback: func(ctx *Context) error { return nil },
	}
	fetch := &Task{
		ID:       "fetch",
		Run:      func(ctx *Context) error { return nil },
		Rollback: func(ctx *Context) error { return errorx.IllegalState.New("Mock error on fetch") },
	}
	notify := &Task{
		ID:       "notify",
		Run:      func(ctx *Context) error { return nil },
		Rollback: func(ctx *Context) error { return nil },
	}
	restart := &Task{
		ID:       "restart",
		Run:      func(ctx *Context) error { return errorx.IllegalState.New("Mock error on restart") },
		Rollback: func(ctx *Context) error { return nil },
	}

	// First workflow: stop -> fetch
	wf1 := NewWorkflow("wf1")
	assert.NoError(t, wf1.AddSteps(stop, fetch))

	// Second workflow: notify
	wf2 := NewWorkflow("wf2")
	assert.NoError(t, wf2.AddSteps(notify))

	// Compose: wf1 -> wf2 -> restart
	composed := NewWorkflow("composed")
	assert.NoError(t, composed.AddSteps(wf1, wf2, restart))

	ctx := NewContext(context.Background())
	result, err := composed.Forward(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, StatusFailed, result.Report.Status)
	assert.Equal(t, 8, len(result.Report.StepReports))
	assert.NotNil(t, result.Report.FirstFailureOnForward)
	assert.Equal(t, restart.GetID(), result.Report.FirstFailureOnForward.StepID)
	assert.NotNil(t, result.Report.LastFailureOnReverse)
	assert.Equal(t, fetch.GetID(), result.Report.LastFailureOnReverse.StepID)

	// The composed workflow should execute all steps in order
	expectedSequence := []string{"wf1", "wf2", "restart"}
	assert.Equal(t, expectedSequence, composed.GetStepSequence())
}
