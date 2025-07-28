package automa

import "github.com/joomcode/errorx"

// SuccessEvent represents a successful event for a workflow step.
// It holds the current WorkflowReport reflecting the state after the step's execution.
type SuccessEvent struct {
	WorkflowReport WorkflowReport `yaml:"WorkflowReport" json:"WorkflowReport"`
}

// FailureEvent represents a failed event for a workflow step.
// It contains the error and the current WorkflowReport reflecting the state after the failure.
type FailureEvent struct {
	Err            error          `yaml:"Error" json:"Error"`
	WorkflowReport WorkflowReport `yaml:"WorkflowReport" json:"WorkflowReport"`
}

// NewSuccessEvent creates a SuccessEvent event for a successful run action.
// Used to trigger the next step's run action when the current run succeeds.
// Sets the step's RunAction status to StatusSuccess.
func NewSuccessEvent(prevSuccess *SuccessEvent, report *StepReport) *SuccessEvent {
	prevSuccess.WorkflowReport.Append(report, RunAction, StatusSuccess)
	return &SuccessEvent{WorkflowReport: prevSuccess.WorkflowReport}
}

// NewFailureEvent creates a FailureEvent event for a successful rollback action.
// Used to trigger rollback of the previous step when the current rollback succeeds.
// Sets the step's RollbackAction status to StatusSuccess.
func NewFailureEvent(prevFailure *FailureEvent, report *StepReport) *FailureEvent {
	prevFailure.WorkflowReport.Append(report, RollbackAction, StatusSuccess)
	return &FailureEvent{Err: prevFailure.Err, WorkflowReport: prevFailure.WorkflowReport}
}

// NewStartTrigger creates a SuccessEvent event for the initial step execution.
// Used to initialize the workflow with the starting WorkflowReport.
func NewStartTrigger(reports *WorkflowReport) *SuccessEvent {
	return &SuccessEvent{
		WorkflowReport: *reports,
	}
}

// NewFailedRunEvent creates a FailureEvent event for a step's failed run action.
// Used to trigger rollback when a step's run fails.
// Sets the step's RunAction status to StatusFailed.
func NewFailedRunEvent(prevSuccess *SuccessEvent, err error, report *StepReport) *FailureEvent {
	report.Action = RunAction
	report.FailureReason = errorx.EnsureStackTrace(err)
	prevSuccess.WorkflowReport.Append(report, RunAction, StatusFailed)
	return &FailureEvent{Err: err, WorkflowReport: prevSuccess.WorkflowReport}
}

// NewFailedRollbackEvent creates a FailureEvent event when a step's rollback action fails.
// Sets the step's RollbackAction status to StatusFailed.
func NewFailedRollbackEvent(prevFailure *FailureEvent, err error, report *StepReport) *FailureEvent {
	report.Action = RollbackAction
	report.FailureReason = errorx.EnsureStackTrace(err)
	prevFailure.WorkflowReport.Append(report, RollbackAction, StatusFailed)
	return &FailureEvent{Err: err, WorkflowReport: prevFailure.WorkflowReport}
}

// NewSkippedRunEvent creates a SuccessEvent event with StatusSkipped for a run action.
// Used when a step's run action is intentionally skipped.
func NewSkippedRunEvent(prevSuccess *SuccessEvent, report *StepReport) *SuccessEvent {
	prevSuccess.WorkflowReport.Append(report, RunAction, StatusSkipped)
	return &SuccessEvent{WorkflowReport: prevSuccess.WorkflowReport}
}

// NewSkippedRollbackEvent creates a FailureEvent event with StatusSkipped for a rollback action.
// Used when a step's rollback action is intentionally skipped.
func NewSkippedRollbackEvent(prevFailure *FailureEvent, report *StepReport) *FailureEvent {
	prevFailure.WorkflowReport.Append(report, RollbackAction, StatusSkipped)
	return &FailureEvent{Err: prevFailure.Err, WorkflowReport: prevFailure.WorkflowReport}
}
