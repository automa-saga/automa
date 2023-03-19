package automa

import (
	"context"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"testing"
)

func TestWorkflowEngine_Start(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := &mockStopContainersStep{
		Step:  Step{ID: "Stop containers"},
		cache: map[string][]byte{},
	}

	fetch := &mockFetchLatestStep{
		Step:  Step{ID: "Fetch latest images"},
		cache: map[string][]byte{},
	}

	notify :=
		&mockNotifyStep{
			Step:  Step{ID: "Notify on Slack"},
			cache: map[string][]byte{},
		}

	restart := &mockRestartContainersStep{
		Step:  Step{ID: "Restart containers"},
		cache: map[string][]byte{},
	}

	registry := NewStepRegistry(zap.NewNop()).RegisterSteps(map[string]AtomicStep{
		stop.ID:    stop,
		fetch.ID:   fetch,
		notify.ID:  notify,
		restart.ID: restart,
	})

	// a new workflow with notify in the middle
	workflow := registry.BuildWorkflow([]string{
		stop.ID,
		fetch.ID,
		notify.ID,
		restart.ID,
	})
	defer workflow.End(ctx)

	reports, err := workflow.Start(ctx)
	assert.Error(t, err)
	assert.NotNil(t, reports)
	assert.Equal(t, 4, len(reports)) // it will reach all steps and rollback

	// a new workflow with notify at the end
	workflow2 := registry.BuildWorkflow([]string{
		stop.ID,
		fetch.ID,
		restart.ID,
		notify.ID,
	})
	defer workflow2.End(ctx)

	reports2, err := workflow2.Start(ctx)
	assert.Error(t, err)
	assert.NotNil(t, reports)
	assert.Equal(t, 3, len(reports2)) // it will not reach notify step
	assert.NotNil(t, reports2[restart.ID].Error)
}
