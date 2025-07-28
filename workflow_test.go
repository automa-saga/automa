package automa

import (
	"context"
	"fmt"
	"github.com/joomcode/errorx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"testing"
)

func TestWorkflowEngine_Start(t *testing.T) {
	ctx, cancel := NewContext(context.Background()).WithCancel()
	defer cancel()

	stop := &Task{
		ID: "stop_containers",
		Run: func(ctx *Context) error {
			return nil
		},
		Rollback: func(ctx *Context) error {
			return errorx.IllegalState.New("Mock error on rollback")
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

	registry := NewRegistry(nil).AddSteps(fetch, stop, notify, restart)

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
	require.Nil(t, err)
	assert.Equal(t, "workflow_1", workflow1.GetID())
	assert.Equal(t, 4, len(workflow1.GetSteps()))
	assert.Equal(t, []string{stop.GetID(), fetch.GetID(), notify.GetID(), restart.GetID()}, workflow1.GetStepSequence())
	assert.True(t, workflow1.HasStep(stop.GetID()))
	assert.False(t, workflow1.HasStep("INVALID"))
	defer workflow1.End(ctx)

	// modifying the step sequence externally should not affect the workflow
	seq := workflow1.GetStepSequence()
	seq = append(seq, "notify_all_on_slack")
	assert.Equal(t, []string{stop.GetID(), fetch.GetID(), notify.GetID(), restart.GetID()}, workflow1.GetStepSequence())

	report, err := workflow1.Execute(ctx)
	assert.Error(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, 8, len(report.StepReports)) // it will reach all steps and rollback

	r, err := yaml.Marshal(report)
	require.NoError(t, err)
	fmt.Println(string(r))

	// a new workflow with notify at the end
	workflow2, err := registry.BuildWorkflow("workflow_2", []string{
		stop.GetID(),
		fetch.GetID(),
		restart.GetID(),
		notify.GetID(),
	})
	require.Nil(t, err)
	assert.Equal(t, "workflow_2", workflow2.GetID())
	defer workflow2.End(ctx)

	report2, err := workflow2.Execute(ctx)
	assert.Error(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, 6, len(report2.StepReports)) // it will not reach notify step
	assert.NotNil(t, report2.StepReports[5].FailureReason)

	// a new workflow with no failure
	workflow3, err := registry.BuildWorkflow("workflow_3", []string{
		stop.GetID(),
		fetch.GetID(),
		notify.GetID(),
	})
	require.Nil(t, err)
	assert.Equal(t, "workflow_3", workflow3.GetID())
	defer workflow3.End(ctx)

	report3, err := workflow3.Execute(ctx)
	require.Nil(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, 3, len(report3.StepReports))
	assert.Equal(t, StatusSuccess, report3.Status)
	assert.Equal(t, []string{stop.GetID(), fetch.GetID(), notify.GetID()}, report3.StepSequence)
	for _, stepReport := range report2.StepReports {
		if (stepReport.StepID == restart.GetID() && stepReport.Action == RunAction) ||
			(stepReport.StepID == stop.GetID() && stepReport.Action == RollbackAction) {
			assert.Equal(t, StatusFailed, stepReport.Status)
		} else {
			assert.Equal(t, StatusSuccess, stepReport.Status)
		}
	}

	// NoOp scenario when first step is null
	noopWorkflow, err := registry.BuildWorkflow("noop_workflow", []string{})
	require.Nil(t, err)
	report4, err := noopWorkflow.Execute(ctx)
	assert.NotNil(t, report4)
	assert.Equal(t, 0, len(report4.StepReports))
	assert.Nil(t, err)
}
