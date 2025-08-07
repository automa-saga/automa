package automa

import (
	"errors"
	"testing"

	"github.com/joomcode/errorx"
	"github.com/stretchr/testify/assert"
)

func newTestContext() *Context {
	ctx := NewContext(nil)
	// Set a dummy previous result to avoid nil errors
	ctx.setPrevResult(&Result{Report: NewWorkflowReport("test", nil)})
	return ctx
}

func TestTask_Forward_Success(t *testing.T) {
	called := false
	task := &Task{
		ID: "step1",
		Run: func(ctx *Context) error {
			called = true
			return nil
		},
	}
	ctx := newTestContext()
	res, err := task.Forward(ctx)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.NotNil(t, res)
	assert.Equal(t, StatusSuccess, res.Report.StepReports[0].Status)
}

func TestTask_Forward_Skip(t *testing.T) {
	task := &Task{
		ID: "step2",
		Run: func(ctx *Context) error {
			t.Fatal("should not be called")
			return nil
		},
		Skip: func(ctx *Context) bool { return true },
	}
	ctx := newTestContext()
	res, err := task.Forward(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, StatusSkipped, res.Report.StepReports[0].Status)
}

func TestTask_Forward_Error(t *testing.T) {
	task := &Task{
		ID: "step3",
		Run: func(ctx *Context) error {
			return errors.New("fail")
		},
	}
	ctx := newTestContext()
	res, err := task.Forward(ctx)
	assert.Error(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, StatusFailed, res.Report.StepReports[0].Status)
	assert.Contains(t, res.Report.StepReports[0].Error.Error(), "fail")
}

func TestTask_Forward_NilContext(t *testing.T) {
	task := &Task{ID: "step4"}
	res, err := task.Forward(nil)
	assert.Nil(t, res)
	assert.True(t, errorx.IsOfType(err, errorx.IllegalArgument))
}

func TestTask_Forward_NilPrevResult(t *testing.T) {
	task := &Task{ID: "step5"}
	ctx := NewContext(nil) // no prev result set
	res, err := task.Forward(ctx)
	assert.Nil(t, res)
	assert.True(t, errorx.IsOfType(err, errorx.IllegalArgument))
}

func TestTask_Reverse_Success(t *testing.T) {
	called := false
	task := &Task{
		ID:  "step6",
		Run: func(ctx *Context) error { return nil },
		Rollback: func(ctx *Context) error {
			called = true
			return nil
		},
	}
	ctx := newTestContext()
	res, err := task.Reverse(ctx)
	assert.NoError(t, err)
	assert.True(t, called)
	assert.NotNil(t, res)
	assert.Equal(t, StatusSuccess, res.Report.StepReports[0].Status)
}

func TestTask_Reverse_Skip(t *testing.T) {
	task := &Task{
		ID: "step7",
		Rollback: func(ctx *Context) error {
			t.Fatal("should not be called")
			return nil
		},
		Skip: func(ctx *Context) bool { return true },
	}
	ctx := newTestContext()
	res, err := task.Reverse(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, StatusSkipped, res.Report.StepReports[0].Status)
}

func TestTask_Reverse_Error(t *testing.T) {
	task := &Task{
		ID: "step8",
		Rollback: func(ctx *Context) error {
			return errors.New("fail-rollback")
		},
	}
	ctx := newTestContext()
	res, err := task.Reverse(ctx)
	assert.Error(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, StatusFailed, res.Report.StepReports[0].Status)
	assert.Contains(t, res.Report.StepReports[0].Error.Error(), "fail-rollback")
}

func TestTask_Reverse_NilContext(t *testing.T) {
	task := &Task{ID: "step9"}
	res, err := task.Reverse(nil)
	assert.Nil(t, res)
	assert.True(t, errorx.IsOfType(err, errorx.IllegalArgument))
}

func TestTask_Reverse_NilPrevResult(t *testing.T) {
	task := &Task{ID: "step10"}
	ctx := NewContext(nil) // no prev result set
	res, err := task.Reverse(ctx)
	assert.Nil(t, res)
	assert.True(t, errorx.IsOfType(err, errorx.IllegalArgument))
}
