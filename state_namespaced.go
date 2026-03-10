package automa

import (
	"encoding/json"
	"sync"

	"gopkg.in/yaml.v3"
)

// SyncNamespacedStateBag is a thread-safe implementation of NamespacedStateBag.
// It maintains separate StateBag instances for local, global, and custom
// namespaces, and lazily initializes missing internal state so the zero value is usable.
type SyncNamespacedStateBag struct {
	local  StateBag
	global StateBag
	custom map[string]StateBag
	mu     sync.RWMutex // protects local, global, and custom fields
}

// NewNamespacedStateBag creates a new SyncNamespacedStateBag with the given local and global bags.
// If local or global is nil, they are initialized to empty SyncStateBag values.
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

// initLocked initializes nil internal fields. Caller MUST hold n.mu.Lock().
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

// Local returns the local namespace. The zero value of SyncNamespacedStateBag is safe to use.
// The method uses a fast-path read lock; if local is nil (zero value), it upgrades to a
// write lock, initializes, and returns the newly created bag.
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

// Global returns the global namespace. The zero value of SyncNamespacedStateBag is safe to use.
// The method uses a fast-path read lock; if global is nil (zero value), it upgrades to a
// write lock, initializes, and returns the newly created bag.
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

// WithNamespace returns a view of a custom namespace.
// Custom namespaces are created on-demand if they don't exist.
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

// Clone creates a deep copy of the SyncNamespacedStateBag including all namespaces.
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

// Merge merges another NamespacedStateBag into this one and returns itself.
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

// marshal snapshot structure
type namespacedSnapshot struct {
	Local  map[string]interface{}            `json:"local" yaml:"local"`
	Global map[string]interface{}            `json:"global" yaml:"global"`
	Custom map[string]map[string]interface{} `json:"custom" yaml:"custom"`
}

// MarshalJSON implements json.Marshaler for SyncNamespacedStateBag.
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

// UnmarshalJSON implements json.Unmarshaler for SyncNamespacedStateBag.
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

// MarshalYAML implements yaml.Marshaler for SyncNamespacedStateBag.
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

// UnmarshalYAML implements yaml.Unmarshaler for SyncNamespacedStateBag.
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
