package automa

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestStepFailure_MarshalUnmarshalJSON(t *testing.T) {
	orig := &StepFailure{StepID: "s1", Action: "run", Error: &marshalError{"fail"}}
	data, err := json.Marshal(orig)
	assert.NoError(t, err)

	var out StepFailure
	assert.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, orig.StepID, out.StepID)
	assert.Equal(t, orig.Action, out.Action)
	assert.Equal(t, orig.Error.Error(), out.Error.Error())
}

func TestStepFailure_MarshalUnmarshalYAML(t *testing.T) {
	orig := &StepFailure{StepID: "s2", Action: "rollback", Error: &marshalError{"fail2"}}
	data, err := yaml.Marshal(orig)
	assert.NoError(t, err)

	var out StepFailure
	assert.NoError(t, yaml.Unmarshal(data, &out))
	assert.Equal(t, orig.StepID, out.StepID)
	assert.Equal(t, orig.Action, out.Action)
	assert.Equal(t, orig.Error.Error(), out.Error.Error())
}

func TestStepReport_MarshalUnmarshalJSON(t *testing.T) {
	now := time.Now()
	orig := &StepReport{
		StepID:    "step1",
		Action:    "run",
		StartTime: now,
		EndTime:   now.Add(time.Second),
		Status:    "success",
		Error:     &marshalError{"err"},
		Metadata:  map[string][]byte{"foo": []byte("bar")},
	}
	data, err := json.Marshal(orig)
	assert.NoError(t, err)

	var out StepReport
	assert.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, orig.StepID, out.StepID)
	assert.Equal(t, orig.Action, out.Action)
	assert.Equal(t, orig.Status, out.Status)
	assert.Equal(t, orig.Error.Error(), out.Error.Error())
	assert.Equal(t, orig.Metadata["foo"], out.Metadata["foo"])
}

func TestStepReport_MarshalUnmarshalYAML(t *testing.T) {
	now := time.Now()
	orig := &StepReport{
		StepID:    "step2",
		Action:    "rollback",
		StartTime: now,
		EndTime:   now.Add(time.Second),
		Status:    "failed",
		Error:     &marshalError{"err2"},
		Metadata:  map[string][]byte{"baz": []byte("qux")},
	}
	data, err := yaml.Marshal(orig)
	assert.NoError(t, err)

	var out StepReport
	assert.NoError(t, yaml.Unmarshal(data, &out))
	assert.Equal(t, orig.StepID, out.StepID)
	assert.Equal(t, orig.Action, out.Action)
	assert.Equal(t, orig.Status, out.Status)
	assert.Equal(t, orig.Error.Error(), out.Error.Error())
	assert.Equal(t, orig.Metadata["baz"], out.Metadata["baz"])
}

func TestWorkflowReport_MarshalUnmarshalJSON(t *testing.T) {
	now := time.Now()
	wfr := &WorkflowReport{
		WorkflowID:   "wf1",
		StartTime:    now,
		EndTime:      now.Add(time.Minute),
		Status:       "success",
		StepSequence: []string{"a", "b"},
		StepReports: []*StepReport{
			{StepID: "a", Action: "run", StartTime: now, EndTime: now, Status: "ok"},
			{StepID: "b", Action: "run", StartTime: now, EndTime: now, Status: "ok"},
		},
		FirstFailureOnForward: &StepFailure{StepID: "a", Action: "run", Error: &marshalError{"fail"}},
		LastFailureOnReverse:  &StepFailure{StepID: "b", Action: "rollback", Error: &marshalError{"fail2"}},
	}
	data, err := json.Marshal(wfr)
	assert.NoError(t, err)

	var out WorkflowReport
	assert.NoError(t, json.Unmarshal(data, &out))
	assert.Equal(t, wfr.WorkflowID, out.WorkflowID)
	assert.Equal(t, wfr.Status, out.Status)
	assert.Equal(t, len(wfr.StepReports), len(out.StepReports))
	assert.Equal(t, wfr.FirstFailureOnForward.Error.Error(), out.FirstFailureOnForward.Error.Error())
	assert.Equal(t, wfr.LastFailureOnReverse.Error.Error(), out.LastFailureOnReverse.Error.Error())
}

func TestWorkflowReport_MarshalUnmarshalYAML(t *testing.T) {
	now := time.Now()
	wfr := &WorkflowReport{
		WorkflowID:   "wf2",
		StartTime:    now,
		EndTime:      now.Add(time.Minute),
		Status:       "failed",
		StepSequence: []string{"x", "y"},
		StepReports: []*StepReport{
			{StepID: "x", Action: "run", StartTime: now, EndTime: now, Status: "fail"},
			{StepID: "y", Action: "run", StartTime: now, EndTime: now, Status: "fail"},
		},
		FirstFailureOnForward: &StepFailure{StepID: "x", Action: "run", Error: &marshalError{"failx"}},
		LastFailureOnReverse:  &StepFailure{StepID: "y", Action: "rollback", Error: &marshalError{"faily"}},
	}
	data, err := yaml.Marshal(wfr)
	assert.NoError(t, err)

	var out WorkflowReport
	assert.NoError(t, yaml.Unmarshal(data, &out))
	assert.Equal(t, wfr.WorkflowID, out.WorkflowID)
	assert.Equal(t, wfr.Status, out.Status)
	assert.Equal(t, len(wfr.StepReports), len(out.StepReports))
	assert.Equal(t, wfr.FirstFailureOnForward.Error.Error(), out.FirstFailureOnForward.Error.Error())
	assert.Equal(t, wfr.LastFailureOnReverse.Error.Error(), out.LastFailureOnReverse.Error.Error())
}
