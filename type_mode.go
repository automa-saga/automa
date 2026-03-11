package automa

import (
	"encoding/json"
	"errors"

	"gopkg.in/yaml.v3"
)

// TypeMode controls the error-handling strategy applied by a [Workflow] during
// step execution or rollback. It answers the question: "when something goes
// wrong, what should happen next?"
//
// Two independent TypeMode values are configured on each workflow:
//   - executionMode — governs what happens when a step's Execute fails.
//   - rollbackMode  — governs what happens when a step's Rollback fails
//     (only relevant when executionMode is [RollbackOnError]).
//
// TypeMode is serialized as a human-readable string in JSON and YAML. Unknown
// values are treated as errors during unmarshaling (unlike [TypeAction] and
// [TypeStatus] which default silently).
type TypeMode uint8

const (
	// ContinueOnError instructs the workflow to keep processing remaining steps
	// even after a failure. When used as executionMode, all steps run regardless
	// of earlier failures and the final workflow report reflects the aggregate
	// outcome. When used as rollbackMode, all previously executed steps are
	// rolled back even if an earlier rollback itself fails.
	//
	// Serialized as "continue".
	ContinueOnError TypeMode = 1

	// StopOnError instructs the workflow to halt immediately when a failure is
	// encountered, skipping all remaining steps. No rollback is triggered.
	// This is the default executionMode.
	//
	// Serialized as "stop".
	StopOnError TypeMode = 2

	// RollbackOnError instructs the workflow to halt on the first failure and
	// then roll back all previously executed steps in reverse order. The
	// rollbackMode setting controls whether the rollback loop itself stops or
	// continues when an individual rollback fails.
	//
	// Serialized as "rollback".
	RollbackOnError TypeMode = 3
)

// String returns the human-readable name of the mode:
// "continue", "stop", "rollback", or "unknown" for unrecognised values.
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

// MarshalJSON implements json.Marshaler. It serializes the mode as its string
// name (e.g. "rollback") rather than the numeric value, making JSON output
// self-describing and stable across future reordering of constant values.
func (rm TypeMode) MarshalJSON() ([]byte, error) {
	return json.Marshal(rm.String())
}

// UnmarshalJSON implements json.Unmarshaler. It accepts the string names
// "continue", "stop", and "rollback". Unlike [TypeAction] and [TypeStatus],
// an unrecognised value returns an error to prevent silent misconfiguration of
// critical workflow behaviour.
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

// MarshalYAML implements yaml.Marshaler. It serializes the mode as its string
// name, consistent with MarshalJSON.
func (rm TypeMode) MarshalYAML() (interface{}, error) {
	return rm.String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler. It accepts the same string names
// as UnmarshalJSON and returns an error for unrecognised values.
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
