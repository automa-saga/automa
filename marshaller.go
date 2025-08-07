package automa

import (
	"encoding/json"
	"gopkg.in/yaml.v3"
	"time"
)

// Custom struct for marshalling/unmarshalling StepFailure
type stepFailureJSON struct {
	StepID string `json:"stepID"`
	Action string `json:"action"`
	Error  string `json:"error"`
}

// Custom struct for marshalling/unmarshalling StepReport
type stepReportJSON struct {
	StepID    string            `json:"stepID"`
	Action    string            `json:"action"`
	StartTime time.Time         `json:"startTime"`
	EndTime   time.Time         `json:"endTime"`
	Status    string            `json:"status"`
	Error     string            `json:"failure_reason"`
	Metadata  map[string][]byte `json:"metadata"`
}

// Custom struct for marshalling/unmarshalling WorkflowReport
type workflowReportJSON struct {
	WorkflowID   string           `json:"workflowID"`
	StartTime    time.Time        `json:"startTime"`
	EndTime      time.Time        `json:"endTime"`
	Status       string           `json:"status"`
	StepSequence []string         `json:"stepSequence"`
	StepReports  []stepReportJSON `json:"stepReports"`
	FirstFailure *stepFailureJSON `json:"firstExecutionFailure"`
	LastFailure  *stepFailureJSON `json:"lastRollbackFailure"`
}

// MarshalJSON for StepFailure
func (sf *StepFailure) MarshalJSON() ([]byte, error) {
	if sf == nil {
		return []byte("null"), nil
	}
	return json.Marshal(&stepFailureJSON{
		StepID: sf.StepID,
		Action: sf.Action,
		Error:  errorToString(sf.Error),
	})
}

// UnmarshalJSON for StepFailure
func (sf *StepFailure) UnmarshalJSON(data []byte) error {
	var tmp stepFailureJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	sf.StepID = tmp.StepID
	sf.Action = tmp.Action
	if tmp.Error != "" {
		sf.Error = stringToError(tmp.Error)
	} else {
		sf.Error = nil
	}
	return nil
}

// MarshalJSON for StepReport
func (sr *StepReport) MarshalJSON() ([]byte, error) {
	return json.Marshal(&stepReportJSON{
		StepID:    sr.StepID,
		Action:    sr.Action,
		StartTime: sr.StartTime,
		EndTime:   sr.EndTime,
		Status:    sr.Status,
		Error:     errorToString(sr.Error),
		Metadata:  sr.Metadata,
	})
}

// UnmarshalJSON for StepReport
func (sr *StepReport) UnmarshalJSON(data []byte) error {
	var tmp stepReportJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	sr.StepID = tmp.StepID
	sr.Action = tmp.Action
	sr.StartTime = tmp.StartTime
	sr.EndTime = tmp.EndTime
	sr.Status = tmp.Status
	sr.Metadata = tmp.Metadata
	if tmp.Error != "" {
		sr.Error = stringToError(tmp.Error)
	} else {
		sr.Error = nil
	}
	return nil
}

// MarshalJSON for WorkflowReport
func (wfr *WorkflowReport) MarshalJSON() ([]byte, error) {
	var stepReports []stepReportJSON
	for _, sr := range wfr.StepReports {
		b, _ := json.Marshal(sr)
		var tmp stepReportJSON
		_ = json.Unmarshal(b, &tmp)
		stepReports = append(stepReports, tmp)
	}
	var firstFailure, lastFailure *stepFailureJSON
	if wfr.FirstFailureOnForward != nil {
		b, _ := json.Marshal(wfr.FirstFailureOnForward)
		_ = json.Unmarshal(b, &firstFailure)
	}
	if wfr.LastFailureOnReverse != nil {
		b, _ := json.Marshal(wfr.LastFailureOnReverse)
		_ = json.Unmarshal(b, &lastFailure)
	}
	return json.Marshal(&workflowReportJSON{
		WorkflowID:   wfr.WorkflowID,
		StartTime:    wfr.StartTime,
		EndTime:      wfr.EndTime,
		Status:       wfr.Status,
		StepSequence: wfr.StepSequence,
		StepReports:  stepReports,
		FirstFailure: firstFailure,
		LastFailure:  lastFailure,
	})
}

// UnmarshalJSON for WorkflowReport
func (wfr *WorkflowReport) UnmarshalJSON(data []byte) error {
	var tmp workflowReportJSON
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	wfr.WorkflowID = tmp.WorkflowID
	wfr.StartTime = tmp.StartTime
	wfr.EndTime = tmp.EndTime
	wfr.Status = tmp.Status
	wfr.StepSequence = tmp.StepSequence
	wfr.StepReports = make([]*StepReport, len(tmp.StepReports))
	for i, srj := range tmp.StepReports {
		var sr StepReport
		b, _ := json.Marshal(srj)
		_ = json.Unmarshal(b, &sr)
		wfr.StepReports[i] = &sr
	}
	if tmp.FirstFailure != nil {
		var sf StepFailure
		b, _ := json.Marshal(tmp.FirstFailure)
		_ = json.Unmarshal(b, &sf)
		wfr.FirstFailureOnForward = &sf
	}
	if tmp.LastFailure != nil {
		var sf StepFailure
		b, _ := json.Marshal(tmp.LastFailure)
		_ = json.Unmarshal(b, &sf)
		wfr.LastFailureOnReverse = &sf
	}
	return nil
}

// Helpers for error <-> string
func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
func stringToError(s string) error {
	if s == "" {
		return nil
	}
	return &marshalError{s}
}

type marshalError struct{ msg string }

func (e *marshalError) Error() string { return e.msg }

// MarshalYAML for StepFailure
func (sf *StepFailure) MarshalYAML() (interface{}, error) {
	if sf == nil {
		return nil, nil
	}
	return &stepFailureJSON{
		StepID: sf.StepID,
		Action: sf.Action,
		Error:  errorToString(sf.Error),
	}, nil
}

// UnmarshalYAML for StepFailure
func (sf *StepFailure) UnmarshalYAML(value *yaml.Node) error {
	var tmp stepFailureJSON
	if err := value.Decode(&tmp); err != nil {
		return err
	}
	sf.StepID = tmp.StepID
	sf.Action = tmp.Action
	sf.Error = stringToError(tmp.Error)
	return nil
}

// MarshalYAML for StepReport
func (sr *StepReport) MarshalYAML() (interface{}, error) {
	return &stepReportJSON{
		StepID:    sr.StepID,
		Action:    sr.Action,
		StartTime: sr.StartTime,
		EndTime:   sr.EndTime,
		Status:    sr.Status,
		Error:     errorToString(sr.Error),
		Metadata:  sr.Metadata,
	}, nil
}

// UnmarshalYAML for StepReport
func (sr *StepReport) UnmarshalYAML(value *yaml.Node) error {
	var tmp stepReportJSON
	if err := value.Decode(&tmp); err != nil {
		return err
	}
	sr.StepID = tmp.StepID
	sr.Action = tmp.Action
	sr.StartTime = tmp.StartTime
	sr.EndTime = tmp.EndTime
	sr.Status = tmp.Status
	sr.Metadata = tmp.Metadata
	sr.Error = stringToError(tmp.Error)
	return nil
}

// MarshalYAML for WorkflowReport
func (wfr *WorkflowReport) MarshalYAML() (interface{}, error) {
	var stepReports []stepReportJSON
	for _, sr := range wfr.StepReports {
		b, _ := sr.MarshalYAML()
		stepReports = append(stepReports, *b.(*stepReportJSON))
	}
	var firstFailure, lastFailure *stepFailureJSON
	if wfr.FirstFailureOnForward != nil {
		b, _ := wfr.FirstFailureOnForward.MarshalYAML()
		firstFailure = b.(*stepFailureJSON)
	}
	if wfr.LastFailureOnReverse != nil {
		b, _ := wfr.LastFailureOnReverse.MarshalYAML()
		lastFailure = b.(*stepFailureJSON)
	}
	return &workflowReportJSON{
		WorkflowID:   wfr.WorkflowID,
		StartTime:    wfr.StartTime,
		EndTime:      wfr.EndTime,
		Status:       wfr.Status,
		StepSequence: wfr.StepSequence,
		StepReports:  stepReports,
		FirstFailure: firstFailure,
		LastFailure:  lastFailure,
	}, nil
}

// UnmarshalYAML for WorkflowReport
func (wfr *WorkflowReport) UnmarshalYAML(value *yaml.Node) error {
	var tmp workflowReportJSON
	if err := value.Decode(&tmp); err != nil {
		return err
	}
	wfr.WorkflowID = tmp.WorkflowID
	wfr.StartTime = tmp.StartTime
	wfr.EndTime = tmp.EndTime
	wfr.Status = tmp.Status
	wfr.StepSequence = tmp.StepSequence
	wfr.StepReports = make([]*StepReport, len(tmp.StepReports))
	for i, srj := range tmp.StepReports {
		var sr StepReport
		node := yaml.Node{}
		b, _ := yaml.Marshal(srj)
		_ = yaml.Unmarshal(b, &node)
		_ = sr.UnmarshalYAML(&node)
		wfr.StepReports[i] = &sr
	}
	if tmp.FirstFailure != nil {
		var sf StepFailure
		node := yaml.Node{}
		b, _ := yaml.Marshal(tmp.FirstFailure)
		_ = yaml.Unmarshal(b, &node)
		_ = sf.UnmarshalYAML(&node)
		wfr.FirstFailureOnForward = &sf
	}
	if tmp.LastFailure != nil {
		var sf StepFailure
		node := yaml.Node{}
		b, _ := yaml.Marshal(tmp.LastFailure)
		_ = yaml.Unmarshal(b, &node)
		_ = sf.UnmarshalYAML(&node)
		wfr.LastFailureOnReverse = &sf
	}
	return nil
}
