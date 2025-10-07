package automa

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func dummyExecute(ctx context.Context, stp Step) *Report                  { return &Report{} }
func dummyRollback(ctx context.Context, stp Step) *Report                 { return &Report{} }
func dummyPrepare(ctx context.Context, stp Step) (context.Context, error) { return ctx, nil }
func dummyOnCompletion(ctx context.Context, stp Step, report *Report)     {}
func dummyOnFailure(ctx context.Context, stp Step, report *Report)        {}

func TestStepBuilder_WithMethods(t *testing.T) {
	logger := zerolog.Nop()
	builder := NewStepBuilder().
		WithId("step1").
		WithLogger(logger).
		WithPrepare(dummyPrepare).
		WithExecute(dummyExecute).
		WithRollback(dummyRollback).
		WithOnCompletion(dummyOnCompletion).
		WithOnFailure(dummyOnFailure)

	assert.Equal(t, "step1", builder.Step.id)
	assert.NotNil(t, builder.Step.logger)
	assert.NotNil(t, builder.Step.prepare)
	assert.NotNil(t, builder.Step.execute)
	assert.NotNil(t, builder.Step.rollback)
	assert.NotNil(t, builder.Step.onCompletion)
	assert.NotNil(t, builder.Step.onFailure)
}

func TestStepBuilder_Validate(t *testing.T) {
	builder := NewStepBuilder()
	err := builder.Validate()
	assert.Error(t, err) // id and execute missing

	builder.WithId("step1")
	err = builder.Validate()
	assert.Error(t, err) // execute missing

	builder.WithExecute(dummyExecute)
	err = builder.Validate()
	assert.NoError(t, err) // valid
}

func TestStepBuilder_Build(t *testing.T) {
	builder := NewStepBuilder().
		WithId("step1").
		WithExecute(dummyExecute)

	step, err := builder.Build()
	assert.NoError(t, err)
	assert.NotNil(t, step)
	assert.NotEqual(t, builder.Step, step) // builder resets

	// After build, builder should be reset
	assert.NotEqual(t, "step1", builder.Step.id)
	assert.Nil(t, builder.Step.execute)
}

func TestStepBuilder_BuildAndCopy(t *testing.T) {
	builder := NewStepBuilder().
		WithId("step1").
		WithExecute(dummyExecute).
		WithPrepare(dummyPrepare)

	step, err := builder.BuildAndCopy()
	assert.NoError(t, err)
	assert.NotNil(t, step)
	assert.NotEqual(t, builder.Step, step)

	// Builder retains previous values after BuildAndCopy
	assert.NotNil(t, builder.Step.prepare)
	assert.NotNil(t, builder.Step.execute)
}

func TestStepBuilder_Build_Invalid(t *testing.T) {
	builder := NewStepBuilder()
	_, err := builder.Build()
	assert.Error(t, err)
}

func TestStepBuilder_WithAsyncCallbacks(t *testing.T) {
	builder := NewStepBuilder().
		WithId("step_async").
		WithExecute(dummyExecute).
		WithAsyncCallbacks(true)

	assert.True(t, builder.Step.enableAsyncCallbacks)
}

func TestStepBuilder_WithState(t *testing.T) {
	state := &SyncStateBag{}
	builder := NewStepBuilder().
		WithId("step_state").
		WithExecute(dummyExecute).
		WithState(state)

	assert.Equal(t, state, builder.Step.state)
}

func TestStepBuilder_Build_ResetsAllFields(t *testing.T) {
	builder := NewStepBuilder().
		WithId("step1").
		WithExecute(dummyExecute).
		WithAsyncCallbacks(true).
		WithState(&SyncStateBag{})

	step, err := builder.Build()
	assert.NoError(t, err)
	assert.NotNil(t, step)
	// Builder resets all fields
	assert.NotEqual(t, "step1", builder.Step.id)
	assert.Nil(t, builder.Step.execute)
	assert.False(t, builder.Step.enableAsyncCallbacks)
	assert.Nil(t, builder.Step.state)
}

func TestStepBuilder_BuildAndCopy_RetainsFields(t *testing.T) {
	builder := NewStepBuilder().
		WithId("step1").
		WithExecute(dummyExecute).
		WithAsyncCallbacks(true).
		WithState(&SyncStateBag{})

	step, err := builder.BuildAndCopy()
	assert.NoError(t, err)
	assert.NotNil(t, step)
	// Builder retains previous values
	assert.NotNil(t, builder.Step.execute)
	assert.True(t, builder.Step.enableAsyncCallbacks)
	assert.NotNil(t, builder.Step.state)
}
