package automa

import (
	"context"
	"fmt"
	"github.com/joomcode/errorx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

type mockSuccessStep struct {
	Step
	cache map[string][]byte
}

func (s *mockSuccessStep) run(ctx context.Context) (skipped bool, err *errorx.Error) {
	fmt.Printf("RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))
	return false, nil
}

func (s *mockSuccessStep) rollback(ctx context.Context) (skipped bool, err *errorx.Error) {
	fmt.Println(string(s.cache["rollbackMsg"]))
	return false, nil
}

// this is an example of AtomicStep that extends Step but overrides the Run and Rollback methods
type mockFailedStep struct {
	Step
	cache map[string][]byte
}

// override the AtomicStep.Run from Step
func (s *mockFailedStep) Run(ctx context.Context, prevSuccess *Success) (WorkflowReport, *errorx.Error) {
	report := NewStepReport(s.ID, RunAction)
	fmt.Printf("SKIP RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("SKIP ROLLBACK - %q", s.ID))
	return s.Rollback(ctx, NewFailedRun(ctx, prevSuccess, errorx.IllegalState.New("Mock error"), report))
}

// override the AtomicStep.Rollback from Step
func (s *mockFailedStep) Rollback(ctx context.Context, prevFailure *Failure) (WorkflowReport, *errorx.Error) {
	report := NewStepReport(s.ID, RollbackAction)
	fmt.Println(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func TestSkippedRun(t *testing.T) {
	s1 := &mockSuccessStep{
		Step:  Step{ID: "Stop containers"},
		cache: map[string][]byte{},
	}
	s1.RegisterSaga(s1.run, s1.rollback)

	s2 := &mockSuccessStep{
		Step:  Step{ID: "Fetch latest images"},
		cache: map[string][]byte{},
	}

	ctx := context.Background()
	mockReport := NewWorkflowReport("skipped-run-test", nil)
	prevSuccess := &Success{workflowReport: *mockReport}
	report := NewStepReport(s1.ID, RunAction)

	reports, err := s1.SkippedRun(ctx, prevSuccess, report)
	require.Nil(t, err)
	assert.Equal(t, 1, len(reports.StepReports))

	s1.SetNext(s2)
	s2.SetPrev(s1)
	reports, err = s1.SkippedRun(ctx, prevSuccess, report)
	require.Nil(t, err)
	assert.Equal(t, 3, len(reports.StepReports))

	// nil report should allow creating a default report
	prevSuccess = &Success{workflowReport: *mockReport}
	reports, err = s1.SkippedRun(ctx, prevSuccess, nil)
	require.Nil(t, err)
	assert.Equal(t, 2, len(reports.StepReports))

	// nil report should allow creating a default report
	prevSuccess = &Success{workflowReport: *mockReport}
	reports, err = s1.RunNext(ctx, prevSuccess, nil)
	require.Nil(t, err)
	assert.Equal(t, 2, len(reports.StepReports))
}

func TestSkippedRollbackPrev(t *testing.T) {
	s1 := &mockSuccessStep{
		Step:  Step{ID: "stop_containers"},
		cache: map[string][]byte{},
	}
	s1.RegisterSaga(s1.run, s1.rollback)

	s2 := &mockFailedStep{
		Step:  Step{ID: "fetch_latest_images"},
		cache: map[string][]byte{},
	}

	ctx := context.Background()
	mockReport := NewWorkflowReport("test", nil)
	prevFailure := &Failure{err: errorx.IllegalState.New("Test"), workflowReport: *mockReport}
	report := NewStepReport(s1.ID, RollbackAction)

	reports, err := s1.SkippedRollback(ctx, prevFailure, report)
	require.Nil(t, err)
	assert.Equal(t, 1, len(reports.StepReports))

	s1.SetNext(s2)
	s2.SetPrev(s1)
	report = NewStepReport(s2.ID, RollbackAction)
	reports, err = s2.SkippedRollback(ctx, prevFailure, report)
	require.Nil(t, err)
	assert.Equal(t, 3, len(reports.StepReports))

	// nil report should allow creating a default report
	prevFailure = &Failure{err: errorx.IllegalState.New("Test"), workflowReport: *mockReport}
	reports, err = s1.SkippedRollback(ctx, prevFailure, nil)
	require.Nil(t, err)
	assert.Equal(t, 1, len(reports.StepReports))
}

func TestRollbackPrev(t *testing.T) {
	s1 := &mockSuccessStep{
		Step:  Step{ID: "stop_containers"},
		cache: map[string][]byte{},
	}
	s1.RegisterSaga(s1.run, s1.rollback)

	s2 := &mockFailedStep{
		Step:  Step{ID: "fetch_latest_images"},
		cache: map[string][]byte{},
	}

	ctx := context.Background()
	mockReport := NewWorkflowReport("test", nil)
	prevFailure := &Failure{err: errorx.IllegalState.New("Test"), workflowReport: *mockReport}
	report := NewStepReport(s1.ID, RollbackAction)

	reports, err := s1.RollbackPrev(ctx, prevFailure, report)
	require.Nil(t, err)
	assert.Equal(t, 1, len(reports.StepReports))

	s1.SetNext(s2)
	s2.SetPrev(s1)
	report = NewStepReport(s2.ID, RollbackAction)
	reports, err = s2.RollbackPrev(ctx, prevFailure, report)
	require.Nil(t, err)
	assert.Equal(t, 3, len(reports.StepReports))

	// nil report should allow creating a default report
	prevFailure = &Failure{err: errorx.IllegalState.New("Test"), workflowReport: *mockReport}
	reports, err = s1.RollbackPrev(ctx, prevFailure, nil)
	require.Nil(t, err)
	assert.Equal(t, 1, len(reports.StepReports))
}

func TestFailedRollback(t *testing.T) {
	s1 := &mockSuccessStep{
		Step:  Step{ID: "stop_containers"},
		cache: map[string][]byte{},
	}
	s1.RegisterSaga(s1.run, s1.rollback)

	s2 := &mockFailedStep{
		Step:  Step{ID: "fetch_latest_images"},
		cache: map[string][]byte{},
	}

	ctx := context.Background()
	mockReport := NewWorkflowReport("test", nil)
	prevFailure := &Failure{err: errorx.IllegalState.New("Test"), workflowReport: *mockReport}
	report := NewStepReport(s1.ID, RollbackAction)

	reports, err := s1.FailedRollback(ctx, prevFailure, errorx.IllegalState.New("test"), report)
	require.Nil(t, err)
	assert.Equal(t, 1, len(reports.StepReports))

	s1.SetNext(s2)
	s2.SetPrev(s1)
	report = NewStepReport(s2.ID, RollbackAction)
	reports, err = s2.FailedRollback(ctx, prevFailure, errorx.IllegalState.New("test2"), report)
	require.Nil(t, err)
	assert.Equal(t, 3, len(reports.StepReports))

	// nil report should allow creating a default report
	prevFailure = &Failure{err: errorx.IllegalState.New("Test"), workflowReport: *mockReport}
	reports, err = s1.FailedRollback(ctx, prevFailure, errorx.IllegalState.New("test3"), nil)
	require.Nil(t, err)
	assert.Equal(t, 1, len(reports.StepReports))
}

func TestNextPrev(t *testing.T) {
	s1 := &mockSuccessStep{
		Step:  Step{ID: "stop_containers"},
		cache: map[string][]byte{},
	}
	s1.RegisterSaga(s1.run, s1.rollback)

	assert.Nil(t, s1.GetNext())
	assert.Nil(t, s1.GetPrev())

	s2 := &mockSuccessStep{
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

func TestRunSuccess(t *testing.T) {
	s1 := &mockSuccessStep{
		Step:  Step{ID: "Step -1"},
		cache: map[string][]byte{},
	}

	s2 := &mockSuccessStep{
		Step:  Step{ID: "Step -2"},
		cache: map[string][]byte{},
	}
	s2.RegisterSaga(s2.run, s2.rollback)

	ctx := context.Background()
	mockReport := NewWorkflowReport("test", nil)

	prevSuccess := &Success{workflowReport: *mockReport}
	reports, err := s1.Run(ctx, prevSuccess)
	require.Nil(t, err)
	assert.Equal(t, 1, len(reports.StepReports))
	assert.Equal(t, StatusSkipped, reports.StepReports[0].Status)

	s1.SetNext(s2)
	s2.SetPrev(s1)
	prevSuccess = &Success{workflowReport: *mockReport}
	reports, err = s1.Run(ctx, prevSuccess)
	require.Nil(t, err)
	assert.Equal(t, 2, len(reports.StepReports))
	assert.Equal(t, StatusSkipped, reports.StepReports[0].Status)
	assert.Equal(t, StatusSuccess, reports.StepReports[1].Status)

	s1.RegisterSaga(s1.run, s1.rollback)
	prevSuccess = &Success{workflowReport: *mockReport}
	reports, err = s1.Run(ctx, prevSuccess)
	require.Nil(t, err)
	assert.Equal(t, 2, len(reports.StepReports))
	assert.Equal(t, StatusSuccess, reports.StepReports[0].Status)
	assert.Equal(t, StatusSuccess, reports.StepReports[1].Status)
}

func TestRunWithFailure(t *testing.T) {
	s1 := &mockSuccessStep{
		Step:  Step{ID: "Step -1"},
		cache: map[string][]byte{},
	}
	s2 := &mockFailedStep{
		Step:  Step{ID: "Step -2"},
		cache: map[string][]byte{},
	}

	ctx := context.Background()
	mockReport := NewWorkflowReport("test", nil)

	prevSuccess := &Success{workflowReport: *mockReport}
	reports, err := s1.Run(ctx, prevSuccess)
	require.Nil(t, err)
	assert.Equal(t, 1, len(reports.StepReports))
	assert.Equal(t, StatusSkipped, reports.StepReports[0].Status)

	s1.SetNext(s2)
	s2.SetPrev(s1)
	prevSuccess = &Success{workflowReport: *mockReport}
	reports, err = s1.Run(ctx, prevSuccess)
	require.Nil(t, err)
	assert.Equal(t, 4, len(reports.StepReports))
	assert.Equal(t, StatusSkipped, reports.StepReports[0].Status)

	s1.RegisterSaga(s1.run, s1.rollback)
	prevSuccess = &Success{workflowReport: *mockReport}
	reports, err = s1.Run(ctx, prevSuccess)
	require.Nil(t, err)
	assert.Equal(t, 4, len(reports.StepReports))
	assert.Equal(t, StatusSuccess, reports.StepReports[0].Status)

}
