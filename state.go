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

// Key is a typed string used as a map key inside a StateBag. Using a named
// type instead of a plain string prevents accidental key collisions between
// packages and enables the compiler to catch misuse at build time.
type Key string

const (
	// KeyState is the context key under which a StateBag is stored and
	// retrieved via ContextWithState / StateFromContext.
	KeyState Key = "automa_state_bag"

	// KeyStep is the context key under which the currently-executing Step is
	// stored for downstream access.
	KeyStep Key = "automa_step"

	// KeyId is the context key under which the identifier of the current
	// workflow or step is stored.
	KeyId Key = "automa_id"

	// KeyIsWorkflow is the context key whose presence (and boolean value)
	// indicates that the current execution context belongs to a Workflow rather
	// than an individual Step.
	KeyIsWorkflow Key = "automa_is_workflow"

	// KeyStartTime is the context key under which the wall-clock start time of
	// an execution is stored.
	KeyStartTime Key = "automa_start_time"

	// KeyEndTime is the context key under which the wall-clock end time of an
	// execution is stored.
	KeyEndTime Key = "automa_end_time"

	// KeyReport is the context key under which the Report produced by the most
	// recently completed step or workflow is stored.
	KeyReport Key = "automa_report"
)

// SyncStateBag is a thread-safe implementation of StateBag backed by a plain
// map[Key]interface{} protected by a sync.RWMutex.
//
// Concurrency model:
//   - Read operations (Get, Items, Keys, Size, String, Int, …) acquire a
//     shared read lock, so multiple goroutines may read concurrently.
//   - Write operations (Set, Delete, Clear, Merge, UnmarshalJSON, UnmarshalYAML)
//     acquire an exclusive write lock.
//   - Snapshot methods (Items, MarshalJSON, MarshalYAML) hold the read lock for
//     the full traversal, producing a consistent point-in-time view.
//
// Zero value: a zero-value SyncStateBag is ready to use; the internal map is
// allocated lazily on the first write.
type SyncStateBag struct {
	mu sync.RWMutex
	m  map[Key]interface{}
}

// init lazily allocates the internal map. It MUST be called at the start of
// every write path (Set, Merge, UnmarshalJSON, UnmarshalYAML, Clone's inner
// bag) while the caller already holds the write lock (s.mu.Lock). It is safe
// to call repeatedly; subsequent calls are no-ops.
func (s *SyncStateBag) init() {
	if s.m == nil {
		s.m = make(map[Key]interface{})
	}
}

// Clone returns a deep copy of the SyncStateBag as a StateBag.
//
// Deep-copy strategy for each stored value:
//   - If the value is nil, nil is stored in the clone.
//   - If the value's type exposes a zero-argument Clone() (T, error) method,
//     that method is called and the returned copy is stored. Any error is
//     wrapped and returned, leaving the partially-built clone unreferenced.
//   - If the value's type exposes a zero-argument Clone() T method (single
//     return), that method is called and the result is stored.
//   - All other values are shallow-copied (the same pointer/value is stored).
//
// Clone acquires no lock of its own; it delegates to Items() which takes a
// read lock. This avoids re-entrant locking on Go's non-reentrant RWMutex.
//
// Returns (nil, error) when the receiver is nil or when a value's Clone method
// returns an error.
func (s *SyncStateBag) Clone() (StateBag, error) {
	if s == nil {
		return nil, IllegalArgument.New("cannot clone a nil SyncStateBag")
	}

	// Items() acquires its own RLock; do not hold mu here to avoid
	// reentrant-lock deadlocks (Go RWMutex is not reentrant).
	items := s.Items()

	clone := &SyncStateBag{}
	clone.init()
	errInterface := reflect.TypeOf((*error)(nil)).Elem()

	for k, v := range items {
		if v == nil {
			clone.m[k] = nil
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
				clone.m[k] = results[0].Interface()
				continue
			}

			// support Clone() value: if any other clone signature without error
			if outCount == 1 {
				results := m.Call([]reflect.Value{})
				clone.m[k] = results[0].Interface()
				continue
			}
		}

		// fallback: shallow copy
		clone.m[k] = v
	}

	return clone, nil
}

// Merge copies every key-value pair from other into s, overwriting existing
// keys in s that also exist in other.
//
// Deadlock prevention: the snapshot of other is taken via other.Items() before
// s's write lock is acquired. This eliminates two classes of deadlock:
//  1. Self-merge (other == s): snapshotting before locking prevents an
//     attempt to acquire an already-held read lock on the same RWMutex.
//  2. Lock-order inversion: if two goroutines call a.Merge(b) and b.Merge(a)
//     concurrently, neither holds a lock while calling the other bag's
//     Items(), so they cannot deadlock waiting for each other.
//
// Returns (s, nil) on success.
// Returns (s, nil) unchanged when other is nil.
func (s *SyncStateBag) Merge(other StateBag) (StateBag, error) {
	if other == nil {
		return s, nil
	}

	// Snapshot other BEFORE acquiring our own lock to avoid deadlock.
	otherItems := other.Items()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.init()

	for k, v := range otherItems {
		s.m[k] = v
	}

	return s, nil
}

// Items returns a shallow snapshot of all key-value pairs held by s as a new
// map[Key]interface{}. The snapshot is taken under a read lock, so it is
// consistent even when other goroutines are writing concurrently.
//
// Callers must not use the returned map to modify s; it is a copy.
func (s *SyncStateBag) Items() map[Key]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make(map[Key]interface{}, len(s.m))
	for k, v := range s.m {
		items[k] = v
	}
	return items
}

// itemsStringMap returns a shallow snapshot with string keys for use by
// MarshalJSON and MarshalYAML. Keys are converted from Key to string.
// The snapshot is taken under a read lock for consistency.
func (s *SyncStateBag) itemsStringMap() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]interface{}, len(s.m))
	for k, v := range s.m {
		out[string(k)] = v
	}
	return out
}

// MarshalJSON implements json.Marshaler. It serializes the bag as a JSON
// object whose keys are the string form of each Key and whose values are the
// JSON representations of the stored values.
//
// A nil receiver marshals as JSON null.
//
// Note: the snapshot is taken under a read lock, so concurrent writes that
// arrive during marshaling are not reflected in the output.
func (s *SyncStateBag) MarshalJSON() ([]byte, error) {
	if s == nil {
		return json.Marshal(nil)
	}
	// create a consistent snapshot
	return json.Marshal(s.itemsStringMap())
}

// UnmarshalJSON implements json.Unmarshaler. It decodes a JSON object into the
// bag, replacing all existing contents. Keys in the JSON object are stored as
// Key values; JSON numbers become float64, objects become
// map[string]interface{}, and arrays become []interface{} — standard
// encoding/json behaviour.
//
// Callers that need typed values after unmarshaling should use the typed
// accessors (Int, String, …) or FromState, which perform coercion from float64
// and other JSON-decoded shapes.
//
// Returns an error if the receiver is nil or if the JSON is malformed.
func (s *SyncStateBag) UnmarshalJSON(data []byte) error {
	if s == nil {
		return IllegalArgument.New("cannot unmarshal into nil SyncStateBag")
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearLocked()
	s.init()
	for k, v := range m {
		s.m[Key(k)] = v
	}
	return nil
}

// MarshalYAML implements yaml.Marshaler. It returns the bag contents as a
// map[string]interface{} that the YAML encoder will serialize. Keys are
// converted from Key to string.
//
// A nil receiver returns (nil, nil), which the encoder renders as a YAML null.
//
// Note: the snapshot is taken under a read lock; concurrent writes are not
// reflected in the output.
func (s *SyncStateBag) MarshalYAML() (interface{}, error) {
	if s == nil {
		return nil, nil
	}
	// snapshot under read lock
	return s.itemsStringMap(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler. It decodes a YAML mapping node
// into the bag, replacing all existing contents. Keys are stored as Key
// values. YAML integers are decoded as int by the yaml.v3 library; floats as
// float64; booleans as bool.
//
// Returns an error if the receiver is nil or if the YAML node cannot be
// decoded into a map.
func (s *SyncStateBag) UnmarshalYAML(node *yaml.Node) error {
	if s == nil {
		return IllegalArgument.New("cannot unmarshal into nil SyncStateBag")
	}
	var m map[string]interface{}
	if err := node.Decode(&m); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearLocked()
	s.init()
	for k, v := range m {
		s.m[Key(k)] = v
	}
	return nil
}

// Get retrieves the raw value stored under key. Returns (value, true) when the
// key is present, or (nil, false) when it is absent. The caller receives the
// exact value that was stored — no coercion is applied.
func (s *SyncStateBag) Get(key Key) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[key]
	return v, ok
}

// Set stores value under key, replacing any previously stored value, and
// returns s to allow method chaining:
//
//	bag.Set("a", 1).Set("b", 2)
//
// A nil value is permitted; it stores an explicit nil entry which is
// distinguishable from an absent key via Get.
func (s *SyncStateBag) Set(key Key, value interface{}) StateBag {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.init()
	s.m[key] = value
	return s
}

// Delete removes the entry for key from the bag. It is a no-op when the key is
// absent. Returns s for method chaining.
func (s *SyncStateBag) Delete(key Key) StateBag {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
	return s
}

// Clear removes all entries from the bag, leaving it empty but still usable.
// Returns s for method chaining.
func (s *SyncStateBag) Clear() StateBag {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clearLocked()
	return s
}

// clearLocked removes all entries without acquiring the mutex. The caller MUST
// hold s.mu.Lock() before calling this method.
func (s *SyncStateBag) clearLocked() {
	s.m = nil
}

// Keys returns a snapshot of all keys currently stored in the bag. The order
// of keys is not guaranteed (Go map iteration order is random). The returned
// slice is a copy; modifying it does not affect the bag.
func (s *SyncStateBag) Keys() []Key {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]Key, 0, len(s.m))
	for k := range s.m {
		keys = append(keys, k)
	}
	return keys
}

// Size returns the number of key-value pairs currently stored in the bag.
func (s *SyncStateBag) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}

// String retrieves a string value from the bag for key using FromState[string].
//
// Coercion rules (applied in order after an exact string match fails):
//   - Numeric types are formatted as their decimal string representation
//     (integers without decimal point, floats using minimal precision).
//   - bool is formatted as "true" or "false".
//   - json.Number is returned as-is via its String() method.
//   - Types that implement the fmt.Stringer interface are formatted via
//     their String() method.
//
// Returns ("", false) when the key is absent, the stored value is nil, or
// the value cannot be coerced to a string.
func (s *SyncStateBag) String(key Key) (string, bool) {
	return FromState[string](s, key, "")
}

// Bool retrieves a bool value from the bag for key using FromState[bool].
//
// Coercion rules (applied after an exact bool match fails):
//   - String "true"/"false" (case-insensitive, as per strconv.ParseBool).
//   - Numeric types: non-zero → true, zero → false.
//
// Returns (false, false) when the key is absent or the value cannot be
// coerced to bool.
func (s *SyncStateBag) Bool(key Key) (bool, bool) {
	return FromState[bool](s, key, false)
}

// Int retrieves an int value from the bag for key using FromState[int].
//
// Coercion rules (applied after an exact int match fails):
//   - Other integer types are converted directly where the value fits.
//   - Float values are truncated toward zero (e.g. 3.9 → 3, -1.2 → -1).
//   - Numeric strings and json.Number are parsed; float strings are truncated.
//
// Returns (0, false) when the key is absent or the value cannot be coerced.
func (s *SyncStateBag) Int(key Key) (int, bool) {
	return FromState[int](s, key, 0)
}

// Int8 retrieves an int8 value from the bag for key. See Int for coercion
// rules. Returns (0, false) when the key is absent or coercion fails.
func (s *SyncStateBag) Int8(key Key) (int8, bool) {
	return FromState[int8](s, key, 0)
}

// Int16 retrieves an int16 value from the bag for key. See Int for coercion
// rules. Returns (0, false) when the key is absent or coercion fails.
func (s *SyncStateBag) Int16(key Key) (int16, bool) {
	return FromState[int16](s, key, 0)
}

// Int32 retrieves an int32 value from the bag for key. See Int for coercion
// rules. Returns (0, false) when the key is absent or coercion fails.
func (s *SyncStateBag) Int32(key Key) (int32, bool) {
	return FromState[int32](s, key, 0)
}

// Int64 retrieves an int64 value from the bag for key. See Int for coercion
// rules. Returns (0, false) when the key is absent or coercion fails.
func (s *SyncStateBag) Int64(key Key) (int64, bool) {
	return FromState[int64](s, key, 0)
}

// Float retrieves a float64 value from the bag for key using FromState[float64].
//
// Coercion rules (applied after an exact float64 match fails):
//   - All integer and float types are converted via float64 cast.
//   - Numeric strings and json.Number are parsed with strconv.ParseFloat.
//
// Returns (0.0, false) when the key is absent or coercion fails.
func (s *SyncStateBag) Float(key Key) (float64, bool) {
	return FromState[float64](s, key, 0.0)
}

// Float32 retrieves a float32 value from the bag for key. See Float for
// coercion rules. Returns (0.0, false) when the key is absent or coercion fails.
func (s *SyncStateBag) Float32(key Key) (float32, bool) {
	return FromState[float32](s, key, 0.0)
}

// Float64 retrieves a float64 value from the bag for key. See Float for
// coercion rules. Returns (0.0, false) when the key is absent or coercion fails.
func (s *SyncStateBag) Float64(key Key) (float64, bool) {
	return FromState[float64](s, key, 0.0)
}

// ContextWithState returns a new context derived from ctx with the given
// StateBag stored under KeyState. The bag can be retrieved later via
// StateFromContext.
func ContextWithState(ctx context.Context, stateBag StateBag) context.Context {
	return context.WithValue(ctx, KeyState, stateBag)
}

// StateFromContext retrieves the StateBag stored in ctx under KeyState. If ctx
// is nil or contains no StateBag, an empty *SyncStateBag is returned so
// callers never need to check for nil before using the result.
func StateFromContext(ctx context.Context) StateBag {
	if ctx != nil {
		if stateBag, ok := ctx.Value(KeyState).(StateBag); ok {
			return stateBag
		}
	}
	return &SyncStateBag{}
}

// normalizeFromState canonicalizes values produced by JSON/YAML decoders into
// stable Go types before type coercion is attempted by FromState.
//
// Steps performed (in order):
//  1. A nil input is returned as (nil, nil) — nil is a valid stored value.
//  2. *yaml.Node and yaml.Node values are decoded into native Go values and
//     re-normalized recursively.
//  3. Pointer chains are dereferenced until a non-pointer value is reached.
//     A nil pointer anywhere in the chain returns (nil, nil).
//  4. json.Number is converted to int64 (preferred), uint64 (for values
//     exceeding MaxInt64), float64, or plain string — in that priority order.
//  5. map[interface{}]interface{} (common in YAML decoding) is converted to
//     map[string]interface{} with keys stringified via stringify.
//  6. map[string]interface{} values are normalized recursively.
//  7. []interface{} elements are normalized recursively; a fresh slice is
//     returned so that the original stored slice is not mutated.
//
// Returns (nil, error) only when a yaml.Node fails to decode or a map key
// cannot be stringified. All other failures are expressed as (nil, nil).
func normalizeFromState(v interface{}) (interface{}, error) {
	// Treat explicit nil as a valid normalized nil rather than an error.
	if v == nil {
		return nil, nil
	}

	// Handle yaml.Node early (both pointer and value) to avoid pointer deref consuming it
	if nodePtr, ok := v.(*yaml.Node); ok && nodePtr != nil {
		var out interface{}
		if err := nodePtr.Decode(&out); err != nil {
			return nil, err
		}
		return normalizeFromState(out)
	}
	if nodeVal, ok := v.(yaml.Node); ok {
		var out interface{}
		if err := (&nodeVal).Decode(&out); err != nil {
			return nil, err
		}
		return normalizeFromState(out)
	}

	// dereference pointers
	for {
		rv := reflect.ValueOf(v)
		if !rv.IsValid() {
			return nil, errorx.IllegalArgument.New("invalid value: %v", v)
		}
		if rv.Kind() == reflect.Ptr {
			if rv.IsNil() {
				return nil, nil
			}
			v = rv.Elem().Interface()
			continue
		}
		break
	}

	// json.Number -> int64, uint64, float64 or string
	if jn, ok := v.(json.Number); ok {
		if i, err := jn.Int64(); err == nil {
			return i, nil
		}
		if u, err := strconv.ParseUint(jn.String(), 10, 64); err == nil {
			return u, nil
		}
		if f, err := jn.Float64(); err == nil {
			return f, nil
		}
		return jn.String(), nil
	}

	// recursively convert YAML map[interface{}]interface{} -> map[string]interface{}
	var nv interface{}
	var err error
	switch t := v.(type) {
	case map[interface{}]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, vv := range t {
			var ks string
			var err error
			switch kk := k.(type) {
			case string:
				ks = kk
			default:
				ks, err = stringify(k)
				if err != nil {
					return nil, err
				}
			}
			nv, err = normalizeFromState(vv)
			if err != nil {
				return nil, err
			}

			out[ks] = nv
		}
		return out, nil
	case map[string]interface{}:
		out := make(map[string]interface{}, len(t))
		for k, vv := range t {
			nv, err = normalizeFromState(vv)
			if err != nil {
				return nil, err
			}
			out[k] = nv
		}
		return out, nil
	case []interface{}:
		out := make([]interface{}, len(t))
		for i := range t {
			nv, err = normalizeFromState(t[i])
			if err != nil {
				return nil, err
			}
			out[i] = nv
		}
		return out, nil
	default:
		return v, nil
	}
}

// stringify converts a scalar value to its string representation.
//
// Supported types:
//   - string: returned as-is.
//   - []byte: converted to string.
//   - Signed integers (int, int8, int16, int32, int64): formatted in base 10.
//   - Unsigned integers (uint, uint8, uint16, uint32, uint64): formatted in
//     base 10 without any float intermediary, so large uint64 values are safe.
//   - float32: formatted with 'g' format and 32-bit precision (no trailing zeros).
//   - float64: formatted with 'g' format and 64-bit precision.
//   - bool: "true" or "false".
//   - json.Number: the integer representation is preferred; if the value is
//     not an integer, the raw string from Number.String() is used.
//
// Returns ("", error) for any unsupported type.
func stringify(v interface{}) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case []byte:
		return string(t), nil
	case int, int8, int16, int32, int64:
		if i, ok := toInt64(t); ok {
			return strconv.FormatInt(i, 10), nil
		}
	case uint:
		return strconv.FormatUint(uint64(t), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(t), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(t), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(t), 10), nil
	case uint64:
		return strconv.FormatUint(t, 10), nil
	case float32:
		// use 'g' and -1 to get a short, non-excessive representation
		return strconv.FormatFloat(float64(t), 'g', -1, 32), nil
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64), nil
	case bool:
		return strconv.FormatBool(t), nil
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return strconv.FormatInt(i, 10), nil
		}
		if u, err := strconv.ParseUint(t.String(), 10, 64); err == nil {
			return strconv.FormatUint(u, 10), nil
		}
		if f, err := t.Float64(); err == nil {
			return strconv.FormatFloat(f, 'g', -1, 64), nil
		}

		return t.String(), nil
	}
	return "", errorx.IllegalState.New("cannot stringify value of type %T", v)
}

const eps = 1e-9

// int64MaxAsFloat64 and int64MinAsFloat64 are the float64 bounds used to
// range-check float-to-int64 conversions. float64(math.MaxInt64) cannot be
// represented exactly — it rounds up to 2^63 (= math.MinInt64 reinterpreted),
// so we use 2^63 as the exclusive upper bound instead.
//
//	int64MinAsFloat64 = -2^63 (exactly representable as float64, == math.MinInt64)
//	int64MaxExclFloat64 = +2^63 (exclusive upper bound; math.MaxInt64+1, also exact)
//
// A truncated float tr is safe to cast only when:
//
//	int64MinAsFloat64 <= tr && tr < int64MaxExclFloat64
const (
	int64MinAsFloat64    = -9223372036854775808.0 // -(1 << 63), exact
	int64MaxExclFloat64  = 9223372036854775808.0  // (1 << 63), exclusive upper bound
	uint64MaxExclFloat64 = 18446744073709551616.0 // (1 << 64), exact exclusive upper bound for uint64
)

// floatIsIntegral reports whether f has no fractional part (i.e. f == Trunc(f)
// within a small epsilon). It is used by toUint64Safe to verify that a float64
// value can be safely cast to an integer without information loss.
func floatIsIntegral(f float64) bool {
	_, frac := math.Modf(f)
	return math.Abs(frac) < eps
}

// toInt64 coerces v to int64 with range and overflow checking.
//
// Conversion rules:
//   - Signed integers are widened directly.
//   - Unsigned integers are accepted only when they fit in [0, MaxInt64];
//     values > MaxInt64 return (0, false).
//   - float32/float64 are truncated toward zero; values outside the int64
//     representable range [MinInt64, MaxInt64) return (0, false).
//   - Numeric strings are parsed with ParseInt; if that fails, ParseFloat is
//     attempted with the same truncation and range rules.
//   - json.Number tries Int64() first, then Float64() with truncation.
//
// Returns (0, false) for any other type or out-of-range value.
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
		if uint64(t) > math.MaxInt64 {
			return 0, false
		}
		return int64(t), true
	case uint8:
		return int64(t), true
	case uint16:
		return int64(t), true
	case uint32:
		return int64(t), true
	case uint64:
		if t > math.MaxInt64 {
			return 0, false
		}
		return int64(t), true
	case float32:
		f := float64(t)
		tr := math.Trunc(f)
		if tr < int64MinAsFloat64 || tr >= int64MaxExclFloat64 {
			return 0, false
		}
		return int64(tr), true
	case float64:
		tr := math.Trunc(t)
		if tr < int64MinAsFloat64 || tr >= int64MaxExclFloat64 {
			return 0, false
		}
		return int64(tr), true
	case string:
		if i, err := strconv.ParseInt(t, 10, 64); err == nil {
			return i, true
		}
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			tr := math.Trunc(f)
			if tr < int64MinAsFloat64 || tr >= int64MaxExclFloat64 {
				return 0, false
			}
			return int64(tr), true
		}
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i, true
		}
		if f, err := t.Float64(); err == nil {
			tr := math.Trunc(f)
			if tr < int64MinAsFloat64 || tr >= int64MaxExclFloat64 {
				return 0, false
			}
			return int64(tr), true
		}
	}
	return 0, false
}

// toFloat64 coerces v to float64.
//
// All integer and float types are converted directly. Numeric strings are
// parsed with strconv.ParseFloat. json.Number is converted via its Float64()
// method. Large integers may lose precision due to float64's 53-bit mantissa.
//
// Returns (0, false) for any unsupported type or unparseable string.
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

// toBool coerces v to bool.
//
// Conversion rules:
//   - bool: returned as-is.
//   - string: parsed by strconv.ParseBool ("1", "t", "T", "TRUE", "true",
//     "True", "0", "f", "F", "FALSE", "false", "False").
//   - Numeric types: non-zero → true, zero → false.
//
// Returns (false, false) for unsupported types or unparseable strings.
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

// FromState retrieves a value of type T from state under key, reporting
// whether the lookup and coercion succeeded.
//
// Lookup and coercion pipeline (executed in order until one succeeds):
//  1. Key absence: returns (zero, false) immediately.
//  2. Exact type match: if the raw stored value is already a T, it is
//     returned without any conversion.
//  3. Normalization: the raw value is passed through normalizeFromState to
//     unwrap pointers, decode yaml.Node, convert json.Number, etc.
//  4. Exact type match on normalized value.
//  5. Type coercion: for primitive target types (string, bool, int*, uint*,
//     float*), the appropriate conversion helper is applied.
//
// Return values:
//   - (value, true)  — key exists and value was successfully coerced to T.
//   - (zero,  false) — key absent, normalization failed, or coercion to T
//     is not possible.
//
// The two-value return eliminates the ambiguity of a single-value form: without
// the bool, a caller cannot distinguish "key missing" from "stored value is
// the zero value of T". Use state.Get directly when you need the raw,
// uncoerced value.
//
// After a JSON round-trip, numbers are stored as float64 by encoding/json.
// FromState handles this transparently: requesting Int for a key whose value
// is float64(5) will return (5, true).
//
// The StateBag convenience methods (String, Int, Bool, …) delegate to FromState
// with the appropriate zero value.
func FromState[T any](state StateBag, key Key, zero T) (T, bool) {
	if state == nil {
		return zero, false
	}
	val, ok := state.Get(key)
	if !ok {
		return zero, false
	}

	// Exact type match on raw value (preserves pointer/precise stored types).
	if typedVal, ok := val.(T); ok {
		return typedVal, true
	}

	v, err := normalizeFromState(val)
	if err != nil {
		return zero, false
	}

	// After normalization the type may now match directly.
	if typedVal, ok := v.(T); ok {
		return typedVal, true
	}

	// Single dispatch on target type for all coercions.
	switch any(zero).(type) {
	case string:
		if s, ok := coerceToString(v); ok {
			return any(s).(T), true
		}
	case bool:
		if b, ok := toBool(v); ok {
			return any(b).(T), true
		}
	case int:
		if i, ok := toInt64(v); ok {
			return any(int(i)).(T), true
		}
	case int8:
		if i, ok := toInt64(v); ok {
			return any(int8(i)).(T), true
		}
	case int16:
		if i, ok := toInt64(v); ok {
			return any(int16(i)).(T), true
		}
	case int32:
		if i, ok := toInt64(v); ok {
			return any(int32(i)).(T), true
		}
	case int64:
		if i, ok := toInt64(v); ok {
			return any(i).(T), true
		}
	case uint:
		if u, ok := toUint64Safe(v, 64); ok {
			return any(uint(u)).(T), true
		}
	case uint8:
		if u, ok := toUint64Safe(v, 8); ok {
			return any(uint8(u)).(T), true
		}
	case uint16:
		if u, ok := toUint64Safe(v, 16); ok {
			return any(uint16(u)).(T), true
		}
	case uint32:
		if u, ok := toUint64Safe(v, 32); ok {
			return any(uint32(u)).(T), true
		}
	case uint64:
		if u, ok := toUint64Safe(v, 64); ok {
			return any(u).(T), true
		}
	case float32:
		if f, ok := toFloat64(v); ok {
			return any(float32(f)).(T), true
		}
	case float64:
		if f, ok := toFloat64(v); ok {
			return any(f).(T), true
		}
	}

	return zero, false
}

// StringFromState is a convenience wrapper around FromState[string]. It returns
// the string value stored under key in state, and whether the lookup and
// coercion succeeded. Returns ("", false) when the key is absent or the value
// cannot be coerced to string.
func StringFromState(state StateBag, key Key) (string, bool) {
	return FromState[string](state, key, "")
}

// IntFromState is a convenience wrapper around FromState[int]. It returns the
// int value stored under key in state, and whether the lookup and coercion
// succeeded. Returns (0, false) when the key is absent or the value cannot be
// coerced to int.
func IntFromState(state StateBag, key Key) (int, bool) {
	return FromState[int](state, key, 0)
}

// BoolFromState is a convenience wrapper around FromState[bool]. It returns
// the bool value stored under key in state, and whether the lookup and
// coercion succeeded. Returns (false, false) when the key is absent or the
// value cannot be coerced to bool.
func BoolFromState(state StateBag, key Key) (bool, bool) {
	return FromState[bool](state, key, false)
}

// FloatFromState is a convenience wrapper around FromState[float64]. It returns
// the float64 value stored under key in state, and whether the lookup and
// coercion succeeded. Returns (0.0, false) when the key is absent or the value
// cannot be coerced to float64.
func FloatFromState(state StateBag, key Key) (float64, bool) {
	return FromState[float64](state, key, 0.0)
}

// StateBagToStringMap returns a shallow snapshot of the bag's contents as a
// map[string]interface{} with string keys, suitable for JSON or YAML
// marshaling. Keys are converted from Key to string. Returns an empty map when
// sb is nil.
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

// SliceFromState retrieves a []T value stored under key. It performs an exact
// type assertion only — no element-by-element coercion is attempted. Returns
// an empty []T when the key is absent or the stored value is not a []T.
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

// StringSliceFromState is a convenience wrapper around SliceFromState[string].
// Returns an empty []string when the key is absent or the value is not []string.
func StringSliceFromState(state StateBag, key Key) []string {
	return SliceFromState[string](state, key)
}

// IntSliceFromState is a convenience wrapper around SliceFromState[int].
// Returns an empty []int when the key is absent or the value is not []int.
func IntSliceFromState(state StateBag, key Key) []int {
	return SliceFromState[int](state, key)
}

// BoolSliceFromState is a convenience wrapper around SliceFromState[bool].
// Returns an empty []bool when the key is absent or the value is not []bool.
func BoolSliceFromState(state StateBag, key Key) []bool {
	return SliceFromState[bool](state, key)
}

// FloatSliceFromState is a convenience wrapper around SliceFromState[float64].
// Returns an empty []float64 when the key is absent or the value is not []float64.
func FloatSliceFromState(state StateBag, key Key) []float64 {
	return SliceFromState[float64](state, key)
}

// MapFromState retrieves a map[K]V value stored under key. It performs an
// exact type assertion only — no key or value coercion is attempted. Returns
// an empty map[K]V when the key is absent or the stored value is not a
// map[K]V.
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

// StringMapFromState is a convenience wrapper around MapFromState[string,string].
// Returns an empty StringMap when the key is absent or the value is not a StringMap.
func StringMapFromState(state StateBag, key Key) StringMap {
	return MapFromState[string, string](state, key)
}

// IntMapFromState is a convenience wrapper around MapFromState[string,int].
// Returns an empty map[string]int when the key is absent or the value is not
// a map[string]int.
func IntMapFromState(state StateBag, key Key) map[string]int {
	return MapFromState[string, int](state, key)
}

// BoolMapFromState is a convenience wrapper around MapFromState[string,bool].
// Returns an empty map[string]bool when the key is absent or the value is not
// a map[string]bool.
func BoolMapFromState(state StateBag, key Key) map[string]bool {
	return MapFromState[string, bool](state, key)
}

// FloatMapFromState is a convenience wrapper around MapFromState[string,float64].
// Returns an empty map[string]float64 when the key is absent or the value is
// not a map[string]float64.
func FloatMapFromState(state StateBag, key Key) map[string]float64 {
	return MapFromState[string, float64](state, key)
}

// NormalizeValue is the exported form of normalizeFromState. It canonicalizes
// common shapes produced by JSON/YAML decoding into stable Go types:
//   - Dereferences pointer chains.
//   - Decodes *yaml.Node / yaml.Node into native Go values.
//   - Converts json.Number to int64, uint64, float64, or string.
//   - Converts map[interface{}]interface{} (YAML) to map[string]interface{}.
//   - Recursively normalizes slices and maps.
//
// This is useful before calling FromState or performing manual type assertions
// on values decoded from JSON or YAML.
//
// Returns (nil, error) only on a yaml.Node decode error or an unstringable map
// key. All other inputs either return a normalized value or (nil, nil) for nil
// inputs.
func NormalizeValue(v interface{}) (interface{}, error) {
	return normalizeFromState(v)
}

// ToInt64 is the exported form of toInt64. It attempts to coerce v to int64
// with range and overflow checking. Float values are truncated toward zero.
// Returns (0, false) for unsupported types, unparseable strings, or
// out-of-range values.
//
// See toInt64 for the full set of supported input types.
func ToInt64(v interface{}) (int64, bool) {
	return toInt64(v)
}

// ToFloat64 is the exported form of toFloat64. It attempts to coerce v to
// float64. All numeric types and numeric strings are supported.
// Returns (0, false) for unsupported types or unparseable strings.
func ToFloat64(v interface{}) (float64, bool) {
	return toFloat64(v)
}

// ToBool is the exported form of toBool. It coerces v to bool using boolean
// strings ("true"/"false") and numeric-to-bool conversions (non-zero → true).
// Returns (false, false) for unsupported types or unparseable inputs.
func ToBool(v interface{}) (bool, bool) {
	return toBool(v)
}

// ToStringSlice converts common slice shapes into a []string.
//
// Supported shapes:
//   - []string: returned as-is.
//   - []interface{}: each element is formatted with fmt.Sprint.
//   - Any other slice (detected via reflection): each element is formatted
//     with fmt.Sprint.
//
// Returns (nil, false) for nil input or non-slice types.
func ToStringSlice(v interface{}) ([]string, bool) {
	return toStringSlice(v)
}

// ToStringMap converts common map shapes into a StringMap (map[string]string).
//
// Supported shapes:
//   - StringMap: returned as-is.
//   - map[string]interface{}: each value is formatted with fmt.Sprint.
//   - Any other map (detected via reflection): keys and values are formatted
//     with fmt.Sprint.
//
// Returns (nil, false) for nil input or non-map types.
func ToStringMap(v interface{}) (StringMap, bool) {
	return toStringMap(v)
}

// toStringSlice is the internal implementation of ToStringSlice.
func toStringSlice(v interface{}) ([]string, bool) {
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

// toStringMap is the internal implementation of ToStringMap.
func toStringMap(v interface{}) (StringMap, bool) {
	if v == nil {
		return nil, false
	}
	switch t := v.(type) {
	case StringMap:
		return t, true
	case map[string]interface{}:
		out := make(StringMap, len(t))
		for k, val := range t {
			out[k] = fmt.Sprint(val)
		}
		return out, true
	default:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Map {
			out := make(StringMap)
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

// coerceToString converts any scalar v to its canonical string representation.
//
// Conversion rules:
//   - string: returned as-is.
//   - bool: "true" or "false".
//   - json.Number: returned via Number.String() (preserves the original
//     representation without float conversion).
//   - Signed integers: base-10 decimal, no decimal point.
//   - Unsigned integers: base-10 decimal, no decimal point, safe for uint64.
//   - float32: shortest decimal representation with 32-bit precision (no
//     trailing zeros; e.g. 3.14 not 3.140000104904175).
//   - float64: shortest decimal representation with 64-bit precision.
//   - Any type that implements fmt.Stringer: result of its String() method.
//
// Returns ("", false) for types not covered above.
func coerceToString(v interface{}) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case bool:
		return strconv.FormatBool(t), true
	case json.Number:
		return t.String(), true
	case int:
		return strconv.FormatInt(int64(t), 10), true
	case int8:
		return strconv.FormatInt(int64(t), 10), true
	case int16:
		return strconv.FormatInt(int64(t), 10), true
	case int32:
		return strconv.FormatInt(int64(t), 10), true
	case int64:
		return strconv.FormatInt(t, 10), true
	case uint:
		return strconv.FormatUint(uint64(t), 10), true
	case uint8:
		return strconv.FormatUint(uint64(t), 10), true
	case uint16:
		return strconv.FormatUint(uint64(t), 10), true
	case uint32:
		return strconv.FormatUint(uint64(t), 10), true
	case uint64:
		return strconv.FormatUint(t, 10), true
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32), true
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64), true
	default:
		if s, ok := t.(interface{ String() string }); ok {
			return s.String(), true
		}
		return "", false
	}
}

// toUint64Safe coerces v to a uint64 value that fits within bitSize bits
// (bitSize ∈ {8, 16, 32, 64}), with explicit rejection of negative and
// out-of-range values.
//
// Conversion pipeline (tried in order until one succeeds):
//  1. string fast-path: strconv.ParseUint(v, 10, bitSize) — handles large
//     uint64 strings (> MaxInt64) without float64 precision loss.
//  2. json.Number fast-path: same ParseUint on the raw string — also avoids
//     float64 precision loss for large unsigned values.
//  3. Integer path via toInt64: negative results are rejected outright;
//     positive results are bounds-checked against maxVal.
//  4. Float fallback: accepted only when f >= 0, f is integral (no fractional
//     part within eps), and the round-trip float64(uint64(f)) == f holds.
//     For bitSize == 64 the exclusive upper bound is 2^64 (uint64MaxExclFloat64).
//     For smaller widths, the value is compared against maxVal.
//
// Returns (0, false) for negative values, out-of-range values, non-integral
// floats, or unsupported types.
func toUint64Safe(v interface{}, bitSize int) (uint64, bool) {
	maxVal := ^uint64(0)
	if bitSize < 64 {
		maxVal = (1 << bitSize) - 1
	}

	// direct string/json.Number parse first — handles large uint64 values beyond int64 range
	// without precision loss through float conversion.
	if s, ok := v.(string); ok {
		if u, err := strconv.ParseUint(s, 10, bitSize); err == nil {
			return u, true
		}
	}
	if n, ok := v.(json.Number); ok {
		if u, err := strconv.ParseUint(n.String(), 10, bitSize); err == nil {
			return u, true
		}
	}

	// integer path — avoids float64 precision loss for smaller values
	if i, ok := toInt64(v); ok {
		if i < 0 || (bitSize < 64 && uint64(i) > maxVal) {
			return 0, false
		}
		return uint64(i), true
	}

	// float fallback — only for non-negative, integral, exactly-representable values in range.
	if f, ok := toFloat64(v); ok && f >= 0 && floatIsIntegral(f) {
		if bitSize == 64 {
			if f >= uint64MaxExclFloat64 {
				return 0, false
			}
			u := uint64(f)
			if float64(u) != f {
				return 0, false
			}
			return u, true
		}

		if f > float64(maxVal) {
			return 0, false
		}
		u := uint64(f)
		if float64(u) != f || u > maxVal {
			return 0, false
		}
		return u, true
	}

	return 0, false
}
