package automa

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// defaultStep implements Step interfaces.
type defaultStep struct {
	id                   string
	logger               *zerolog.Logger
	prepare              PrepareFunc
	execute              ExecuteFunc
	rollback             RollbackFunc
	onCompletion         OnCompletionFunc
	onFailure            OnFailureFunc
	enableAsyncCallbacks bool
	state                NamespacedStateBag
}

func (s *defaultStep) State() NamespacedStateBag {
	if s.state == nil {
		// lazy initialization with empty local and global namespaces
		s.state = NewNamespacedStateBag(nil, nil)
	}
	return s.state
}

func (s *defaultStep) WithState(st NamespacedStateBag) Step {
	// avoid redundant assignment when same state is provided
	if s.state == st {
		return s
	}

	s.state = st
	return s
}

func (s *defaultStep) Id() string {
	return s.id
}

func (s *defaultStep) Prepare(ctx context.Context) (context.Context, error) {
	preparedCtx := ctx
	if s.prepare != nil {
		c, err := s.prepare(preparedCtx, s)
		if err != nil {
			return nil, err
		}
		preparedCtx = c // use the context returned by user prepare function
	}

	return preparedCtx, nil
}

func (s *defaultStep) Execute(ctx context.Context) *Report {
	start := time.Now()
	if s.execute != nil {
		report := s.execute(ctx, s)
		if report == nil {
			return FailureReport(s,
				WithError(StepExecutionError.New("step %q execution returned nil report", s.id)),
				WithActionType(ActionExecute),
				WithStartTime(start))
		}

		var execReport *Report
		if report.IsFailed() {
			if report.Error == nil {
				// this should not happen, but just in case
				report.Error = StepExecutionError.New("step %q failed", s.id)
			}

			execReport = FailureReport(s,
				WithReport(report),
				WithActionType(ActionExecute),
				WithStartTime(start))

			s.handleFailure(ctx, execReport)

			return execReport
		}

		if report.Status == StatusSkipped {
			execReport = SkippedReport(s,
				WithReport(report),
				WithActionType(ActionExecute),
				WithStartTime(start))
		} else {
			execReport = SuccessReport(s,
				WithReport(report),
				WithActionType(ActionExecute),
				WithStartTime(start))
		}

		s.handleCompletion(ctx, execReport)
		return execReport
	}

	return SkippedReport(s, WithActionType(ActionExecute), WithStartTime(start))
}

func (s *defaultStep) Rollback(ctx context.Context) *Report {
	start := time.Now()
	if s.rollback != nil {
		report := s.rollback(ctx, s)
		if report == nil {
			return FailureReport(s,
				WithError(StepExecutionError.New("step %q rollback returned nil report", s.id)),
				WithActionType(ActionRollback),
				WithStartTime(start))
		}

		// we regenerate the completion report here to ensure consistency
		// in case the user-provided rollback function does not
		// follow the expected conventions
		var rollbackReport *Report
		if report.IsFailed() {
			if report.Error == nil {
				// this should not happen, but just in case
				report.Error = StepExecutionError.New("step %q rollback failed", s.id)
			}

			rollbackReport = FailureReport(s,
				WithReport(report), // include user report details
				WithActionType(ActionRollback),
				WithStartTime(start))
		} else if report.Status == StatusSkipped {
			rollbackReport = SkippedReport(s,
				WithReport(report), // include user report details
				WithActionType(ActionRollback),
				WithStartTime(start))
		} else {
			rollbackReport = SuccessReport(s,
				WithReport(report), // include user report details
				WithActionType(ActionRollback),
				WithStartTime(start))
		}

		return rollbackReport
	}

	return SkippedReport(s,
		WithActionType(ActionRollback),
		WithStartTime(start))
}

func (s *defaultStep) handleCompletion(ctx context.Context, report *Report) {
	if s.onCompletion == nil {
		return
	}

	if s.enableAsyncCallbacks {
		clonedReport := report.Clone() // assuming Clone() creates a deep copy
		go s.onCompletion(ctx, s, clonedReport)
	} else {
		s.onCompletion(ctx, s, report)
	}
}

func (s *defaultStep) handleFailure(ctx context.Context, report *Report) {
	if s.onFailure == nil {
		return
	}

	if s.enableAsyncCallbacks {
		clonedReport := report.Clone() // assuming Clone() creates a deep copy
		go s.onFailure(ctx, s, clonedReport)
	} else {
		s.onFailure(ctx, s, report)
	}
}

func newDefaultStep() *defaultStep {
	return &defaultStep{}
}
