package main

import (
	"context"
	"fmt"
	"github.com/automa-saga/automa"
	"gopkg.in/yaml.v3"
	"log"
	"runtime"
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

// Sets up a linux based dev environment with some common tools.
// It demonstrates how to use the automa framework to create a series of steps to install tools.
// The tools installed include:
// - task
// - helm
// - kubectl
// - kind
//
// Design considerations:
// - Each step is idempotent, meaning that if the step is run multiple times, it will not cause any issues.
// - Each step has a rollback function that will undo the changes made by the step in case of failure.
// - The workflow is designed to stop on error and rollback all previously executed steps to ensure a clean state.
// - The steps are registered in a registry, which allows for easy management and composition of steps and workflows.
// - The workflow is built using a workflow builder, which allows for easy configuration of the workflow.
// - The workflow is executed in a context, which allows for cancellation and timeout of the workflow.
//
// Here we only have one workflow so it's not very useful, but in a real world scenario, you might have multiple
// workflows that can be composed together.
// For example, you might have a "setup_ci_cd_workflow" that uses the "setup_local_dev_workflow" as one of its steps.
// This allows for reusability and modularity of workflows.
// Note that when composing workflows, the rollback mode of the parent workflow does not affect the child workflow.
// Each workflow maintains its own rollback mode.
// So if the parent workflow is set to RollbackModeContinueOnError, but the child workflow is set to RollbackModeStopOnError,
// the child workflow will still stop on error.
// This allows for more granular control over the behavior of each workflow.
// In this example, we set the rollback mode of the workflow builder to RollbackModeStopOnError,
// which means that if any step fails, the workflow will stop and rollback will be initiated for all previously executed steps.
// This is a common pattern for setup scripts, as we want to ensure that if any step fails, we don't leave the system in a partially configured state.
// However, in some cases, you might want to continue executing steps even if one fails, in which case you would use RollbackModeContinueOnError.
// The choice of rollback mode depends on the specific requirements and constraints of your setup process.
// In this example, we chose RollbackModeStopOnError to ensure a clean and consistent state in case of failures.
// This is especially important when dealing with system-level changes, such as installing software or modifying configurations.
// By stopping on error and rolling back, we can avoid leaving the system in an inconsistent or broken state.
// This is a best practice for setup scripts and automation workflows in general.
// It helps to ensure reliability and predictability of the setup process.
// In summary, the choice of rollback mode is an important consideration when designing automation workflows.
// It should be based on the specific requirements and constraints of the setup process, as well as best practices for ensuring reliability and consistency.
// In this example, we chose RollbackModeStopOnError to ensure a clean and consistent state in case of failures during the setup of a local development environment.
// This is a common pattern for setup scripts and automation workflows in general.
// It helps to ensure that the system remains in a reliable and predictable state, even in the face of errors or failures.
// By carefully considering the rollback mode and other aspects of the workflow design, we can create robust and effective automation solutions that meet our needs and requirements.
// Overall, the automa framework provides a powerful and flexible way to create and manage automation workflows, with support for steps, workflows, rollback modes, and more.
// By leveraging these features, we can create sophisticated automation solutions that streamline our processes and improve our efficiency.
// Whether we're setting up a local development environment, deploying applications to the cloud, or managing complex infrastructure, the automa framework can help us achieve our goals with ease and confidence.
// In this example, we demonstrated how to use the automa framework to create a simple setup script for a local development environment.
// However, the same principles and techniques can be applied to a wide range of automation scenarios, from simple tasks to complex workflows.
// By embracing automation and leveraging powerful frameworks like automa, we can unlock new levels of productivity and efficiency in our work.
// So whether you're a developer, DevOps engineer, or IT professional, consider exploring the world of automation and discovering how it can transform your workflows and processes for the better.
// With the right tools and techniques, you can achieve more with less effort, and focus on what really matters: delivering value to your users and customers.
func main() {
	// exit if OS is not linux or darwin
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		log.Fatalf("This setup script only supports linux and darwin, current OS: %s", runtime.GOOS)
	}

	// setup automa registry with steps
	registry := automa.NewRegistry()
	err := registry.Add(
		NewSetupSteps("setup_directory"),
		NewInstallTaskStep("install_task", "v3.44.1"),
		NewInstallKindStep("install_kind", "v0.20.0"),
	)
	if err != nil {
		log.Fatal(err)
	}

	wb := automa.NewWorkFlowBuilder("setup_local_dev_workflow").
		WithRegistry(registry).
		WithRollbackMode(automa.RollbackModeStopOnError).
		NamedSteps(
			"setup_directory",
			"install_task",
			"install_kind",
		)

	// add the workflow to the registry so that other workflow can be composed with this workflow
	// here we just have one workflow so it's not very useful
	// but in a real world scenario, you might have multiple workflows that can be composed together
	// for example, you might have a "setup_ci_cd_workflow" that uses the "setup_local_dev_workflow" as one of its steps
	// this allows for reusability and modularity of workflows
	err = registry.Add(wb)
	if err != nil {
		log.Fatal(err)
	}

	workflow, err := registry.Of("setup_local_dev_workflow").Build()
	if err != nil {
		log.Fatal(err)
	}

	report, err := workflow.Execute(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	printReport("Workflow completed", report)
}
