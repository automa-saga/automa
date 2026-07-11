package automa

import (
	"encoding/json"
	"errors"

	"gopkg.in/yaml.v3"
)

// TypeStatus represents the outcome of a [Step] lifecycle phase (Prepare,
// Execute, or Rollback). It is recorded in every [Report] and is the primary
// signal used by the workflow's execution-mode logic to decide whether to
// continue, stop, or roll back.
//
// The zero value of TypeStatus is intentionally unused so that an
// uninitialised report is distinguishable from one with a real outcome.
// It is serialized as a human-readable string in JSON and YAML output.
type TypeStatus uint8

const (
	// StatusSuccess indicates the phase completed without error. A workflow
	// proceeds to the next step after a successful execute.
	StatusSuccess TypeStatus = 1

	// StatusFailed indicates the phase encountered an error. Depending on the
	// workflow's [TypeMode], a failed execute may stop, continue, or trigger
	// rollback.
	StatusFailed TypeStatus = 2

	// StatusSkipped indicates the phase was bypassed. This happens when a step
	// has no execute function configured, or when the step's own logic
	// determines that no work is needed. A skipped step does not count as a
	// failure and does not stop execution.
	StatusSkipped TypeStatus = 3
)

// String returns the human-readable name of the status:
// "success", "failed", "skipped", or "unknown" for unrecognised values.
func (s TypeStatus) String() string {
	switch s {
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

// MarshalJSON implements json.Marshaler. It serializes the status as its
// string name (e.g. "success") rather than the numeric value, making JSON
// output self-describing.
func (s TypeStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON implements json.Unmarshaler. It accepts the string names
// "success", "failed", and "skipped". Because the report is a cross-language
// wire contract, an unrecognised value returns an error rather than silently
// defaulting, so that format drift is surfaced instead of hidden.
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
		return errors.New("unknown TypeStatus")
	}
	return nil
}

// MarshalYAML implements yaml.Marshaler. It serializes the status as its
// string name, consistent with MarshalJSON.
func (s TypeStatus) MarshalYAML() (interface{}, error) {
	return s.String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler. It accepts the same string names
// as UnmarshalJSON and returns an error for unrecognised values.
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
		return errors.New("unknown TypeStatus")
	}
	return nil
}
