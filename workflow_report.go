package automa

import "time"

type workflowReport struct {
	stepReport
	stepReports []Report
}

func (wr *workflowReport) addStepReport(report Report) {
	if report == nil {
		return // Skip nil reports
	}

	wr.stepReports = append(wr.stepReports, report)

	if report.StartTime().Before(wr.startTime) {
		wr.startTime = report.StartTime() // Update start time if this step starts earlier than the current start time
	}

	if report.EndTime().After(wr.endTime) {
		wr.endTime = report.EndTime() // Update end time to the latest step's end time
	}
}

// NewWorkflowReport creates a new workflow report with the given ID, status, message, and step reports.
// It initializes the start and end times based on the provided step reports, if any, assuming they are ordered by start time.
func NewWorkflowReport(id string, status Status, reports []Report, message string) Report {
	wr := &workflowReport{
		stepReport: stepReport{
			id:        id,
			startTime: time.Now(), // Default start time is now, will be updated if steps are provided
			endTime:   time.Now(), // Default end time is now, will be updated if steps are provided
			status:    status,
			message:   message,
		},
		stepReports: make([]Report, len(reports)),
	}

	if reports != nil {
		wr.startTime = reports[0].StartTime()
		wr.endTime = reports[len(reports)-1].EndTime()

		// assume steps are ordered by start time
		for _, r := range reports {
			wr.stepReports = append(wr.stepReports, r)
		}
	}

	return wr
}
