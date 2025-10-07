package automa

import (
	"context"
	"errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

func TestDefaultStep_Prepare(t *testing.T) {
	step := newDefaultStep()
	called := false
	step.prepare = func(ctx context.Context, stp Step) (context.Context, error) {
		called = true
		return context.WithValue(ctx, "k", "v"), nil
	}
	ctx := context.Background()
	newCtx, err := step.Prepare(ctx)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "v", newCtx.Value("k"))
}

func TestDefaultStep_Prepare_Error(t *testing.T) {
	step := newDefaultStep()
	step.prepare = func(ctx context.Context, stp Step) (context.Context, error) {
		return nil, errors.New("prep error")
	}
	ctx := context.Background()
	newCtx, err := step.Prepare(ctx)
	assert.Error(t, err)
	assert.Nil(t, newCtx)
}

func TestDefaultStep_Execute_Success(t *testing.T) {
	step := newDefaultStep()
	step.id = "exec"
	step.execute = func(ctx context.Context, stp Step) *Report {
		return SuccessReport(step, WithActionType(ActionExecute))
	}
	called := false
	step.onCompletion = func(ctx context.Context, stp Step, report *Report) {
		called = true
	}
	report := step.Execute(context.Background())
	assert.NoError(t, report.Error)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.True(t, called)
}

func TestDefaultStep_Execute_Failure(t *testing.T) {
	step := newDefaultStep()
	step.id = "fail"
	step.execute = func(ctx context.Context, stp Step) *Report {
		return FailureReport(step, WithActionType(ActionExecute),
			WithError(errors.New("exec error")))
	}
	called := false
	step.onFailure = func(ctx context.Context, stp Step, report *Report) {
		called = true
	}
	report := step.Execute(context.Background())
	assert.Error(t, report.Error)
	assert.Equal(t, StatusFailed, report.Status)
	assert.True(t, called)
}

func TestDefaultStep_Execute_Skipped(t *testing.T) {
	step := newDefaultStep()
	step.id = "skip"
	report := step.Execute(context.Background())
	assert.NoError(t, report.Error)
	assert.Equal(t, "skip", report.Id)
	assert.Equal(t, StatusSkipped, report.Status)
}

func TestDefaultStep_Rollback_Success(t *testing.T) {
	step := newDefaultStep()
	step.id = "rb"
	called := false
	step.rollback = func(ctx context.Context, stp Step) *Report {
		called = true
		return SuccessReport(step, WithActionType(ActionRollback))
	}
	report := step.Rollback(context.Background())
	assert.NoError(t, report.Error)
	assert.Equal(t, "rb", report.Id)
	assert.Equal(t, StatusSuccess, report.Status)
	assert.True(t, called)
}

func TestDefaultStep_Rollback_Failure(t *testing.T) {
	step := newDefaultStep()
	step.id = "rbfail"
	called := false
	step.rollback = func(ctx context.Context, stp Step) *Report {
		called = true
		return FailureReport(step, WithError(errors.New("rollback error")))
	}

	report := step.Rollback(context.Background())
	assert.Error(t, report.Error)
	assert.Equal(t, "rbfail", report.Id)
	assert.Equal(t, StatusFailed, report.Status)
	assert.True(t, called)
}

func TestDefaultStep_Rollback_Skipped(t *testing.T) {
	step := newDefaultStep()
	step.id = "rbskip"
	report := step.Rollback(context.Background())
	assert.NoError(t, report.Error)
	assert.Equal(t, "rbskip", report.Id)
	assert.Equal(t, StatusSkipped, report.Status)
}

func TestDefaultStep_State_LazyInit(t *testing.T) {
	step := newDefaultStep()
	assert.NotNil(t, step.State())
	assert.Equal(t, 0, step.State().Size())
	step.State().Set("foo", "bar")
	val, ok := step.State().Get("foo")
	assert.True(t, ok)
	assert.Equal(t, "bar", val)
}

func TestDefaultStep_Id(t *testing.T) {
	step := newDefaultStep()
	step.id = "myid"
	assert.Equal(t, "myid", step.Id())
}

func TestDefaultStep_AsyncCallbacks(t *testing.T) {
	step := newDefaultStep()
	step.enableAsyncCallbacks = true
	step.id = "async"
	var wg sync.WaitGroup
	wg.Add(1)
	step.onCompletion = func(ctx context.Context, stp Step, report *Report) {
		wg.Done()
	}
	step.execute = func(ctx context.Context, stp Step) *Report {
		return SuccessReport(step, WithActionType(ActionExecute))
	}
	step.Execute(context.Background())
	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()
	select {
	case <-waitCh:
	case <-time.After(time.Second):
		t.Fatal("async callback not called")
	}
}

func TestDefaultStep_Logger(t *testing.T) {
	step := newDefaultStep()
	logger := zerolog.Nop()
	step.logger = &logger
	assert.Equal(t, &logger, step.logger)
}

func TestDefaultStep_State_Singleton(t *testing.T) {
	step := newDefaultStep()
	s1 := step.State()
	s2 := step.State()
	assert.Equal(t, s1, s2)
}

func TestDefaultStep_HandleCompletion_NoCallback(t *testing.T) {
	step := newDefaultStep()
	// Should not panic or do anything
	step.handleCompletion(context.Background(), &Report{})
}

func TestDefaultStep_HandleFailure_NoCallback(t *testing.T) {
	step := newDefaultStep()
	// Should not panic or do anything
	step.handleFailure(context.Background(), &Report{})
}

func TestDefaultStep_Logger_DefaultNil(t *testing.T) {
	step := newDefaultStep()
	assert.Nil(t, step.logger)
}
