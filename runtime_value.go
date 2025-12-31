package automa

import (
	"bytes"
	"context"
	"encoding/gob"
	"sync"

	"golang.org/x/sync/singleflight"

	"github.com/joomcode/errorx"
)

// Cloner marks values that can return a cloned copy of themselves.
// Clone returns a typed value of type T and an error; the caller decides what to store.
type Cloner[T any] interface {
	Clone() (T, error)
}

// Value provides primitives for building and resolving typed
// configuration values used by the automa framework. It is centered around
// the concept of a runtime-resolvable value container represented
// by the Value[T] interface which supports producing a deep-cloned copy.
//
// This file contains the default Value implementation and the RuntimeValue
// container which composes default values, optional user input and an
// optional effective resolution function.
type Value[T any] interface {
	Cloner[Value[T]]
	Val() T
}

// defaultValue is a concrete implementation of Value.
//
// It holds the actual value and an optional custom cloner function.
// If cloner is nil, encoding/gob is used to perform deep cloning.
type defaultValue[T any] struct {
	v      T
	cloner func(v T) (Value[T], error)
}

// NewValue constructs a new Value instance validated to be encodable by
// encoding/gob. It performs a test gob encode of the value to ensure
// that Clone (which uses gob) will succeed. Returns an error if the
// value (or nested types) are not encodable by gob.
//
// For types that are not encodable by encoding/gob, use NewValueWithCloner
// to provide a custom cloner.
func NewValue[T any](v T) (Value[T], error) {
	// exported wrapper so gob can encode fields of T if needed
	type exported[T any] struct {
		V T
	}

	ev := exported[T]{V: v}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&ev); err != nil {
		return nil, errorx.IllegalArgument.Wrap(err,
			"value is not encodable by encoding/gob; register concrete types with RegisterGobTypes or provide a custom cloner")
	}

	return &defaultValue[T]{v: v, cloner: nil}, nil
}

// NewValueWithCloner constructs a new Value instance with a custom cloner
// function. The provided clonerFunc is used to create a deep copy of the
// value when Clone is called. If clonerFunc is nil, the default gob-based
// cloning will be used.
func NewValueWithCloner[T any](v T, clonerFunc func(v T) (Value[T], error)) Value[T] {
	return &defaultValue[T]{v: v, cloner: clonerFunc}
}

// Val returns the underlying value stored in the defaultValue.
//
// This method is nil-receiver safe: calling Val on a nil *defaultValue
// returns the zero value of T to avoid panics.
func (v *defaultValue[T]) Val() T {
	if v == nil {
		var zero T
		return zero
	}
	return v.v
}

// Clone returns a deep copy of the Value.
//
// Behavior:
// - If the receiver is nil, returns an error (errorx.IllegalState).
// - If a custom cloner is provided, it is used to clone the value.
// - Otherwise encoding/gob is used to encode+decode a copy of the value.
//
// Errors are wrapped with errorx.IllegalState to indicate cloning failures.
func (v *defaultValue[T]) Clone() (Value[T], error) {
	if v == nil {
		return nil, errorx.IllegalState.New("cannot clone nil Value")
	}

	if v.cloner != nil {
		cloned, err := v.cloner(v.v)
		if err != nil {
			return nil, errorx.IllegalState.Wrap(err,
				"failed to clone value using custom clone")
		}
		// preserve custom cloner on the returned value
		return NewValueWithCloner(cloned.Val(), v.cloner), nil
	}

	// exported wrapper so gob can encode fields of T if needed
	type exported[T any] struct {
		V T
	}

	ev := exported[T]{V: v.v}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&ev); err != nil {
		return nil, errorx.IllegalState.Wrap(err,
			"failed to gob.encode value for cloning, "+
				"ensure all nested types are encodable with gob or provide a custom clone")
	}

	var ev2 exported[T]
	if err := gob.NewDecoder(&buf).Decode(&ev2); err != nil {
		return nil, errorx.IllegalState.Wrap(err,
			"failed to gob.decode value for cloning, "+
				"ensure all nested types are decodable with gob or provide a custom clone")
	}

	return NewValue(ev2.V)
}

// EffectiveFunc is a function type used to compute the effective Value[T]
// based on the provided defaultVal and userInput.
//
// The function should return the computed Value[T], a boolean indicating
// whether the result should be cached for future calls, and an error if
// the computation fails.
//
// Semantics:
//   - If error != nil the call fails.
//   - If returned Value[T] is nil, the call fails (treated as an implementation error).
//   - If shouldCache == true the first successful non-nil result will be cached
//     in the RuntimeValue instance and returned to subsequent callers.
//   - If shouldCache == false the computed result will be returned to the caller
//     but not cached; subsequent calls will re-evaluate.
//
// The function is expected to be side-effect-free if caching is enabled.
type EffectiveFunc[T any] func(ctx context.Context, defaultVal Value[T], userInput Value[T]) (Value[T], bool, error)

// RuntimeValue is a generic container for a default value together with
// optional user input and an optional effective function.
//
// Resolution semantics:
//   - If an effectiveFunc is provided, it is invoked during the first call
//     to Effective() to compute the effective value. The function is expected
//     to return a non-nil Value and to be side-effect-free when it returns shouldCache=true.
//   - If effectiveFunc is not provided, the effective value is determined
//     at construction time: userInput (if provided) otherwise defaultVal.
//   - Default() returns the configured defaultVal.
//   - UserInput() returns the configured userInput, if any.
//
// Concurrency:
//   - RuntimeValue is safe for concurrent use by multiple goroutines.
//   - The type uses an internal sync.RWMutex for protecting its fields and
//     golang.org/x/sync/singleflight to deduplicate concurrent evaluations
//     of effectiveFunc when the cached effective value is not yet set.
//   - Provided Value implementations are assumed to be safe for concurrent use
//     or immutable; if they are mutable, callers should take care to clone or
//     otherwise protect them.
type RuntimeValue[T any] struct {
	mu            sync.RWMutex
	effective     Value[T]
	defaultVal    Value[T]
	userInput     Value[T]
	effectiveFunc EffectiveFunc[T]

	sf singleflight.Group
}

// Effective returns the effective Value for the runtime value.
//
// If an effectiveFunc is provided, it is invoked to compute the effective
// value on demand. If the effectiveFunc returns a value with shouldCache==true,
// the first successful non-nil result is cached for subsequent calls. If
// shouldCache==false the computed result is returned but not cached.
//
// Concurrency: this method is safe for concurrent callers. It uses a
// read-mostly pattern: read locks for fast returns and a write lock only when
// the effective value must be cached after computing it via effectiveFunc.
// Concurrent evaluations of effectiveFunc are deduplicated using singleflight.
func (v *RuntimeValue[T]) Effective(ctx context.Context) (Value[T], error) {
	if v == nil {
		return nil, errorx.IllegalState.New("RuntimeValue receiver is nil")
	}

	// Snapshot under read lock
	v.mu.RLock()
	eff := v.effective
	effFunc := v.effectiveFunc
	def := v.defaultVal
	user := v.userInput
	v.mu.RUnlock()

	// fast path: already resolved or no effective function configured
	if eff != nil || effFunc == nil {
		return eff, nil
	}

	// Use singleflight to dedupe concurrent evaluations for this instance.
	res, err, _ := v.sf.Do("effective", func() (interface{}, error) {
		val, shouldCache, err := effFunc(ctx, def, user)
		if err != nil {
			return nil, err
		}
		if val == nil {
			return nil, errorx.IllegalState.New("effectiveFunc returned nil Value")
		}

		// If caller requested not to cache, return value directly.
		if !shouldCache {
			return val, nil
		}

		// Cache the computed value if still unset.
		v.mu.Lock()
		if v.effective == nil {
			v.effective = val
		}
		cached := v.effective
		v.mu.Unlock()

		return cached, nil
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}

	vres, ok := res.(Value[T])
	if !ok {
		return nil, errorx.IllegalState.New("singleflight invocation for effectiveFunc returned unexpected type")
	}
	return vres, nil
}

// Default returns the configured default Value.
//
// This method is nil-receiver safe: calling Default on a nil *RuntimeValue
// returns nil.
func (v *RuntimeValue[T]) Default() Value[T] {
	if v == nil {
		return nil
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.defaultVal
}

// UserInput returns the configured user input Value, if any.
//
// This method is nil-receiver safe: calling UserInput on a nil *RuntimeValue
// returns nil.
func (v *RuntimeValue[T]) UserInput() Value[T] {
	if v == nil {
		return nil
	}
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.userInput
}

// Clone produces a deep copy of the RuntimeValue.
//
// The cloned RuntimeValue preserves the effectiveFunc reference and clones
// effective, defaultVal and userInput (if non-nil) using their Clone methods.
//
// If the receiver is nil, Clone returns an error (errorx.IllegalState).
//
// Clone snapshots the pointers under a read lock, then performs the potentially
// expensive Clone operations outside the lock to avoid blocking concurrent readers.
//
// Note: the singleflight.Group is intentionally not copied (zero-value is fine).
// The clone will deduplicate its own concurrent Effective invocations.
func (v *RuntimeValue[T]) Clone() (*RuntimeValue[T], error) {
	if v == nil {
		return nil, errorx.IllegalState.New("cannot clone nil RuntimeValue")
	}

	// Snapshot under read lock
	v.mu.RLock()
	eff := v.effective
	def := v.defaultVal
	user := v.userInput
	effFunc := v.effectiveFunc
	v.mu.RUnlock()

	var err error
	var ceff, cdef, cuser Value[T]

	if eff != nil {
		ceff, err = eff.Clone()
		if err != nil {
			return nil, err
		}
	}
	if def != nil {
		cdef, err = def.Clone()
		if err != nil {
			return nil, err
		}
	}
	if user != nil {
		cuser, err = user.Clone()
		if err != nil {
			return nil, err
		}
	}

	c := &RuntimeValue[T]{
		effective:     ceff,
		defaultVal:    cdef,
		userInput:     cuser,
		effectiveFunc: effFunc,
	}

	return c, nil
}

// ValueOption is a functional option used to configure a RuntimeValue.
type ValueOption[T any] func(*RuntimeValue[T])

// WithEffectiveFunc returns a ValueOption that sets the effective function
// which will be invoked by Effective.
//
// Note: these option setters mutate fields without internal locking and are
// intended for construction-time configuration only. For runtime-safe updates
// use the SetEffectiveFunc method on RuntimeValue.
func WithEffectiveFunc[T any](f EffectiveFunc[T]) ValueOption[T] {
	return func(v *RuntimeValue[T]) {
		v.effectiveFunc = f
	}
}

// WithUserInput returns a ValueOption that sets a user input Value to be
// used as the effective value (unless an effectiveFunc is provided).
//
// Note: see WithEffectiveFunc regarding runtime mutation.
func WithUserInput[T any](userInput Value[T]) ValueOption[T] {
	return func(v *RuntimeValue[T]) {
		v.userInput = userInput
	}
}

// NewRuntimeValue constructs a new RuntimeValue instance.
//
// The defaultVal must be provided and cannot be nil. Optional ValueOptions
// may set userInput and effectiveFunc. If effectiveFunc is not provided,
// the effective value is set to userInput if present, otherwise to defaultVal.
func NewRuntimeValue[T any](defaultVal Value[T], opts ...ValueOption[T]) (*RuntimeValue[T], error) {
	if defaultVal == nil {
		return nil, IllegalArgument.New("defaultVal must be provided and cannot be nil")
	}

	v := &RuntimeValue[T]{
		defaultVal: defaultVal,
	}

	for _, opt := range opts {
		opt(v)
	}

	// Determine effective defaultValue if effectiveFunc is not provided
	if v.effectiveFunc == nil {
		if v.userInput != nil {
			v.effective = v.userInput
		} else {
			v.effective = defaultVal
		}
	}

	return v, nil
}

// SetEffectiveFunc safely sets a new EffectiveFunc at runtime.
// It clears any cached effective value to ensure the new function will be used.
func (v *RuntimeValue[T]) SetEffectiveFunc(f EffectiveFunc[T]) {
	if v == nil {
		return
	}
	v.mu.Lock()
	v.effectiveFunc = f
	v.effective = nil
	v.mu.Unlock()
}

// SetUserInput safely sets (or clears) the user input Value at runtime.
// If an effectiveFunc is not configured the effective value is updated
// immediately to reflect the new userInput; otherwise the cached effective
// is cleared so future calls will re-evaluate.
func (v *RuntimeValue[T]) SetUserInput(user Value[T]) {
	if v == nil {
		return
	}
	v.mu.Lock()
	v.userInput = user
	if v.effectiveFunc == nil {
		if user != nil {
			v.effective = user
		} else {
			v.effective = v.defaultVal
		}
	} else {
		v.effective = nil
	}
	v.mu.Unlock()
}

// ClearCache clears any cached effective value so subsequent Effective calls
// will re-evaluate according to the configured effectiveFunc (if any).
func (v *RuntimeValue[T]) ClearCache() {
	if v == nil {
		return
	}
	v.mu.Lock()
	v.effective = nil
	v.mu.Unlock()
}
