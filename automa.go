// Package automa provides interfaces for implementing the Saga pattern for steps and workflows.
package automa

import (
	"context"

	"github.com/rs/zerolog"
)

type Step interface {
	Id() string
	Prepare(ctx context.Context) (context.Context, error)
	Execute(ctx context.Context) (*Report, error)
	Rollback(ctx context.Context) (*Report, error)
	State() StateBag
}

type StateBag interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}) interface{}
	Delete(key string)
	Clear()
	Keys() []string
	Size() int
}

type Workflow interface {
	Step

	Steps() []Step
}

type Builder interface {
	Id() string
	Validate() error
	Build() (Step, error)
}

type Registry interface {
	Add(steps ...Builder) error // return error if step with same id already exists
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
