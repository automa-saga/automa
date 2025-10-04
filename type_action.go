package automa

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

type TypeAction uint8

const (
	ActionPrepare  TypeAction = 0
	ActionExecute  TypeAction = 1
	ActionRollback TypeAction = 2
)

func (a TypeAction) String() string {
	switch a {
	case ActionPrepare:
		return "prepare"
	case ActionExecute:
		return "execute"
	case ActionRollback:
		return "rollback"
	default:
		return "unknown"
	}
}

func (a TypeAction) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

func (a *TypeAction) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "prepare":
		*a = ActionPrepare
	case "execute":
		*a = ActionExecute
	case "rollback":
		*a = ActionRollback
	default:
		*a = 0
	}
	return nil
}

func (a TypeAction) MarshalYAML() (interface{}, error) {
	return a.String(), nil
}

func (a *TypeAction) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	switch s {
	case "prepare":
		*a = ActionPrepare
	case "execute":
		*a = ActionExecute
	case "rollback":
		*a = ActionRollback
	default:
		*a = 0
	}
	return nil
}
