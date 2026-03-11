// Package automa provides primitives for composing and executing automated
// workflows using the Saga pattern.
//
// # Core concepts
//
// A [Step] is the smallest unit of work. Every step has a unique ID and three
// lifecycle phases:
//
//  1. Prepare — sets up context and state before execution begins.
//  2. Execute — performs the actual work and returns a [Report].
//  3. Rollback — undoes the work when a subsequent step fails.
//
// A [Workflow] is itself a Step that owns an ordered list of child Steps. The
// workflow drives the execution loop, applying the configured [TypeMode]
// (StopOnError, ContinueOnError, RollbackOnError) and collecting per-step
// [Report] values into a final workflow report.
//
// State is carried through a workflow via [NamespacedStateBag], which isolates
// each step's private data (Local) from data that is intentionally shared
// across all steps (Global) while also supporting arbitrary named partitions
// (WithNamespace).
//
// # Building workflows
//
// Use [NewWorkflowBuilder] and [NewStepBuilder] to construct workflows and
// steps declaratively:
//
//	wf, err := automa.NewWorkflowBuilder().
//	    WithId("deploy").
//	    WithExecutionMode(automa.RollbackOnError).
//	    Steps(
//	        automa.NewStepBuilder().
//	            WithId("provision").
//	            WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
//	                // ... provision resources ...
//	                return automa.SuccessReport(stp)
//	            }).
//	            WithRollback(func(ctx context.Context, stp automa.Step) *automa.Report {
//	                // ... tear down resources ...
//	                return automa.SuccessReport(stp)
//	            }),
//	    ).
//	    Build()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	report := wf.Execute(ctx)
//
// # Registry
//
// A [Registry] stores named [Builder] instances so that workflows can look up
// steps by ID without hard-coding constructor calls.
package automa

import (
	"context"
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// Step is the fundamental unit of work in an automa workflow.
//
// # Lifecycle
//
// Each Step passes through up to three phases during a workflow execution:
//
//  1. Prepare(ctx) — called before Execute. Use it to enrich the context,
//     validate preconditions, or pre-populate local state. Returning a
//     non-nil error aborts execution of this step (and, depending on the
//     workflow's [TypeMode], possibly the whole workflow).
//
//  2. Execute(ctx) — performs the step's primary work. Returns a [Report]
//     whose [TypeStatus] (Success, Failed, Skipped) drives the workflow's
//     execution-mode logic.
//
//  3. Rollback(ctx) — called in reverse order when the workflow needs to undo
//     previously executed steps. It is only invoked when the workflow's
//     execution mode is [RollbackOnError] and a later step has failed.
//     Rollback should be idempotent: it may be called more than once.
//
// # State
//
// State() returns the [NamespacedStateBag] associated with this step. The
// workflow provides each step with its own namespaced view so that local writes
// are isolated while global writes remain visible to subsequent steps.
// WithState returns a shallow copy of the step with the given bag attached,
// which is how the workflow injects per-step state before calling Prepare.
//
// # Implementations
//
// Use [NewStepBuilder] to construct a Step without implementing this interface
// directly. For sub-workflows, a [Workflow] also satisfies Step and can be
// nested inside another workflow's step list.
type Step interface {
	// Id returns the unique identifier for this step within its workflow.
	// IDs are used in reports, logging, and the Registry.
	Id() string

	// Prepare is called by the workflow before Execute. It may enrich ctx
	// (e.g. inject a logger or deadline), validate preconditions, or
	// pre-populate the step's local state. The returned context is passed to
	// Execute and Rollback. A non-nil error aborts this step.
	Prepare(ctx context.Context) (context.Context, error)

	// Execute performs the step's primary work and returns a [Report]
	// describing the outcome. The report's [TypeStatus] determines whether
	// the workflow continues, stops, or begins rolling back.
	Execute(ctx context.Context) *Report

	// Rollback undoes the work performed by Execute. It is called in reverse
	// step order when the workflow is operating in [RollbackOnError] mode and
	// a later step has failed. Rollback should be idempotent.
	Rollback(ctx context.Context) *Report

	// State returns the [NamespacedStateBag] attached to this step.
	// Local() is private to this step; Global() is shared across all steps in
	// the workflow; WithNamespace(name) provides an isolated named partition.
	State() NamespacedStateBag

	// WithState returns a shallow copy of the step with the provided
	// [NamespacedStateBag] attached. The workflow calls this before Prepare to
	// inject each step's own namespaced view of the workflow state.
	WithState(s NamespacedStateBag) Step
}

// StateBag is a thread-safe key-value store used as the backing storage for
// each namespace partition inside a [NamespacedStateBag].
//
// It is intentionally low-level: it has no notion of local vs. global scope.
// Callers that need namespace isolation should work through
// [NamespacedStateBag] and its Local(), Global(), and WithNamespace() methods
// rather than operating on a StateBag directly.
//
// The primary implementation is [SyncStateBag].
//
// # Typed accessors
//
// The String, Bool, Int, … methods are coercing accessors: they attempt to
// convert the stored value to the target type even when the types do not match
// exactly (e.g. a float64 stored by a JSON round-trip will be returned by
// Int()). Each returns (value, true) on success and (zero, false) when the key
// is absent or coercion fails.
//
// When you need to distinguish "key absent / coercion failed" from an
// intentionally stored zero value, use [FromState][T] which surfaces the same
// semantics explicitly. For the raw, uncoerced value use Get.
//
// # Serialization
//
// StateBag embeds json.Marshaler, json.Unmarshaler, yaml.Marshaler, and
// yaml.Unmarshaler so that workflow state can be persisted and restored. After
// a JSON round-trip, numbers are decoded as float64 by encoding/json; the
// typed accessors handle this transparently via internal coercion.
type StateBag interface {
	// Get returns the raw value stored under key and whether it was present.
	// No coercion is applied; the caller receives exactly what was passed to Set.
	Get(key Key) (interface{}, bool)

	// Set stores value under key, replacing any prior value, and returns the
	// StateBag to allow method chaining: bag.Set("a", 1).Set("b", 2).
	// Storing a nil value is permitted and is distinguishable from an absent
	// key via Get.
	Set(key Key, value interface{}) StateBag

	// Delete removes the entry for key. It is a no-op when the key is absent.
	// Returns the StateBag for chaining.
	Delete(key Key) StateBag

	// Clear removes all entries, leaving the bag empty but still usable.
	// Returns the StateBag for chaining.
	Clear() StateBag

	// Keys returns a snapshot of all keys currently stored in the bag.
	// Order is not guaranteed. The returned slice is a copy.
	Keys() []Key

	// Size returns the number of key-value pairs currently in the bag.
	Size() int

	// Items returns a shallow snapshot of all key-value pairs as a new
	// map[Key]interface{}. Modifying the returned map does not affect the bag.
	Items() map[Key]interface{}

	// Merge copies every key-value pair from other into this bag, overwriting
	// existing keys. Returns the receiver and any error. Passing nil is a no-op.
	Merge(other StateBag) (StateBag, error)

	// Clone returns a deep copy of the bag. Values that implement a Clone()
	// method are deep-copied; all others are shallow-copied.
	Clone() (StateBag, error)

	// String returns the string value stored under key, coercing numeric and
	// boolean types to their string representations. Returns ("", false) when
	// the key is absent or coercion is not possible.
	String(key Key) (string, bool)

	// Bool returns the bool value stored under key, coercing numeric types
	// (non-zero → true) and the strings "true"/"false". Returns (false, false)
	// when the key is absent or coercion is not possible.
	Bool(key Key) (bool, bool)

	// Int returns the int value stored under key, truncating floats toward
	// zero and parsing numeric strings. Returns (0, false) when absent or
	// not coercible.
	Int(key Key) (int, bool)

	// Int8 is like Int but returns int8. Returns (0, false) when absent or
	// not coercible.
	Int8(key Key) (int8, bool)

	// Int16 is like Int but returns int16. Returns (0, false) when absent or
	// not coercible.
	Int16(key Key) (int16, bool)

	// Int32 is like Int but returns int32. Returns (0, false) when absent or
	// not coercible.
	Int32(key Key) (int32, bool)

	// Int64 is like Int but returns int64. Returns (0, false) when absent or
	// not coercible.
	Int64(key Key) (int64, bool)

	// Float returns the float64 value stored under key, converting all numeric
	// types and parsing numeric strings. Returns (0.0, false) when absent or
	// not coercible.
	Float(key Key) (float64, bool)

	// Float32 is like Float but returns float32. Returns (0.0, false) when
	// absent or not coercible.
	Float32(key Key) (float32, bool)

	// Float64 is like Float. Returns (0.0, false) when absent or not coercible.
	Float64(key Key) (float64, bool)

	// Marshaler serializes the bag as a JSON object (key → value).
	json.Marshaler

	// Unmarshaler replaces the bag's contents by decoding a JSON object.
	// After decoding, JSON numbers are stored as float64; use the typed
	// accessors to coerce them back to integer or other types.
	json.Unmarshaler

	// Marshaler serializes the bag as a YAML mapping (key → value).
	yaml.Marshaler

	// Unmarshaler replaces the bag's contents by decoding a YAML mapping.
	yaml.Unmarshaler
}

// NamespacedStateBag provides namespace-aware state management for workflow
// steps.
//
// # Why namespacing?
//
// When multiple steps in a workflow share a single flat StateBag, they can
// accidentally overwrite each other's data by using the same key. For example,
// two "setup-bind-mount" steps both writing "bind-mount-source" to the same
// bag will have the second step silently overwrite the first step's value.
//
// NamespacedStateBag solves this by providing three distinct partitions:
//
//   - Local — private to one step. The workflow gives each step its own
//     local bag, so writes are never visible to any other step.
//   - Global — shared across all steps in the workflow. Use this for
//     configuration or counters that steps need to publish and consume.
//   - Custom — an arbitrary named partition (e.g. "database-primary"). Use
//     this when a reusable step implementation runs multiple times in the same
//     workflow and each instance needs a collision-free namespace.
//
// # Usage
//
//	// Step-private write (not visible to other steps)
//	stp.State().Local().Set("bind-mount", bindMount)
//
//	// Read from shared global state set by a previous step
//	env, ok := stp.State().Global().String("env")
//
//	// Named partition for a reusable step
//	stp.State().WithNamespace("database-primary").Set("conn", conn)
//
//	// Read back in Rollback (same namespace, same key)
//	val, ok := stp.State().WithNamespace("database-primary").Get("conn")
//
// # Workflow integration
//
// Before each step's Prepare call, the workflow injects a NamespacedStateBag
// whose Global() pointer is shared across all steps and whose Local() is a
// fresh empty bag. Sub-workflows receive a deep clone of the parent's global
// state so that their mutations do not propagate back to the parent.
//
// The primary implementation is [SyncNamespacedStateBag].
type NamespacedStateBag interface {
	// Local returns the StateBag for the local namespace.
	//
	// This bag is private to the step that owns this NamespacedStateBag.
	// No other step in the workflow can read or write it.
	Local() StateBag

	// Global returns the StateBag for the global namespace.
	//
	// This bag is shared across all steps in the workflow. Writes by one step
	// are immediately visible to subsequent steps that call Global().
	Global() StateBag

	// WithNamespace returns the StateBag for the named custom namespace,
	// creating it on first access. The returned bag is stable: repeated calls
	// with the same name return the same underlying bag.
	//
	// Custom namespaces are isolated from Local() and Global() and from each
	// other. They are useful when a single step implementation is instantiated
	// multiple times in one workflow.
	WithNamespace(name string) StateBag

	// Clone returns a fully independent deep copy of this NamespacedStateBag,
	// including the local bag, the global bag, and all custom namespace bags.
	// Mutations to the clone do not affect the original, and vice versa.
	Clone() (NamespacedStateBag, error)

	// Merge copies every namespace from other into this bag, overwriting
	// conflicting keys. Each namespace kind is merged independently:
	//   - Local bags are merged (other's keys overwrite matching keys here).
	//   - Global bags are merged.
	//   - Custom namespaces are merged by name: matching namespaces are merged,
	//     new namespaces in other are deep-cloned and added.
	//
	// Returns the receiver and any error. Passing nil is a no-op.
	Merge(other NamespacedStateBag) (NamespacedStateBag, error)
}

// Workflow is a Step that owns and drives an ordered list of child Steps.
//
// A workflow satisfies the Step interface, which means workflows can be nested:
// a parent workflow may include a child workflow as one of its steps. The child
// receives a deep clone of the parent's global state so that its mutations do
// not propagate back to the parent unless the parent explicitly reads them
// after execution.
//
// Use [NewWorkflowBuilder] to construct a Workflow.
type Workflow interface {
	Step

	// Steps returns the ordered list of child Steps owned by this workflow.
	// The list is the sequence in which steps are executed and (in reverse)
	// rolled back.
	Steps() []Step
}

// Builder constructs a [Step] from a declarative specification.
//
// Builders are stored in a [Registry] so that workflows can look up step
// factories by ID. Use [NewStepBuilder] or [NewWorkflowBuilder] to obtain a
// Builder; implement this interface directly only when building a custom step
// factory.
type Builder interface {
	// Id returns the unique identifier of the step this Builder produces.
	// The ID is used as the registry key and must be non-empty.
	Id() string

	// Validate checks the builder's configuration for consistency and
	// completeness. It is called by Build and also by the workflow builder
	// during assembly. Returns a descriptive error when the configuration is
	// invalid (e.g. missing ID or execute function).
	Validate() error

	// Build constructs and returns the configured Step. Build may be called
	// more than once; each call should return a new, independent Step instance.
	// Returns an error if Validate fails or construction otherwise fails.
	Build() (Step, error)
}

// Registry is a thread-safe store of named [Builder] instances.
//
// It allows workflows to look up step factories by ID rather than by direct
// constructor reference, which is useful for dynamic or configuration-driven
// workflow assembly.
//
// Use [NewRegistry] to obtain an instance.
type Registry interface {
	// Add registers one or more Builders. Returns an error if any Builder's ID
	// is already present; in that case no builders from the batch are added.
	Add(steps ...Builder) error

	// Remove deletes the Builder with the given ID. Returns true if the ID
	// existed and was removed, false if it was not found.
	Remove(id string) bool

	// Has reports whether a Builder with the given ID is registered.
	Has(id string) bool

	// Of returns the Builder registered under id, or nil if not found.
	Of(id string) Builder
}
