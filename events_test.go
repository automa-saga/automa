package automa

import (
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewSkippedRun(t *testing.T) {
	prevSuccess := &Success{}
	success := NewSkippedRun(prevSuccess, nil)
	assert.NotNil(t, success)
	assert.Nil(t, success.reports)

	report := NewReport("TEST")
	success = NewSkippedRun(prevSuccess, report)
	assert.NotNil(t, success)
	assert.NotNil(t, success.reports)
	assert.Equal(t, 1, len(success.reports))
}

func TestNewSkippedRollback(t *testing.T) {
	prevFailure := &Failure{error: errors.New("Test"), reports: map[string]*Report{}}
	failure := NewSkippedRollback(prevFailure, nil)
	assert.NotNil(t, failure)
	assert.Nil(t, failure.reports)

	report := NewReport("TEST")
	failure = NewSkippedRollback(prevFailure, report)
	assert.NotNil(t, failure)
	assert.NotNil(t, failure.reports)
	assert.Equal(t, 1, len(failure.reports))
}
