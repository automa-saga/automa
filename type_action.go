package automa

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// TypeAction identifies which lifecycle phase of a [Step] produced a [Report].
// It is serialized as a human-readable string in JSON and YAML output.
type TypeAction uint8

const (
	// ActionPrepare indicates the report was produced during the Prepare phase.
	// This value is zero so it also serves as the default/unset state.
	ActionPrepare TypeAction = 0

	// ActionExecute indicates the report was produced during the Execute phase.
	// This is the most common action type for step reports.
	ActionExecute TypeAction = 1

	// ActionRollback indicates the report was produced during the Rollback phase.
	// Reports with this action are nested inside a parent report's Rollback field.
	ActionRollback TypeAction = 2
)

// String returns the human-readable name of the action:
// "prepare", "execute", "rollback", or "unknown" for unrecognised values.
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

// MarshalJSON implements json.Marshaler. It serializes the action as its
// string name (e.g. "execute") rather than the numeric value, making JSON
// output self-describing.
func (a TypeAction) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

// UnmarshalJSON implements json.Unmarshaler. It accepts the string names
// "prepare", "execute", and "rollback". Unrecognised values are silently
// mapped to 0 (ActionPrepare) rather than returning an error.
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

// MarshalYAML implements yaml.Marshaler. It serializes the action as its
// string name, consistent with MarshalJSON.
func (a TypeAction) MarshalYAML() (interface{}, error) {
	return a.String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler. It accepts the same string names
// as UnmarshalJSON. Unrecognised values are silently mapped to 0 (ActionPrepare).
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
