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
	state                StateBag
}

func (df *defaultStep) State() StateBag {
	if df.state == nil {
		df.state = &SyncStateBag{} // lazy initialization
	}
	return df.state
}

func (df *defaultStep) Id() string {
	return df.id
}

func (df *defaultStep) Prepare(ctx context.Context) (context.Context, error) {
	preparedCtx := ctx
	if df.prepare != nil {
		c, err := df.prepare(preparedCtx, df)
		if err != nil {
			return nil, err
		}
		preparedCtx = c // use the context returned by user prepare function
	}

	return preparedCtx, nil
}

func (df *defaultStep) Execute(ctx context.Context) *Report {
	start := time.Now()
	if df.execute != nil {
		report := df.execute(ctx, df)
		if report == nil {
			return FailureReport(df,
				WithError(StepExecutionError.New("step %q execution returned nil report", df.id)),
				WithActionType(ActionExecute),
				WithStartTime(start))
		}

		var execReport *Report
		if report.IsFailed() {
			if report.Error == nil {
				// this should not happen, but just in case
				report.Error = StepExecutionError.New("step %q failed", df.id)
			}

			execReport = FailureReport(df,
				WithReport(report),
				WithActionType(ActionExecute),
				WithStartTime(start))

			df.handleFailure(ctx, execReport)

			return execReport
		}

		if report.Status == StatusSkipped {
			execReport = SkippedReport(df,
				WithReport(report),
				WithActionType(ActionExecute),
				WithStartTime(start))
		} else {
			execReport = SuccessReport(df,
				WithReport(report),
				WithActionType(ActionExecute),
				WithStartTime(start))
		}

		df.handleCompletion(ctx, execReport)
		return execReport
	}

	return SkippedReport(df, WithActionType(ActionExecute), WithStartTime(start))
}

func (df *defaultStep) Rollback(ctx context.Context) *Report {
	start := time.Now()
	if df.rollback != nil {
		report := df.rollback(ctx, df)
		if report == nil {
			return FailureReport(df,
				WithError(StepExecutionError.New("step %q rollback returned nil report", df.id)),
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
				report.Error = StepExecutionError.New("step %q rollback failed", df.id)
			}

			rollbackReport = FailureReport(df,
				WithReport(report), // include user report details
				WithActionType(ActionRollback),
				WithStartTime(start))
		} else if report.Status == StatusSkipped {
			rollbackReport = SkippedReport(df,
				WithReport(report), // include user report details
				WithActionType(ActionRollback),
				WithStartTime(start))
		} else {
			rollbackReport = SuccessReport(df,
				WithReport(report), // include user report details
				WithActionType(ActionRollback),
				WithStartTime(start))
		}

		return rollbackReport
	}

	return SkippedReport(df,
		WithActionType(ActionRollback),
		WithStartTime(start))
}

func (df *defaultStep) handleCompletion(ctx context.Context, report *Report) {
	if df.onCompletion == nil {
		return
	}

	if df.enableAsyncCallbacks {
		clonedReport := report.Clone() // assuming Clone() creates a deep copy
		go df.onCompletion(ctx, df, clonedReport)
	} else {
		df.onCompletion(ctx, df, report)
	}
}

func (df *defaultStep) handleFailure(ctx context.Context, report *Report) {
	if df.onFailure == nil {
		return
	}

	if df.enableAsyncCallbacks {
		clonedReport := report.Clone() // assuming Clone() creates a deep copy
		go df.onFailure(ctx, df, clonedReport)
	} else {
		df.onFailure(ctx, df, report)
	}
}

func newDefaultStep() *defaultStep {
	return &defaultStep{}
}
