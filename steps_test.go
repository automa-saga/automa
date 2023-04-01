package automa

import (
	"context"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

type mockSuccessStepStep struct {
	Step
	cache map[string][]byte
}

type mockFailedStep struct {
	Step
	cache map[string][]byte
}

func (s *mockSuccessStepStep) Run(ctx context.Context, prevSuccess *Success) (*WorkflowReport, error) {
	report := NewStepReport(s.ID, RunAction)
	fmt.Printf("RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))
	return s.RunNext(ctx, prevSuccess, report)
}

func (s *mockSuccessStepStep) Rollback(ctx context.Context, prevFailure *Failure) (*WorkflowReport, error) {
	report := NewStepReport(s.ID, RollbackAction)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *mockFailedStep) Run(ctx context.Context, prevSuccess *Success) (*WorkflowReport, error) {
	report := NewStepReport(s.ID, RunAction)
	fmt.Printf("SKIP RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("SKIP ROLLBACK - %q", s.ID))
	return s.SkippedRun(ctx, prevSuccess, report)
}

func (s *mockFailedStep) Rollback(ctx context.Context, prevFailure *Failure) (*WorkflowReport, error) {
	report := NewStepReport(s.ID, RollbackAction)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func TestSkippedRun(t *testing.T) {
	s1 := &mockSuccessStepStep{
		Step:  Step{ID: "Stop containers"},
		cache: map[string][]byte{},
	}

	s2 := &mockSuccessStepStep{
		Step:  Step{ID: "Fetch latest images"},
		cache: map[string][]byte{},
	}
	ctx := context.Background()
	prevSuccess := &Success{workflowReport: NewWorkflowReport("skipped-run-test", nil)}
	report := NewStepReport(s1.ID, RunAction)

	reports, err := s1.SkippedRun(ctx, prevSuccess, report)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(reports.StepReports))

	s1.SetNext(s2)
	s2.SetPrev(s1)
	reports, err = s1.SkippedRun(ctx, prevSuccess, report)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(reports.StepReports))
}

func TestRollbackPrev(t *testing.T) {
	s1 := &mockSuccessStepStep{
		Step:  Step{ID: "stop_containers"},
		cache: map[string][]byte{},
	}

	s2 := &mockFailedStep{
		Step:  Step{ID: "fetch_latest_images"},
		cache: map[string][]byte{},
	}

	ctx := context.Background()
	prevFailure := &Failure{error: errors.New("Test"), workflowReport: NewWorkflowReport("rollback_test", nil)}
	report := NewStepReport(s1.ID, RollbackAction)

	reports, err := s1.SkippedRollback(ctx, prevFailure, report)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(reports.StepReports))

	s1.SetNext(s2)
	s2.SetPrev(s1)
	report = NewStepReport(s2.ID, RollbackAction)
	reports, err = s2.SkippedRollback(ctx, prevFailure, report)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(reports.StepReports))
}

func TestSkippedRollbackPrev(t *testing.T) {
	s1 := &mockSuccessStepStep{
		Step:  Step{ID: "stop_containers"},
		cache: map[string][]byte{},
	}

	s2 := &mockFailedStep{
		Step:  Step{ID: "fetch_latest_images"},
		cache: map[string][]byte{},
	}

	ctx := context.Background()
	prevFailure := &Failure{error: errors.New("Test"), workflowReport: NewWorkflowReport("rollback_test", nil)}
	report := NewStepReport(s1.ID, RollbackAction)

	reports, err := s1.RollbackPrev(ctx, prevFailure, report)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(reports.StepReports))

	s1.SetNext(s2)
	s2.SetPrev(s1)
	report = NewStepReport(s2.ID, RollbackAction)
	reports, err = s2.RollbackPrev(ctx, prevFailure, report)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(reports.StepReports))
}

func TestFailedRollback(t *testing.T) {
	s1 := &mockSuccessStepStep{
		Step:  Step{ID: "stop_containers"},
		cache: map[string][]byte{},
	}

	s2 := &mockFailedStep{
		Step:  Step{ID: "fetch_latest_images"},
		cache: map[string][]byte{},
	}

	ctx := context.Background()
	prevFailure := &Failure{error: errors.New("Test"), workflowReport: NewWorkflowReport("rollback_test", nil)}
	report := NewStepReport(s1.ID, RollbackAction)

	reports, err := s1.FailedRollback(ctx, prevFailure, errors.New("test"), report)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(reports.StepReports))

	s1.SetNext(s2)
	s2.SetPrev(s1)
	report = NewStepReport(s2.ID, RollbackAction)
	reports, err = s2.FailedRollback(ctx, prevFailure, errors.New("test2"), report)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(reports.StepReports))
}

func TestNextPrev(t *testing.T) {
	s1 := &mockSuccessStepStep{
		Step:  Step{ID: "stop_containers"},
		cache: map[string][]byte{},
	}

	assert.Nil(t, s1.GetNext())
	assert.Nil(t, s1.GetPrev())

	s2 := &mockSuccessStepStep{
		Step:  Step{ID: "fetch_latest_images"},
		cache: map[string][]byte{},
	}

	assert.Nil(t, s2.GetNext())
	assert.Nil(t, s2.GetPrev())

	s1.SetNext(s2)
	s2.SetPrev(s1)

	assert.NotNil(t, s1.GetNext())
	assert.Nil(t, s1.GetPrev())
	assert.Nil(t, s2.GetNext())
	assert.NotNil(t, s2.GetPrev())

}

func TestRun(t *testing.T) {
	s1 := &mockSuccessStepStep{
		Step:  Step{ID: "Step -1"},
		cache: map[string][]byte{},
	}

	s2 := &successStep{}
	ctx := context.Background()
	prevSuccess := &Success{workflowReport: NewWorkflowReport("run_test", nil)}

	s1.SetNext(s2)

	reports, err := s1.Run(ctx, prevSuccess)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(reports.StepReports))
}
