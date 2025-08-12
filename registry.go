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

func (r *registry) Add(steps ...Builder) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range steps {
		id := s.Id()
		if _, exists := r.steps[id]; exists {
			return StepAlreadyExists.New("step with id %q already exists", id)
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
