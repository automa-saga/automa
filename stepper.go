package automa

import (
	"context"
)

// Forward forwards the state machine on success event
type Forward interface {
	Run(ctx context.Context, prevSuccess *Success) (Reports, error)
}

// Backward moves the state machine backward by running rollbacks or cleanups
type Backward interface {
	Rollback(ctx context.Context, prevFailure *Failure) (Reports, error)
}

// NextPrevStep interface defines the methods to support double link list of states
type NextPrevStep interface {
	SetNext(next Forward)
	SetPrev(prev Backward)
	GetNext() Forward
	GetPrev() Backward
}

// RollbackableStep provides interface for an internal state
type RollbackableStep interface {
	Forward
	Backward
	NextPrevStep
}

// WorkflowEngine defines interface for Workflow
type WorkflowEngine interface {
	Start(ctx context.Context) (Reports, error)
	End(ctx context.Context)
}
