// Package automa provides interfaces for implementing the Saga pattern for steps and workflows.
package automa

import (
	"context"
	"github.com/rs/zerolog"
)

type TypeAction uint8

const (
	ActionExecute  TypeAction = 1 // "execute"
	ActionRollback TypeAction = 2 // "rollback"
)

type TypeRollbackMode uint8

const (
	RollbackModeContinueOnError TypeRollbackMode = 1 // "continue" // continue rolling back previous steps even if one fails
	RollbackModeStopOnError     TypeRollbackMode = 2 // "stop"     // stop rolling back previous steps on first failure
)

type TypeReport uint8

const (
	StepReportType     TypeReport = 1
	WorkflowReportType TypeReport = 2
)

type TypeStatus uint8

const (
	StatusSuccess TypeStatus = 1 // "success"
	StatusFailed  TypeStatus = 2 // "failed"
	StatusSkipped TypeStatus = 3 //"skipped"
)

type Step interface {
	Id() string
	Prepare(ctx context.Context) (context.Context, error)
	Execute(ctx context.Context) (*Report, error)
	OnCompletion(ctx context.Context, report *Report)
	OnRollback(ctx context.Context) (*RollbackReport, error)
}

type Workflow Step

type Builder interface {
	Id() string
	Validate() error
	Build() (Step, error)
}

type Registry interface {
	Add(steps ...Builder) error // return error if step with same ID already exists
	Remove(id string) bool
	Has(id string) bool
	Of(id string) Builder
}

type WorkFlowBuilder interface {
	Builder
	Steps(steps ...Builder) WorkFlowBuilder
	NamedSteps(stepIds ...string) WorkFlowBuilder
	WithRegistry(sr Registry) WorkFlowBuilder
	WithLogger(logger zerolog.Logger) WorkFlowBuilder
	WithRollbackMode(mode TypeRollbackMode) WorkFlowBuilder
}
