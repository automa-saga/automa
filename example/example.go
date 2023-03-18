package main

import (
	"context"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/leninmehedy/automa"
	"go.uber.org/zap"
)

type StopContainers struct {
	automa.Step
	cache  map[string][]byte
	logger *zap.Logger
}

type FetchLatest struct {
	automa.Step
	cache  map[string][]byte
	logger *zap.Logger
}

// NotifyAll notifies on Slack
// it cannot be rollback
type NotifyAll struct {
	automa.Step
	cache  map[string][]byte
	logger *zap.Logger
}

type RestartContainers struct {
	automa.Step
	cache  map[string][]byte
	logger *zap.Logger
}

func (s *StopContainers) Run(ctx context.Context, prevSuccess *automa.Success) (automa.Reports, error) {
	report := automa.NewReport(s.ID)
	s.logger.Debug(fmt.Sprintf("RUN - %q", s.ID))
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))
	return s.RunNext(ctx, prevSuccess, report)
}

func (s *StopContainers) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.Reports, error) {
	report := automa.NewReport(s.ID)
	s.logger.Debug(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *FetchLatest) Run(ctx context.Context, prevSuccess *automa.Success) (automa.Reports, error) {
	s.logger.Debug(fmt.Sprintf("RUN - %q", s.ID))
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))

	return s.RunNext(ctx, prevSuccess, nil)
}

func (s *FetchLatest) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.Reports, error) {
	report := automa.NewReport(s.ID)
	s.logger.Debug(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *NotifyAll) Run(ctx context.Context, prevSuccess *automa.Success) (automa.Reports, error) {
	s.logger.Debug(fmt.Sprintf("SKIP RUN- %q", s.ID))
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("SKIP ROLLBACK - %q", s.ID))
	return s.SkippedRun(ctx, prevSuccess, nil)
}

func (s *NotifyAll) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.Reports, error) {
	report := automa.NewReport(s.ID)
	s.logger.Debug(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *RestartContainers) Run(ctx context.Context, prevSuccess *automa.Success) (automa.Reports, error) {
	report := automa.NewReport(s.ID)

	s.logger.Debug(fmt.Sprintf("RUN - %q", s.ID))
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))

	// trigger rollback on error
	err := errors.New("error running step 3")
	report.Error = errors.EncodeError(ctx, err)
	if err != nil {
		return s.Rollback(ctx, automa.NewRollbackTrigger(prevSuccess, err, report))
	}

	return s.RunNext(ctx, prevSuccess, report)
}

func (s *RestartContainers) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.Reports, error) {
	report := automa.NewReport(s.ID)
	s.logger.Debug(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	stop := &StopContainers{
		Step:   automa.Step{ID: "Stop containers"},
		cache:  map[string][]byte{},
		logger: logger,
	}

	fetch := &FetchLatest{
		Step:   automa.Step{ID: "Fetch latest images"},
		cache:  map[string][]byte{},
		logger: logger,
	}

	notify :=
		&NotifyAll{
			Step:   automa.Step{ID: "NotifyAll on Slack"},
			cache:  map[string][]byte{},
			logger: logger,
		}

	restart := &RestartContainers{
		Step:   automa.Step{ID: "Restart containers"},
		cache:  map[string][]byte{},
		logger: logger,
	}

	registry := automa.NewStepRegistry(zap.NewNop()).RegisterSteps(map[string]automa.AtomicStep{
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
	if err == nil {
		logger.Error("Was expecting error, no error received")
	}

	logger.Debug("* ----------------------------------------- *")
	logger.Debug("*        Execution Report - Workflow1       *")
	logger.Debug("* ----------------------------------------- *")
	for _, report := range reports { // note the items in the map is not ordered
		logger.Sugar().Debugf("\t%s: %s", report.StepID, report.Status)
	}
	logger.Debug("* ----------------------------------------- *")

	// a new workflow with notify at the end
	workflow2 := registry.BuildWorkflow([]string{
		stop.ID,
		fetch.ID,
		restart.ID,
		notify.ID,
	})
	defer workflow2.End(ctx)

	reports2, err := workflow2.Start(ctx)
	if err == nil {
		logger.Error("Was expecting error, no error received")
	}

	logger.Debug("* ----------------------------------------- *")
	logger.Debug("*        Execution Report - Workflow2       *")
	logger.Debug("* ----------------------------------------- *")
	for _, report := range reports2 { // note the items in the map is not ordered
		logger.Sugar().Debugf("\t%s: %s", report.StepID, report.Status)
	}
	logger.Debug("* ----------------------------------------- *")

}
