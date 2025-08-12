// Package automa provides interfaces for implementing the Saga pattern for steps and workflows.
package automa

import (
	"context"
	"github.com/rs/zerolog"
	"time"
)

type Status string

const (
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusSkipped Status = "skipped"
)

type RollbackMode string

const (
	RollbackModeContinueOnError RollbackMode = "continue" // continue rolling back previous steps even if one fails
	RollbackModeStopOnError     RollbackMode = "stop"     // stop rolling back previous steps on first failure
)

type ActionType string

const (
	ActionExecute  ActionType = "execute"
	ActionRollback ActionType = "rollback"
)

type Report interface {
	Id() string
	Action() ActionType
	StartTime() time.Time
	EndTime() time.Time
	Status() Status
	Error() error
	Message() string
	Metadata() map[string]string // optional metadata for additional information
}

type Step interface {
	Id() string
	Prepare(ctx context.Context) (context.Context, error)
	Execute(ctx context.Context) (Report, error)
	OnSuccess(ctx context.Context, report Report) // FIXME: Should we pass the report here? Do we need this?
	OnRollback(ctx context.Context) (Report, error)
}

type Workflow Step

type Builder interface {
	Id() string
	Build() Step
}

type Registry interface {
	Add(steps ...Builder) error // return error if step with same ID already exists
	Of(id string) Builder
}

type WorkFlowBuilder interface {
	Builder
	AddSteps(steps ...Builder) error
	WithNamed(stepIds ...string) error
	WithRegistry(sr Registry) WorkFlowBuilder
	WithLogger(logger zerolog.Logger) WorkFlowBuilder
	WithRollbackMode(mode RollbackMode) WorkFlowBuilder
}
