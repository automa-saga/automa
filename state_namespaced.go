package automa

import (
	"encoding/json"
	"sync"

	"gopkg.in/yaml.v3"
)

// SyncNamespacedStateBag is a thread-safe implementation of NamespacedStateBag
// that partitions state into three kinds of namespace:
//
//   - Local — private to a single step; each step receives its own local bag
//     so writes in one step cannot accidentally overwrite another step's data.
//   - Global — shared across all steps in a workflow; mutations are visible to
//     every subsequent step that reads from the same global bag.
//   - Custom — an arbitrary set of named bags (e.g. "database-1", "cache"),
//     useful when a reusable step implementation needs a stable, collision-free
//     namespace regardless of how many instances are running.
//
// Concurrency model:
//   - n.mu (sync.RWMutex) protects the three pointer fields (local, global,
//     custom map) and the custom map itself.
//   - Read operations on the pointers (Local, Global) use a fast-path RLock
//     and upgrade to a write lock only when lazy initialization is needed.
//   - Write operations (WithNamespace, Merge, UnmarshalJSON, UnmarshalYAML,
//     MarshalJSON, MarshalYAML) acquire the write lock for the duration.
//   - The individual StateBag instances (local, global, custom entries) are
//     themselves thread-safe SyncStateBag values; their own internal lock
//     serialises Set/Get/Delete calls.
//
// Zero value: a zero-value SyncNamespacedStateBag is safe to use without
// explicit construction. All fields are lazily initialized on first access via
// initLocked, so the following is valid:
//
//	var ns automa.SyncNamespacedStateBag
//	ns.Local().Set("key", "value")
type SyncNamespacedStateBag struct {
	local  StateBag
	global StateBag
	custom map[string]StateBag
	mu     sync.RWMutex // protects local, global, and the custom map
}

// NewNamespacedStateBag constructs a SyncNamespacedStateBag with explicit
// initial local and global bags. Either argument may be nil, in which case an
// empty *SyncStateBag is substituted. The custom namespace map is always
// initialized to an empty map so that WithNamespace can write without a nil-map
// panic.
//
// Example:
//
//	// Share a common global bag between steps while giving each step its own local bag.
//	global := automa.NewStateBag()
//	global.Set("env", "production")
//	ns := automa.NewNamespacedStateBag(nil, global)
func NewNamespacedStateBag(local, global StateBag) *SyncNamespacedStateBag {
	if global == nil {
		global = &SyncStateBag{}
	}
	if local == nil {
		local = &SyncStateBag{}
	}

	return &SyncNamespacedStateBag{
		local:  local,
		global: global,
		custom: make(map[string]StateBag),
	}
}

// initLocked lazily allocates nil fields so the zero value is always usable.
// Caller MUST hold n.mu.Lock() before calling this method.
// It is safe to call repeatedly; subsequent calls are no-ops for already-set fields.
func (n *SyncNamespacedStateBag) initLocked() {
	if n.local == nil {
		n.local = &SyncStateBag{}
	}
	if n.global == nil {
		n.global = &SyncStateBag{}
	}
	if n.custom == nil {
		n.custom = make(map[string]StateBag)
	}
}

// Local returns the StateBag for the local namespace.
//
// The local bag is private to the step that owns this SyncNamespacedStateBag.
// Writes to it are never visible to other steps, even when they share the same
// global bag.
//
// Thread-safety: a fast-path read lock checks whether the field is already
// initialized. Only if it is nil (zero-value receiver) is the lock upgraded to
// a write lock for initialization. After initialization the same bag pointer is
// always returned.
func (n *SyncNamespacedStateBag) Local() StateBag {
	// Fast path: already initialized.
	n.mu.RLock()
	l := n.local
	n.mu.RUnlock()
	if l != nil {
		return l
	}
	// Slow path: initialize under write lock and return.
	n.mu.Lock()
	n.initLocked()
	l = n.local
	n.mu.Unlock()
	return l
}

// Global returns the StateBag for the global namespace.
//
// The global bag is shared across all steps in a workflow. A step can publish
// data (e.g. configuration, counters) by writing to Global(), and any
// subsequent step can read it. Because the bag is shared, concurrent writes
// from different goroutines are serialised by SyncStateBag's own mutex.
//
// Thread-safety: same fast-path/slow-path pattern as Local().
func (n *SyncNamespacedStateBag) Global() StateBag {
	// Fast path: already initialized.
	n.mu.RLock()
	g := n.global
	n.mu.RUnlock()
	if g != nil {
		return g
	}
	// Slow path: initialize under write lock and return.
	n.mu.Lock()
	n.initLocked()
	g = n.global
	n.mu.Unlock()
	return g
}

// WithNamespace returns the StateBag for the named custom namespace, creating
// it on demand if it does not already exist.
//
// Custom namespaces are useful for step implementations that are instantiated
// multiple times in the same workflow (e.g. "setup-bind-mount-for-app1" and
// "setup-bind-mount-for-app2"). By writing to WithNamespace(name) each
// instance gets an isolated bag without the caller needing to worry about key
// collisions with other instances.
//
// The returned bag is stable: repeated calls with the same name always return
// the same underlying StateBag pointer.
//
// Thread-safety: the write lock is always acquired to guarantee atomic
// check-then-create semantics.
func (n *SyncNamespacedStateBag) WithNamespace(name string) StateBag {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.initLocked()

	bag, exists := n.custom[name]
	if !exists {
		bag = &SyncStateBag{}
		n.custom[name] = bag
	}

	return bag
}

// Clone returns a fully independent deep copy of the SyncNamespacedStateBag,
// including all three namespace kinds.
//
// Clone strategy:
//  1. A write lock is acquired to snapshot the field pointers and the custom
//     map (so the set of namespace names cannot change mid-clone).
//  2. The lock is released before cloning individual bags to avoid holding n's
//     write lock while the inner SyncStateBag Clone() calls acquire their own
//     locks, which would otherwise risk lock ordering issues.
//  3. Local, global, and each custom bag are deep-copied in sequence via their
//     own Clone() methods.
//
// The returned NamespacedStateBag shares no memory with the original: mutations
// to the clone do not affect the original, and vice versa.
//
// Returns (nil, error) when the receiver is nil or when any inner Clone fails.
func (n *SyncNamespacedStateBag) Clone() (NamespacedStateBag, error) {
	if n == nil {
		return nil, IllegalArgument.New("cannot clone a nil SyncNamespacedStateBag")
	}

	// Snapshot the field references under one lock.
	n.mu.Lock()
	n.initLocked()
	local := n.local
	global := n.global
	customCopy := make(map[string]StateBag, len(n.custom))
	for name, bag := range n.custom {
		customCopy[name] = bag
	}
	n.mu.Unlock()

	// Clone each bag outside the lock — Clone() acquires inner locks.
	localClone, err := local.Clone()
	if err != nil {
		return nil, err
	}

	globalClone, err := global.Clone()
	if err != nil {
		return nil, err
	}

	clonedCustom := make(map[string]StateBag, len(customCopy))
	for name, bag := range customCopy {
		clonedBag, err := bag.Clone()
		if err != nil {
			return nil, err
		}
		clonedCustom[name] = clonedBag
	}

	return &SyncNamespacedStateBag{
		local:  localClone,
		global: globalClone,
		custom: clonedCustom,
	}, nil
}

// Merge merges every namespace from other into n, overwriting conflicting keys,
// and returns n.
//
// Merge semantics per namespace:
//   - Local: other's local keys are merged into n's local bag (other wins on
//     key conflicts).
//   - Global: other's global keys are merged into n's global bag.
//   - Custom: for each custom namespace in other —
//   - If the same namespace already exists in n, the two bags are merged
//     (other wins on key conflicts).
//   - If the namespace is new to n, other's bag is deep-cloned and added so
//     that n and other do not share the same StateBag pointer.
//
// Type constraint: other must be a *SyncNamespacedStateBag. If a different
// implementation is passed, an error is returned immediately so that callers
// are not surprised by silently dropped custom namespaces.
//
// Deadlock prevention: other's field snapshots are read before n's write lock
// is acquired, following the same snapshot-before-lock pattern used by
// SyncStateBag.Merge.
//
// Returns (n, nil) on success.
// Returns (n, nil) unchanged when other is nil.
// Returns (nil, error) on type mismatch or inner merge/clone failure.
func (n *SyncNamespacedStateBag) Merge(other NamespacedStateBag) (NamespacedStateBag, error) {
	if other == nil {
		return n, nil
	}

	// Type check upfront
	otherSync, ok := other.(*SyncNamespacedStateBag)
	if !ok {
		return nil, IllegalArgument.New(
			"cannot merge: other NamespacedStateBag must be *SyncNamespacedStateBag, got %T", other)
	}

	// Read from other first (without holding n's lock).
	// Local() and Global() handle their own lazy initialization.
	otherLocal := otherSync.Local()
	otherGlobal := otherSync.Global()

	otherSync.mu.RLock()
	otherCustom := make(map[string]StateBag, len(otherSync.custom))
	for name, bag := range otherSync.custom {
		otherCustom[name] = bag
	}
	otherSync.mu.RUnlock()

	// Now lock n and perform merges
	n.mu.Lock()
	defer n.mu.Unlock()
	n.initLocked()

	// Merge local namespaces
	mergedLocal, err := n.local.Merge(otherLocal)
	if err != nil {
		return nil, err
	}
	n.local = mergedLocal

	// Merge global namespaces
	mergedGlobal, err := n.global.Merge(otherGlobal)
	if err != nil {
		return nil, err
	}
	n.global = mergedGlobal

	// Merge custom namespaces
	for name, otherBag := range otherCustom {
		if existingBag, exists := n.custom[name]; exists {
			merged, err := existingBag.Merge(otherBag)
			if err != nil {
				return nil, err
			}
			n.custom[name] = merged
		} else {
			clonedBag, err := otherBag.Clone()
			if err != nil {
				return nil, err
			}
			n.custom[name] = clonedBag
		}
	}

	return n, nil
}

// namespacedSnapshot is the wire format used by MarshalJSON, UnmarshalJSON,
// MarshalYAML, and UnmarshalYAML. Each namespace is serialized as a flat
// map[string]interface{} (keys are the string form of Key, values are
// whatever encoding/json or gopkg.in/yaml.v3 produces for the stored value).
type namespacedSnapshot struct {
	Local  map[string]interface{}            `json:"local" yaml:"local"`
	Global map[string]interface{}            `json:"global" yaml:"global"`
	Custom map[string]map[string]interface{} `json:"custom" yaml:"custom"`
}

// MarshalJSON implements json.Marshaler. It serializes all three namespace
// kinds into the namespacedSnapshot wire format:
//
//	{
//	  "local":  { "key": value, … },
//	  "global": { "key": value, … },
//	  "custom": { "ns-name": { "key": value, … }, … }
//	}
//
// A nil receiver marshals as JSON null.
//
// Thread-safety: a write lock is acquired briefly to initialize nil fields and
// snapshot the field pointers. Individual bag marshaling then proceeds outside
// the lock to avoid holding n's write lock while inner SyncStateBag read locks
// are taken.
func (n *SyncNamespacedStateBag) MarshalJSON() ([]byte, error) {
	if n == nil {
		return json.Marshal(nil)
	}

	// Take write lock to initialize if needed, then snapshot atomically.
	n.mu.Lock()
	n.initLocked()
	local := n.local
	global := n.global
	customCopy := make(map[string]StateBag, len(n.custom))
	for name, bag := range n.custom {
		customCopy[name] = bag
	}
	n.mu.Unlock()

	// Build snapshot outside the lock — StateBagToStringMap acquires inner locks.
	snapshot := namespacedSnapshot{
		Local:  StateBagToStringMap(local),
		Global: StateBagToStringMap(global),
		Custom: make(map[string]map[string]interface{}, len(customCopy)),
	}
	for name, bag := range customCopy {
		snapshot.Custom[name] = StateBagToStringMap(bag)
	}

	return json.Marshal(snapshot)
}

// UnmarshalJSON implements json.Unmarshaler. It decodes the namespacedSnapshot
// wire format into the bag, completely replacing all existing namespace
// contents.
//
// Type notes (standard encoding/json behaviour applies to values):
//   - JSON numbers become float64.
//   - JSON objects become map[string]interface{}.
//   - JSON arrays become []interface{}.
//
// Use the typed accessors (Int, String, …) or FromState after unmarshaling
// to coerce float64 numbers back to integer types.
//
// Returns an error if the receiver is nil or if the JSON is malformed.
func (n *SyncNamespacedStateBag) UnmarshalJSON(data []byte) error {
	if n == nil {
		return IllegalArgument.New("cannot unmarshal into nil SyncNamespacedStateBag")
	}
	var snapshot namespacedSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return err
	}

	local := &SyncStateBag{}
	for k, v := range snapshot.Local {
		local.Set(Key(k), v)
	}
	global := &SyncStateBag{}
	for k, v := range snapshot.Global {
		global.Set(Key(k), v)
	}
	custom := make(map[string]StateBag)
	for name, mp := range snapshot.Custom {
		b := &SyncStateBag{}
		for k, v := range mp {
			b.Set(Key(k), v)
		}
		custom[name] = b
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	n.local = local
	n.global = global
	n.custom = custom
	return nil
}

// MarshalYAML implements yaml.Marshaler. It returns a namespacedSnapshot
// struct that the YAML encoder serializes into the same three-section layout
// as MarshalJSON:
//
//	local:
//	  key: value
//	global:
//	  key: value
//	custom:
//	  ns-name:
//	    key: value
//
// A nil receiver returns (nil, nil), which the encoder renders as YAML null.
//
// Thread-safety: same snapshot-then-release pattern as MarshalJSON.
func (n *SyncNamespacedStateBag) MarshalYAML() (interface{}, error) {
	if n == nil {
		return nil, nil
	}

	// Take write lock to initialize if needed, then snapshot atomically.
	n.mu.Lock()
	n.initLocked()
	local := n.local
	global := n.global
	customCopy := make(map[string]StateBag, len(n.custom))
	for name, bag := range n.custom {
		customCopy[name] = bag
	}
	n.mu.Unlock()

	// Build snapshot outside the lock — StateBagToStringMap acquires inner locks.
	snapshot := namespacedSnapshot{
		Local:  StateBagToStringMap(local),
		Global: StateBagToStringMap(global),
		Custom: make(map[string]map[string]interface{}, len(customCopy)),
	}
	for name, bag := range customCopy {
		snapshot.Custom[name] = StateBagToStringMap(bag)
	}

	return snapshot, nil
}

// UnmarshalYAML implements yaml.Unmarshaler. It decodes a YAML mapping node
// in the namespacedSnapshot format into the bag, completely replacing all
// existing namespace contents.
//
// Type notes (gopkg.in/yaml.v3 behaviour applies to values):
//   - YAML integers decode as int.
//   - YAML floats decode as float64.
//   - YAML booleans decode as bool.
//
// Returns an error if the receiver is nil or if the YAML node cannot be
// decoded into the expected structure.
func (n *SyncNamespacedStateBag) UnmarshalYAML(node *yaml.Node) error {
	if n == nil {
		return IllegalArgument.New("cannot unmarshal into nil SyncNamespacedStateBag")
	}
	var snapshot namespacedSnapshot
	if err := node.Decode(&snapshot); err != nil {
		return err
	}

	local := &SyncStateBag{}
	for k, v := range snapshot.Local {
		local.Set(Key(k), v)
	}
	global := &SyncStateBag{}
	for k, v := range snapshot.Global {
		global.Set(Key(k), v)
	}
	custom := make(map[string]StateBag)
	for name, mp := range snapshot.Custom {
		b := &SyncStateBag{}
		for k, v := range mp {
			b.Set(Key(k), v)
		}
		custom[name] = b
	}

	n.mu.Lock()
	defer n.mu.Unlock()
	n.local = local
	n.global = global
	n.custom = custom
	return nil
}
