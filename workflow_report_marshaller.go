package automa

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"time"
)

// Shared struct for both JSON and YAML marshalling
type workflowReportCodec struct {
	ID          string            `json:"id" yaml:"id"`
	StartTime   time.Time         `json:"start_time" yaml:"start_time"`
	EndTime     time.Time         `json:"end_time" yaml:"end_time"`
	Status      Status            `json:"status" yaml:"status"`
	Error       string            `json:"error,omitempty" yaml:"error,omitempty"`
	Message     string            `json:"message,omitempty" yaml:"message,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	StepReports []*stepReport     `json:"step_reports,omitempty" yaml:"step_reports,omitempty"`
}

func toStepReports(reports []Report) []*stepReport {
	var stepReports []*stepReport
	for _, report := range reports {
		if r, ok := report.(*stepReport); ok {
			stepReports = append(stepReports, r)
		}
	}

	return stepReports
}

func toReports(stepReports []*stepReport) []Report {
	var reports []Report
	for _, r := range stepReports {
		reports = append(reports, r)
	}
	return reports
}

func (wr *workflowReport) MarshalYAML() (interface{}, error) {
	var errStr string
	if wr.err != nil {
		errStr = wr.err.Error()
	}

	return &workflowReportCodec{
		ID:          wr.id,
		StartTime:   wr.startTime,
		EndTime:     wr.endTime,
		Status:      wr.status,
		Error:       errStr,
		Message:     wr.message,
		Metadata:    wr.metadata,
		StepReports: toStepReports(wr.stepReports),
	}, nil
}

func (wr *workflowReport) UnmarshalYAML(value *yaml.Node) error {
	var temp workflowReportCodec
	if err := value.Decode(&temp); err != nil {
		return err
	}
	wr.id = temp.ID
	wr.startTime = temp.StartTime
	wr.endTime = temp.EndTime
	wr.status = temp.Status
	wr.message = temp.Message
	wr.metadata = temp.Metadata
	wr.stepReports = toReports(temp.StepReports)
	if temp.Error != "" {
		wr.err = fmt.Errorf(temp.Error)
	} else {
		wr.err = nil
	}
	return nil
}

func (wr *workflowReport) MarshalJSON() ([]byte, error) {
	var errStr string
	if wr.err != nil {
		errStr = wr.err.Error()
	}
	return json.Marshal(&workflowReportCodec{
		ID:          wr.id,
		StartTime:   wr.startTime,
		EndTime:     wr.endTime,
		Status:      wr.status,
		Error:       errStr,
		Message:     wr.message,
		Metadata:    wr.metadata,
		StepReports: toStepReports(wr.stepReports),
	})
}

func (wr *workflowReport) UnmarshalJSON(data []byte) error {
	var temp workflowReportCodec
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	wr.id = temp.ID
	wr.startTime = temp.StartTime
	wr.endTime = temp.EndTime
	wr.status = temp.Status
	wr.message = temp.Message
	wr.metadata = temp.Metadata
	wr.stepReports = toReports(temp.StepReports)
	if temp.Error != "" {
		wr.err = fmt.Errorf(temp.Error)
	} else {
		wr.err = nil
	}
	return nil
}
