package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/automa-saga/automa"
	"github.com/cockroachdb/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
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

func (s *StopContainers) Run(ctx context.Context, prevSuccess *automa.Success) (automa.WorkflowReport, error) {
	report := automa.NewStepReport(s.ID, automa.RunAction)

	// reset cache
	s.cache = InMemCache{}

	s.logger.Debug(fmt.Sprintf("RUN - %q", s.ID))
	s.cache.SetString(keyRollbackMsg, fmt.Sprintf("ROLLBACK - %q", s.ID))

	return s.RunNext(ctx, prevSuccess, report)
}

func (s *StopContainers) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.WorkflowReport, error) {
	report := automa.NewStepReport(s.ID, automa.RollbackAction)
	s.logger.Debug(s.cache.GetString(keyRollbackMsg))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *FetchLatest) Run(ctx context.Context, prevSuccess *automa.Success) (automa.WorkflowReport, error) {
	report := automa.NewStepReport(s.ID, automa.RunAction)

	// reset cache
	s.cache = InMemCache{}

	s.logger.Debug(fmt.Sprintf("RUN - %q", s.ID))
	s.cache.SetString(keyRollbackMsg, fmt.Sprintf("ROLLBACK - %q", s.ID))

	return s.RunNext(ctx, prevSuccess, report)
}

func (s *FetchLatest) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.WorkflowReport, error) {
	report := automa.NewStepReport(s.ID, automa.RollbackAction)
	s.logger.Debug(s.cache.GetString(keyRollbackMsg))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *NotifyAll) Run(ctx context.Context, prevSuccess *automa.Success) (automa.WorkflowReport, error) {
	report := automa.NewStepReport(s.ID, automa.RunAction)

	// reset cache
	s.cache = InMemCache{}

	s.logger.Debug(fmt.Sprintf("RUN - %q", s.ID))
	s.cache.SetString(keyRollbackMsg, fmt.Sprintf("ROLLBACK - %q", s.ID))
	return s.SkippedRun(ctx, prevSuccess, report)
}

func (s *NotifyAll) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.WorkflowReport, error) {
	report := automa.NewStepReport(s.ID, automa.RollbackAction)
	s.logger.Debug(s.cache.GetString(keyRollbackMsg))
	return s.SkippedRollback(ctx, prevFailure, report)
}

func (s *RestartContainers) Run(ctx context.Context, prevSuccess *automa.Success) (automa.WorkflowReport, error) {
	report := automa.NewStepReport(s.ID, automa.RunAction)

	// reset cache
	s.cache = InMemCache{}

	s.logger.Debug(fmt.Sprintf("RUN - %q", s.ID))
	s.cache.SetString(keyRollbackMsg, fmt.Sprintf("ROLLBACK - %q", s.ID))

	// trigger rollback on error
	err := errors.New("error running step 3")
	if err != nil {
		return s.Rollback(ctx, automa.NewFailedRun(ctx, prevSuccess, err, report))
	}

	return s.RunNext(ctx, prevSuccess, report)
}

func (s *RestartContainers) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.WorkflowReport, error) {
	report := automa.NewStepReport(s.ID, automa.RollbackAction)
	s.logger.Debug(s.cache.GetString(keyRollbackMsg))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func buildWorkflow1(logger *zap.Logger) (automa.AtomicWorkflow, error) {
	stop := &StopContainers{
		Step:   automa.Step{ID: "stop_containers"},
		cache:  InMemCache{},
		logger: logger,
	}

	fetch := &FetchLatest{
		Step:   automa.Step{ID: "fetch_latest_images"},
		cache:  InMemCache{},
		logger: logger,
	}

	notify :=
		&NotifyAll{
			Step:   automa.Step{ID: "notify_all_on_slack"},
			cache:  InMemCache{},
			logger: logger,
		}

	restart := &RestartContainers{
		Step:   automa.Step{ID: "restart_containers"},
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
	workflow, err := registry.BuildWorkflow("workflow_1", []string{
		stop.ID,
		fetch.ID,
		notify.ID,
		restart.ID,
	})

	if err != nil {
		return nil, err
	}

	return workflow, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, err := zap.NewDevelopment()
	if err != nil {
		logger.Fatal("Failed to setup logger", zap.Error(err))
	}

	workflow, err := buildWorkflow1(logger)
	if err != nil {
		logger.Fatal("Failed to build workflow-1", zap.Error(err))
	}
	defer workflow.End(ctx)

	report, err := workflow.Start(ctx)
	if err == nil {
		logger.Error("Was expecting error, no error received")
	}

	printReport(&report, logger)
}

func printReport(report *automa.WorkflowReport, logger *zap.Logger) {
	logger.Debug("----------------------------------------- ")
	logger.Sugar().Debugf("        Execution StepReport - %s", report.WorkflowID)
	logger.Debug("----------------------------------------- ")
	out, err := yaml.Marshal(report)
	if err != nil {
		logger.Fatal("Could not marshall report to YAML", zap.Error(err))
		return
	}

	fmt.Println(string(out))

	logger.Debug("----------------------------------------- ")

	out, err = json.Marshal(report)
	if err != nil {
		logger.Fatal("Could not marshall report to JSON", zap.Error(err))
		return
	}

	fmt.Println(string(out))
	logger.Debug("----------------------------------------- ")
}
