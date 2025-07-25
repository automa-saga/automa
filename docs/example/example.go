package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/automa-saga/automa"
	"github.com/joomcode/errorx"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
	"os"
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
	logger *zerolog.Logger
}

type FetchLatest struct {
	automa.Step
	cache  InMemCache
	logger *zerolog.Logger
}

// NotifyAll notifies on Slack
// it cannot be rollback
type NotifyAll struct {
	automa.Step
	cache  InMemCache
	logger *zerolog.Logger
}

type RestartContainers struct {
	automa.Step
	cache  InMemCache
	logger *zerolog.Logger
}

func (s *StopContainers) run(ctx context.Context) (skipped bool, err error) {
	// reset cache
	s.cache = InMemCache{}

	s.logger.Debug().Msgf("RUN - %q", s.ID)
	s.cache.SetString(keyRollbackMsg, fmt.Sprintf("ROLLBACK - %q", s.ID))

	return false, nil
}

func (s *StopContainers) rollback(ctx context.Context) (skipped bool, err error) {
	// use cache
	s.logger.Debug().Msg(s.cache.GetString(keyRollbackMsg))

	return false, nil
}

func (s *FetchLatest) run(ctx context.Context) (skipped bool, err error) {
	// reset cache
	s.cache = InMemCache{}

	s.logger.Debug().Msgf("RUN - %q", s.ID)
	s.cache.SetString(keyRollbackMsg, fmt.Sprintf("ROLLBACK - %q", s.ID))

	return false, nil
}

func (s *FetchLatest) rollback(ctx context.Context) (skipped bool, err error) {
	// use cache
	s.logger.Debug().Msg(s.cache.GetString(keyRollbackMsg))

	return false, nil
}

func (s *NotifyAll) run(ctx context.Context) (skipped bool, err error) {
	// reset cache
	s.cache = InMemCache{}

	s.logger.Debug().Msgf("RUN - %q", s.ID)
	s.cache.SetString(keyRollbackMsg, fmt.Sprintf("ROLLBACK - %q", s.ID))

	// skip step
	return true, nil
}

func (s *NotifyAll) rollback(ctx context.Context) (skipped bool, err error) {
	// use cache
	s.logger.Debug().Msg(s.cache.GetString(keyRollbackMsg))

	return true, nil
}

func (s *RestartContainers) run(ctx context.Context) (skipped bool, err error) {
	// reset cache
	s.cache = InMemCache{}

	s.logger.Debug().Msgf("RUN - %q", s.ID)
	s.cache.SetString(keyRollbackMsg, fmt.Sprintf("ROLLBACK - %q", s.ID))

	return false, errorx.IllegalState.New("Mock error during %q", s.GetID())
}

func (s *RestartContainers) rollback(ctx context.Context) (skipped bool, err error) {
	// use cache
	s.logger.Debug().Msg(s.cache.GetString(keyRollbackMsg))

	return false, nil
}

func buildWorkflow1(logger *zerolog.Logger) (automa.AtomicWorkflow, error) {
	stop := &StopContainers{
		Step:   automa.Step{ID: "stop_containers"},
		cache:  InMemCache{},
		logger: logger,
	}

	stop.RegisterSaga(stop.run, stop.rollback)

	fetch := &FetchLatest{
		Step:   automa.Step{ID: "fetch_latest_images"},
		cache:  InMemCache{},
		logger: logger,
	}
	fetch.RegisterSaga(fetch.run, fetch.rollback)

	notify := &NotifyAll{
		Step:   automa.Step{ID: "notify_all_on_slack"},
		cache:  InMemCache{},
		logger: logger,
	}
	notify.RegisterSaga(notify.run, notify.rollback)

	restart := &RestartContainers{
		Step:   automa.Step{ID: "restart_containers"},
		cache:  InMemCache{},
		logger: logger,
	}
	restart.RegisterSaga(restart.run, restart.rollback)

	registry := automa.NewStepRegistry(nil).RegisterSteps(map[string]automa.AtomicStep{
		stop.ID:    stop,
		fetch.ID:   fetch,
		notify.ID:  notify,
		restart.ID: restart,
	})

	// a new workflow with notify in the middle
	workflow, err := registry.BuildWorkflow("workflow_1", automa.StepIDs{
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

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).
		With().Timestamp().Logger()

	workflow, err := buildWorkflow1(&logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to build workflow-1")
	}
	defer workflow.End(ctx)

	report, err := workflow.Start(ctx)
	if err == nil {
		logger.Error().Err(err).Msg("Was expecting error, no error received")
	}

	printReport(&report, &logger)
}

func printReport(report *automa.WorkflowReport, logger *zerolog.Logger) {
	logger.Debug().Msg("----------------------------------------- ")
	logger.Debug().Msgf("        Execution StepReport - %s", report.WorkflowID)
	logger.Debug().Msg("----------------------------------------- ")
	out, err := yaml.Marshal(report)
	if err != nil {
		logger.Fatal().Err(err).Msg("Could not marshall report to YAML")
		return
	}

	fmt.Println(string(out))

	logger.Debug().Msg("----------------------------------------- ")

	out, err = json.Marshal(report)
	if err != nil {
		logger.Fatal().Err(err).Msg("Could not marshall report to JSON")
		return
	}

	fmt.Println(string(out))
	logger.Debug().Msg("----------------------------------------- ")
}
