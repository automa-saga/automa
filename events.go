package automa

// Success defines a success event for a step
type Success struct {
	reports Reports
}

// Failure defines a failure event for a step
type Failure struct {
	error
	reports Reports
}

// NewRollbackTrigger returns a Failure event to be used for Rollback method
// It reports the step status as StatusFailed
func NewRollbackTrigger(prevSuccess *Success, err error, report *Report) *Failure {
	var reports Reports
	if report != nil {
		reports = report.Append(prevSuccess.reports, StatusFailed)
	}
	return &Failure{error: err, reports: reports}
}

// NewStartTrigger returns a Success event to be use for Run method
// It reports the step status as StatusSuccess
func NewStartTrigger(reports Reports) *Success {
	return &Success{
		reports: reports,
	}
}

// NewFailure creates a Failure event for rollback
// It reports the step status as StatusFailed
func NewFailure(prevFailure *Failure, report *Report) *Failure {
	var reports Reports
	if report != nil {
		reports = report.Append(prevFailure.reports, StatusFailed)
	}

	return &Failure{error: prevFailure.error, reports: reports}
}

// NewSuccess creates a Success event
// It reports the step status as StatusSuccess
func NewSuccess(prevSuccess *Success, report *Report) *Success {
	var reports Reports
	if report != nil {
		reports = report.Append(prevSuccess.reports, StatusSuccess)
	}
	return &Success{reports: reports}
}

// NewSkipped creates a Success event
// It reports the step status as StatusSkipped
func NewSkipped(prevSuccess *Success, report *Report) *Success {
	reports := Reports{}
	if report != nil {
		reports = report.Append(prevSuccess.reports, StatusSkipped)
	}

	return &Success{reports: reports}
}

// NewSkippedFailure creates a Failure event
// It reports the step status as StatusSkipped
func NewSkippedFailure(prevFailure *Failure, report *Report) *Failure {
	var reports Reports
	if report != nil {
		reports = report.Append(prevFailure.reports, StatusSkipped)
	}

	return &Failure{error: prevFailure.error, reports: reports}
}
