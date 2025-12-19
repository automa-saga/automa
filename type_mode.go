package automa

import (
	"encoding/json"
	"errors"

	"gopkg.in/yaml.v3"
)

// TypeMode defines the behavior when errors occur during workflow execution or rollback.
// It controls whether to stop or continue processing remaining steps.
type TypeMode uint8

const (
	ContinueOnError TypeMode = 1
	StopOnError     TypeMode = 2
	RollbackOnError TypeMode = 3
)

func (rm TypeMode) String() string {
	switch rm {
	case ContinueOnError:
		return "continue"
	case RollbackOnError:
		return "rollback"
	case StopOnError:
		return "stop"
	default:
		return "unknown"
	}
}

func (rm TypeMode) MarshalJSON() ([]byte, error) {
	return json.Marshal(rm.String())
}

func (rm *TypeMode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "continue":
		*rm = ContinueOnError
	case "rollback":
		*rm = RollbackOnError
	case "stop":
		*rm = StopOnError
	default:
		return errors.New("unknown TypeMode")
	}
	return nil
}

func (rm TypeMode) MarshalYAML() (interface{}, error) {
	return rm.String(), nil
}

func (rm *TypeMode) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	switch s {
	case "continue":
		*rm = ContinueOnError
	case "rollback":
		*rm = RollbackOnError
	case "stop":
		*rm = StopOnError
	default:
		return errors.New("unknown TypeMode")
	}
	return nil
}
