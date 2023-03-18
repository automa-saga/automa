package automa

import (
	"context"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"testing"
)

type StopContainers struct {
	Step
	cache map[string][]byte
}

type FetchLatest struct {
	Step
	cache map[string][]byte
}

// it cannot be rollback
type NotifyStep struct {
	Step
	cache map[string][]byte
}

type RestartContainers struct {
	Step
	cache map[string][]byte
}

func (s *StopContainers) Run(ctx context.Context, prevSuccess *Success) (Reports, error) {
	report := NewReport(s.ID)
	fmt.Println(fmt.Sprintf("RUN - %q", s.ID))
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))
	return s.RunNext(ctx, prevSuccess, report)
}

func (s *StopContainers) Rollback(ctx context.Context, prevFailure *Failure) (Reports, error) {
	report := NewReport(s.ID)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *FetchLatest) Run(ctx context.Context, prevSuccess *Success) (Reports, error) {
	fmt.Println(fmt.Sprintf("RUN - %q", s.ID))
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))

	return s.RunNext(ctx, prevSuccess, nil)
}

func (s *FetchLatest) Rollback(ctx context.Context, prevFailure *Failure) (Reports, error) {
	report := NewReport(s.ID)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *NotifyStep) Run(ctx context.Context, prevSuccess *Success) (Reports, error) {
	fmt.Println(fmt.Sprintf("SKIP RUN- %q", s.ID))
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("SKIP ROLLBACK - %q", s.ID))
	return s.SkippedRun(ctx, prevSuccess, nil)
}

func (s *NotifyStep) Rollback(ctx context.Context, prevFailure *Failure) (Reports, error) {
	report := NewReport(s.ID)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *RestartContainers) Run(ctx context.Context, prevSuccess *Success) (Reports, error) {
	report := NewReport(s.ID)

	fmt.Println(fmt.Sprintf("RUN - %q", s.ID))
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))

	// trigger rollback on error
	err := errors.New("error running step 3")
	report.Error = errors.EncodeError(ctx, err)
	if err != nil {
		return s.Rollback(ctx, NewRollbackTrigger(prevSuccess, err, report))
	}

	return s.RunNext(ctx, prevSuccess, report)
}

func (s *RestartContainers) Rollback(ctx context.Context, prevFailure *Failure) (Reports, error) {
	report := NewReport(s.ID)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func TestWorkflowEngine_Start(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := &StopContainers{
		Step:  Step{ID: "Stop containers"},
		cache: map[string][]byte{},
	}

	fetch := &FetchLatest{
		Step:  Step{ID: "Fetch latest images"},
		cache: map[string][]byte{},
	}

	notify :=
		&NotifyStep{
			Step:  Step{ID: "Notify on Slack"},
			cache: map[string][]byte{},
		}

	restart := &RestartContainers{
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
}
