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
	mu     sync.RWMutex // protects custom map
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

	// Clone local namespace
	localClone, err := n.Local().Clone() // use Local() to ensure lazy initialization
	if err != nil {
		return nil, err
	}

	// Clone global namespace
	globalClone, err := n.global.Clone()
	if err != nil {
		return nil, err
	}

	// Clone custom namespaces
	customClone := make(map[string]StateBag)
	n.mu.RLock()
	for name, bag := range n.custom {
		clonedBag, err := bag.Clone()
		if err != nil {
			n.mu.RUnlock()
			return nil, err
		}
		customClone[name] = clonedBag
	}
	n.mu.RUnlock()

	return &SyncNamespacedStateBag{
		local:  localClone,
		global: globalClone,
		custom: customClone,
	}, nil
}

// Merge merges another NamespacedStateBag into this one and returns itself.
// It merges local, global, and custom namespaces separately.
func (n *SyncNamespacedStateBag) Merge(other NamespacedStateBag) NamespacedStateBag {
	if other == nil {
		return n
	}

	// Merge local namespaces
	n.Local().Merge(other.Local()) // use Local() to ensure lazy initialization

	// Merge global namespaces
	n.global.Merge(other.Global())

	// Merge custom namespaces
	if otherSync, ok := other.(*SyncNamespacedStateBag); ok {
		otherSync.mu.RLock()
		defer otherSync.mu.RUnlock()

		n.mu.Lock()
		defer n.mu.Unlock()

		for name, otherBag := range otherSync.custom {
			if existingBag, exists := n.custom[name]; exists {
				// Merge with existing custom namespace
				existingBag.Merge(otherBag)
			} else {
				// Add new custom namespace (clone to avoid sharing reference)
				if clonedBag, err := otherBag.Clone(); err == nil {
					n.custom[name] = clonedBag
				}
			}
		}
	}

	return n
}
