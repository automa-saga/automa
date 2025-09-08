package types

import (
	"encoding/json"
	"gopkg.in/yaml.v3"
)

func (a *Action) String() string {
	switch *a {
	case ActionExecute:
		return "execute"
	case ActionRollback:
		return "rollback"
	default:
		return "unknown"
	}
}

func (a *Action) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

func (a *Action) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "execute":
		*a = ActionExecute
	case "rollback":
		*a = ActionRollback
	default:
		*a = 0
	}
	return nil
}

func (a *Action) MarshalYAML() (interface{}, error) {
	return a.String(), nil
}

func (a *Action) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	switch s {
	case "execute":
		*a = ActionExecute
	case "rollback":
		*a = ActionRollback
	default:
		*a = 0
	}
	return nil
}
