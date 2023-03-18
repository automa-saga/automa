package automa

import (
	"github.com/cockroachdb/errors"
	"time"
)

// Reports defines a map of Report with key as the step ID
type Reports map[string]*Report

// Report defines the report data model for each AtomicStep execution
type Report struct {
	StepID    string
	StartTime time.Time
	EndTime   time.Time
	Status    Status
	Error     errors.EncodedError
	metadata  map[string][]byte
}

// Append appends the current report to the previous reports
// It adds an end time and sets the status for the current report
func (r *Report) Append(prevReports Reports, status Status) Reports {
	r.EndTime = time.Now()
	r.Status = status

	clone := Reports{}
	for key, val := range prevReports {
		clone[key] = val
	}

	clone[r.StepID] = r

	return clone
}

// NewReport returns a new report with a given stepID
func NewReport(stepID string) *Report {
	return &Report{
		StepID:    stepID,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Status:    StatusFailed,
		Error:     errors.EncodedError{},
		metadata:  map[string][]byte{},
	}
}
