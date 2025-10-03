package automa

import (
	"context"
	"fmt"
	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
	"time"
)

type workflow struct {
	id           string
	steps        []Step
	logger       zerolog.Logger
	rollbackMode TypeRollbackMode
}

// rollbackFrom rollbacks the workflow backward from the given index to the start.
func (w *workflow) rollbackFrom(ctx context.Context, index int) map[string]*Report {
	stepReports := map[string]*Report{}
	for i := index; i >= 0; i-- {
		startTime := time.Now()
		step := w.steps[i]
		if report, rollbackErr := step.Rollback(ctx); rollbackErr != nil {
			failedReport := StepFailureReport(step.Id(), WithActionType(ActionRollback), WithStartTime(startTime), WithError(rollbackErr))
			stepReports[step.Id()] = failedReport

			switch w.rollbackMode {
			case RollbackModeContinueOnError:
				continue
			case RollbackModeStopOnError:
				return stepReports
			}
		} else if report.Status == StatusSkipped {
			skippedReport := StepSkippedReport(step.Id(), WithActionType(ActionRollback), WithReport(report), WithStartTime(startTime))
			stepReports[step.Id()] = skippedReport
		} else if report.Status == StatusSuccess {
			successReport := StepSuccessReport(step.Id(), WithActionType(ActionRollback), WithReport(report), WithStartTime(startTime))
			stepReports[step.Id()] = successReport
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
			// create failure report for the failed step
			failureReport := StepFailureReport(step.Id(), WithActionType(ActionExecute), WithStartTime(startTime), WithError(err))
			stepReports = append(stepReports, failureReport)

			// Perform rollback for all executed steps up to the current one
			rollbackReports := w.rollbackFrom(ctx, index)
			if len(rollbackReports) != len(stepReports) && w.rollbackMode != RollbackModeContinueOnError {
				return nil, errorx.IllegalState.New("mismatched rollback reports and step reports lengths, "+
					"rollback reports: %d, step reports: %d", len(rollbackReports), len(stepReports))
			}

			// Attach rollback reports to corresponding step reports
			for _, stepReport := range stepReports {
				if rollback, ok := rollbackReports[stepReport.Id]; ok {
					stepReport.Rollback = rollback
				}
			}

			// Return the workflow report with failure status and step reports
			return NewReport(w.id, WithActionType(ActionExecute), WithStatus(StatusFailed), WithStepReports(stepReports...)), nil
		}

		if report == nil {
			return nil, errorx.IllegalState.New("step %s returned no report", step.Id())
		}

		if report.Status == StatusSkipped {
			skippedReport := StepSkippedReport(step.Id(), WithActionType(ActionExecute), WithStartTime(startTime))
			stepReports = append(stepReports, skippedReport)
		} else if report.Status == StatusSuccess {
			successReport := StepSuccessReport(step.Id(), WithActionType(ActionExecute), WithStartTime(startTime))
			stepReports = append(stepReports, successReport)
		}

		if _, ok := step.(Workflow); ok {
		}

		step.OnCompletion(stepCtx, report)
	}

	return NewReport(w.id, WithActionType(ActionExecute), WithStatus(StatusSuccess), WithStepReports(stepReports...)), nil
}

func (w *workflow) OnCompletion(ctx context.Context, report *Report) {
	// any post successful execution logic can be added here
	// no-op for now
}

func (w *workflow) Rollback(ctx context.Context) (*Report, error) {
	startTime := time.Now()
	rollbackReports := w.rollbackFrom(ctx, len(w.steps)-1)
	var stepReports []*Report
	for _, step := range w.steps {
		if report, ok := rollbackReports[step.Id()]; ok {
			stepReports = append(stepReports, report)
		}
	}
	return StepSuccessReport(w.id, WithActionType(ActionRollback), WithStartTime(startTime), WithStepReports(stepReports...)), nil
}

// WorkflowOption defines a function that modifies a workflow.
type WorkflowOption func(*workflow)

// WithWorkflowLogger returns a WorkflowOption that sets the logger.
func WithWorkflowLogger(logger zerolog.Logger) WorkflowOption {
	return func(w *workflow) {
		w.logger = logger
	}
}

func WithRollbackMode(mode TypeRollbackMode) WorkflowOption {
	return func(w *workflow) {
		w.rollbackMode = mode
	}
}

func NewWorkflow(id string, steps []Step, opts ...WorkflowOption) Step {
	w := &workflow{
		id:           id,
		rollbackMode: RollbackModeContinueOnError,
		steps:        steps,
		logger:       zerolog.Nop(),
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}
