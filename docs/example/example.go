package main

import (
	"context"
	"fmt"
	"github.com/automa-saga/automa"
	"github.com/automa-saga/automa/docs/example/bl"
	"github.com/joomcode/errorx"
	"gopkg.in/yaml.v3"
	"log"
)

//w1, err := NewWorkFlowBuilder("workflow1").
//AddSteps(NewStep("step1", bl.doSomething, WithPrepare(bl.prepare), WithRollback(bl.doRollback))).
//AddSteps(NewStep("step2", bl.doSomething2, WithRollback(bl.doRollback2)))
//
//sr := NewStepRegistry()
//err := sr.AddSteps(NewStep("step1", bl.doSomething, WithRollback(bl.doRollback)))
//
//
//w1 := NewWorkFlowBuilder("workflow1").
//WithRegistry(sr).
//WithNamed(stepIDs ...string).
//Build()
//
//w2, err := NewWorkFlowBuilder().
//WithId("workflow2").
//WithLogger(logger).
//RollbackMode("FAIL_ON_ERR"). // or "CONTINUE_ON_ERR" WARN_OR_ERR, IGNORE_ERR
//AddSteps(s1, s2, sr.Of("wf1", "wf2"), s3, sr.Of("wf3"), sr.Of("s4", "s5"))

func SimpleWorkflowExample() (automa.Workflow, error) {
	// Create a workflow builder
	wb := automa.NewWorkFlowBuilder("example_workflow")

	// Add steps to the workflow
	err := wb.AddSteps(
		automa.NewStep("step1", bl.Step1Execute, automa.WithRollback(bl.Step1Rollback)),
		automa.NewStep("step2", bl.Step2Execute),
	)

	// Check for errors in adding steps
	if err != nil {
		return nil, errorx.IllegalState.New("Error adding steps: %v", err)
	}

	// Build the workflow
	return wb.Build(), nil
}

func CompositeWorkflowExample() (automa.Workflow, error) {
	// Create a workflow builder
	wf1 := automa.NewWorkFlowBuilder("workflow1")

	// Add steps to the first workflow
	err := wf1.AddSteps(
		automa.NewStep("step1", bl.Step1Execute, automa.WithRollback(bl.Step1Rollback)),
		automa.NewStep("step2", bl.Step2Execute),
	)

	// Check for errors in adding steps
	if err != nil {
		return nil, errorx.IllegalState.New("Error adding steps: %v", err)
	}

	// Build the second workflow
	wf2 := automa.NewWorkFlowBuilder("composite_workflow")

	// Add the first workflow and a new step to the second workflow
	err = wf2.AddSteps(wf1, automa.NewStep("step3", bl.Step3Execute, automa.WithRollback(bl.Step3Rollback)))

	// Check for errors in adding steps
	if err != nil {
		return nil, errorx.IllegalState.New("Error adding steps to composite workflow: %v", err)
	}

	// Build the composite workflow
	return wf2.Build(), nil
}

func WorkflowUsingStepRegistry() (automa.Workflow, error) {
	// Create a step registry
	sr := automa.NewRegistry()

	// Add steps to the registry
	err := sr.Add(
		automa.NewStep("step1", bl.Step1Execute, automa.WithRollback(bl.Step1Rollback)),
		automa.NewStep("step2", bl.Step2Execute),
		// we can also add another workflow as a step
	)

	if err != nil {
		return nil, errorx.IllegalState.New("Error adding steps to the registry: %v", err)
	}

	// Create a workflow builder using the registry
	wb := automa.NewWorkFlowBuilder("workflow_with_registry").
		WithRegistry(sr)

	err = wb.WithNamed("step1", "step2")
	if err != nil {
		return nil, errorx.IllegalState.New("Error creating workflow from registry: %v", err)
	}

	return wb.Build(), nil
}

func printReport(msg string, report automa.Report) string {
	out, err := yaml.Marshal(report)
	if err != nil {
		log.Fatalf("Error marshalling report to YAML: %v", err)
	}
	fmt.Printf("\n%s\n", msg)
	fmt.Println("--------------------------------------------------------")
	fmt.Printf("%s\n", out)
	return string(out)
}

func main() {
	ctx := context.Background()
	wf, err := SimpleWorkflowExample()
	if err != nil {
		log.Fatalf("Error creating simple workflow: %v", err)
	}

	report, err := wf.Execute(ctx)
	if err != nil {
		log.Fatalf("Error executing workflow: %v", err)
	}
	printReport("Simple Workflow executed successfully", report)

	wf, err = CompositeWorkflowExample()
	if err != nil {
		log.Fatalf("Error creating composite workflow: %v", err)
	}

	report, err = wf.Execute(ctx)
	if err != nil {
		log.Fatalf("Error executing workflow: %v", err)
	}

	printReport("Composite Workflow executed successfully", report)

	wf, err = WorkflowUsingStepRegistry()
	if err != nil {
		log.Fatalf("Error creating workflow using step registry: %v", err)
	}

	report, err = wf.Execute(ctx)
	if err != nil {
		log.Fatalf("Error executing workflow: %v", err)
	}
	printReport("Workflow using registry executed successfully", report)
}
