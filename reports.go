package automa

import (
	"github.com/cockroachdb/errors"
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
	WorkflowID   string                 `yaml:"workflow_id" json:"workflowID"`
	StartTime    time.Time              `yaml:"start_time" json:"startTime"`
	EndTime      time.Time              `yaml:"end_time" json:"endTime"`
	Status       Status                 `yaml:"status" json:"status"`
	StepSequence []string               `yaml:"step_sequence" json:"stepSequence"`
	StepReports  map[string]*StepReport `yaml:"step_reports" json:"stepReports"`
}

// StepReport defines the report data model for each AtomicStep execution
type StepReport struct {
	StepID  string                           `yaml:"step_id" json:"stepID"`
	Actions map[StepActionType]*ActionReport `yaml:"actions" json:"actions"`
}

// ActionReport defines the report data model for each AtomicStep's Run and Rollback execution
type ActionReport struct {
	StartTime time.Time           `yaml:"start_time" json:"startTime"`
	EndTime   time.Time           `yaml:"end_time" json:"endTime"`
	Status    Status              `yaml:"status" json:"status"`
	Error     errors.EncodedError `yaml:"error" json:"error"`
	Metadata  map[string][]byte   `yaml:"metadata" json:"metadata"`
}

// Append appends the current report to the previous report
// It adds an end time and sets the status for the current report
func (wfr *WorkflowReport) Append(stepReport *StepReport, action StepActionType, status Status) {
	if stepReport == nil {
		return
	}

	if _, ok := stepReport.Actions[action]; !ok {
		stepReport.Actions[action] = NewActionReport()
	} else {
		stepReport.Actions[action].EndTime = time.Now()
		stepReport.Actions[action].Status = status

	}

	if wfr.StepReports == nil {
		wfr.StepReports = map[string]*StepReport{}
	}

	if _, ok := wfr.StepReports[stepReport.StepID]; !ok {
		wfr.StepReports[stepReport.StepID] = stepReport
	} else {
		wfr.StepReports[stepReport.StepID].Actions[action] = stepReport.Actions[action]
	}
}

// NewWorkflowReport returns an instance of WorkflowReport
func NewWorkflowReport(id string, steps []string) *WorkflowReport {
	return &WorkflowReport{
		WorkflowID:   id,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		Status:       StatusUndefined,
		StepSequence: steps,
		StepReports:  map[string]*StepReport{},
	}
}

// NewStepReport returns a new report with a given stepID
func NewStepReport(id string, action StepActionType) *StepReport {
	r := &StepReport{
		StepID:  id,
		Actions: map[StepActionType]*ActionReport{},
	}

	r.Actions[action] = NewActionReport()

	return r
}

// NewActionReport returns an instance of ActionReport
func NewActionReport() *ActionReport {
	return &ActionReport{
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Status:    StatusUndefined,
		Error:     errors.EncodedError{},
		Metadata:  map[string][]byte{},
	}
}
