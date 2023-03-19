package automa

import (
	"context"
	"fmt"
	"github.com/cockroachdb/errors"
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
