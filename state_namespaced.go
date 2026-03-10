package automa

import (
	"encoding/json"
	"sync"

	"gopkg.in/yaml.v3"
)

// SyncNamespacedStateBag is a thread-safe implementation of NamespacedStateBag.
// local is eagerly initialized to avoid concurrent writes during marshaling.
type SyncNamespacedStateBag struct {
	local  StateBag
	global StateBag
	custom map[string]StateBag
	mu     sync.RWMutex // protects local, global, and custom fields during writes (Merge)
}

// NewNamespacedStateBag creates a new SyncNamespacedStateBag with the given local and global bags.
// If local is nil, it is eagerly initialized to avoid concurrent initialization races.
func NewNamespacedStateBag(local, global StateBag) *SyncNamespacedStateBag {
	if global == nil {
		global = &SyncStateBag{}
	}
	if local == nil {
		local = &SyncStateBag{} // eager initialize to avoid races with readers
	}

	return &SyncNamespacedStateBag{
		local:  local,
		global: global,
		custom: make(map[string]StateBag),
	}
}

// Local returns a view of the local namespace.
func (n *SyncNamespacedStateBag) Local() StateBag {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.local
}

// Global returns a view of the global namespace.
func (n *SyncNamespacedStateBag) Global() StateBag {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.global
}

// WithNamespace returns a view of a custom namespace.
// Custom namespaces are created on-demand if they don't exist.
func (n *SyncNamespacedStateBag) WithNamespace(name string) StateBag {
	n.mu.Lock()
	defer n.mu.Unlock()

	bag, exists := n.custom[name]
	if !exists {
		bag = &SyncStateBag{}
		n.custom[name] = bag
	}

	return bag
}

// Clone creates a deep copy of the SyncNamespacedStateBag including all namespaces.
// This clones the local, global, and all custom namespaces.
func (n *SyncNamespacedStateBag) Clone() (NamespacedStateBag, error) {
	if n == nil {
		return nil, IllegalArgument.New("cannot clone a nil SyncNamespacedStateBag")
	}

	n.mu.RLock()
	defer n.mu.RUnlock()

	// Clone local namespace (use direct field access, not Local() to avoid lock)
	var localClone StateBag
	var err error
	if n.local != nil {
		localClone, err = n.local.Clone()
		if err != nil {
			return nil, err
		}
	} else {
		localClone = &SyncStateBag{} // create empty if nil
	}

	// Clone global namespace
	globalClone, err := n.global.Clone()
	if err != nil {
		return nil, err
	}

	// Clone custom namespaces
	customClone := make(map[string]StateBag)
	for name, bag := range n.custom {
		clonedBag, err := bag.Clone()
		if err != nil {
			return nil, err
		}
		customClone[name] = clonedBag
	}

	return &SyncNamespacedStateBag{
		local:  localClone,
		global: globalClone,
		custom: customClone,
	}, nil
}

// Merge merges another NamespacedStateBag into this one and returns itself.
// It merges local, global, and custom namespaces separately.
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

	// Read from other first (without holding n's lock)
	otherLocal := otherSync.Local()
	otherGlobal := otherSync.Global()

	otherSync.mu.RLock()
	// Clone the map to avoid holding lock during merge
	otherCustom := make(map[string]StateBag, len(otherSync.custom))
	for name, bag := range otherSync.custom {
		otherCustom[name] = bag
	}
	otherSync.mu.RUnlock()

	// Now lock n and perform merges
	n.mu.Lock()
	defer n.mu.Unlock()

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

	// snapshot under read lock
	n.mu.RLock()
	defer n.mu.RUnlock()

	snapshot := namespacedSnapshot{
		Local:  StateBagToStringMap(n.local),
		Global: StateBagToStringMap(n.global),
		Custom: make(map[string]map[string]interface{}),
	}

	for name, bag := range n.custom {
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

	// construct new internal structures from snapshot
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

	// apply under lock
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

	n.mu.RLock()
	defer n.mu.RUnlock()

	snapshot := namespacedSnapshot{
		Local:  StateBagToStringMap(n.local),
		Global: StateBagToStringMap(n.global),
		Custom: make(map[string]map[string]interface{}),
	}

	for name, bag := range n.custom {
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
