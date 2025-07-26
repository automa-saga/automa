package automa

import (
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewStepRegistry(t *testing.T) {
	registry := NewStepRegistry(nil)
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.(*stepRegistry).logger)

	logger := zerolog.Nop()
	registry = NewStepRegistry(&logger)
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.(*stepRegistry).logger)
}

func TestStepRegistry_GetStep(t *testing.T) {
	registry := NewStepRegistry(nil)
	assert.NotNil(t, registry)

	s1 := &mockSuccessStep{AbstractStep: AbstractStep{ID: "test"}}
	s1.RegisterSaga(s1.run, s1.run)

	registry.RegisterSteps(map[string]Step{s1.ID: s1})
	assert.NotNil(t, registry.GetStep(s1.ID))
	assert.Nil(t, registry.GetStep("INVALID"))

}
