package automa

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"sync"

	"github.com/joomcode/errorx"
	"gopkg.in/yaml.v3"
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

// itemsStringMap returns a copy of the internal items with string keys suitable for JSON/YAML marshalling.
func (s *SyncStateBag) itemsStringMap() map[string]interface{} {
	out := make(map[string]interface{})
	if s == nil {
		return out
	}
	s.m.Range(func(k, v interface{}) bool {
		if key, ok := k.(Key); ok {
			out[string(key)] = v
		}
		return true
	})
	return out
}

// MarshalJSON implements json.Marshaler for SyncStateBag.
func (s *SyncStateBag) MarshalJSON() ([]byte, error) {
	if s == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(s.itemsStringMap())
}

// UnmarshalJSON implements json.Unmarshaler for SyncStateBag.
func (s *SyncStateBag) UnmarshalJSON(data []byte) error {
	if s == nil {
		return IllegalArgument.New("cannot unmarshal into nil SyncStateBag")
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	// clear current contents
	s.Clear()
	for k, v := range m {
		s.m.Store(Key(k), v)
	}
	return nil
}

// MarshalYAML implements yaml.Marshaler for SyncStateBag.
func (s *SyncStateBag) MarshalYAML() (interface{}, error) {
	if s == nil {
		return nil, nil
	}
	return s.itemsStringMap(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler for SyncStateBag.
func (s *SyncStateBag) UnmarshalYAML(node *yaml.Node) error {
	if s == nil {
		return IllegalArgument.New("cannot unmarshal into nil SyncStateBag")
	}
	var m map[string]interface{}
	if err := node.Decode(&m); err != nil {
		return err
	}
	s.Clear()
	for k, v := range m {
		s.m.Store(Key(k), v)
	}
	return nil
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

// normalizeFromState dereferences pointers, decodes yaml.Node/json.Number,
// and converts YAML-style maps recursively to map[string]interface{}.
func normalizeFromState(v interface{}) interface{} {
	// Handle yaml.Node early (both pointer and value) to avoid pointer deref consuming it
	if nodePtr, ok := v.(*yaml.Node); ok && nodePtr != nil {
		var out interface{}
		_ = nodePtr.Decode(&out)
		if out != nil {
			return normalizeFromState(out)
		}
	}
	if nodeVal, ok := v.(yaml.Node); ok {
		var out interface{}
		_ = (&nodeVal).Decode(&out)
		if out != nil {
			return normalizeFromState(out)
		}
	}

	// dereference pointers
	for {
		rv := reflect.ValueOf(v)
		if !rv.IsValid() {
			return nil
		}
		if rv.Kind() == reflect.Ptr {
			if rv.IsNil() {
				return nil
			}
			v = rv.Elem().Interface()
			continue
		}
		break
	}

	// json.Number -> int64 or float64 or string
	if jn, ok := v.(json.Number); ok {
		if i, err := jn.Int64(); err == nil {
			return i
		}
		if f, err := jn.Float64(); err == nil {
			return f
		}
		return jn.String()
	}

	// recursively convert YAML map[interface{}]interface{} -> map[string]interface{}
	switch t := v.(type) {
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, vv := range t {
			var ks string
			switch kk := k.(type) {
			case string:
				ks = kk
			default:
				ks = stringify(k)
			}
			out[ks] = normalizeFromState(vv)
		}
		return out
	case map[string]interface{}:
		m := make(map[string]interface{}, len(t))
		for k, vv := range t {
			m[k] = normalizeFromState(vv)
		}
		return m
	case []interface{}:
		for i := range t {
			t[i] = normalizeFromState(t[i])
		}
		return t
	default:
		return v
	}
}

func stringify(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case int, int8, int16, int32, int64:
		if i, ok := toInt64(t); ok {
			return strconv.FormatInt(i, 10)
		}
	case uint, uint8, uint16, uint32, uint64:
		if f, ok := toFloat64(t); ok {
			if floatIsIntegral(f) {
				return strconv.FormatInt(int64(f), 10)
			}
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
	case float32, float64:
		if f, ok := toFloat64(t); ok {
			if floatIsIntegral(f) {
				return strconv.FormatInt(int64(f), 10)
			}
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
	}
	return ""
}

const eps = 1e-9

func floatIsIntegral(f float64) bool {
	_, frac := math.Modf(f)
	return math.Abs(frac) < eps
}

func toInt64(v interface{}) (int64, bool) {
	switch t := v.(type) {
	case int:
		return int64(t), true
	case int8:
		return int64(t), true
	case int16:
		return int64(t), true
	case int32:
		return int64(t), true
	case int64:
		return t, true
	case uint:
		return int64(t), true
	case uint8:
		return int64(t), true
	case uint16:
		return int64(t), true
	case uint32:
		return int64(t), true
	case uint64:
		return int64(t), true
	case float32:
		f := float64(t)
		return int64(math.Trunc(f)), true
	case float64:
		return int64(math.Trunc(t)), true
	case string:
		if i, err := strconv.ParseInt(t, 10, 64); err == nil {
			return i, true
		}
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return int64(math.Trunc(f)), true
		}
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i, true
		}
		if f, err := t.Float64(); err == nil {
			return int64(math.Trunc(f)), true
		}
	}
	return 0, false
}

func toFloat64(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int8:
		return float64(t), true
	case int16:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case uint:
		return float64(t), true
	case uint8:
		return float64(t), true
	case uint16:
		return float64(t), true
	case uint32:
		return float64(t), true
	case uint64:
		return float64(t), true
	case string:
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return f, true
		}
	case json.Number:
		if f, err := t.Float64(); err == nil {
			return f, true
		}
	}
	return 0, false
}

func toBool(v interface{}) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		if b, err := strconv.ParseBool(t); err == nil {
			return b, true
		}
	case int, int8, int16, int32, int64:
		if i, ok := toInt64(t); ok {
			return i != 0, true
		}
	case uint, uint8, uint16, uint32, uint64:
		if f, ok := toFloat64(t); ok {
			return f != 0, true
		}
	case float32, float64:
		if f, ok := toFloat64(t); ok {
			return f != 0, true
		}
	}
	return false, false
}

func isString(v interface{}) bool {
	_, ok := v.(string)
	return ok
}

// Updated generic FromState which normalizes and attempts primitive coercions.
func FromState[T any](state StateBag, key Key, zero T) T {
	if state == nil {
		return zero
	}
	val, ok := state.Get(key)
	if !ok {
		return zero
	}

	v := normalizeFromState(val)

	// If the normalized value is a string, allow coercion to numeric or bool when requested.
	if s, ok := v.(string); ok {
		switch any(zero).(type) {
		case string:
			return any(s).(T)
		case bool:
			if b, err := strconv.ParseBool(s); err == nil {
				return any(b).(T)
			}
		case int, int8, int16, int32, int64:
			if i, err := strconv.ParseInt(s, 10, 64); err == nil {
				switch any(zero).(type) {
				case int:
					return any(int(i)).(T)
				case int8:
					return any(int8(i)).(T)
				case int16:
					return any(int16(i)).(T)
				case int32:
					return any(int32(i)).(T)
				case int64:
					return any(i).(T)
				}
			}
			// fallthrough: try parse float if int parse failed
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				switch any(zero).(type) {
				case float32:
					return any(float32(f)).(T)
				case float64:
					return any(f).(T)
				}
			}
		case float32, float64:
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				switch any(zero).(type) {
				case float32:
					return any(float32(f)).(T)
				case float64:
					return any(f).(T)
				}
			}
		}
		// if parsing fails, continue to non-string handling to attempt other coercions
	}

	// Try primitive target coercions based on zero's dynamic type for non-string values.
	switch any(zero).(type) {
	case int:
		if i, ok := toInt64(v); ok {
			return any(int(i)).(T)
		}
	case int8:
		if i, ok := toInt64(v); ok {
			return any(int8(i)).(T)
		}
	case int16:
		if i, ok := toInt64(v); ok {
			return any(int16(i)).(T)
		}
	case int32:
		if i, ok := toInt64(v); ok {
			return any(int32(i)).(T)
		}
	case int64:
		if i, ok := toInt64(v); ok {
			return any(i).(T)
		}
	case uint:
		if f, ok := toFloat64(v); ok {
			return any(uint(f)).(T)
		}
	case uint8:
		if f, ok := toFloat64(v); ok {
			return any(uint8(f)).(T)
		}
	case uint16:
		if f, ok := toFloat64(v); ok {
			return any(uint16(f)).(T)
		}
	case uint32:
		if f, ok := toFloat64(v); ok {
			return any(uint32(f)).(T)
		}
	case uint64:
		if f, ok := toFloat64(v); ok {
			return any(uint64(f)).(T)
		}
	case float32:
		if f, ok := toFloat64(v); ok {
			return any(float32(f)).(T)
		}
	case float64:
		if f, ok := toFloat64(v); ok {
			return any(f).(T)
		}
	case bool:
		// be strict: only accept real bool values or numeric/bool conversions
		if b, ok := v.(bool); ok {
			return any(b).(T)
		}
		if b, ok := toBool(v); ok {
			return any(b).(T)
		}
	case string:
		if s, ok := v.(string); ok {
			return any(s).(T)
		}
	}

	// fallback: exact type assertion for complex types
	if typedVal, ok := v.(T); ok {
		return typedVal
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

// StateBagToStringMap converts a StateBag.Items() map to a map[string]interface{}
// suitable for JSON/YAML marshalling. It performs a shallow snapshot of items.
func StateBagToStringMap(sb StateBag) map[string]interface{} {
	out := make(map[string]interface{})
	if sb == nil {
		return out
	}
	for k, v := range sb.Items() {
		out[string(k)] = v
	}
	return out
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

// NormalizeValue canonicalizes common shapes produced by JSON/YAML decoding.
//
// It performs these steps:
//   - Dereferences pointers
//   - Decodes *yaml.Node into native Go values
//   - Converts json.Number into int64 or float64 when possible
//   - Converts YAML-style map[interface{}]interface{} into map[string]interface{}
//   - Recursively normalizes slices and maps
//
// This is useful when consuming values produced by `encoding/json` or `gopkg.in/yaml.v3`
// before attempting type coercion.
func NormalizeValue(v interface{}) interface{} {
	return normalizeFromState(v)
}

// ToInt64 attempts to coerce v into an int64.
// It supports numeric types, json.Number, and numeric strings. When coercing floats, it truncates toward zero.
// Returns (0,false) if conversion is not possible.
func ToInt64(v interface{}) (int64, bool) {
	return toInt64(v)
}

// ToFloat64 attempts to coerce v into a float64.
// It supports numeric types and numeric strings. Returns (0,false) on failure.
func ToFloat64(v interface{}) (float64, bool) {
	return toFloat64(v)
}

// ToBool attempts to coerce v into a bool.
// It supports bool values, boolean strings ("true"/"false"), and numeric-to-bool conversions (non-zero => true).
// Returns (false,false) if conversion is not possible.
func ToBool(v interface{}) (bool, bool) {
	return toBool(v)
}

// ToStringSlice converts common decoded slice shapes into []string. Returns (nil,false) if not possible.
// Example: []interface{} -> []string, []string -> []string
func ToStringSlice(v interface{}) ([]string, bool) {
	return ToStringSliceInternal(v)
}

// ToStringMap converts common decoded map shapes into map[string]string. Returns (nil,false) if not possible.
// Example: map[string]interface{} -> map[string]string, map[string]string -> map[string]string
func ToStringMap(v interface{}) (map[string]string, bool) {
	return ToStringMapInternal(v)
}

// ToStringSliceInternal is the actual implementation used by the exported ToStringSlice wrapper.
func ToStringSliceInternal(v interface{}) ([]string, bool) {
	if v == nil {
		return nil, false
	}
	switch t := v.(type) {
	case []string:
		return t, true
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, e := range t {
			out = append(out, fmt.Sprint(e))
		}
		return out, true
	default:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Slice {
			out := make([]string, 0, rv.Len())
			for i := 0; i < rv.Len(); i++ {
				out = append(out, fmt.Sprint(rv.Index(i).Interface()))
			}
			return out, true
		}
	}
	return nil, false
}

// ToStringMapInternal is the actual implementation used by the exported ToStringMap wrapper.
func ToStringMapInternal(v interface{}) (map[string]string, bool) {
	if v == nil {
		return nil, false
	}
	switch t := v.(type) {
	case map[string]string:
		return t, true
	case map[string]interface{}:
		out := make(map[string]string, len(t))
		for k, val := range t {
			out[k] = fmt.Sprint(val)
		}
		return out, true
	default:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Map {
			out := make(map[string]string)
			for _, key := range rv.MapKeys() {
				ks := fmt.Sprint(key.Interface())
				val := rv.MapIndex(key).Interface()
				out[ks] = fmt.Sprint(val)
			}
			return out, true
		}
	}
	return nil, false
}
