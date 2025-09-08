package types

import (
	"encoding/json"
	"errors"
	"gopkg.in/yaml.v3"
)

func (rm *RollbackMode) String() string {
	switch *rm {
	case RollbackModeContinueOnError:
		return "continue"
	case RollbackModeStopOnError:
		return "stop"
	default:
		return "unknown"
	}
}

func (rm *RollbackMode) MarshalJSON() ([]byte, error) {
	return json.Marshal(rm.String())
}

func (rm *RollbackMode) UnmarshalJSON(data []byte) error {
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
		return errors.New("unknown RollbackMode")
	}
	return nil
}

func (rm *RollbackMode) MarshalYAML() (interface{}, error) {
	return rm.String(), nil
}

func (rm *RollbackMode) UnmarshalYAML(value *yaml.Node) error {
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
		return errors.New("unknown RollbackMode")
	}
	return nil
}
