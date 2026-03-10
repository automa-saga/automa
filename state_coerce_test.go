package automa

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// toInt64 — unsigned overflow protection
// ---------------------------------------------------------------------------

func TestToInt64_UnsignedOverflowRejected(t *testing.T) {
	// uint64 > math.MaxInt64 should fail
	_, ok := toInt64(uint64(math.MaxUint64))
	assert.False(t, ok)

	_, ok = toInt64(uint64(math.MaxInt64 + 1))
	assert.False(t, ok)

	// uint64 == math.MaxInt64 should succeed
	got, ok := toInt64(uint64(math.MaxInt64))
	assert.True(t, ok)
	assert.Equal(t, int64(math.MaxInt64), got)

	// uint64 zero should succeed
	got, ok = toInt64(uint64(0))
	assert.True(t, ok)
	assert.Equal(t, int64(0), got)
}

func TestToInt64_SmallUnsignedTypesAlwaysFit(t *testing.T) {
	// uint8, uint16, uint32 always fit in int64
	got, ok := toInt64(uint8(255))
	assert.True(t, ok)
	assert.Equal(t, int64(255), got)

	got, ok = toInt64(uint16(65535))
	assert.True(t, ok)
	assert.Equal(t, int64(65535), got)

	got, ok = toInt64(uint32(math.MaxUint32))
	assert.True(t, ok)
	assert.Equal(t, int64(math.MaxUint32), got)
}

func TestFromState_LargeUint64ToInt64Rejected(t *testing.T) {
	s := &SyncStateBag{}

	// Large uint64 that overflows int64 should return zero when requesting int/int64
	s.Set("big", uint64(math.MaxUint64))
	assert.Equal(t, 0, FromState[int](s, "big", 0))
	assert.Equal(t, int64(0), FromState[int64](s, "big", 0))

	// But should work for uint64 target
	assert.Equal(t, uint64(math.MaxUint64), FromState[uint64](s, "big", 0))
}

// ---------------------------------------------------------------------------
// toInt64 — float→int64 bounds: the old code used float64(math.MaxInt64) which
// rounds UP to 2^63, making the > check wrong. The fixed code uses 2^63 as the
// exclusive upper bound (>= check). These tests pin that behaviour.
// ---------------------------------------------------------------------------

func TestToInt64_FloatBoundsExact(t *testing.T) {
	// 2^63 as float64 — exactly 9223372036854775808.0.
	// float64(math.MaxInt64) rounds UP to this same value, which is why the old
	// `tr > float64(math.MaxInt64)` check was broken: it was `tr > 2^63` and
	// never fired for values in (MaxInt64, 2^63]. The fix uses `tr >= 2^63`.
	const pow63 float64 = 9223372036854775808.0 // 2^63, exactly representable

	// 2^63 itself must be rejected (overflows int64)
	_, ok := toInt64(pow63)
	assert.False(t, ok, "2^63 as float64 must be rejected")

	// Nextafter gives the largest float64 strictly less than 2^63 — must be accepted
	justBelow := math.Nextafter(pow63, 0)
	got, ok := toInt64(justBelow)
	assert.True(t, ok, "largest float64 < 2^63 must be accepted")
	assert.Equal(t, int64(justBelow), got)

	// -(2^63) == math.MinInt64 exactly — must be accepted
	got, ok = toInt64(-pow63)
	assert.True(t, ok, "-(2^63) must be accepted")
	assert.Equal(t, int64(math.MinInt64), got)

	// One ULP below -(2^63) must be rejected
	justBelowMin := math.Nextafter(-pow63, math.Inf(-1))
	_, ok = toInt64(justBelowMin)
	assert.False(t, ok, "value below -(2^63) must be rejected")
}

func TestToInt64_Float32Bounds(t *testing.T) {
	// float32 max is ~3.4e38, way outside int64 range
	_, ok := toInt64(float32(math.MaxFloat32))
	assert.False(t, ok)

	_, ok = toInt64(float32(-math.MaxFloat32))
	assert.False(t, ok)

	// small float32 should work
	got, ok := toInt64(float32(42.9))
	assert.True(t, ok)
	assert.Equal(t, int64(42), got) // truncation toward zero
}

func TestToInt64_StringFloatBounds(t *testing.T) {
	// string representing a value way outside int64 range
	_, ok := toInt64("1e30")
	assert.False(t, ok)

	_, ok = toInt64("-1e30")
	assert.False(t, ok)

	// string float within range
	got, ok := toInt64("3.9")
	assert.True(t, ok)
	assert.Equal(t, int64(3), got)
}

func TestToInt64_JSONNumberFloatBounds(t *testing.T) {
	_, ok := toInt64(json.Number("1e30"))
	assert.False(t, ok)

	_, ok = toInt64(json.Number("-1e30"))
	assert.False(t, ok)

	got, ok := toInt64(json.Number("7.7"))
	assert.True(t, ok)
	assert.Equal(t, int64(7), got)
}

// ---------------------------------------------------------------------------
// toUint64Safe
// ---------------------------------------------------------------------------

func TestToUint64Safe_ValidIntegers(t *testing.T) {
	tests := []struct {
		name    string
		val     interface{}
		bitSize int
		want    uint64
		ok      bool
	}{
		{"int zero", 0, 64, 0, true},
		{"int positive", 42, 64, 42, true},
		{"int64 positive", int64(100), 64, 100, true},
		{"uint64 value", uint64(999), 64, 999, true},
		{"float64 integral", float64(7), 64, 7, true},
		{"float32 integral", float32(3), 32, 3, true},
		{"string integer", "255", 64, 255, true},
		{"json.Number", json.Number("12345"), 64, 12345, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := toUint64Safe(tc.val, tc.bitSize)
			assert.Equal(t, tc.ok, ok)
			if ok {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestToUint64Safe_NegativeValuesRejected(t *testing.T) {
	tests := []struct {
		name    string
		val     interface{}
		bitSize int
	}{
		{"negative int", -1, 64},
		{"negative int64", int64(-100), 64},
		{"negative string", "-1", 64},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := toUint64Safe(tc.val, tc.bitSize)
			assert.False(t, ok)
		})
	}
}

func TestToUint64Safe_NegativeFloatTruncatesToZero(t *testing.T) {
	// -0.5 truncates to 0 via toInt64, which is a valid uint value
	got, ok := toUint64Safe(float64(-0.5), 64)
	assert.True(t, ok)
	assert.Equal(t, uint64(0), got)
}

func TestToUint64Safe_OverflowRejected(t *testing.T) {
	tests := []struct {
		name    string
		val     interface{}
		bitSize int
	}{
		{"uint8 overflow 256", 256, 8},
		{"uint8 overflow 300", 300, 8},
		{"uint16 overflow", 65536, 16},
		{"uint32 overflow", int64(1 << 32), 32},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := toUint64Safe(tc.val, tc.bitSize)
			assert.False(t, ok)
		})
	}
}

func TestToUint64Safe_BoundaryValues(t *testing.T) {
	tests := []struct {
		name    string
		val     interface{}
		bitSize int
		want    uint64
		ok      bool
	}{
		{"uint8 max 255", 255, 8, 255, true},
		{"uint16 max", 65535, 16, 65535, true},
		{"uint32 max", int64(math.MaxUint32), 32, math.MaxUint32, true},
		{"uint8 zero", 0, 8, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := toUint64Safe(tc.val, tc.bitSize)
			assert.Equal(t, tc.ok, ok)
			if ok {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestToUint64Safe_LargeUint64ViaString(t *testing.T) {
	// Values larger than int64 max must be parsed via string path
	largeVal := "18446744073709551615" // math.MaxUint64
	got, ok := toUint64Safe(largeVal, 64)
	assert.True(t, ok)
	assert.Equal(t, uint64(math.MaxUint64), got)
}

func TestToUint64Safe_NonIntegralFloatTruncates(t *testing.T) {
	// Non-integral floats are truncated via toInt64 (truncation toward zero)
	got, ok := toUint64Safe(3.7, 64)
	assert.True(t, ok)
	assert.Equal(t, uint64(3), got)

	got, ok = toUint64Safe(float32(2.5), 32)
	assert.True(t, ok)
	assert.Equal(t, uint64(2), got)
}

func TestToUint64Safe_UnsupportedType(t *testing.T) {
	_, ok := toUint64Safe(struct{}{}, 64)
	assert.False(t, ok)

	_, ok = toUint64Safe(nil, 64)
	assert.False(t, ok)
}

// ---------------------------------------------------------------------------
// toUint64Safe — additional edge cases
// ---------------------------------------------------------------------------

func TestToUint64Safe_JSONNumberEdgeCases(t *testing.T) {
	// json.Number with a negative value
	_, ok := toUint64Safe(json.Number("-5"), 64)
	assert.False(t, ok)

	// json.Number with overflow for uint8
	_, ok = toUint64Safe(json.Number("300"), 8)
	assert.False(t, ok)

	// json.Number with valid uint8
	got, ok := toUint64Safe(json.Number("200"), 8)
	assert.True(t, ok)
	assert.Equal(t, uint64(200), got)

	// json.Number float integral
	got, ok = toUint64Safe(json.Number("10"), 64)
	assert.True(t, ok)
	assert.Equal(t, uint64(10), got)
}

func TestToUint64Safe_StringOverflowForSmallBitSize(t *testing.T) {
	// "256" should fail for 8-bit
	_, ok := toUint64Safe("256", 8)
	assert.False(t, ok)

	// "65536" should fail for 16-bit
	_, ok = toUint64Safe("65536", 16)
	assert.False(t, ok)

	// "255" should pass for 8-bit
	got, ok := toUint64Safe("255", 8)
	assert.True(t, ok)
	assert.Equal(t, uint64(255), got)
}

// ---------------------------------------------------------------------------
// coerceToString
// ---------------------------------------------------------------------------

func TestCoerceToString_Primitives(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want string
		ok   bool
	}{
		{"string passthrough", "hello", "hello", true},
		{"bool true", true, "true", true},
		{"bool false", false, "false", true},
		{"int", 42, "42", true},
		{"int8", int8(-5), "-5", true},
		{"int16", int16(1000), "1000", true},
		{"int32", int32(100000), "100000", true},
		{"int64", int64(9999999999), "9999999999", true},
		{"uint", uint(7), "7", true},
		{"uint8", uint8(255), "255", true},
		{"uint16", uint16(65535), "65535", true},
		{"uint32", uint32(4294967295), "4294967295", true},
		{"uint64", uint64(18446744073709551615), "18446744073709551615", true},
		{"float64 integral", float64(42), "42", true},
		{"float64 fractional", float64(3.14), "3.14", true},
		{"float32 integral", float32(7), "7", true},
		{"float32 fractional", float32(2.5), "2.5", true},
		{"json.Number int", json.Number("99"), "99", true},
		{"json.Number float", json.Number("1.23"), "1.23", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := coerceToString(tc.val)
			assert.Equal(t, tc.ok, ok)
			if ok {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestCoerceToString_Stringer(t *testing.T) {
	type myStringer struct{}
	ms := myStringer{}
	// myStringer doesn't implement String(), so should fail
	_, ok := coerceToString(ms)
	assert.False(t, ok)
}

type testStringer struct{ v string }

func (s testStringer) String() string { return s.v }

func TestCoerceToString_ImplementsStringer(t *testing.T) {
	s, ok := coerceToString(testStringer{v: "custom"})
	assert.True(t, ok)
	assert.Equal(t, "custom", s)
}

func TestCoerceToString_NilAndUnsupported(t *testing.T) {
	_, ok := coerceToString(nil)
	assert.False(t, ok)

	_, ok = coerceToString(struct{}{})
	assert.False(t, ok)

	ch := make(chan int)
	_, ok = coerceToString(ch)
	assert.False(t, ok)
}

func TestCoerceToString_IntegralFloatNoTrailingZero(t *testing.T) {
	// 42.0 should become "42", not "42.0"
	s, ok := coerceToString(float64(42.0))
	assert.True(t, ok)
	assert.Equal(t, "42", s)

	s, ok = coerceToString(float32(100.0))
	assert.True(t, ok)
	assert.Equal(t, "100", s)
}

// ---------------------------------------------------------------------------
// coerceToString — additional edge cases
// ---------------------------------------------------------------------------

func TestCoerceToString_EmptyString(t *testing.T) {
	s, ok := coerceToString("")
	assert.True(t, ok)
	assert.Equal(t, "", s)
}

func TestCoerceToString_NegativeNumbers(t *testing.T) {
	s, ok := coerceToString(-42)
	assert.True(t, ok)
	assert.Equal(t, "-42", s)

	s, ok = coerceToString(float64(-3.14))
	assert.True(t, ok)
	assert.Equal(t, "-3.14", s)
}

func TestCoerceToString_LargeUint64(t *testing.T) {
	s, ok := coerceToString(uint64(math.MaxUint64))
	assert.True(t, ok)
	assert.Equal(t, "18446744073709551615", s)
}

func TestCoerceToString_ZeroValues(t *testing.T) {
	s, ok := coerceToString(0)
	assert.True(t, ok)
	assert.Equal(t, "0", s)

	s, ok = coerceToString(float64(0))
	assert.True(t, ok)
	assert.Equal(t, "0", s)

	s, ok = coerceToString(uint(0))
	assert.True(t, ok)
	assert.Equal(t, "0", s)
}

// ---------------------------------------------------------------------------
// FromState — comprehensive tests for the simplified version
// ---------------------------------------------------------------------------

func TestFromState_ExactTypeMatch(t *testing.T) {
	s := &SyncStateBag{}

	// Pointer type preserved via exact match before normalization
	v := 42
	s.Set("ptr", &v)
	got := FromState[*int](s, "ptr", nil)
	require.NotNil(t, got)
	assert.Equal(t, 42, *got)
}

func TestFromState_StringCoercion(t *testing.T) {
	s := &SyncStateBag{}

	// numeric to string
	s.Set("int", 42)
	assert.Equal(t, "42", FromState[string](s, "int", ""))

	s.Set("float", 3.14)
	assert.Equal(t, "3.14", FromState[string](s, "float", ""))

	s.Set("floatIntegral", 7.0)
	assert.Equal(t, "7", FromState[string](s, "floatIntegral", ""))

	s.Set("bool", true)
	assert.Equal(t, "true", FromState[string](s, "bool", ""))

	s.Set("uint64", uint64(999))
	assert.Equal(t, "999", FromState[string](s, "uint64", ""))

	// string passthrough
	s.Set("str", "hello")
	assert.Equal(t, "hello", FromState[string](s, "str", ""))
}

func TestFromState_BoolCoercion(t *testing.T) {
	s := &SyncStateBag{}

	s.Set("boolStr", "true")
	assert.Equal(t, true, FromState[bool](s, "boolStr", false))

	s.Set("boolStrFalse", "false")
	assert.Equal(t, false, FromState[bool](s, "boolStrFalse", true))

	s.Set("boolInt1", 1)
	assert.Equal(t, true, FromState[bool](s, "boolInt1", false))

	s.Set("boolInt0", 0)
	assert.Equal(t, false, FromState[bool](s, "boolInt0", true))

	s.Set("boolFloat", 0.0)
	assert.Equal(t, false, FromState[bool](s, "boolFloat", true))

	s.Set("boolNative", true)
	assert.Equal(t, true, FromState[bool](s, "boolNative", false))
}

func TestFromState_IntCoercion(t *testing.T) {
	s := &SyncStateBag{}

	// float64 -> int (truncation)
	s.Set("f64", float64(3.99))
	assert.Equal(t, 3, FromState[int](s, "f64", 0))

	// string -> int
	s.Set("str", "123")
	assert.Equal(t, 123, FromState[int](s, "str", 0))

	// int64 -> int
	s.Set("i64", int64(42))
	assert.Equal(t, 42, FromState[int](s, "i64", 0))

	// json.Number -> int
	s.Set("jn", json.Number("77"))
	assert.Equal(t, 77, FromState[int](s, "jn", 0))
}

func TestFromState_Int8_Int16_Int32_Int64(t *testing.T) {
	s := &SyncStateBag{}

	s.Set("val", float64(42))
	assert.Equal(t, int8(42), FromState[int8](s, "val", 0))
	assert.Equal(t, int16(42), FromState[int16](s, "val", 0))
	assert.Equal(t, int32(42), FromState[int32](s, "val", 0))
	assert.Equal(t, int64(42), FromState[int64](s, "val", 0))
}

func TestFromState_UintCoercion(t *testing.T) {
	s := &SyncStateBag{}

	s.Set("val", 42)
	assert.Equal(t, uint(42), FromState[uint](s, "val", 0))
	assert.Equal(t, uint8(42), FromState[uint8](s, "val", 0))
	assert.Equal(t, uint16(42), FromState[uint16](s, "val", 0))
	assert.Equal(t, uint32(42), FromState[uint32](s, "val", 0))
	assert.Equal(t, uint64(42), FromState[uint64](s, "val", 0))
}

func TestFromState_UintRejectsNegative(t *testing.T) {
	s := &SyncStateBag{}

	s.Set("neg", -1)
	assert.Equal(t, uint(0), FromState[uint](s, "neg", 0))
	assert.Equal(t, uint8(0), FromState[uint8](s, "neg", 0))
	assert.Equal(t, uint16(0), FromState[uint16](s, "neg", 0))
	assert.Equal(t, uint32(0), FromState[uint32](s, "neg", 0))
	assert.Equal(t, uint64(0), FromState[uint64](s, "neg", 0))
}

func TestFromState_UintRejectsOverflow(t *testing.T) {
	s := &SyncStateBag{}

	s.Set("big", 256)
	assert.Equal(t, uint8(0), FromState[uint8](s, "big", 0))

	s.Set("big16", 65536)
	assert.Equal(t, uint16(0), FromState[uint16](s, "big16", 0))
}

func TestFromState_Float32_Float64(t *testing.T) {
	s := &SyncStateBag{}

	s.Set("val", "3.14")
	assert.InDelta(t, float32(3.14), FromState[float32](s, "val", 0), 0.01)
	assert.InDelta(t, float64(3.14), FromState[float64](s, "val", 0), 1e-9)

	s.Set("intVal", 42)
	assert.Equal(t, float32(42), FromState[float32](s, "intVal", 0))
	assert.Equal(t, float64(42), FromState[float64](s, "intVal", 0))
}

func TestFromState_MissingKey(t *testing.T) {
	s := &SyncStateBag{}
	assert.Equal(t, 0, FromState[int](s, "missing", 0))
	assert.Equal(t, "", FromState[string](s, "missing", ""))
	assert.Equal(t, false, FromState[bool](s, "missing", false))
}

func TestFromState_NilState(t *testing.T) {
	assert.Equal(t, 0, FromState[int](nil, "k", 0))
	assert.Equal(t, "default", FromState[string](nil, "k", "default"))
}

func TestFromState_ComplexTypeFallback(t *testing.T) {
	type custom struct{ N int }
	s := &SyncStateBag{}
	s.Set("obj", custom{N: 7})

	got := FromState[custom](s, "obj", custom{})
	assert.Equal(t, 7, got.N)
}

func TestFromState_JSONRoundTrip_UintSafe(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("u8", uint8(200))
	s.Set("u16", uint16(60000))
	s.Set("u32", uint32(3000000000))

	b, err := json.Marshal(s)
	require.NoError(t, err)

	var s2 SyncStateBag
	require.NoError(t, json.Unmarshal(b, &s2))

	// After JSON round-trip, values are float64; accessors should coerce safely
	assert.Equal(t, uint8(200), FromState[uint8](&s2, "u8", 0))
	assert.Equal(t, uint16(60000), FromState[uint16](&s2, "u16", 0))
	assert.Equal(t, uint32(3000000000), FromState[uint32](&s2, "u32", 0))
}

func TestFromState_StringFromNumericAfterJSONRoundTrip(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("n", 42)

	b, err := json.Marshal(s)
	require.NoError(t, err)

	var s2 SyncStateBag
	require.NoError(t, json.Unmarshal(b, &s2))

	// After JSON round-trip n is float64(42); String accessor should format as "42"
	assert.Equal(t, "42", s2.String("n"))
}

// ---------------------------------------------------------------------------
// SyncStateBag typed accessor integration tests
// ---------------------------------------------------------------------------

func TestSyncStateBag_StringFormatsNumeric(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("int", 123)
	assert.Equal(t, "123", s.String("int"))

	s.Set("float", 3.14)
	assert.Equal(t, "3.14", s.String("float"))

	s.Set("bool", true)
	assert.Equal(t, "true", s.String("bool"))

	s.Set("uint", uint64(999))
	assert.Equal(t, "999", s.String("uint"))
}

func TestSyncStateBag_IntFromFloat(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("f", 3.14)
	assert.Equal(t, 3, s.Int("f"))
}

func TestSyncStateBag_BoolFromNumeric(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("one", 1)
	assert.Equal(t, true, s.Bool("one"))

	s.Set("zero", 0)
	assert.Equal(t, false, s.Bool("zero"))
}

// ---------------------------------------------------------------------------
// FromState — additional edge cases
// ---------------------------------------------------------------------------

func TestFromState_StringToUintCoercion(t *testing.T) {
	s := &SyncStateBag{}

	// string -> uint types
	s.Set("val", "42")
	assert.Equal(t, uint(42), FromState[uint](s, "val", 0))
	assert.Equal(t, uint8(42), FromState[uint8](s, "val", 0))
	assert.Equal(t, uint16(42), FromState[uint16](s, "val", 0))
	assert.Equal(t, uint32(42), FromState[uint32](s, "val", 0))
	assert.Equal(t, uint64(42), FromState[uint64](s, "val", 0))

	// negative string should return zero for uint
	s.Set("neg", "-1")
	assert.Equal(t, uint(0), FromState[uint](s, "neg", 0))
	assert.Equal(t, uint8(0), FromState[uint8](s, "neg", 0))

	// overflow string should return zero for uint8
	s.Set("big", "300")
	assert.Equal(t, uint8(0), FromState[uint8](s, "big", 0))
}

func TestFromState_JSONNumberForAllTargetTypes(t *testing.T) {
	s := &SyncStateBag{}

	s.Set("jn", json.Number("42"))

	assert.Equal(t, "42", FromState[string](s, "jn", ""))
	assert.Equal(t, 42, FromState[int](s, "jn", 0))
	assert.Equal(t, int64(42), FromState[int64](s, "jn", 0))
	assert.Equal(t, uint(42), FromState[uint](s, "jn", 0))
	assert.Equal(t, uint64(42), FromState[uint64](s, "jn", 0))
	assert.Equal(t, float64(42), FromState[float64](s, "jn", 0))
	assert.Equal(t, float32(42), FromState[float32](s, "jn", 0))
}

func TestFromState_JSONNumberFloat(t *testing.T) {
	s := &SyncStateBag{}

	s.Set("jf", json.Number("3.14"))
	assert.Equal(t, "3.14", FromState[string](s, "jf", ""))
	assert.InDelta(t, 3.14, FromState[float64](s, "jf", 0), 1e-9)
	assert.Equal(t, 3, FromState[int](s, "jf", 0)) // truncation
}

func TestFromState_NormalizeDoesNotMutateOriginalSlice(t *testing.T) {
	s := &SyncStateBag{}

	original := []interface{}{json.Number("1"), json.Number("2"), json.Number("3")}
	// keep a copy to verify original is not mutated
	originalCopy := make([]interface{}, len(original))
	copy(originalCopy, original)

	s.Set("slice", original)

	// Access the value which triggers normalization
	_ = FromState[[]interface{}](s, "slice", nil)

	// Original slice should NOT be mutated
	for i, v := range original {
		assert.Equal(t, originalCopy[i], v, "original slice element %d was mutated", i)
	}
}

func TestFromState_DefaultZeroReturned(t *testing.T) {
	s := &SyncStateBag{}

	// unsupported coercion: struct -> int should return zero
	s.Set("struct", struct{ X int }{X: 1})
	assert.Equal(t, 0, FromState[int](s, "struct", 0))
	assert.Equal(t, "", FromState[string](s, "struct", ""))
	assert.Equal(t, false, FromState[bool](s, "struct", false))
}

func TestFromState_CustomZeroValue(t *testing.T) {
	s := &SyncStateBag{}

	// missing key returns custom zero
	assert.Equal(t, 99, FromState[int](s, "missing", 99))
	assert.Equal(t, "fallback", FromState[string](s, "missing", "fallback"))
	assert.Equal(t, true, FromState[bool](s, "missing", true))
}

func TestFromState_NilValueStored(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("nil", nil)

	// nil stored: should return zero for primitive types
	assert.Equal(t, 0, FromState[int](s, "nil", 0))
	assert.Equal(t, "", FromState[string](s, "nil", ""))
	assert.Equal(t, false, FromState[bool](s, "nil", false))
}

func TestFromState_PointerToPointer(t *testing.T) {
	s := &SyncStateBag{}

	v := 42
	p := &v
	pp := &p
	s.Set("pp", pp)

	// exact type match should work for **int
	got := FromState[**int](s, "pp", nil)
	require.NotNil(t, got)
	require.NotNil(t, *got)
	assert.Equal(t, 42, **got)
}

// ---------------------------------------------------------------------------
// SyncStateBag typed accessors — additional integration tests
// ---------------------------------------------------------------------------

func TestSyncStateBag_StringFromMissing(t *testing.T) {
	s := &SyncStateBag{}
	assert.Equal(t, "", s.String("missing"))
}

func TestSyncStateBag_IntFromString(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("n", "42")
	assert.Equal(t, 42, s.Int("n"))
}

func TestSyncStateBag_Float64FromInt(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("n", 7)
	assert.Equal(t, float64(7), s.Float64("n"))
}

func TestSyncStateBag_Float32FromFloat64(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("n", float64(2.5))
	assert.InDelta(t, float32(2.5), s.Float32("n"), 0.01)
}

func TestSyncStateBag_Int64FromFloat64(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("n", float64(99.9))
	assert.Equal(t, int64(99), s.Int64("n")) // truncation
}

func TestSyncStateBag_BoolFromString(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("b", "true")
	assert.Equal(t, true, s.Bool("b"))

	s.Set("bf", "false")
	assert.Equal(t, false, s.Bool("bf"))

	s.Set("invalid", "notabool")
	assert.Equal(t, false, s.Bool("invalid"))
}

func TestToUint64Safe_Float64UpperBoundExactness(t *testing.T) {
	const pow64 float64 = 18446744073709551616.0 // 2^64, exactly representable
	const pow63 float64 = 9223372036854775808.0  // 2^63, exactly representable

	// 2^63 should be accepted for uint64 via float fallback (toInt64 rejects it, fallback accepts it).
	got, ok := toUint64Safe(pow63, 64)
	assert.True(t, ok)
	assert.Equal(t, uint64(1)<<63, got)

	// 2^64 must be rejected (exclusive upper bound)
	_, ok = toUint64Safe(pow64, 64)
	assert.False(t, ok)

	// The largest float64 below 2^64 is still an exactly representable integer and should be accepted.
	justBelowPow64 := math.Nextafter(pow64, 0)
	got, ok = toUint64Safe(justBelowPow64, 64)
	assert.True(t, ok)
	assert.Equal(t, uint64(justBelowPow64), got)
}

func TestToUint64Safe_FloatFallback_RespectsSmallBitSize(t *testing.T) {
	// toInt64 rejects this large float, so the float fallback handles it.
	_, ok := toUint64Safe(300.0, 8)
	assert.False(t, ok)

	got, ok := toUint64Safe(255.0, 8)
	assert.True(t, ok)
	assert.Equal(t, uint64(255), got)
}

func TestToUint64Safe_LargeUint64ViaJSONNumber(t *testing.T) {
	got, ok := toUint64Safe(json.Number("18446744073709551615"), 64)
	assert.True(t, ok)
	assert.Equal(t, uint64(math.MaxUint64), got)
}

func TestToUint64Safe_JSONNumberBoundaryAndBitSizeChecks(t *testing.T) {
	tests := []struct {
		name    string
		val     json.Number
		bitSize int
		want    uint64
		ok      bool
	}{
		{"uint64 max", json.Number("18446744073709551615"), 64, uint64(math.MaxUint64), true},
		{"uint64 overflow", json.Number("18446744073709551616"), 64, 0, false},
		{"uint8 overflow", json.Number("256"), 8, 0, false},
		{"uint8 max", json.Number("255"), 8, 255, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := toUint64Safe(tc.val, tc.bitSize)
			assert.Equal(t, tc.ok, ok)
			if ok {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestToUint64Safe_JSONNumberDecimalFallsBackToTruncation(t *testing.T) {
	got, ok := toUint64Safe(json.Number("3.7"), 64)
	assert.True(t, ok)
	assert.Equal(t, uint64(3), got)
}

func TestFromState_LargeUint64ViaJSONNumber(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("big", json.Number("18446744073709551615"))
	s.Set("small", json.Number("42"))

	assert.Equal(t, uint64(math.MaxUint64), FromState[uint64](s, "big", 0))
	assert.Equal(t, uint32(42), FromState[uint32](s, "small", 0))
}

func TestFromState_JSONNumberUintOverflowRejected(t *testing.T) {
	s := &SyncStateBag{}
	s.Set("overflow64", json.Number("18446744073709551616"))
	s.Set("overflow8", json.Number("256"))

	assert.Equal(t, uint64(0), FromState[uint64](s, "overflow64", 0))
	assert.Equal(t, uint8(0), FromState[uint8](s, "overflow8", 0))
}
