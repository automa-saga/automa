package automa

import (
	"time"
)

// Result represents the outcome of a workflow execution.
type Result struct {
	Report *WorkflowReport
}

// WorkflowReport aggregates execution details for an entire workflow.
// It tracks the workflow ID, timing, status, step sequence, and individual step reports.
type WorkflowReport struct {
	WorkflowID   string        `yaml:"workflow_id" json:"workflowID"`     // Unique identifier for the workflow.
	StartTime    time.Time     `yaml:"start_time" json:"startTime"`       // Timestamp when the workflow started.
	EndTime      time.Time     `yaml:"end_time" json:"endTime"`           // Timestamp when the workflow ended.
	Status       string        `yaml:"status" json:"status"`              // Latest status of the workflow.
	StepSequence []string      `yaml:"step_sequence" json:"stepSequence"` // Ordered list of step IDs in the workflow.
	StepReports  []*StepReport `yaml:"step_reports" json:"stepReports"`   // Reports for each executed step.

	FirstFailureOnForward *StepFailure `yaml:"first_execution_failure" json:"firstExecutionFailure"` // First failure encountered during forward execution.
	LastFailureOnReverse  *StepFailure `yaml:"last_rollback_failure" json:"lastRollbackFailure"`     // Last failure encountered during rollback execution.
}

type StepFailure struct {
	StepID string `yaml:"step_id" json:"stepID"` // ID of the step that failed during forward execution.
	Action string `yaml:"action" json:"action"`  // Action that failed (e.g., run).
	Error  error  `yaml:"error" json:"error"`
}

// StepReport captures execution details for a single workflow step.
// It includes timing, status, error information, and custom metadata.
type StepReport struct {
	StepID    string            `yaml:"step_id" json:"stepID"`                // Unique identifier for the step.
	Action    string            `yaml:"action" json:"action"`                 // Action performed (e.g., run, rollback).
	StartTime time.Time         `yaml:"start_time" json:"startTime"`          // Timestamp when the step started.
	EndTime   time.Time         `yaml:"end_time" json:"endTime"`              // Timestamp when the step ended.
	Status    string            `yaml:"status" json:"status"`                 // Status of the step execution.
	Error     error             `yaml:"failure_reason" json:"failure_reason"` // Error encountered during execution, if any.
	Metadata  map[string][]byte `yaml:"metadata" json:"metadata"`             // Arbitrary metadata for the step.
}

// Append adds a StepReport to the WorkflowReport.
// It sets the action, end time, and status for the step before appending.
// If stepReport is nil, the method does nothing.
func (wfr *WorkflowReport) Append(stepReport *StepReport, status string) {
	if stepReport == nil {
		return
	}

	stepReport.EndTime = time.Now()
	stepReport.Status = status
	wfr.StepReports = append(wfr.StepReports, stepReport)
}

// NewWorkflowReport creates and initializes a new WorkflowReport.
// The workflow starts with StatusInitialized and empty step reports.
func NewWorkflowReport(id string, stepIDs []string) *WorkflowReport {
	if stepIDs == nil {
		stepIDs = []string{}
	}

	return &WorkflowReport{
		WorkflowID:   id,
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		Status:       StatusInitialized,
		StepSequence: stepIDs,
		StepReports:  []*StepReport{},
	}
}

// NewStepReport creates and initializes a new StepReport for a given step ID and action.
// The report starts with StatusInitialized and empty metadata.
func NewStepReport(id string, action string) *StepReport {
	return &StepReport{
		StepID:    id,
		Action:    action,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Status:    StatusInitialized,
		Error:     nil,
		Metadata:  map[string][]byte{},
	}
}
