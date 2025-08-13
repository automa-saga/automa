package automa

import (
	"encoding/json"
	"gopkg.in/yaml.v3"
)

func (rt *TypeReport) String() string {
	switch *rt {
	case StepReportType:
		return "StepReport"
	case WorkflowReportType:
		return "WorkflowReport"
	default:
		return "UnknownReportType"
	}
}

func (rt *TypeReport) MarshalJSON() ([]byte, error) {
	return json.Marshal(rt.String())
}

func (rt *TypeReport) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "StepReport":
		*rt = StepReportType
	case "WorkflowReport":
		*rt = WorkflowReportType
	default:
		*rt = 0
	}
	return nil
}

func (rt *TypeReport) MarshalYAML() (interface{}, error) {
	return rt.String(), nil
}

func (rt *TypeReport) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	switch s {
	case "StepReport":
		*rt = StepReportType
	case "WorkflowReport":
		*rt = WorkflowReportType
	default:
		*rt = 0
	}
	return nil
}
