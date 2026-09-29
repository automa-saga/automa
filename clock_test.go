package automa

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// walkReports calls fn on r and every report nested beneath it.
func walkReports(r *Report, fn func(*Report)) {
	if r == nil {
		return
	}
	fn(r)
	for _, sr := range r.StepReports {
		walkReports(sr, fn)
	}
	walkReports(r.Rollback, fn)
}

func TestSetClock_UTCReachesEveryReportInAWorkflowRun(t *testing.T) {
	origLocal := time.Local
	time.Local = time.FixedZone("AEST", 10*60*60)
	t.Cleanup(func() {
		time.Local = origLocal
		SetClock(nil)
	})
	SetClock(func() time.Time { return time.Now().UTC() })

	wf, err := NewWorkflowBuilder().WithId("wf").Steps(
		NewStepBuilder().WithId("a").WithExecute(func(ctx context.Context, stp Step) *Report {
			return SuccessReport(stp)
		}),
		NewStepBuilder().WithId("b").WithExecute(func(ctx context.Context, stp Step) *Report {
			return StepSuccessReport(stp.Id())
		}),
	).Build()
	require.NoError(t, err)

	report := wf.Execute(context.Background())
	require.NotNil(t, report)

	count := 0
	walkReports(report, func(r *Report) {
		count++
		assert.Equal(t, time.UTC, r.StartTime.Location(), "report %q start", r.Id)
		assert.Equal(t, time.UTC, r.EndTime.Location(), "report %q end", r.Id)
	})
	assert.GreaterOrEqual(t, count, 3)
}

func TestSetClock_NilRestoresLocalTime(t *testing.T) {
	origLocal := time.Local
	zone := time.FixedZone("AEST", 10*60*60)
	time.Local = zone
	t.Cleanup(func() {
		time.Local = origLocal
		SetClock(nil)
	})

	SetClock(func() time.Time { return time.Now().UTC() })
	SetClock(nil)

	r := NewReport("x")
	assert.Equal(t, zone, r.StartTime.Location())
}

func TestSetClock_UsesTheGivenFunction(t *testing.T) {
	t.Cleanup(func() { SetClock(nil) })
	fixed := time.Date(2026, 9, 29, 5, 0, 0, 0, time.UTC)
	SetClock(func() time.Time { return fixed })

	r := StepFailureReport("x")
	assert.Equal(t, fixed, r.StartTime)
	assert.Equal(t, fixed, r.EndTime)
}
