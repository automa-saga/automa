package automa

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestStepBuilder_Validate(t *testing.T) {
	sb := &StepBuilder{}
	err := sb.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "step id cannot be empty")

	sb = &StepBuilder{id: "foo"}
	err = sb.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "execute function cannot be nil")

	sb = &StepBuilder{
		id:      "foo",
		execute: func(ctx context.Context) (*Report, error) { return nil, nil },
	}
	err = sb.Validate()
	assert.NoError(t, err)

	sb.OnValidate = func() error { return IllegalArgument.New("custom error") }
	err = sb.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "custom error")
}

func TestStepBuilder_Build_Default(t *testing.T) {
	sb := &StepBuilder{
		id:      "bar",
		execute: func(ctx context.Context) (*Report, error) { return nil, nil },
	}
	step, err := sb.Build()
	assert.NoError(t, err)
	assert.NotNil(t, step)
	assert.Equal(t, "bar", step.Id())
}

func TestStepBuilder_Build_WithOnBuild(t *testing.T) {
	sb := &StepBuilder{
		id:      "baz",
		execute: func(ctx context.Context) (*Report, error) { return nil, nil },
		OnBuild: func() (Step, error) {
			return &defaultStep{id: "custom"}, nil
		},
	}
	step, err := sb.Build()
	assert.NoError(t, err)
	assert.NotNil(t, step)
	assert.Equal(t, "custom", step.Id())
}

func TestNewStepBuilder_WithOptions(t *testing.T) {
	logger := zerolog.Nop()
	called := false
	sb := NewStepBuilder(
		"opt",
		WithLogger(logger),
		WithOnExecute(func(ctx context.Context) (*Report, error) { called = true; return nil, nil }),
	)
	assert.Equal(t, "opt", sb.id)
	assert.Equal(t, logger, sb.Logger)
	assert.NotNil(t, sb.execute)

	_, err := sb.execute(context.Background())
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestWithOnValidate(t *testing.T) {
	sb := &StepBuilder{}
	called := false
	WithOnValidate(func() error { called = true; return nil })(sb)
	assert.NotNil(t, sb.OnValidate)
	_ = sb.OnValidate()
	assert.True(t, called)
}

func TestWithOnExecute(t *testing.T) {
	sb := &StepBuilder{}
	called := false
	WithOnExecute(func(ctx context.Context) (*Report, error) { called = true; return nil, nil })(sb)
	assert.NotNil(t, sb.execute)
	_, _ = sb.execute(context.Background())
	assert.True(t, called)
}

func TestWithOnPrepare(t *testing.T) {
	sb := &StepBuilder{}
	called := false
	WithOnPrepare(func(ctx context.Context) (context.Context, error) { called = true; return ctx, nil })(sb)
	assert.NotNil(t, sb.prepare)
	_, _ = sb.prepare(context.Background())
	assert.True(t, called)
}

func TestWithOnCompletion(t *testing.T) {
	sb := &StepBuilder{}
	called := false
	WithOnCompletion(func(ctx context.Context, report *Report) { called = true })(sb)
	assert.NotNil(t, sb.completion)
	sb.completion(context.Background(), nil)
	assert.True(t, called)
}

func TestWithOnRollback(t *testing.T) {
	sb := &StepBuilder{}
	called := false
	WithOnRollback(func(ctx context.Context) (*Report, error) { called = true; return nil, nil })(sb)
	assert.NotNil(t, sb.rollback)
	_, _ = sb.rollback(context.Background())
	assert.True(t, called)
}

func TestWithOnBuild(t *testing.T) {
	sb := &StepBuilder{}
	called := false
	WithOnBuild(func() (Step, error) { called = true; return nil, nil })(sb)
	assert.NotNil(t, sb.OnBuild)
	_, _ = sb.OnBuild()
	assert.True(t, called)
}

func TestStepBuilder_Id(t *testing.T) {
	sb := &StepBuilder{id: "id"}
	assert.Equal(t, "id", sb.Id())
}
