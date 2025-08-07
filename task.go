package automa

import (
	"github.com/joomcode/errorx"
)

// Task implements the automa.Step interface and represents a workflow step.
// Each Task can execute (Run) and rollback (Rollback) its operation.
// Optionally, a Skip function can be defined to conditionally skip execution or rollback.
// Task is stateless, meaning it does not maintain any internal state between executions.
// If Task needs to maintain state, it should use the Context provided during execution.
type Task struct {
	ID string // Unique identifier for the step.

	// User-defined execution logic for the step.
	// If not specified, the operation will be skipped.
	Run      func(ctx *Context) error
	Rollback func(ctx *Context) error

	// Optional function to determine if the step should be skipped.
	Skip func(ctx *Context) bool
}

// GetID returns the unique identifier of the step.
func (t *Task) GetID() string {
	return t.ID
}

// Forward runs the step's execution logic.
// If Run is not defined or Skip returns true, the step is skipped.
// On error, triggers rollback via Reverse. On success, proceeds to the next step.
func (t *Task) Forward(ctx *Context) (*Result, error) {
	if ctx == nil {
		return nil, errorx.IllegalArgument.New("context cannot be nil")
	}

	// extract the previous result from the context
	result := ctx.getPrevResult()
	if result == nil {
		return nil, errorx.IllegalArgument.New("previous result cannot be nil")
	}

	// Create a new step report for this execution.
	report := NewStepReport(t.GetID(), RunAction)

	// Skip execution if Run is not defined or Skip returns true.
	if t.Run == nil || (t.Skip != nil && t.Skip(ctx)) {
		result.Report.Append(report, StatusSkipped)
		return result, nil
	}

	// Forward the Run logic.
	err := t.Run(ctx)
	if err != nil {
		report.Error = errorx.EnsureStackTrace(err)
		result.Report.Append(report, StatusFailed)
		return result, err
	}

	// On success, append success report and proceed to next step.
	result.Report.Append(report, StatusSuccess)

	return result, nil
}

// Reverse runs the step's rollback logic.
// If Rollback is not defined or Skip returns true, the rollback is skipped.
// On error, triggers rollback of previous steps. On success, proceeds to previous step's rollback.
func (t *Task) Reverse(ctx *Context) (*Result, error) {
	if ctx == nil {
		return nil, errorx.IllegalArgument.New("context cannot be nil")
	}

	result := ctx.getPrevResult()
	if result == nil {
		return nil, errorx.IllegalArgument.New("previous result cannot be nil")
	}

	// Create a new step report for this execution.
	report := NewStepReport(t.GetID(), RollbackAction)

	// Skip rollback if Rollback is not defined or Skip returns true.
	if t.Run == nil || (t.Skip != nil && t.Skip(ctx)) {
		result.Report.Append(report, StatusSkipped)
		return result, nil
	}

	// Execute the Rollback logic.
	err := t.Rollback(ctx)
	if err != nil {
		report.Error = errorx.EnsureStackTrace(err)
		result.Report.Append(report, StatusFailed)
		return result, err
	}

	// On success, append success report and proceed to next step.
	result.Report.Append(report, StatusSuccess)

	return result, nil
}
