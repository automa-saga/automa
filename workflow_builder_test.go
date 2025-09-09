package automa

import (
	"errors"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// Mock Builder
type mockStepBuilder struct {
	id          string
	buildErr    error
	validateErr error
	step        Step
}

func (m *mockStepBuilder) Id() string           { return m.id }
func (m *mockStepBuilder) Build() (Step, error) { return m.step, m.buildErr }
func (m *mockStepBuilder) Validate() error      { return m.validateErr }

func TestNewWorkFlowBuilder_PanicsOnEmptyId(t *testing.T) {
	assert.Panics(t, func() { NewWorkFlowBuilder("") })
}

func TestWorkFlowBuilder_Steps_AddsStepsAndSkipsDuplicates(t *testing.T) {
	wb := NewWorkFlowBuilder("wf").(*workflowBuilder)
	wb.logger = zerolog.Nop()
	b1 := &mockStepBuilder{id: "a"}
	b2 := &mockStepBuilder{id: "a"} // duplicate id
	wb.Steps(b1, b2)
	assert.Len(t, wb.stepBuilders, 1)
}

func TestWorkFlowBuilder_NamedSteps_AddsFromRegistryAndSkipsUnknownOrDuplicates(t *testing.T) {
	b1 := &mockStepBuilder{id: "a"}
	b2 := &mockStepBuilder{id: "b"}
	reg := NewRegistry()
	err := reg.Add(b1, b2)
	require.NoError(t, err)

	wb := NewWorkFlowBuilder("wf").(*workflowBuilder)
	wb.logger = zerolog.Nop()
	wb.WithRegistry(reg)
	wb.NamedSteps("a", "b", "c") // "c" not in registry
	assert.Len(t, wb.stepBuilders, 2)
	wb.NamedSteps("a") // duplicate
	assert.Len(t, wb.stepBuilders, 2)
}

func TestWorkFlowBuilder_Validate_NoSteps(t *testing.T) {
	wb := NewWorkFlowBuilder("wf").(*workflowBuilder)
	err := wb.Validate()
	assert.Error(t, err)
}

func TestWorkFlowBuilder_Validate_StepValidationError(t *testing.T) {
	b1 := &mockStepBuilder{id: "a", validateErr: errors.New("fail")}
	wb := NewWorkFlowBuilder("wf").(*workflowBuilder)
	wb.Steps(b1)
	err := wb.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation errors")
}

func TestWorkFlowBuilder_Validate_Success(t *testing.T) {
	b1 := &mockStepBuilder{id: "a"}
	wb := NewWorkFlowBuilder("wf").(*workflowBuilder)
	wb.Steps(b1)
	assert.NoError(t, wb.Validate())
}

func TestWorkFlowBuilder_Build_StepBuildError(t *testing.T) {
	b1 := &mockStepBuilder{id: "a", buildErr: errors.New("fail")}
	wb := NewWorkFlowBuilder("wf").(*workflowBuilder)
	wb.Steps(b1)
	_, err := wb.Build()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to build step")
}

func TestWorkFlowBuilder_Build_Success(t *testing.T) {
	b1 := &mockStepBuilder{id: "a", step: nil}
	wb := NewWorkFlowBuilder("wf").(*workflowBuilder)
	wb.Steps(b1)
	_, err := wb.Build()
	assert.NoError(t, err)
}

func TestWorkFlowBuilder_WithLogger_WithRollbackMode_WithRegistry(t *testing.T) {
	wb := NewWorkFlowBuilder("wf").(*workflowBuilder)
	logger := zerolog.Nop()
	reg := NewRegistry()
	wb.WithLogger(logger)
	wb.WithRollbackMode(RollbackModeStopOnError)
	wb.WithRegistry(reg)
	assert.Equal(t, logger, wb.logger)
	assert.Equal(t, RollbackModeStopOnError, wb.rollbackMode)
	assert.Equal(t, reg, wb.registry)
}
