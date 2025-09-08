package automa

import (
	"errors"
	"github.com/automa-saga/automa/types"
	"testing"
	"time"
)

func TestReport_WorkflowWithRollback(t *testing.T) {
	// Simulate step reports
	step1 := StepSuccessReport("step1")
	step2 := StepFailureReport("step2", WithError(errors.New("step2 failed")))

	// Simulate rollback for step2
	rollbackStep1 := &RollbackReport{
		Id:        "step1_rollback",
		Type:      types.StepReport,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Status:    types.StatusSuccess,
	}

	// Simulate rollback for step2
	rollbackStep2 := &RollbackReport{
		Id:        "step2_rollback",
		Type:      types.StepReport,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Status:    types.StatusSuccess,
	}

	// Simulate workflow rollback report
	workflowRollback := &RollbackReport{
		Id:                  "workflow_rollback",
		Type:                types.WorkflowReport,
		StartTime:           time.Now(),
		EndTime:             time.Now(),
		Status:              types.StatusSuccess,
		StepRollbackReports: []*RollbackReport{rollbackStep2, rollbackStep1}, // rollback in reverse order
	}

	// Create workflow report with steps and rollback
	workflowReport := NewReport(
		"workflow1",
		WithReportType(types.WorkflowReport),
		WithReports(step1, step2),
		WithRollbackReport(workflowRollback),
	)

	// Assertions
	if workflowReport.Type != types.WorkflowReport {
		t.Errorf("expected TypeWorkflowReport, got %v", workflowReport.Type)
	}
	if len(workflowReport.StepReports) != 2 {
		t.Errorf("expected 2 step reports, got %d", len(workflowReport.StepReports))
	}
	if workflowReport.StepReports[1].Status != types.StatusFailed {
		t.Errorf("expected step2 to fail, got %v", workflowReport.StepReports[1].Status)
	}
	if workflowReport.Rollback == nil {
		t.Fatal("expected rollback report to be present")
	}
	if len(workflowReport.Rollback.StepRollbackReports) != 2 {
		t.Errorf("expected 2 step rollback, got %d", len(workflowReport.Rollback.StepRollbackReports))
	}
	if workflowReport.Rollback.StepRollbackReports[0].Status != types.StatusSuccess {
		t.Errorf("expected rollback step to succeed, got %v", workflowReport.Rollback.StepRollbackReports[0].Status)
	}
}
