package automa

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"testing"
)

func TestWorkflowReport_Append(t *testing.T) {
	workflowReport := NewWorkflowReport("test", nil)
	stepReport := NewStepReport("step-1", RunAction)
	workflowReport.Append(stepReport, RunAction, StatusSuccess)
	assert.Equal(t, 1, len(workflowReport.StepReports))
	assert.Equal(t, StatusSuccess, workflowReport.StepReports[0].Status)

	workflowReport = &WorkflowReport{}
	workflowReport.Append(stepReport, RunAction, StatusSuccess)
	assert.Equal(t, 1, len(workflowReport.StepReports))
}

func TestReportYAML(t *testing.T) {
	stepReport1Run := NewStepReport("step-1", RunAction)
	stepReport1Rollback := NewStepReport("step-1", RollbackAction)
	stepReport2Run := NewStepReport("step-2", RunAction)
	stepReport2Rollback := NewStepReport("step-2", RollbackAction)
	workflowReport := NewWorkflowReport("workflow-id", []string{stepReport1Run.StepID, stepReport2Run.StepID})
	workflowReport.Append(stepReport1Run, RunAction, StatusSuccess)
	workflowReport.Append(stepReport2Run, RunAction, StatusSuccess)
	workflowReport.Append(stepReport2Rollback, RollbackAction, StatusSuccess)
	workflowReport.Append(stepReport1Rollback, RollbackAction, StatusSuccess)

	out, err := yaml.Marshal(workflowReport)
	require.Nil(t, err)
	assert.NotNil(t, out)
}
