package automa

import (
	"context"
	"github.com/cockroachdb/errors"
)

// Success defines a success event for a step
type Success struct {
	workflowReport *WorkflowReport
}

// Failure defines a failure event for a step
type Failure struct {
	error
	workflowReport *WorkflowReport
}

// NewFailedRun returns a Failure event to be used for first Rollback method
// It is used by a step to trigger its own rollback action
// It sets the step's RunAction status as StatusFailed
func NewFailedRun(ctx context.Context, prevSuccess *Success, err error, report *StepReport) *Failure {
	report.Actions[RunAction].Error = errors.EncodeError(ctx, err)
	prevSuccess.workflowReport.Append(report, RunAction, StatusFailed)
	return &Failure{error: err, workflowReport: prevSuccess.workflowReport}
}

// NewFailedRollback returns a Failure event when steps rollback action failed
// It sets the step's RollbackAction status as StatusFailed
func NewFailedRollback(ctx context.Context, prevFailure *Failure, err error, report *StepReport) *Failure {
	report.Actions[RollbackAction].Error = errors.EncodeError(ctx, err)
	prevFailure.workflowReport.Append(report, RollbackAction, StatusFailed)
	return &Failure{error: err, workflowReport: prevFailure.workflowReport}
}

// NewStartTrigger returns a Success event to be use for Run method
// It is used by the Workflow to trigger the execution of the first step
func NewStartTrigger(reports *WorkflowReport) *Success {
	return &Success{
		workflowReport: reports,
	}
}

// NewFailure creates a Failure event for rollback action
// It is used by a step to trigger rollback action of the previous step when its own run succeeds.
// It sets the step's RollbackAction status as StatusSuccess.
func NewFailure(prevFailure *Failure, report *StepReport) *Failure {
	prevFailure.workflowReport.Append(report, RollbackAction, StatusSuccess)
	return &Failure{error: prevFailure.error, workflowReport: prevFailure.workflowReport}
}

// NewSuccess creates a Success event for run action
// It is used by a step to trigger run action of the nex step when its own run succeeds.
// It sets the step's RunAction status as StatusSuccess.
func NewSuccess(prevSuccess *Success, report *StepReport) *Success {
	prevSuccess.workflowReport.Append(report, RunAction, StatusSuccess)
	return &Success{workflowReport: prevSuccess.workflowReport}
}

// NewSkippedRun creates a Success event with StatusSkipped for RunAction
// This is a helper method to be used in run action where the action is skipped.
func NewSkippedRun(prevSuccess *Success, report *StepReport) *Success {
	prevSuccess.workflowReport.Append(report, RunAction, StatusSkipped)
	return &Success{workflowReport: prevSuccess.workflowReport}
}

// NewSkippedRollback creates a Failure event with StatusSkipped for RollbackAction
// This is a helper method to be used in rollback action where the action is skipped.
func NewSkippedRollback(prevFailure *Failure, report *StepReport) *Failure {
	prevFailure.workflowReport.Append(report, RollbackAction, StatusSkipped)
	return &Failure{error: prevFailure.error, workflowReport: prevFailure.workflowReport}
}
