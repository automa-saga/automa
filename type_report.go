package automa

import (
	"encoding/json"
	"gopkg.in/yaml.v3"
)

func (rt *TypeReport) String() string {
	switch *rt {
	case StepReport:
		return "StepReport"
	case WorkflowReport:
		return "WorkflowReport"
	default:
		return "UnknownReport"
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
		*rt = StepReport
	case "WorkflowReport":
		*rt = WorkflowReport
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
		*rt = StepReport
	case "WorkflowReport":
		*rt = WorkflowReport
	default:
		*rt = 0
	}
	return nil
}
