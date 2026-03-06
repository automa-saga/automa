package main

import (
	"context"
	"fmt"
	"log"

	"gopkg.in/yaml.v3"

	"github.com/automa-saga/automa"
)

// A tiny example demonstrating a basic workflow, local/global state usage and printing the final report.
func main() {
	// Step 1 builder: set a global configuration and a local value
	step1 := automa.NewStepBuilder().WithId("set-config").
		WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
			stp.State().Global().Set("env", "dev")
			stp.State().Local().Set("tmp", "/tmp/example")
			return automa.SuccessReport(stp)
		})

	// Step 2 builder: read global state and intentionally succeed
	step2 := automa.NewStepBuilder().WithId("read-config").
		WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
			env := stp.State().Global().String("env")
			tmp := stp.State().Local().String("tmp") // will be empty because local is per-step
			return automa.SuccessReport(stp, automa.WithMetadata(map[string]string{"env": env, "tmp": tmp}))
		})

	wfStep, err := automa.NewWorkflowBuilder().WithId("hello-workflow").Steps(step1, step2).Build()
	if err != nil {
		log.Fatalf("build workflow: %v", err)
	}

	// wfStep is a Step (workflow) — execute it.
	report := wfStep.Execute(context.Background())

	out, err := yaml.Marshal(report)
	if err != nil {
		log.Fatalf("marshal report: %v", err)
	}

	fmt.Println(string(out))
}
