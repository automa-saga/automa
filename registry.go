package automa

import (
	"sync"
)

type registry struct {
	mu    sync.RWMutex
	steps map[string]Builder
}

func NewRegistry() Registry {
	return &registry{
		steps: make(map[string]Builder),
	}
}

func (r *registry) Has(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.steps[id]
	return exists
}

func (r *registry) Remove(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.steps[id]; exists {
		delete(r.steps, id)
		return true
	}
	return false
}

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

func (r *registry) Of(id string) Builder {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.steps[id]
}
