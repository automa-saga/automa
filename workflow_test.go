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

func (s *mockStopContainersStep) Run(ctx context.Context, prevSuccess *Success) (Reports, error) {
	report := NewReport(s.ID)
	fmt.Printf("RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))
	return s.RunNext(ctx, prevSuccess, report)
}

func (s *mockStopContainersStep) Rollback(ctx context.Context, prevFailure *Failure) (Reports, error) {
	report := NewReport(s.ID)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *mockFetchLatestStep) Run(ctx context.Context, prevSuccess *Success) (Reports, error) {
	report := NewReport(s.ID)
	fmt.Printf("RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))

	return s.RunNext(ctx, prevSuccess, report)
}

func (s *mockFetchLatestStep) Rollback(ctx context.Context, prevFailure *Failure) (Reports, error) {
	report := NewReport(s.ID)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *mockNotifyStep) Run(ctx context.Context, prevSuccess *Success) (Reports, error) {
	report := NewReport(s.ID)
	fmt.Printf("SKIP RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("SKIP ROLLBACK - %q", s.ID))
	return s.SkippedRun(ctx, prevSuccess, report)
}

func (s *mockNotifyStep) Rollback(ctx context.Context, prevFailure *Failure) (Reports, error) {
	report := NewReport(s.ID)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *mockRestartContainersStep) Run(ctx context.Context, prevSuccess *Success) (Reports, error) {
	report := NewReport(s.ID)
	fmt.Printf("RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))

	// trigger rollback on error
	err := errors.New("error running step 3")
	report.Error = errors.EncodeError(ctx, err)
	if err != nil {
		return s.Rollback(ctx, NewRollbackTrigger(prevSuccess, err, report))
	}

	return s.RunNext(ctx, prevSuccess, report)
}

func (s *mockRestartContainersStep) Rollback(ctx context.Context, prevFailure *Failure) (Reports, error) {
	report := NewReport(s.ID)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func TestWorkflowEngine_Start(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := &mockStopContainersStep{
		Step:  Step{ID: "Stop containers"},
		cache: map[string][]byte{},
	}

	fetch := &mockFetchLatestStep{
		Step:  Step{ID: "Fetch latest images"},
		cache: map[string][]byte{},
	}

	notify := &mockNotifyStep{
		Step:  Step{ID: "Notify on Slack"},
		cache: map[string][]byte{},
	}

	restart := &mockRestartContainersStep{
		Step:  Step{ID: "Restart containers"},
		cache: map[string][]byte{},
	}

	registry := NewStepRegistry(zap.NewNop()).RegisterSteps(map[string]AtomicStep{
		stop.ID:    stop,
		fetch.ID:   fetch,
		notify.ID:  notify,
		restart.ID: restart,
	})

	// a new workflow with notify in the middle
	workflow := registry.BuildWorkflow([]string{
		stop.ID,
		fetch.ID,
		notify.ID,
		restart.ID,
	})
	defer workflow.End(ctx)

	reports, err := workflow.Start(ctx)
	assert.Error(t, err)
	assert.NotNil(t, reports)
	assert.Equal(t, 4, len(reports)) // it will reach all steps and rollback

	// a new workflow with notify at the end
	workflow2 := registry.BuildWorkflow([]string{
		stop.ID,
		fetch.ID,
		restart.ID,
		notify.ID,
	})
	defer workflow2.End(ctx)

	reports2, err := workflow2.Start(ctx)
	assert.Error(t, err)
	assert.NotNil(t, reports)
	assert.Equal(t, 3, len(reports2)) // it will not reach notify step
	assert.NotNil(t, reports2[restart.ID].Error)

	// NoOp scenario when first step is null
	noopWorkflow := registry.BuildWorkflow([]string{"INVALID-step1", "INVALID-step2"})
	noReports, err := noopWorkflow.Start(ctx)
	assert.NotNil(t, noReports)
	assert.Equal(t, 0, len(noReports))
	assert.Nil(t, err)
}
