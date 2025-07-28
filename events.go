package automa

import "github.com/joomcode/errorx"

// Success represents a successful event for a workflow step.
// It holds the current WorkflowReport reflecting the state after the step's execution.
type Success struct {
	workflowReport WorkflowReport
}

// Failure represents a failed event for a workflow step.
// It contains the error and the current WorkflowReport reflecting the state after the failure.
type Failure struct {
	err            error
	workflowReport WorkflowReport
}

// NewFailedRun creates a Failure event for a step's failed run action.
// Used to trigger rollback when a step's run fails.
// Sets the step's RunAction status to StatusFailed.
func NewFailedRun(prevSuccess *Success, err error, report *StepReport) *Failure {
	report.Action = RunAction
	report.FailureReason = errorx.EnsureStackTrace(err)
	prevSuccess.workflowReport.Append(report, RunAction, StatusFailed)
	return &Failure{err: err, workflowReport: prevSuccess.workflowReport}
}

// NewFailedRollback creates a Failure event when a step's rollback action fails.
// Sets the step's RollbackAction status to StatusFailed.
func NewFailedRollback(prevFailure *Failure, err error, report *StepReport) *Failure {
	report.Action = RollbackAction
	report.FailureReason = errorx.EnsureStackTrace(err)
	prevFailure.workflowReport.Append(report, RollbackAction, StatusFailed)
	return &Failure{err: err, workflowReport: prevFailure.workflowReport}
}

// NewStartTrigger creates a Success event for the initial step execution.
// Used to initialize the workflow with the starting WorkflowReport.
func NewStartTrigger(reports *WorkflowReport) *Success {
	return &Success{
		workflowReport: *reports,
	}
}

// NewFailure creates a Failure event for a successful rollback action.
// Used to trigger rollback of the previous step when the current rollback succeeds.
// Sets the step's RollbackAction status to StatusSuccess.
func NewFailure(prevFailure *Failure, report *StepReport) *Failure {
	prevFailure.workflowReport.Append(report, RollbackAction, StatusSuccess)
	return &Failure{err: prevFailure.err, workflowReport: prevFailure.workflowReport}
}

// NewSuccess creates a Success event for a successful run action.
// Used to trigger the next step's run action when the current run succeeds.
// Sets the step's RunAction status to StatusSuccess.
func NewSuccess(prevSuccess *Success, report *StepReport) *Success {
	prevSuccess.workflowReport.Append(report, RunAction, StatusSuccess)
	return &Success{workflowReport: prevSuccess.workflowReport}
}

// NewSkippedRun creates a Success event with StatusSkipped for a run action.
// Used when a step's run action is intentionally skipped.
func NewSkippedRun(prevSuccess *Success, report *StepReport) *Success {
	prevSuccess.workflowReport.Append(report, RunAction, StatusSkipped)
	return &Success{workflowReport: prevSuccess.workflowReport}
}

// NewSkippedRollback creates a Failure event with StatusSkipped for a rollback action.
// Used when a step's rollback action is intentionally skipped.
func NewSkippedRollback(prevFailure *Failure, report *StepReport) *Failure {
	prevFailure.workflowReport.Append(report, RollbackAction, StatusSkipped)
	return &Failure{err: prevFailure.err, workflowReport: prevFailure.workflowReport}
}
