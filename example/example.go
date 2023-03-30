package main

import (
	"context"
	"fmt"
	"github.com/automa-saga/automa"
	"github.com/cockroachdb/errors"
	"go.uber.org/zap"
)

// InMemCache is the simples map based in-memory cache
// It is assumed the type casting will be done properly and safely when values are retrieved
// This doesn't use mutex so shouldn't be used with coroutines
// This is just for example purposes.
type InMemCache map[string]interface{}

// GetString returns the string value for the given key
func (ic InMemCache) GetString(key string) string {
	if s, ok := ic[key].(string); ok {
		return s
	}

	return ""
}

// SetString returns the string value for the given key
func (ic InMemCache) SetString(key string, val interface{}) {
	s, ok := val.(string)
	if ok {
		ic[key] = s
	}
}

const (
	keyRollbackMsg = "rollbackMsg"
)

type StopContainers struct {
	automa.Step
	cache  InMemCache
	logger *zap.Logger
}

type FetchLatest struct {
	automa.Step
	cache  InMemCache
	logger *zap.Logger
}

// NotifyAll notifies on Slack
// it cannot be rollback
type NotifyAll struct {
	automa.Step
	cache  InMemCache
	logger *zap.Logger
}

type RestartContainers struct {
	automa.Step
	cache  InMemCache
	logger *zap.Logger
}

func (s *StopContainers) Run(ctx context.Context, prevSuccess *automa.Success) (automa.Reports, error) {
	report := automa.NewReport(s.ID)
	s.logger.Debug(fmt.Sprintf("RUN - %q", s.ID))
	s.cache.SetString(keyRollbackMsg, fmt.Sprintf("RUN - %q", s.ID))

	return s.RunNext(ctx, prevSuccess, report)
}

func (s *StopContainers) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.Reports, error) {
	report := automa.NewReport(s.ID)
	s.logger.Debug(s.cache.GetString(keyRollbackMsg))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *FetchLatest) Run(ctx context.Context, prevSuccess *automa.Success) (automa.Reports, error) {
	s.logger.Debug(fmt.Sprintf("RUN - %q", s.ID))
	s.cache.SetString(keyRollbackMsg, fmt.Sprintf("RUN - %q", s.ID))

	return s.RunNext(ctx, prevSuccess, nil)
}

func (s *FetchLatest) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.Reports, error) {
	report := automa.NewReport(s.ID)
	s.logger.Debug(s.cache.GetString(keyRollbackMsg))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *NotifyAll) Run(ctx context.Context, prevSuccess *automa.Success) (automa.Reports, error) {
	s.cache.SetString(keyRollbackMsg, fmt.Sprintf("RUN - %q", s.ID))
	return s.SkippedRun(ctx, prevSuccess, nil)
}

func (s *NotifyAll) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.Reports, error) {
	report := automa.NewReport(s.ID)
	s.logger.Debug(s.cache.GetString(keyRollbackMsg))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *RestartContainers) Run(ctx context.Context, prevSuccess *automa.Success) (automa.Reports, error) {
	report := automa.NewReport(s.ID)
	s.cache.SetString(keyRollbackMsg, fmt.Sprintf("RUN - %q", s.ID))

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
	s.logger.Debug(s.cache.GetString(keyRollbackMsg))
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
		cache:  InMemCache{},
		logger: logger,
	}

	fetch := &FetchLatest{
		Step:   automa.Step{ID: "Fetch latest images"},
		cache:  InMemCache{},
		logger: logger,
	}

	notify :=
		&NotifyAll{
			Step:   automa.Step{ID: "NotifyAll on Slack"},
			cache:  InMemCache{},
			logger: logger,
		}

	restart := &RestartContainers{
		Step:   automa.Step{ID: "Restart containers"},
		cache:  InMemCache{},
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
