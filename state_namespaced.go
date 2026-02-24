package automa

import (
	"sync"
)

// SyncNamespacedStateBag is a thread-safe implementation of NamespacedStateBag.
// It maintains separate StateBag instances for local, global, and custom namespaces.
type SyncNamespacedStateBag struct {
	local  StateBag
	global StateBag
	custom map[string]StateBag
	mu     sync.RWMutex
}

// NewNamespacedStateBag creates a new SyncNamespacedStateBag with the given local and global bags.
// If local is nil, a new empty SyncStateBag is created.
// If global is nil, a new empty SyncStateBag is created.
func NewNamespacedStateBag(local, global StateBag) *SyncNamespacedStateBag {
	if global == nil {
		global = &SyncStateBag{}
	}

	return &SyncNamespacedStateBag{
		local:  local, // could be nil, will be lazily initialized
		global: global,
		custom: make(map[string]StateBag),
	}
}

// Local returns a view of the local namespace.
func (n *SyncNamespacedStateBag) Local() StateBag {
	if n.local == nil {
		n.mu.Lock()
		if n.local == nil { // double-check locking
			n.local = &SyncStateBag{}
		}
		n.mu.Unlock()
	}
	return n.local
}

// Global returns a view of the global namespace.
func (n *SyncNamespacedStateBag) Global() StateBag {
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

	// Read from other first (without holding n's lock)
	otherLocal := other.Local()
	otherGlobal := other.Global()

	var otherCustom map[string]StateBag
	if otherSync, ok := other.(*SyncNamespacedStateBag); ok {
		otherSync.mu.RLock()
		// Clone the map to avoid holding lock during merge
		otherCustom = make(map[string]StateBag, len(otherSync.custom))
		for name, bag := range otherSync.custom {
			otherCustom[name] = bag
		}
		otherSync.mu.RUnlock()
	}

	// Now lock n and perform merges
	n.mu.Lock()
	defer n.mu.Unlock()

	// Merge local namespaces (use direct field access)
	if n.local == nil {
		n.local = &SyncStateBag{}
	}

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
