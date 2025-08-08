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

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()

	// Define workflow steps
	stop := &automa.Task{
		ID:       "stop_containers",
		Run:      func(ctx *automa.Context) error { return nil },
		Rollback: func(ctx *automa.Context) error { return nil },
	}
	fetch := &automa.Task{
		ID:       "fetch_latest_images",
		Run:      func(ctx *automa.Context) error { return nil },
		Rollback: func(ctx *automa.Context) error { return nil },
	}
	notify := &automa.Task{
		ID:       "notify_all_on_slack",
		Run:      func(ctx *automa.Context) error { return nil },
		Skip:     func(ctx *automa.Context) bool { return true },
		Rollback: func(ctx *automa.Context) error { return nil },
	}
	restart := &automa.Task{
		ID: "restart_containers",
		Run: func(ctx *automa.Context) error {
			return errorx.IllegalState.New("error during restart_containers")
		},
		Rollback: func(ctx *automa.Context) error { return nil },
	}

	// Create registry and add steps
	registry := automa.NewRegistry().AddSteps(stop, fetch, notify, restart)

	// Build workflow
	workflow, err := registry.Build("workflow_1", []string{
		stop.ID, fetch.ID, notify.ID, restart.ID,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to build workflow-1")
		os.Exit(1)
	}

	// Execute workflow
	result, err := workflow.Forward(automa.NewContext(ctx))
	if err != nil {
		logger.Error().Err(err).Msg("Wasn't expecting error, but error received")
		os.Exit(1)
	}

	printReport(result.Report, &logger)
}

func printReport(report *automa.WorkflowReport, logger *zerolog.Logger) {
	if report == nil {
		logger.Fatal().Msg("Workflow report is nil")
		return
	}

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
