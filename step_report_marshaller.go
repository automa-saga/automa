package automa

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"time"
)

// Shared struct for both JSON and YAML marshalling
type stepReportCodec struct {
	ID        string            `json:"id" yaml:"id"`
	Action    ActionType        `json:"action" yaml:"action"`
	StartTime time.Time         `json:"start_time" yaml:"start_time"`
	EndTime   time.Time         `json:"end_time" yaml:"end_time"`
	Status    Status            `json:"status" yaml:"status"`
	Error     string            `json:"error,omitempty" yaml:"error,omitempty"`
	Message   string            `json:"message,omitempty" yaml:"message,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

func (sr *stepReport) MarshalYAML() (interface{}, error) {
	var errStr string
	if sr.err != nil {
		errStr = sr.err.Error()
	}
	return &stepReportCodec{
		ID:        sr.id,
		Action:    sr.action,
		StartTime: sr.startTime,
		EndTime:   sr.endTime,
		Status:    sr.status,
		Error:     errStr,
		Message:   sr.message,
		Metadata:  sr.metadata,
	}, nil
}

func (sr *stepReport) UnmarshalYAML(value *yaml.Node) error {
	var temp stepReportCodec
	if err := value.Decode(&temp); err != nil {
		return err
	}
	sr.id = temp.ID
	sr.startTime = temp.StartTime
	sr.endTime = temp.EndTime
	sr.status = temp.Status
	sr.message = temp.Message
	sr.metadata = temp.Metadata
	if temp.Error != "" {
		sr.err = fmt.Errorf(temp.Error)
	} else {
		sr.err = nil
	}
	return nil
}

func (sr *stepReport) MarshalJSON() ([]byte, error) {
	var errStr string
	if sr.err != nil {
		errStr = sr.err.Error()
	}
	return json.Marshal(&stepReportCodec{
		ID:        sr.id,
		Action:    sr.action,
		StartTime: sr.startTime,
		EndTime:   sr.endTime,
		Status:    sr.status,
		Error:     errStr,
		Message:   sr.message,
		Metadata:  sr.metadata,
	})
}

func (sr *stepReport) UnmarshalJSON(data []byte) error {
	var temp stepReportCodec
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	sr.id = temp.ID
	sr.startTime = temp.StartTime
	sr.endTime = temp.EndTime
	sr.status = temp.Status
	sr.message = temp.Message
	sr.metadata = temp.Metadata
	if temp.Error != "" {
		sr.err = fmt.Errorf(temp.Error)
	} else {
		sr.err = nil
	}
	return nil
}
