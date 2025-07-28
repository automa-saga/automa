package automa

import (
	"time"
)

// WorkflowReport defines a map of StepReport with key as the step ID
type WorkflowReport struct {
	WorkflowID   string        `yaml:"workflow_id" json:"workflowID"`
	StartTime    time.Time     `yaml:"start_time" json:"startTime"`
	EndTime      time.Time     `yaml:"end_time" json:"endTime"`
	Status       string        `yaml:"status" json:"status"`
	StepSequence []string      `yaml:"step_sequence" json:"stepSequence"`
	StepReports  []*StepReport `yaml:"step_reports" json:"stepReports"`
}

// StepReport defines the report data model for each Step execution
type StepReport struct {
	StepID        string            `yaml:"step_id" json:"stepID"`
	Action        string            `yaml:"action" json:"action"`
	StartTime     time.Time         `yaml:"start_time" json:"startTime"`
	EndTime       time.Time         `yaml:"end_time" json:"endTime"`
	Status        string            `yaml:"status" json:"status"`
	FailureReason error             `yaml:"failure_reason" json:"failure_reason"`
	Metadata      map[string][]byte `yaml:"metadata" json:"metadata"`
}

// Append appends the current report to the previous report
// It adds an end time and sets the status for the current report
func (wfr *WorkflowReport) Append(stepReport *StepReport, action string, status string) {
	if stepReport == nil {
		return
	}

	stepReport.Action = action
	stepReport.EndTime = time.Now()
	stepReport.Status = status
	wfr.StepReports = append(wfr.StepReports, stepReport)
}

// NewWorkflowReport returns an instance of WorkflowReport
func NewWorkflowReport(id string, stepIDs []string) *WorkflowReport {
	return &WorkflowReport{
		WorkflowID:   id,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		Status:       StatusUndefined,
		StepSequence: stepIDs,
		StepReports:  []*StepReport{},
	}
}

// NewStepReport returns a new report with a given stepID
func NewStepReport(id string, action string) *StepReport {
	r := &StepReport{
		StepID:        id,
		Action:        action,
		StartTime:     time.Now(),
		EndTime:       time.Now(),
		Status:        StatusUndefined,
		FailureReason: nil,
		Metadata:      map[string][]byte{},
	}

	return r
}
