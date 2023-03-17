package automa

import (
	"context"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

type Step1 struct {
	Step
	cache map[string][]byte
}

type Step2 struct {
	Step
	cache map[string][]byte
}

type StepSkipped struct {
	Step
	cache map[string][]byte
}

type Step3 struct {
	Step
	cache map[string][]byte
}

func (s *Step1) Run(ctx context.Context, prevSuccess *Success) (Reports, error) {
	report := StartReport(s.ID)
	fmt.Println(fmt.Sprintf("RUN - %q", s.ID))
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))
	return s.RunNext(ctx, prevSuccess, report)
}

func (s *Step1) Rollback(ctx context.Context, prevFailure *Failure) (Reports, error) {
	report := StartReport(s.ID)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *Step2) Run(ctx context.Context, prevSuccess *Success) (Reports, error) {
	fmt.Println(fmt.Sprintf("RUN - %q", s.ID))
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))

	return s.RunNext(ctx, prevSuccess, nil)
}

func (s *Step2) Rollback(ctx context.Context, prevFailure *Failure) (Reports, error) {
	report := StartReport(s.ID)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *StepSkipped) Run(ctx context.Context, prevSuccess *Success) (Reports, error) {
	fmt.Println(fmt.Sprintf("SKIP RUN- %q", s.ID))
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("SKIP ROLLBACK - %q", s.ID))
	return s.SkippedRun(ctx, prevSuccess, nil)
}

func (s *StepSkipped) Rollback(ctx context.Context, prevFailure *Failure) (Reports, error) {
	report := StartReport(s.ID)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.SkippedRollback(ctx, prevFailure, report)
}

func (s *Step3) Run(ctx context.Context, prevSuccess *Success) (Reports, error) {
	report := StartReport(s.ID)

	fmt.Println(fmt.Sprintf("RUN - %q", s.ID))
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))

	// trigger rollback on error
	err := errors.New("error running state 3")
	report.Error = errors.EncodeError(ctx, err)
	if err != nil {
		return s.Rollback(ctx, NewRollbackTrigger(prevSuccess, err, report))
	}

	return s.RunNext(ctx, prevSuccess, report)
}

func (s *Step3) Rollback(ctx context.Context, prevFailure *Failure) (Reports, error) {
	report := StartReport(s.ID)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func TestWorkflowEngine_Start(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	state1 := &Step1{
		Step:  Step{ID: "Step1"},
		cache: map[string][]byte{},
	}

	state2 := &Step2{
		Step:  Step{ID: "Step2"},
		cache: map[string][]byte{},
	}

	skippedState :=
		&StepSkipped{
			Step:  Step{ID: "SkippedStep"},
			cache: map[string][]byte{},
		}

	state3 := &Step3{
		Step:  Step{ID: "Step3"},
		cache: map[string][]byte{},
	}

	fsm := NewWorkflow(WithSteps(state1, state2, skippedState, state3))
	defer fsm.End(ctx)

	reports, err := fsm.Start(ctx)
	assert.Error(t, err)
	assert.NotNil(t, reports[state3.ID].Error)
}
