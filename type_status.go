package automa

import (
	"encoding/json"
	"gopkg.in/yaml.v3"
)

func (s *TypeStatus) String() string {
	switch *s {
	case StatusSuccess:
		return "success"
	case StatusFailed:
		return "failed"
	case StatusSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}

func (s *TypeStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *TypeStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "success":
		*s = StatusSuccess
	case "failed":
		*s = StatusFailed
	case "skipped":
		*s = StatusSkipped
	default:
		*s = 0
	}
	return nil
}

func (s *TypeStatus) MarshalYAML() (interface{}, error) {
	return s.String(), nil
}

func (s *TypeStatus) UnmarshalYAML(value *yaml.Node) error {
	var str string
	if err := value.Decode(&str); err != nil {
		return err
	}
	switch str {
	case "success":
		*s = StatusSuccess
	case "failed":
		*s = StatusFailed
	case "skipped":
		*s = StatusSkipped
	default:
		*s = 0
	}
	return nil
}
