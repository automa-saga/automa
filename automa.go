// Package automa provides interfaces for implementing the Saga pattern for steps and workflows.
package automa

import (
	"context"
)

type Step interface {
	Id() string
	Prepare(ctx context.Context) (context.Context, error)
	Execute(ctx context.Context) *Report
	Rollback(ctx context.Context) *Report
	State() StateBag
}

type StateBag interface {
	Get(key Key) (interface{}, bool)
	Set(key Key, value interface{}) StateBag
	Delete(key Key) StateBag
	Clear() StateBag
	Keys() []Key
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
