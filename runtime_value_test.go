package automa

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joomcode/errorx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test gob-based cloning produces an independent deep copy for pointer-with-map types.
func TestNewValue_GobDeepCopyPointerMap(t *testing.T) {
	type S struct {
		M map[string]int
	}

	orig := &S{M: map[string]int{"a": 1}}
	v, err := NewValue[*S](orig)
	if err != nil {
		t.Fatalf("NewValue failed: %v", err)
	}

	cloneV, err := v.Clone()
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	cloned := cloneV.Val()

	// Mutate original
	orig.M["a"] = 42

	if cloned.M["a"] != 1 {
		t.Fatalf("expected cloned map to keep original value 1, got %d", cloned.M["a"])
	}
}

// Test NewValue returns an error for a type that is not encodable by encoding/gob (contains a func).
func TestNewValue_InvalidGobType(t *testing.T) {
	type Bad struct {
		F func()
	}

	_, err := NewValue(Bad{F: func() {}})
	if err == nil {
		t.Fatalf("expected NewValue to fail for non-gob-encodable type, got nil error")
	}
}

// Test that NewValueWithCloner uses the provided custom cloner.
func TestNewValueWithCloner_UsesCustomCloner(t *testing.T) {
	type NonEnc struct {
		Ch chan int
	}

	called := false
	var cloner func(NonEnc) (Value[NonEnc], error)
	cloner = func(v NonEnc) (Value[NonEnc], error) {
		called = true
		// return a new Value using NewValueWithCloner to avoid gob encoding
		return NewValueWithCloner(NonEnc{Ch: v.Ch}, cloner), nil
	}

	val := NewValueWithCloner(NonEnc{Ch: make(chan int)}, cloner)

	cloned, err := val.Clone()
	if err != nil {
		t.Fatalf("Clone with custom cloner returned error: %v", err)
	}
	if !called {
		t.Fatalf("expected custom cloner to be called")
	}
	_ = cloned.Val() // ensure we can inspect the value without panicking
}

// Test RuntimeValue cloning deep-copies the contained Values.
func TestRuntimeValue_CloneDeepCopiesInternalValues(t *testing.T) {
	type S struct {
		M map[string]int
	}

	orig := &S{M: map[string]int{"x": 1}}
	defVal, err := NewValue[*S](orig)
	if err != nil {
		t.Fatalf("NewValue failed: %v", err)
	}

	rv, err := NewRuntimeValue[*S](defVal)
	require.NoError(t, err)
	require.NotNil(t, rv)

	clonedRV, err := rv.Clone()
	if err != nil {
		t.Fatalf("RuntimeValue.Clone failed: %v", err)
	}

	// Modify original
	orig.M["x"] = 99

	if rv.Default().Val().M["x"] != 99 {
		t.Fatalf("expected original RuntimeValue default to reflect mutated value 99, got %d", rv.Default().Val().M["x"])
	}

	// cloned default should remain unchanged
	clonedDefault := clonedRV.Default().Val()
	if clonedDefault.M["x"] != 1 {
		t.Fatalf("expected cloned default to keep value 1, got %d", clonedDefault.M["x"])
	}

}

// Test that WithUserInput makes userInput the effective value and Clone deep-copies default and userInput.
func TestRuntimeValue_UserInputBecomesEffectiveAndCloneDeepCopies(t *testing.T) {
	type S struct {
		M map[string]int
	}

	origDef := &S{M: map[string]int{"d": 1}}
	origUser := &S{M: map[string]int{"u": 2}}

	defVal, err := NewValue[*S](origDef)
	if err != nil {
		t.Fatalf("NewValue(def) failed: %v", err)
	}

	userVal, err := NewValue[*S](origUser)
	if err != nil {
		t.Fatalf("NewValue(user) failed: %v", err)
	}

	rv, err := NewRuntimeValue[*S](defVal, WithUserInput[*S](userVal))
	require.NoError(t, err)
	require.NotNil(t, rv)

	// Effective should be userInput
	eff, err := rv.Effective()
	if err != nil {
		t.Fatalf("Effective returned error: %v", err)
	}
	if eff == nil {
		t.Fatalf("Effective returned nil value")
	}
	if eff.Get().Val().M["u"] != 2 {
		t.Fatalf("expected effective user value 2, got %d", eff.Get().Val().M["u"])
	}

	// Clone and then mutate originals; clone should remain unchanged
	clone, err := rv.Clone()
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}
	// mutate originals
	origDef.M["d"] = 99
	origUser.M["u"] = 99

	// cloned default should keep original default value 1
	clonedDef := clone.Default()
	if clonedDef == nil {
		t.Fatalf("cloned default is nil")
	}
	if clonedDef.Val().M["d"] != 1 {
		t.Fatalf("expected cloned default d==1, got %d", clonedDef.Val().M["d"])
	}

	// cloned effective (userInput) should keep original user value 2
	clonedEff, err := clone.Effective()
	if err != nil {
		t.Fatalf("cloned Effective returned error: %v", err)
	}
	if clonedEff == nil {
		t.Fatalf("cloned Effective is nil")
	}
	if clonedEff.Get().Val().M["u"] != 2 {
		t.Fatalf("expected cloned effective u==2, got %d", clonedEff.Get().Val().M["u"])
	}
}

// Test that effectiveFunc is invoked once for the original (cached) and preserved on Clone,
// where the clone will not invoke it since the cached effective is copied into the clone.
func TestRuntimeValue_EffectiveFuncInvokedAndPreservedOnClone(t *testing.T) {
	type S struct {
		M map[string]int
	}

	var counter int32
	effFunc := func(ctx context.Context, _ Value[*S], _ Value[*S]) (*EffectiveValue[*S], bool, error) {
		v := atomic.AddInt32(&counter, 1)
		val, err := NewValue(&S{M: map[string]int{"ctr": int(v)}})
		if err != nil {
			return nil, false, err
		}

		ev, err := NewEffectiveValue(val, StrategyCustom)
		if err != nil {
			return nil, false, err
		}

		// cache the computed result
		return ev, true, nil
	}

	// provide a non-nil defaultVal as required by NewRuntimeValue
	defVal, err := NewValue(&S{M: map[string]int{}})
	if err != nil {
		t.Fatalf("NewValue(default) failed: %v", err)
	}

	// create a runtime value with an effectiveFunc
	rv, err := NewRuntimeValue[*S](defVal, WithEffectiveFunc[*S](effFunc))
	require.NoError(t, err)
	require.NotNil(t, rv)

	// first call computes and caches (counter -> 1)
	v1, err := rv.Effective()
	if err != nil {
		t.Fatalf("Effective call 1 failed: %v", err)
	}
	if v1.Get().Val().M["ctr"] != 1 {
		t.Fatalf("expected ctr==1 on first call, got %d", v1.Get().Val().M["ctr"])
	}
	assert.Equal(t, StrategyCustom, v1.Strategy())

	// second call should return cached value (counter still 1)
	v2, err := rv.Effective()
	if err != nil {
		t.Fatalf("Effective call 2 failed: %v", err)
	}
	if v2.Get().Val().M["ctr"] != 1 {
		t.Fatalf("expected ctr==1 on second call (cached), got %d", v2.Get().Val().M["ctr"])
	}
	assert.Equal(t, StrategyCustom, v2.Strategy())

	// Clone the runtime value; cloned effective is copied, so clone should not invoke effFunc
	clone, err := rv.Clone()
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	cv, err := clone.Effective()
	if err != nil {
		t.Fatalf("clone Effective failed: %v", err)
	}
	// clone had cached effective, so counter remains 1
	if cv.Get().Val().M["ctr"] != 1 {
		t.Fatalf("expected ctr==1 after clone Effective, got %d", cv.Get().Val().M["ctr"])
	}
}

// Test that NewRuntimeValue returns an error if defaultVal is nil.
func TestNewRuntimeValue_NilDefaultReturnsError(t *testing.T) {
	rv, err := NewRuntimeValue[*int](nil)
	require.Error(t, err)
	require.Nil(t, rv)
	require.True(t, errorx.IsOfType(err, IllegalArgument))
	require.Contains(t, err.Error(), "defaultVal must be provided")
}

// Test that concurrent Effective calls are deduplicated using singleflight.
func TestRuntimeValue_SingleFlightDedupesConcurrentEvaluations(t *testing.T) {
	type S struct {
		M map[string]int
	}

	var counter int32
	effFunc := func(ctx context.Context, _ Value[*S], _ Value[*S]) (*EffectiveValue[*S], bool, error) {
		// simulate expensive work
		time.Sleep(100 * time.Millisecond)
		v := atomic.AddInt32(&counter, 1)
		val, err := NewValue(&S{M: map[string]int{"ctr": int(v)}})
		if err != nil {
			return nil, false, err
		}

		ev, err := NewEffectiveValue(val, StrategyCustom)
		if err != nil {
			return nil, false, err
		}

		// cache the computed result
		return ev, true, nil
	}

	defVal, err := NewValue(&S{M: map[string]int{}})
	if err != nil {
		t.Fatalf("NewValue(default) failed: %v", err)
	}

	rv, err := NewRuntimeValue[*S](defVal, WithEffectiveFunc[*S](effFunc))
	require.NoError(t, err)
	require.NotNil(t, rv)

	var wg sync.WaitGroup
	const goroutines = 20
	results := make([]*EffectiveValue[*S], goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			v, e := rv.Effective()
			results[idx] = v
			errs[idx] = e
		}(i)
	}
	wg.Wait()

	// ensure effectiveFunc ran exactly once
	if atomic.LoadInt32(&counter) != 1 {
		t.Fatalf("expected effectiveFunc to be invoked once, got %d", atomic.LoadInt32(&counter))
	}

	for i := 0; i < goroutines; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: Effective returned error: %v", i, errs[i])
		}
		if results[i] == nil {
			t.Fatalf("goroutine %d: Effective returned nil", i)
		}
		if results[i].Get().Val().M["ctr"] != 1 {
			t.Fatalf("goroutine %d: expected ctr==1, got %d", i, results[i].Get().Val().M["ctr"])
		}
	}
}

// Test default effective when no effectiveFunc configured.
func TestRuntimeValue_DefaultWhenNoEffectiveFunc(t *testing.T) {
	type S struct{ V int }
	orig := &S{V: 7}
	defVal, err := NewValue[*S](orig)
	require.NoError(t, err)

	rv, err := NewRuntimeValue[*S](defVal)
	require.NoError(t, err)

	eff, err := rv.Effective()
	require.NoError(t, err)
	require.NotNil(t, eff)
	require.Equal(t, 7, eff.Get().Val().V)
	assert.Equal(t, StrategyDefault, eff.Strategy())
}

// Test SetUserInput updates effective when no effectiveFunc and clears cache when effectiveFunc present.
func TestRuntimeValue_SetUserInputUpdatesEffective(t *testing.T) {
	type S struct{ V int }

	origDef := &S{V: 1}
	origUser := &S{V: 42}

	defVal, err := NewValue[*S](origDef)
	require.NoError(t, err)
	userVal, err := NewValue[*S](origUser)
	require.NoError(t, err)

	rv, err := NewRuntimeValue[*S](defVal)
	require.NoError(t, err)

	// initially effective is default
	eff, err := rv.Effective()
	require.NoError(t, err)
	require.Equal(t, 1, eff.Get().Val().V)

	// set user input, effective should switch
	rv.SetUserInput(userVal)
	eff2, err := rv.Effective()
	require.NoError(t, err)
	require.Equal(t, 42, eff2.Get().Val().V)
	assert.Equal(t, StrategyDefault, eff.Strategy())

	// now configure an effectiveFunc that caches a computed value
	var counter int32
	effFunc := func(ctx context.Context, _ Value[*S], _ Value[*S]) (*EffectiveValue[*S], bool, error) {
		v := atomic.AddInt32(&counter, 1)
		val, err := NewValue(&S{V: int(v)})
		if err != nil {
			return nil, false, err
		}

		ev, err := NewEffectiveValue(val, StrategyCustom)
		if err != nil {
			return nil, false, err
		}

		// cache the computed result
		return ev, true, nil
	}

	rv.SetEffectiveFunc(effFunc)
	// first Effective will compute & cache
	v1, err := rv.Effective()
	require.NoError(t, err)
	require.Equal(t, 1, v1.Get().Val().V)
	assert.Equal(t, StrategyCustom, v1.Strategy())

	// set user input while effectiveFunc configured -> cache cleared
	rv.SetUserInput(userVal)
	// since effectiveFunc present, Effective will recompute using effFunc
	v2, err := rv.Effective()
	require.NoError(t, err)
	require.Equal(t, 2, v2.Get().Val().V)
	assert.Equal(t, StrategyCustom, v2.Strategy())

}

// Test SetEffectiveFunc clears previous cache and new function is used.
func TestRuntimeValue_SetEffectiveFuncClearsCache(t *testing.T) {
	type S struct{ V int }

	defVal, err := NewValue(&S{V: 0})
	require.NoError(t, err)

	var c1 int32
	f1 := func(ctx context.Context, _ Value[*S], _ Value[*S]) (*EffectiveValue[*S], bool, error) {
		v := atomic.AddInt32(&c1, 1)
		val, err := NewValue(&S{V: int(v)})
		if err != nil {
			return nil, false, err
		}

		ev, err := NewEffectiveValue(val, StrategyCustom)
		if err != nil {
			return nil, false, err
		}

		return ev, true, nil
	}

	rv, err := NewRuntimeValue[*S](defVal, WithEffectiveFunc[*S](f1))
	require.NoError(t, err)

	v1, err := rv.Effective()
	require.NoError(t, err)
	require.Equal(t, 1, v1.Get().Val().V)

	// setting nil effectiveFunc won't set it
	assert.NotNil(t, rv.effectiveFunc)
	rv.SetEffectiveFunc(nil)
	assert.NotNil(t, rv.effectiveFunc)

	// replace effective func
	var c2 int32
	f2 := func(ctx context.Context, _ Value[*S], _ Value[*S]) (*EffectiveValue[*S], bool, error) {
		v := atomic.AddInt32(&c2, 1)
		val, err := NewValue(&S{V: int(v * 10)})
		if err != nil {
			return nil, false, err
		}

		ev, err := NewEffectiveValue(val, StrategyCustom)
		if err != nil {
			return nil, false, err
		}

		return ev, true, nil
	}

	rv.SetEffectiveFunc(f2)
	v2, err := rv.Effective()
	require.NoError(t, err)
	require.Equal(t, 10, v2.Get().Val().V)
}

// Test that effectiveFunc returning shouldCache==false is re-evaluated on each call.
func TestRuntimeValue_Effective_ReevaluatesWhenNotCaching(t *testing.T) {
	type S struct{ V int }

	defVal, err := NewValue(&S{V: 0})
	require.NoError(t, err)

	var counter int32
	f := func(ctx context.Context, _ Value[*S], _ Value[*S]) (*EffectiveValue[*S], bool, error) {
		v := atomic.AddInt32(&counter, 1)
		val, err := NewValue(&S{V: int(v)})
		if err != nil {
			return nil, false, err
		}

		ev, err := NewEffectiveValue(val, StrategyCustom)
		if err != nil {
			return nil, false, err
		}

		return ev, false, nil
	}

	rv, err := NewRuntimeValue[*S](defVal, WithEffectiveFunc[*S](f))
	require.NoError(t, err)

	a, err := rv.Effective()
	require.NoError(t, err)
	require.Equal(t, 1, a.Get().Val().V)

	b, err := rv.Effective()
	require.NoError(t, err)
	require.Equal(t, 2, b.Get().Val().V)
}

// Test cloning a nil *defaultValue returns an error.
func TestDefaultValue_CloneNilReceiver(t *testing.T) {
	var dv *defaultValue[int] = nil
	_, err := dv.Clone()
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot clone nil Value")
}

func TestValue_Val(t *testing.T) {
	v, err := NewValue[int](5)
	require.NoError(t, err)
	require.Equal(t, 5, v.Val())
}

func TestRuntimeValue_UserInput(t *testing.T) {
	v, err := NewValue(10)
	require.NoError(t, err)

	rv, err := NewRuntimeValue[int](v)
	require.NoError(t, err)

	ui := rv.UserInput()
	require.Nil(t, ui)

	uv, err := NewValue(20)
	require.NoError(t, err)

	rv.SetUserInput(uv)

	ui2 := rv.UserInput()
	require.NotNil(t, ui2)
	require.Equal(t, 20, ui2.Val())

	// nil-receiver: calling UserInput on a nil *RuntimeValue should return nil
	var nilRV *RuntimeValue[int] = nil
	nilUI := nilRV.UserInput()
	require.Nil(t, nilUI)
}

func TestRuntimeValue_ClearCache(t *testing.T) {
	type S struct{ V int }

	defVal, err := NewValue(&S{V: 0})
	require.NoError(t, err)

	var counter int32
	f := func(ctx context.Context, _ Value[*S], _ Value[*S]) (*EffectiveValue[*S], bool, error) {
		v := atomic.AddInt32(&counter, 1)
		val, err := NewValue(&S{V: int(v)})
		if err != nil {
			return nil, false, err
		}

		ev, err := NewEffectiveValue(val, StrategyCustom)
		if err != nil {
			return nil, false, err
		}

		return ev, true, nil
	}

	rv, err := NewRuntimeValue[*S](defVal, WithEffectiveFunc[*S](f))
	require.NoError(t, err)

	// First call should compute & cache (counter == 1)
	v1, err := rv.Effective()
	require.NoError(t, err)
	require.Equal(t, 1, v1.Get().Val().V)

	// Clear cache and ensure next call recomputes (counter == 2)
	rv.ClearCache()
	v2, err := rv.Effective()
	require.NoError(t, err)
	require.Equal(t, 2, v2.Get().Val().V)

	// nil-receiver: calling ClearCache on a nil *RuntimeValue should be safe (no panic)
	var nilRV *RuntimeValue[*S] = nil
	nilRV.ClearCache()
}

// Test that WithContext stores the provided context and that Effective()
// uses the stored context when invoking the EffectiveFunc.
func TestRuntimeValue_WithContext_EffectiveReceivesContext(t *testing.T) {
	type S struct{ V int }

	seen := false
	effFunc := func(ctx context.Context, _ Value[*S], _ Value[*S]) (*EffectiveValue[*S], bool, error) {
		// ensure the context value is propagated
		if ctx.Value("test-key") == "test-val" {
			seen = true
		}
		val, err := NewValue(&S{V: 123})
		if err != nil {
			return nil, false, err
		}
		ev, err := NewEffectiveValue(val, StrategyCustom)
		if err != nil {
			return nil, false, err
		}
		return ev, true, nil
	}

	defVal, err := NewValue(&S{V: 0})
	require.NoError(t, err)

	rv, err := NewRuntimeValue[*S](defVal, WithEffectiveFunc[*S](effFunc))
	require.NoError(t, err)
	require.NotNil(t, rv)

	ctx := context.WithValue(context.Background(), "test-key", "test-val")
	rv = rv.WithContext(ctx)

	ev, err := rv.Effective()
	require.NoError(t, err)
	require.NotNil(t, ev)
	assert.True(t, seen, "effectiveFunc should receive the context set via WithContext")
	assert.Equal(t, 123, ev.Get().Val().V)
}

// Test Clone deep-copies contained Values and preserves the stored context.
func TestRuntimeValue_ClonePreservesContextAndDeepCopies(t *testing.T) {
	type S struct {
		M map[string]int
	}

	origDef := &S{M: map[string]int{"d": 1}}
	origUser := &S{M: map[string]int{"u": 2}}

	defVal, err := NewValue[*S](origDef)
	require.NoError(t, err)
	userVal, err := NewValue[*S](origUser)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), "ctx-key", "ctx-val")

	rv, err := NewRuntimeValue[*S](defVal, WithUserInput[*S](userVal))
	require.NoError(t, err)
	require.NotNil(t, rv)
	rv = rv.WithContext(ctx)

	clone, err := rv.Clone()
	require.NoError(t, err)
	require.NotNil(t, clone)

	// Mutate original underlying objects after cloning
	origDef.M["d"] = 99
	origUser.M["u"] = 99

	// cloned default and user input should preserve original values
	clonedDef := clone.Default()
	require.NotNil(t, clonedDef)
	assert.Equal(t, 1, clonedDef.Val().M["d"])

	clonedUser := clone.UserInput()
	require.NotNil(t, clonedUser)
	assert.Equal(t, 2, clonedUser.Val().M["u"])

	// clone should preserve stored context value
	require.NotNil(t, rv)
	require.NotNil(t, clone)
	assert.Equal(t, rv.ctx.Value("ctx-key"), clone.ctx.Value("ctx-key"))
}

// Test that Clone on a nil *RuntimeValue returns an error.
func TestRuntimeValue_CloneNilReceiver(t *testing.T) {
	var rv *RuntimeValue[int] = nil
	_, err := rv.Clone()
	require.Error(t, err)
	assert.True(t, errorx.IsOfType(err, errorx.IllegalState) || errorx.IsOfType(err, IllegalArgument))
}
