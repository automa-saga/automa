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
	WithState(s StateBag) Step
}

type StateBag interface {
	Get(key Key) (interface{}, bool)
	Set(key Key, value interface{}) StateBag
	Delete(key Key) StateBag
	Clear() StateBag
	Keys() []Key
	Size() int
	Items() map[Key]interface{}
	Merge(other StateBag) StateBag
	Clone() (StateBag, error)
	// Helper methods for extracting typed values
	String(key Key) string
	Bool(key Key) bool
	Int(key Key) int
	Int8(key Key) int8
	Int16(key Key) int16
	Int32(key Key) int32
	Int64(key Key) int64
	Float(key Key) float64
	Float32(key Key) float32
	Float64(key Key) float64
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
