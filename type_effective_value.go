package automa

import (
	"encoding/json"

	"github.com/joomcode/errorx"
	"gopkg.in/yaml.v3"
)

// EffectiveStrategy describes how an effective value was determined.
//
// It is used to indicate whether a value came from the system default,
// from explicit user input, or from custom resolution logic.
type EffectiveStrategy uint8

const (
	// StrategyDefault indicates that the effective value was determined by
	// the system's default value.
	StrategyDefault EffectiveStrategy = 0

	// StrategyUserInput indicates that the effective value was provided
	// explicitly by user input.
	StrategyUserInput EffectiveStrategy = 1

	// StrategyCustom indicates that the effective value was determined by
	// custom logic
	StrategyCustom EffectiveStrategy = 2

	// StrategyCurrent indicates that the effective value was determined by
	// current state
	StrategyCurrent EffectiveStrategy = 3
)

// String returns the textual representation of the EffectiveStrategy.
//
// Known values are "default", "userInput" and "custom". Unknown values
// return "unknown".
func (es EffectiveStrategy) String() string {
	switch es {
	case StrategyDefault:
		return "default"
	case StrategyUserInput:
		return "userInput"
	case StrategyCustom:
		return "custom"
	case StrategyCurrent:
		{
			return "current"
		}
	default:
		return "unknown"
	}
}

// MarshalJSON implements json.Marshaler for EffectiveStrategy.
//
// It encodes the strategy as a quoted string using the same values as
// String().
func (es EffectiveStrategy) MarshalJSON() ([]byte, error) {
	return json.Marshal(es.String())
}

// UnmarshalJSON implements json.Unmarshaler for EffectiveStrategy.
//
// It accepts the quoted string representations "default", "userInput" and
// "custom". Unknown values are mapped to StrategyDefault for backward
// compatibility.
func (es *EffectiveStrategy) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "default":
		*es = StrategyDefault
	case "userInput":
		*es = StrategyUserInput
	case "custom":
		*es = StrategyCustom
	case "current":
		{
			*es = StrategyCurrent
		}
	default:
		*es = StrategyDefault
	}
	return nil
}

// MarshalYAML encodes the EffectiveStrategy as its string representation.
func (es EffectiveStrategy) MarshalYAML() (interface{}, error) {
	return es.String(), nil
}

// UnmarshalYAML decodes the EffectiveStrategy from a YAML node containing the string.
// Unknown values are mapped to StrategyDefault for compatibility.
func (es *EffectiveStrategy) UnmarshalYAML(node *yaml.Node) error {
	var s string
	if err := node.Decode(&s); err != nil {
		return err
	}
	switch s {
	case "default":
		*es = StrategyDefault
	case "userInput":
		*es = StrategyUserInput
	case "custom":
		*es = StrategyCustom
	case "current":
		*es = StrategyCurrent
	default:
		*es = StrategyDefault
	}
	return nil
}

// EffectiveValue wraps a Value[T] together with information about how that
// value was chosen (the EffectiveStrategy).
//
// The wrapper provides cloning and accessors to retrieve the underlying
// value and the associated strategy.
type EffectiveValue[T any] struct {
	v        Value[T]
	strategy EffectiveStrategy
}

// NewEffectiveValue constructs a new EffectiveValue that pairs the provided
// Value with the given EffectiveStrategy.
// An error is returned if the provided Value is nil.
func NewEffectiveValue[T any](v Value[T], strategy EffectiveStrategy) (*EffectiveValue[T], error) {
	if v == nil {
		return nil, errorx.IllegalArgument.New("value cannot be nil")
	}

	return &EffectiveValue[T]{
		v:        v,
		strategy: strategy,
	}, nil
}

// NewEffective constructs a new EffectiveValue that pairs the provided
// Value with the given EffectiveStrategy.
// An error is returned if the provided Value is nil.
func NewEffective[T any](v T, strategy EffectiveStrategy) (*EffectiveValue[T], error) {
	if IsNil(v) {
		return nil, errorx.IllegalArgument.New("value cannot be nil")
	}

	vv, err := NewValue[T](v)
	if err != nil {
		return nil, err
	}

	return NewEffectiveValue(vv, strategy)
}

// Clone produces a deep clone of the EffectiveValue by cloning the
// underlying Value using its Clone method.
//
// If the underlying Value's Clone fails, the error is returned.
func (ev EffectiveValue[T]) Clone() (*EffectiveValue[T], error) {
	cv, err := ev.v.Clone()
	if err != nil {
		return nil, err
	}

	clone := &EffectiveValue[T]{
		v:        cv,
		strategy: ev.strategy,
	}

	return clone, nil
}

// Get returns the underlying Value[T] wrapped by this EffectiveValue.
//
// The returned Value should be treated according to its own concurrency and
// cloning semantics.
func (ev EffectiveValue[T]) Get() Value[T] {
	return ev.v
}

// Strategy returns the EffectiveStrategy indicating how the underlying
// value was selected.
func (ev EffectiveValue[T]) Strategy() EffectiveStrategy {
	return ev.strategy
}
