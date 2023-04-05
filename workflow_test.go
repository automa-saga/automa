package automa

import (
	"context"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"testing"
)

type mockStopContainersStep struct {
	Step
	cache map[string][]byte
}

type mockFetchLatestStep struct {
	Step
	cache map[string][]byte
}

// it cannot be rollback
type mockNotifyStep struct {
	Step
	cache map[string][]byte
}

type mockRestartContainersStep struct {
	Step
	cache map[string][]byte
}

// run implements SagaRun function for execution of run logic
func (s *mockStopContainersStep) run(ctx context.Context) (skipped bool, err error) {
	fmt.Printf("RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))
	return false, nil
}

// rollback implements SagaRollback function for execution of rollback logic
func (s *mockStopContainersStep) rollback(ctx context.Context) (skipped bool, err error) {
	fmt.Println(string(s.cache["rollbackMsg"]))

	// mock error on rollback
	return false, errors.New("Mock error")
}

// run implements SagaRun function for execution of run logic
func (s *mockFetchLatestStep) run(ctx context.Context) (skipped bool, err error) {
	fmt.Printf("RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))
	return false, nil
}

// rollback implements SagaRollback function for execution of rollback logic
func (s *mockFetchLatestStep) rollback(ctx context.Context) (skipped bool, err error) {
	fmt.Println(string(s.cache["rollbackMsg"]))
	return false, nil
}

// run implements SagaRun function for execution of run logic
func (s *mockNotifyStep) run(ctx context.Context) (skipped bool, err error) {
	fmt.Printf("SKIP RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("SKIP ROLLBACK - %q", s.ID))
	return true, nil
}

// rollback implements SagaRollback function for execution of rollback logic
func (s *mockNotifyStep) rollback(ctx context.Context) (skipped bool, err error) {
	fmt.Println(string(s.cache["rollbackMsg"]))
	return true, nil
}

// run implements SagaRun function for execution of run logic
func (s *mockRestartContainersStep) run(ctx context.Context) (skipped bool, err error) {
	fmt.Printf("RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))
	return false, errors.New("Mock error on restart")
}

// rollback implements SagaRollback function for execution of rollback logic
func (s *mockRestartContainersStep) rollback(ctx context.Context) (skipped bool, err error) {
	fmt.Println(string(s.cache["rollbackMsg"]))
	return false, nil
}

func TestWorkflowEngine_Start(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := &mockStopContainersStep{
		Step:  Step{ID: "stop_containers"},
		cache: map[string][]byte{},
	}
	stop.RegisterSaga(stop.run, stop.rollback)

	fetch := &mockFetchLatestStep{
		Step:  Step{ID: "fetch_latest_images"},
		cache: map[string][]byte{},
	}
	fetch.RegisterSaga(fetch.run, fetch.rollback)

	notify := &mockNotifyStep{
		Step:  Step{ID: "notify_on_slack"},
		cache: map[string][]byte{},
	}
	notify.RegisterSaga(notify.run, notify.rollback)

	restart := &mockRestartContainersStep{
		Step:  Step{ID: "restart_containers"},
		cache: map[string][]byte{},
	}
	restart.RegisterSaga(restart.run, restart.rollback)

	registry := NewStepRegistry(zap.NewNop()).RegisterSteps(map[string]AtomicStep{
		stop.GetID():    stop,
		fetch.GetID():   fetch,
		notify.GetID():  notify,
		restart.GetID(): restart,
	})

	_, err := registry.BuildWorkflow("workflow_1", StepIDs{
		"INVALID",
	})
	assert.Error(t, err)

	// a new workflow with notify in the middle
	workflow1, err := registry.BuildWorkflow("workflow_1", StepIDs{
		stop.GetID(),
		fetch.GetID(),
		notify.GetID(),
		restart.GetID(),
	})
	assert.NoError(t, err)
	assert.Equal(t, "workflow_1", workflow1.GetID())
	defer workflow1.End(ctx)

	report, err := workflow1.Start(ctx)
	assert.Error(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, 8, len(report.StepReports)) // it will reach all steps and rollback

	// a new workflow with notify at the end
	workflow2, err := registry.BuildWorkflow("workflow_2", StepIDs{
		stop.GetID(),
		fetch.GetID(),
		restart.GetID(),
		notify.GetID(),
	})
	assert.NoError(t, err)
	assert.Equal(t, "workflow_2", workflow2.GetID())
	defer workflow2.End(ctx)

	report2, err := workflow2.Start(ctx)
	assert.Error(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, 6, len(report2.StepReports)) // it will not reach notify step
	assert.NotNil(t, report2.StepReports[5].Error)

	// a new workflow with no failure
	workflow3, err := registry.BuildWorkflow("workflow_3", StepIDs{
		stop.GetID(),
		fetch.GetID(),
		notify.GetID(),
	})
	assert.NoError(t, err)
	assert.Equal(t, "workflow_3", workflow3.GetID())
	defer workflow2.End(ctx)

	report3, err := workflow3.Start(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, 3, len(report3.StepReports))
	assert.Equal(t, StatusSuccess, report3.Status)
	assert.Equal(t, StepIDs{stop.GetID(), fetch.GetID(), notify.GetID()}, report3.StepSequence)
	for _, stepReport := range report2.StepReports {
		if (stepReport.StepID == restart.GetID() && stepReport.Action == RunAction) ||
			(stepReport.StepID == stop.GetID() && stepReport.Action == RollbackAction) {
			assert.Equal(t, StatusFailed, stepReport.Status)
		} else {
			assert.Equal(t, StatusSuccess, stepReport.Status)
		}
	}

	// NoOp scenario when first step is null
	noopWorkflow, err := registry.BuildWorkflow("noop_workflow", StepIDs{})
	assert.NoError(t, err)
	report4, err := noopWorkflow.Start(ctx)
	assert.NotNil(t, report4)
	assert.Equal(t, 0, len(report4.StepReports))
	assert.Nil(t, err)
}
