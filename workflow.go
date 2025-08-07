package automa

import (
	"github.com/joomcode/errorx"
	"sync"
	"time"
)

// workflow implements the Workflow interface.
// It manages a Saga workflow using a double linked list of Steps,
// enabling forward execution on success and reverse traversal on failure.
// workflow is stateful and thread-safe, allowing concurrent access
type workflow struct {
	id string

	// mutex ensures thread-safe access to the workflow's state.
	mutex sync.Mutex

	stepSequence []string // Ordered list of Step IDs for execution order.

	// stepMap caches Steps by their ID for quick lookup.
	stepMap map[string]Step
}

// GetID returns the unique identifier of the workflow.
func (wf *workflow) GetID() string {
	return wf.id
}

// addStep adds a Step to the workflow.
// It initializes the stepMap and stepSequence if they are nil,
// ensures the step ID is unique, and appends the step to the sequence.
func (wf *workflow) addStep(s Step) error {
	// ensure it is not the same step as the workflow itself
	if s == wf || s.GetID() == wf.GetID() {
		return errorx.IllegalArgument.New("step cannot be the workflow itself: %s", wf.GetID())
	}

	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	// initialize the stepMap and stepSequence if they are nil
	if wf.stepMap == nil {
		wf.stepMap = make(map[string]Step)
	}

	if wf.stepSequence == nil {
		wf.stepSequence = make([]string, 0, 3) // initial capacity for performance
	}

	// ensure the step ID is unique
	if _, exists := wf.stepMap[s.GetID()]; exists {
		return errorx.IllegalArgument.New("step with ID %s already exists in workflow", s.GetID())
	}

	wf.stepMap[s.GetID()] = s
	wf.stepSequence = append(wf.stepSequence, s.GetID())

	return nil
}

// AddSteps is a convenience function to add multiple Steps to the workflow.
func (wf *workflow) AddSteps(steps ...Step) error {
	for _, step := range steps {
		err := wf.addStep(step)
		if err != nil {
			return errorx.IllegalArgument.Wrap(err, "failed to add step %s to workflow %s", step.GetID(), wf.GetID())
		}
	}
	return nil
}

// RemoveSteps removes Steps from the workflow by their IDs.
func (wf *workflow) RemoveSteps(stepIDs ...string) error {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()
	for _, stepID := range stepIDs {
		if _, exists := wf.stepMap[stepID]; exists {
			delete(wf.stepMap, stepID)

			// Remove the step ID from the sequence
			for i, id := range wf.stepSequence {
				if id == stepID {
					wf.stepSequence = append(wf.stepSequence[:i], wf.stepSequence[i+1:]...)
					break // exit after removing the first occurrence since IDs are unique
				}
			}
		}
	}

	return nil
}

// Forward starts the workflow execution and returns a WorkflowReport.
// This method executes all Steps in the order defined by stepSequence. If error occurs, it rolls back the steps in reverse order.
func (wf *workflow) Forward(ctx *Context) (*Result, error) {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	var index int
	var stepID string
	var err error

	result := wf.initResult(ctx)

	// forward execution
	for index, stepID = range wf.stepSequence {
		step, exists := wf.stepMap[stepID]
		if !exists {
			return nil, errorx.IllegalArgument.New("step %s not found in workflow", stepID)
		}

		// Forward the step
		result, err = step.Forward(ctx.setPrevResult(result))
		if err != nil {
			result.Report.FirstFailureOnForward = &StepFailure{
				StepID: stepID,
				Action: RunAction,
				Error:  errorx.EnsureStackTrace(err),
			}
			break // start rolling back steps
		}
	}

	// reverse execution if there was an error
	if err != nil {
		result.Report.Status = StatusFailed
		for ; index >= 0; index-- {
			stepID = wf.stepSequence[index]
			step, exists := wf.stepMap[stepID]
			if !exists {
				return nil, errorx.IllegalArgument.New("step %s not found in workflow", stepID)
			}

			// Reverse the step
			result, err = step.Reverse(ctx.setPrevResult(result))
			if err != nil {
				result.Report.LastFailureOnReverse = &StepFailure{
					StepID: stepID,
					Action: RollbackAction,
					Error:  errorx.EnsureStackTrace(err),
				}

				continue // continue rolling back previous steps
			}
		}
	} else {
		// If all steps succeeded, set the final status to success
		result.Report.Status = StatusSuccess
	}

	// Set the end time for the workflow report
	result.Report.EndTime = time.Now()

	return result, nil
}

func (wf *workflow) initResult(ctx *Context) *Result {
	result := ctx.getPrevResult()
	if result == nil {
		result = &Result{
			Report: NewWorkflowReport(wf.GetID(), wf.GetStepSequence()),
		}
	}
	return result
}

// Reverse rolls back the workflow execution.
// This is invoked once a workflow has already been executed, but needed to be reversed because of a failure in
// subsequent steps or workflows. Therefore, it traverses the stepSequence in reverse order from the last executed step.
func (wf *workflow) Reverse(ctx *Context) (*Result, error) {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	if len(wf.stepSequence) == 0 {
		return nil, errorx.IllegalState.New("no steps to reverse in workflow %s", wf.GetID())
	}

	var index int
	var stepID string
	var err error

	result := wf.initResult(ctx)

	// reverse execution
	for index = len(wf.stepSequence) - 1; index >= 0; index-- {
		stepID = wf.stepSequence[index]
		step, exists := wf.stepMap[stepID]
		if !exists {
			return nil, errorx.IllegalArgument.New("step %s not found in workflow", stepID)
		}

		// Reverse the step
		result, err = step.Reverse(ctx.SetValue(KeyPrevResult, result))
		if err != nil {
			result.Report.LastFailureOnReverse = &StepFailure{
				StepID: stepID,
				Action: RollbackAction,
				Error:  errorx.EnsureStackTrace(err),
			}
			continue // continue rolling back previous steps
		}
	}

	// Set the end time for the workflow report
	result.Report.EndTime = time.Now()
	return result, nil
}

// GetStepSequence returns the ordered list of Step IDs in the workflow.
// This method returns a copy of the stepSequence to avoid external modifications.
func (wf *workflow) GetStepSequence() []string {
	copied := make([]string, len(wf.stepSequence))
	copy(copied, wf.stepSequence)
	return copied
}

// HasStep checks if the workflow contains a Step with the given stepID.
func (wf *workflow) HasStep(stepID string) bool {
	wf.mutex.Lock()
	defer wf.mutex.Unlock()

	_, exists := wf.stepMap[stepID]
	return exists
}

// WorkflowOption defines a functional option for configuring a workflow.
type WorkflowOption func(wf *workflow)

// NewWorkflow creates a new Workflow instance with the given ID and options.
// Applies all provided WorkflowOptions for configuration.
func NewWorkflow(id string, opts ...WorkflowOption) Workflow {
	wf := &workflow{
		id:           id,
		stepSequence: make([]string, 0, 3), // initial capacity for performance,
		stepMap:      make(map[string]Step),
	}
	for _, opt := range opts {
		opt(wf)
	}

	return wf
}
