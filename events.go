package automa

type Success struct {
	reports Reports
}

type Failure struct {
	error
	reports Reports
}

// NewRollbackTrigger returns a Failure event to be used for Rollback method
func NewRollbackTrigger(prevSuccess *Success, err error, report *Report) *Failure {
	var reports Reports
	if report != nil {
		reports = report.End(prevSuccess.reports, StatusSuccess)
	}
	return &Failure{error: err, reports: reports}
}

// NewStartTrigger returns a Success event to be use for Run method
func NewStartTrigger(reports Reports) *Success {
	return &Success{
		reports: reports,
	}
}

func NewFailure(prevFailure *Failure, report *Report) *Failure {
	var reports Reports
	if report != nil {
		reports = report.End(prevFailure.reports, StatusSuccess)
	}

	return &Failure{error: prevFailure.error, reports: reports}
}

func NewSuccess(prevSuccess *Success, report *Report) *Success {
	var reports Reports
	if report != nil {
		reports = report.End(prevSuccess.reports, StatusSuccess)
	}
	return &Success{reports: reports}
}

func NewSkipped(prevSuccess *Success, report *Report) *Success {
	reports := Reports{}
	if report != nil {
		reports = report.End(prevSuccess.reports, StatusFailed)
	}

	return &Success{reports: reports}
}

func NewSkippedFailure(prevFailure *Failure, report *Report) *Failure {
	var reports Reports
	if report != nil {
		reports = report.End(prevFailure.reports, StatusSkipped)
	}

	return &Failure{error: prevFailure.error, reports: reports}
}
