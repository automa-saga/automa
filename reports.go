package automa

import (
	"time"
)

// StepActionType defines the action taken by a step
// It is used as key for StepReport.Actions
type StepActionType string

const (
	RunAction      StepActionType = "run"
	RollbackAction StepActionType = "rollback"
)

// WorkflowReport defines a map of StepReport with key as the step ID
type WorkflowReport struct {
	WorkflowID   string        `yaml:"workflow_id" json:"workflowID"`
	StartTime    time.Time     `yaml:"start_time" json:"startTime"`
	EndTime      time.Time     `yaml:"end_time" json:"endTime"`
	Status       Status        `yaml:"status" json:"status"`
	StepSequence StepIDs       `yaml:"step_sequence" json:"stepSequence"`
	StepReports  []*StepReport `yaml:"step_reports" json:"stepReports"`
}

// StepReport defines the report data model for each AtomicStep execution
type StepReport struct {
	StepID        string            `yaml:"step_id" json:"stepID"`
	Action        StepActionType    `yaml:"action" json:"action"`
	StartTime     time.Time         `yaml:"start_time" json:"startTime"`
	EndTime       time.Time         `yaml:"end_time" json:"endTime"`
	Status        Status            `yaml:"status" json:"status"`
	FailureReason error             `yaml:"failure_reason" json:"failure_reason"`
	Metadata      map[string][]byte `yaml:"metadata" json:"metadata"`
}

// Append appends the current report to the previous report
// It adds an end time and sets the status for the current report
func (wfr *WorkflowReport) Append(stepReport *StepReport, action StepActionType, status Status) {
	if stepReport == nil {
		return
	}

	stepReport.Action = action
	stepReport.EndTime = time.Now()
	stepReport.Status = status
	wfr.StepReports = append(wfr.StepReports, stepReport)
}

// NewWorkflowReport returns an instance of WorkflowReport
func NewWorkflowReport(id string, steps StepIDs) *WorkflowReport {
	return &WorkflowReport{
		WorkflowID:   id,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		Status:       StatusUndefined,
		StepSequence: steps,
		StepReports:  []*StepReport{},
	}
}

// NewStepReport returns a new report with a given stepID
func NewStepReport(id string, action StepActionType) *StepReport {
	r := &StepReport{
		StepID:        id,
		StartTime:     time.Now(),
		EndTime:       time.Now(),
		Status:        StatusUndefined,
		FailureReason: nil,
		Metadata:      map[string][]byte{},
	}

	return r
}
