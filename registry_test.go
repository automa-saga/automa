package automa

import (
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry(nil)
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.(*stepRegistry).logger)

	logger := zerolog.Nop()
	registry = NewRegistry(&logger)
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.(*stepRegistry).logger)
}

func TestRegistry_GetStep(t *testing.T) {
	registry := NewRegistry(nil)
	assert.NotNil(t, registry)

	s1 := &Task{
		ID: "test",
		Run: func(ctx *Context) error {
			return nil
		},
		Rollback: func(ctx *Context) error {
			return nil
		},
	}

	registry.AddSteps(s1)
	assert.NotNil(t, registry.(*stepRegistry).GetStep(s1.ID))
	assert.Nil(t, registry.(*stepRegistry).GetStep("INVALID"))

}
