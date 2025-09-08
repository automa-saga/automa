package automa

import (
	"context"
	"fmt"
	"github.com/automa-saga/automa/types"
	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
	"time"
)

type workflow struct {
	id           string
	steps        []Step
	logger       zerolog.Logger
	rollbackMode types.RollbackMode
}

// rollbackFrom rollbacks the workflow backward from the given index to the start.
func (w *workflow) rollbackFrom(ctx context.Context, index int) []*Report {
	var stepReports []*Report
	for i := index; i >= 0; i-- {
		startTime := time.Now()
		step := w.steps[i]
		if report, rollbackErr := step.OnRollback(ctx); rollbackErr != nil {
			failedReport := StepFailureReport(step.Id(), WithStartTime(startTime), WithError(rollbackErr))
			stepReports = append(stepReports, failedReport)

			switch w.rollbackMode {
			case types.RollbackModeContinueOnError:
				continue
			case types.RollbackModeStopOnError:
				break
			}
		} else if report.Status == types.StatusSkipped {
			skippedReport := NewReport(step.Id(), WithStatus(types.StatusSkipped), WithStartTime(startTime))
			stepReports = append(stepReports, skippedReport)
		} else if report.Status == types.StatusSuccess {
			successReport := StepSuccessReport(step.Id(), WithStartTime(startTime))
			stepReports = append(stepReports, successReport)
		}
	}

	return stepReports
}

func (w *workflow) Id() string {
	return w.id
}

func (w *workflow) Prepare(ctx context.Context) (context.Context, error) {
	// Preparation logic for the workflow can be added here
	// For now, we just return the context as is
	return ctx, nil
}

func (w *workflow) Execute(ctx context.Context) (*Report, error) {
	if len(w.steps) == 0 {
		return nil, fmt.Errorf("workflow %s has no steps to execute", w.id)
	}

	var stepReports []*Report
	for index, step := range w.steps {
		startTime := time.Now()
		stepCtx, err := step.Prepare(ctx)
		if err != nil {
			return nil, err
		}

		report, err := step.Execute(stepCtx)
		if err != nil {
			failureReport := StepFailureReport(step.Id(), WithStartTime(startTime), WithError(err))
			stepReports = append(stepReports, failureReport)
			rollbackReports := w.rollbackFrom(ctx, index)
			stepReports = append(stepReports, rollbackReports...)

			return NewReport(w.id, WithStatus(types.StatusFailed), WithReports(stepReports...)), nil
		}

		if report == nil {
			return nil, errorx.IllegalState.New("step %s returned no report", step.Id())
		}

		if report.Status == types.StatusSkipped {
			skippedReport := StepSkippedReport(step.Id(), types.ActionExecute, WithStartTime(startTime))
			stepReports = append(stepReports, skippedReport)
		} else if report.Status == types.StatusSuccess {
			successReport := StepSuccessReport(step.Id(), WithStartTime(startTime))
			stepReports = append(stepReports, successReport)
		}

		step.OnCompletion(stepCtx, report)
	}

	return NewReport(w.id, WithStatus(types.StatusSuccess), WithReports(stepReports...)), nil
}

func (w *workflow) OnCompletion(ctx context.Context, report *Report) {
	// any post successful execution logic can be added here
	// no-op for now
}

func (w *workflow) OnRollback(ctx context.Context) (*Report, error) {
	startTime := time.Now()
	reports := w.rollbackFrom(ctx, len(w.steps)-1)
	return StepSuccessReport(w.id, WithStartTime(startTime), WithReports(reports...)), nil
}

// WorkflowOption defines a function that modifies a workflow.
type WorkflowOption func(*workflow)

// WithLogger returns a WorkflowOption that sets the logger.
func WithLogger(logger zerolog.Logger) WorkflowOption {
	return func(w *workflow) {
		w.logger = logger
	}
}

func WithRollbackMode(mode types.RollbackMode) WorkflowOption {
	return func(w *workflow) {
		w.rollbackMode = mode
	}
}

func NewWorkflow(id string, steps []Step, opts ...WorkflowOption) Step {
	w := &workflow{
		id:           id,
		rollbackMode: types.RollbackModeContinueOnError,
		steps:        steps,
		logger:       zerolog.Nop(),
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}
