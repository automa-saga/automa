package automa

// Success defines a success event for a step
type Success struct {
	workflowReport *WorkflowReport
}

// Failure defines a failure event for a step
type Failure struct {
	error
	workflowReport *WorkflowReport
}

// NewRollbackTrigger returns a Failure event to be used for Rollback method
// It sets the step status as StatusFailed
func NewRollbackTrigger(prevSuccess *Success, err error, report *StepReport) *Failure {
	prevSuccess.workflowReport.Append(report, RunAction, StatusFailed)
	return &Failure{error: err, workflowReport: prevSuccess.workflowReport}
}

// NewStartTrigger returns a Success event to be use for Run method
// It sets the step status as StatusSuccess
func NewStartTrigger(reports *WorkflowReport) *Success {
	return &Success{
		workflowReport: reports,
	}
}

// NewFailure creates a Failure event for rollback
// It sets the step status as StatusFailed
func NewFailure(prevFailure *Failure, report *StepReport) *Failure {
	prevFailure.workflowReport.Append(report, RollbackAction, StatusSuccess)
	return &Failure{error: prevFailure.error, workflowReport: prevFailure.workflowReport}
}

// NewSuccess creates a Success event
// It sets the step status as StatusSuccess
func NewSuccess(prevSuccess *Success, report *StepReport) *Success {
	prevSuccess.workflowReport.Append(report, RunAction, StatusSuccess)
	return &Success{workflowReport: prevSuccess.workflowReport}
}

// NewSkippedRun creates a Success event with StatusSkipped
// It report the step status as StatusSkipped
func NewSkippedRun(prevSuccess *Success, report *StepReport) *Success {
	prevSuccess.workflowReport.Append(report, RunAction, StatusSkipped)
	return &Success{workflowReport: prevSuccess.workflowReport}
}

// NewSkippedRollback creates a Failure event with StatusSkipped
// It sets the step status as StatusSkipped
// This is a helper method to be used in Rollback method where rollback is skipped
func NewSkippedRollback(prevFailure *Failure, report *StepReport) *Failure {
	prevFailure.workflowReport.Append(report, RollbackAction, StatusSkipped)
	return &Failure{error: prevFailure.error, workflowReport: prevFailure.workflowReport}
}
