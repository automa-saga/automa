package automa

import (
	"context"
	"sync"
)

// Key is an exported type for context keys to avoid collisions.
type Key string

const (
	// KeyState is the context key for storing StateBag.
	KeyState      Key = "automa_state_bag"
	KeyStep       Key = "automa_step"
	KeyId         Key = "automa_id"
	KeyIsWorkflow Key = "automa_is_workflow"
	KeyStartTime  Key = "automa_start_time"
	KeyEndTime    Key = "automa_end_time"
	KeyReport     Key = "automa_report"
)

// SyncStateBag is a thread-safe implementation of StateBag.
type SyncStateBag struct {
	m sync.Map
}

func (s *SyncStateBag) Clone() StateBag {
	clone := &SyncStateBag{}
	s.m.Range(func(key, value interface{}) bool {
		clone.m.Store(key, value)
		return true
	})

	return clone
}

func (s *SyncStateBag) Merge(other StateBag) StateBag {
	if other == nil {
		return s
	}

	for k, v := range other.Items() {
		s.m.Store(k, v)
	}

	return s
}

func (s *SyncStateBag) Items() map[Key]interface{} {
	items := make(map[Key]interface{})
	s.m.Range(func(key, value interface{}) bool {
		if k, ok := key.(Key); ok {
			items[k] = value
		}
		return true
	})

	return items
}

func (s *SyncStateBag) Get(key Key) (interface{}, bool) {
	return s.m.Load(key)
}

func (s *SyncStateBag) Set(key Key, value interface{}) StateBag {
	s.m.Store(key, value)
	return s
}

func (s *SyncStateBag) Delete(key Key) StateBag {
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

func (s *SyncStateBag) Keys() []Key {
	var keys []Key
	s.m.Range(func(key, _ interface{}) bool {
		if k, ok := key.(Key); ok {
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

// ContextWithState returns a context with the given StateBag.
func ContextWithState(ctx context.Context, stateBag StateBag) context.Context {
	return context.WithValue(ctx, KeyState, stateBag)
}

// StateFromContext retrieves the StateBag from context.
func StateFromContext(ctx context.Context) StateBag {
	if ctx != nil {
		if stateBag, ok := ctx.Value(KeyState).(StateBag); ok {
			return stateBag
		}
	}
	return &SyncStateBag{}
}

func FromState[T any](state StateBag, key Key, zero T) T {
	if state != nil {
		if val, ok := state.Get(key); ok {
			if typedVal, ok := val.(T); ok {
				return typedVal
			}
		}
	}
	return zero
}

func StringFromState(state StateBag, key Key) string {
	return FromState[string](state, key, "")
}
func IntFromState(state StateBag, key Key) int {
	return FromState[int](state, key, 0)
}
func BoolFromState(state StateBag, key Key) bool {
	return FromState[bool](state, key, false)
}
func FloatFromState(state StateBag, key Key) float64 {
	return FromState[float64](state, key, 0.0)
}

func SliceFromState[T any](state StateBag, key Key) []T {
	if state != nil {
		if val, ok := state.Get(key); ok {
			if sliceVal, ok := val.([]T); ok {
				return sliceVal
			}
		}
	}
	return []T{}
}

func StringSliceFromState(state StateBag, key Key) []string {
	return SliceFromState[string](state, key)
}
func IntSliceFromState(state StateBag, key Key) []int {
	return SliceFromState[int](state, key)
}
func BoolSliceFromState(state StateBag, key Key) []bool {
	return SliceFromState[bool](state, key)
}
func FloatSliceFromState(state StateBag, key Key) []float64 {
	return SliceFromState[float64](state, key)
}

func MapFromState[K comparable, V any](state StateBag, key Key) map[K]V {
	if state != nil {
		if val, ok := state.Get(key); ok {
			if mapVal, ok := val.(map[K]V); ok {
				return mapVal
			}
		}
	}
	return map[K]V{}
}

func StringMapFromState(state StateBag, key Key) map[string]string {
	return MapFromState[string, string](state, key)
}
func IntMapFromState(state StateBag, key Key) map[string]int {
	return MapFromState[string, int](state, key)
}
func BoolMapFromState(state StateBag, key Key) map[string]bool {
	return MapFromState[string, bool](state, key)
}
func FloatMapFromState(state StateBag, key Key) map[string]float64 {
	return MapFromState[string, float64](state, key)
}
