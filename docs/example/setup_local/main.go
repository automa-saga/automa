package main

import (
	"context"
	"fmt"
	"github.com/automa-saga/automa"
	"gopkg.in/yaml.v3"
	"log"
)

func printReport(msg string, report *automa.Report) string {
	out, err := yaml.Marshal(report)
	if err != nil {
		log.Fatalf("Error marshalling report to YAML: %v", err)
	}
	fmt.Printf("\n%s\n", msg)
	fmt.Println("--------------------------------------------------------")
	fmt.Printf("%s\n", out)
	return string(out)
}

// Sets up a dev environment with some common tools.
// It demonstrates how to use the automa framework to create a series of steps to install tools.
// The tools installed include:
// - task
// - helm
// - kubectl
// - kind
func main() {
	// setup automa registry with steps
	registry := automa.NewRegistry()

	err := registry.Add(
		NewInstallTaskStep("install_task", "v3.44.1"),
		NewInstallKubectlStep("install_kubectl", "v1.27.3"),
		NewInstallKindStep("install_kind", "v0.20.0"),
		NewInstallHelmStep("install_helm", "v3.12.3"),
	)

	workflow, err := automa.NewWorkFlowBuilder("setup_local_dev_env").
		WithRegistry(registry).
		WithRollbackMode(automa.RollbackModeStopOnError).
		NamedSteps(
			"install_task",
			"install_helm",
			"install_kubectl",
			"install_kind",
		).Build()
	if err != nil {
		panic(err)
	}

	report, err := workflow.Execute(context.Background())
	if err != nil {
		panic(err)
	}

	printReport("Workflow completed", report)
}
