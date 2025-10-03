package automa

import (
	"context"
	"sync"
)

const (
	// KeyState is the context key for storing StateBag.
	KeyState      = "automa_state_bag"
	KeyStep       = "automa_step"
	KeyId         = "automa_id"
	KeyIsWorkflow = "automa_is_workflow"
	KeyStartTime  = "automa_start_time"
	KeyEndTime    = "automa_end_time"
	KeyReport     = "automa_report"
)

// SyncStateBag is a thread-safe implementation of StateBag.
type SyncStateBag struct {
	m sync.Map
}

func (s *SyncStateBag) Get(key string) (interface{}, bool) {
	return s.m.Load(key)
}

func (s *SyncStateBag) Set(key string, value interface{}) StateBag {
	s.m.Store(key, value)
	return s
}

func (s *SyncStateBag) Delete(key string) StateBag {
	s.m.Delete(key)
	return s
}

func (s *SyncStateBag) Clear() StateBag {
	s.m.Range(func(key, _ interface{}) bool {
		s.m.Delete(key)
		return true
	})

	return s
}

func (s *SyncStateBag) Keys() []string {
	keys := []string{}
	s.m.Range(func(key, _ interface{}) bool {
		if k, ok := key.(string); ok {
			keys = append(keys, k)
		}
		return true
	})
	return keys
}

func (s *SyncStateBag) Size() int {
	count := 0
	s.m.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// ContextWithStateBag returns a context with the given StateBag.
func ContextWithStateBag(ctx context.Context, stateBag StateBag) context.Context {
	return context.WithValue(ctx, KeyState, stateBag)
}

// GetStateBagFromContext retrieves the StateBag from context.
func GetStateBagFromContext(ctx context.Context) StateBag {
	if ctx != nil {
		if stateBag, ok := ctx.Value(KeyState).(StateBag); ok {
			return stateBag
		}
	}
	return &SyncStateBag{}
}

// Generic getter for simple types.
func getFromState[T any](state StateBag, key string, zero T) T {
	if state != nil {
		if val, ok := state.Get(key); ok {
			if typedVal, ok := val.(T); ok {
				return typedVal
			}
		}
	}
	return zero
}

// Typed getters.
func GetStringFromState(state StateBag, key string) string {
	return getFromState[string](state, key, "")
}
func GetIntFromState(state StateBag, key string) int {
	return getFromState[int](state, key, 0)
}
func GetBoolFromState(state StateBag, key string) bool {
	return getFromState[bool](state, key, false)
}
func GetFloatFromState(state StateBag, key string) float64 {
	return getFromState[float64](state, key, 0.0)
}

// Generic getter for slices.
func getSliceFromState[T any](state StateBag, key string) []T {
	if state != nil {
		if val, ok := state.Get(key); ok {
			if sliceVal, ok := val.([]T); ok {
				return sliceVal
			}
		}
	}
	return []T{}
}

func GetStringSliceFromState(state StateBag, key string) []string {
	return getSliceFromState[string](state, key)
}
func GetIntSliceFromState(state StateBag, key string) []int {
	return getSliceFromState[int](state, key)
}
func GetBoolSliceFromState(state StateBag, key string) []bool {
	return getSliceFromState[bool](state, key)
}
func GetFloatSliceFromState(state StateBag, key string) []float64 {
	return getSliceFromState[float64](state, key)
}

// Generic getter for maps.
func getMapFromState[K comparable, V any](state StateBag, key string) map[K]V {
	if state != nil {
		if val, ok := state.Get(key); ok {
			if mapVal, ok := val.(map[K]V); ok {
				return mapVal
			}
		}
	}
	return map[K]V{}
}

func GetStringMapFromState(state StateBag, key string) map[string]string {
	return getMapFromState[string, string](state, key)
}
func GetIntMapFromState(state StateBag, key string) map[string]int {
	return getMapFromState[string, int](state, key)
}
func GetBoolMapFromState(state StateBag, key string) map[string]bool {
	return getMapFromState[string, bool](state, key)
}
func GetFloatMapFromState(state StateBag, key string) map[string]float64 {
	return getMapFromState[string, float64](state, key)
}
