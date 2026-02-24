// Package automa provides interfaces for implementing the Saga pattern for steps and workflows.
package automa

import (
	"context"
)

type Step interface {
	Id() string
	Prepare(ctx context.Context) (context.Context, error)
	Execute(ctx context.Context) *Report
	Rollback(ctx context.Context) *Report
	State() NamespacedStateBag
	WithState(s NamespacedStateBag) Step
}

// StateBag is a thread-safe key-value store for workflow state management.
// It provides basic storage operations without namespace support.
//
// StateBag is used as the underlying storage for individual namespaces in NamespacedStateBag.
// For step state management with namespace isolation, use NamespacedStateBag instead.
type StateBag interface {
	Get(key Key) (interface{}, bool)
	Set(key Key, value interface{}) StateBag
	Delete(key Key) StateBag
	Clear() StateBag
	Keys() []Key
	Size() int
	Items() map[Key]interface{}
	Merge(other StateBag) (StateBag, error)
	Clone() (StateBag, error)

	// Helper methods for extracting typed values
	String(key Key) string
	Bool(key Key) bool
	Int(key Key) int
	Int8(key Key) int8
	Int16(key Key) int16
	Int32(key Key) int32
	Int64(key Key) int64
	Float(key Key) float64
	Float32(key Key) float32
	Float64(key Key) float64
}

// NamespacedStateBag provides namespace-aware state management for workflow steps.
//
// It supports three types of namespaces:
//   - Local: Step-private state that is not visible to other steps
//   - Global: Workflow-shared state that all steps can access
//   - Custom: Named namespaces for specific use cases
//
// Usage Examples:
//
//	// Write to local state (isolated, not visible to other steps)
//	step.State().Local().Set("my-data", value)
//
//	// Write to global state (shared across all steps)
//	step.State().Global().Set("shared-config", config)
//
//	// Read from local namespace
//	val, ok := step.State().Local().Get("key")
//
//	// Read from global namespace
//	val, ok := step.State().Global().Get("shared-config")
//
//	// Custom namespace
//	step.State().WithNamespace("custom").Set("key", value)
//
// Implementation:
//
// SyncNamespacedStateBag is the primary implementation.
type NamespacedStateBag interface {
	// Local returns a view of the local namespace (step-private state).
	// Operations on the returned StateBag affect only the local namespace.
	Local() StateBag

	// Global returns a view of the global namespace (workflow-shared state).
	// Operations on the returned StateBag affect only the global namespace.
	Global() StateBag

	// WithNamespace returns a view of a custom namespace.
	// Operations on the returned StateBag affect only the specified namespace.
	// Custom namespaces are isolated from local and global namespaces.
	WithNamespace(name string) StateBag

	// Clone creates a deep copy of the NamespacedStateBag including all namespaces.
	// This clones the local, global, and all custom namespaces.
	Clone() (NamespacedStateBag, error)

	// Merge merges another NamespacedStateBag into this one.
	// It merges local, global, and custom namespaces separately:
	// - Local namespaces are merged
	// - Global namespaces are merged
	// - Custom namespaces are merged individually by name
	//   - If a custom namespace exists in both, they are merged
	//   - If a custom namespace only exists in other, it is added
	Merge(other NamespacedStateBag) (NamespacedStateBag, error)
}

type Workflow interface {
	Step
	Steps() []Step
}

type Builder interface {
	Id() string
	Validate() error
	Build() (Step, error)
}

type Registry interface {
	Add(steps ...Builder) error // return error if step with same id already exists
	Remove(id string) bool
	Has(id string) bool
	Of(id string) Builder
}
