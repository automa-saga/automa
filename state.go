package automa

import (
	"context"
	"reflect"
	"sync"

	"github.com/joomcode/errorx"
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

// Clone creates a deep copy of the SyncStateBag if all items implement Clone method and returns deep copy when invoked.
// If any item does not implement Cloner or Clone method, it performs a shallow copy for that item.
func (s *SyncStateBag) Clone() (StateBag, error) {
	if s == nil {
		return nil, IllegalArgument.New("cannot clone a nil SyncStateBag")
	}

	clone := &SyncStateBag{}
	errInterface := reflect.TypeOf((*error)(nil)).Elem()

	for k, v := range s.Items() {
		if v == nil {
			clone.m.Store(k, nil)
			continue
		}

		rv := reflect.ValueOf(v)
		m := rv.MethodByName("Clone")
		if m.IsValid() && m.Type().NumIn() == 0 {
			outCount := m.Type().NumOut()

			// support Clone() (value, error): Cloner interface
			if outCount == 2 && m.Type().Out(1).Implements(errInterface) {
				results := m.Call([]reflect.Value{})
				// check error
				if !results[1].IsNil() {
					errVal := results[1].Interface().(error)
					return nil, errorx.IllegalState.Wrap(errVal, "failed to clone value for key %v", k)
				}
				clone.m.Store(k, results[0].Interface())
				continue
			}

			// support Clone() value: if any other clone signature without error
			if outCount == 1 {
				results := m.Call([]reflect.Value{})
				clone.m.Store(k, results[0].Interface())
				continue
			}
		}

		// fallback: shallow copy
		clone.m.Store(k, v)
	}

	return clone, nil
}

func (s *SyncStateBag) Merge(other StateBag) (StateBag, error) {
	if other == nil {
		return s, nil
	}

	for k, v := range other.Items() {
		s.m.Store(k, v)
	}

	return s, nil
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

// String retrieves a string value from the StateBag for the given key.
// If the key does not exist or the value is not a string, it returns an empty string ("").
func (s *SyncStateBag) String(key Key) string {
	return FromState[string](s, key, "")
}

// Bool retrieves a bool value from the StateBag for the given key.
// If the key does not exist or the value is not a bool, it returns false.
func (s *SyncStateBag) Bool(key Key) bool {
	return FromState[bool](s, key, false)
}

// Int retrieves an int value from the StateBag for the given key.
// If the key does not exist or the value is not an int, it returns 0.
func (s *SyncStateBag) Int(key Key) int {
	return FromState[int](s, key, 0)
}

// Int8 retrieves an int8 value from the StateBag for the given key.
// If the key does not exist or the value is not an int8, it returns 0.
func (s *SyncStateBag) Int8(key Key) int8 {
	return FromState[int8](s, key, 0)
}

// Int16 retrieves an int16 value from the StateBag for the given key.
// If the key does not exist or the value is not an int16, it returns 0.
func (s *SyncStateBag) Int16(key Key) int16 {
	return FromState[int16](s, key, 0)
}

// Int32 retrieves an int32 value from the StateBag for the given key.
// If the key does not exist or the value is not an int32, it returns 0.
func (s *SyncStateBag) Int32(key Key) int32 {
	return FromState[int32](s, key, 0)
}

// Int64 retrieves an int64 value from the StateBag for the given key.
// If the key does not exist or the value is not an int64, it returns 0.
func (s *SyncStateBag) Int64(key Key) int64 {
	return FromState[int64](s, key, 0)
}

// Float retrieves a float64 value from the StateBag for the given key.
// If the key does not exist or the value is not a float64, it returns 0.0.
func (s *SyncStateBag) Float(key Key) float64 {
	return FromState[float64](s, key, 0.0)
}

// Float32 retrieves a float32 value from the StateBag for the given key.
// If the key does not exist or the value is not a float32, it returns 0.0.
func (s *SyncStateBag) Float32(key Key) float32 {
	return FromState[float32](s, key, 0.0)
}

// Float64 retrieves a float64 value from the StateBag for the given key.
// If the key does not exist or the value is not a float64, it returns 0.0.
func (s *SyncStateBag) Float64(key Key) float64 {
	return FromState[float64](s, key, 0.0)
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
