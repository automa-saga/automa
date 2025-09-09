package automa

import (
	"encoding/json"
	"errors"
	"gopkg.in/yaml.v3"
)

type TypeRollbackMode uint8

const (
	RollbackModeContinueOnError TypeRollbackMode = 1
	RollbackModeStopOnError     TypeRollbackMode = 2
)

func (rm TypeRollbackMode) String() string {
	switch rm {
	case RollbackModeContinueOnError:
		return "continue"
	case RollbackModeStopOnError:
		return "stop"
	default:
		return "unknown"
	}
}

func (rm TypeRollbackMode) MarshalJSON() ([]byte, error) {
	return json.Marshal(rm.String())
}

func (rm *TypeRollbackMode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "continue":
		*rm = RollbackModeContinueOnError
	case "stop":
		*rm = RollbackModeStopOnError
	default:
		return errors.New("unknown TypeRollbackMode")
	}
	return nil
}

func (rm TypeRollbackMode) MarshalYAML() (interface{}, error) {
	return rm.String(), nil
}

func (rm *TypeRollbackMode) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	switch s {
	case "continue":
		*rm = RollbackModeContinueOnError
	case "stop":
		*rm = RollbackModeStopOnError
	default:
		return errors.New("unknown TypeRollbackMode")
	}
	return nil
}
