package automa

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSkippedRun(t *testing.T) {
	s1 := &mockStopContainersStep{
		Step:  Step{ID: "Stop containers"},
		cache: map[string][]byte{},
	}

	s2 := &mockFetchLatestStep{
		Step:  Step{ID: "Fetch latest images"},
		cache: map[string][]byte{},
	}
	ctx := context.Background()
	prevSuccess := &Success{reports: map[string]*Report{}}
	report := NewReport(s1.ID)

	reports, err := s1.SkippedRun(ctx, prevSuccess, report)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(reports))

	s1.SetNext(s2)
	s2.SetPrev(s1)
	reports, err = s1.SkippedRun(ctx, prevSuccess, report)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(reports))
}
