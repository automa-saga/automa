package automa

import (
	"sync"
)

// registry is the concrete, thread-safe implementation of the [Registry]
// interface. It stores [Builder] instances keyed by their string ID and
// serialises all access with a sync.RWMutex so that Has/Of (read operations)
// can run concurrently while Add/Remove (write operations) are exclusive.
type registry struct {
	mu    sync.RWMutex
	steps map[string]Builder
}

// NewRegistry returns a new, empty [Registry] backed by an in-memory map.
// The returned Registry is safe for concurrent use.
func NewRegistry() Registry {
	return &registry{
		steps: make(map[string]Builder),
	}
}

// Has reports whether a [Builder] with the given ID is present in the registry.
// It acquires a shared read lock so multiple goroutines may call Has concurrently.
func (r *registry) Has(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.steps[id]
	return exists
}

// Remove deletes the [Builder] registered under id.
// Returns true if the ID existed and was removed; false if it was not found.
// It acquires an exclusive write lock.
func (r *registry) Remove(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.steps[id]; exists {
		delete(r.steps, id)
		return true
	}
	return false
}

// Add registers one or more [Builder] instances. All builders are checked for
// ID conflicts before any are added: if any ID already exists, the method
// returns a [StepAlreadyExists] error and no builders from the batch are
// written. This all-or-nothing semantic prevents partial registration.
// It acquires an exclusive write lock for the duration of the operation.
func (r *registry) Add(steps ...Builder) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range steps {
		id := s.Id()
		if _, exists := r.steps[id]; exists {
			return StepAlreadyExists.New("step with Id %q already exists", id)
		}
		r.steps[id] = s
	}
	return nil
}

// Of returns the [Builder] registered under id, or nil if no such Builder
// exists. It acquires a shared read lock so multiple goroutines may call Of
// concurrently.
func (r *registry) Of(id string) Builder {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.steps[id]
}
