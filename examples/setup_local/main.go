package main

import (
	"context"
	"fmt"
	"github.com/automa-saga/automa"
	"github.com/briandowns/spinner"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"runtime"
	"sync"
	"time"
)

const (
	greenTick      = "\033[32m✔\033[0m"
	spinnerCharset = 14 // Dots
)

// buildWorkflow builds the workflow with steps and spinners
//
// This function builds the workflow with the steps defined in setup_steps.go.
// It also sets up spinners for each step to indicate progress.
// The spinners will run in separate goroutines,
// so we need to ensure that we wait for them to complete before exiting the program.
//
// We use a wait group to wait for all steps to complete,
// and another wait group to wait for the workflow to complete.
// This ensures that we don't exit the program before all steps are done.
//
// We also set up onPrepare, onCompletion, and onFailure callbacks
// to handle starting and stopping the spinners appropriately.
//
// The onPrepare callback starts the spinner for the step,
// and the onCompletion and onFailure callbacks stop the spinner
// and print the result of the step (success or failure).
//
// This is important because the spinners run in separate goroutines.
// We need to start them before the step starts executing,
// so that we can see the spinner while the step is running.
// And we need to stop them when the step completes,
// so that we can see the final result of the step.
//
// So always remember to wait for your spinners to complete!
// Check the OnCompletion callback for the workflow below.
// Otherwise, you may end up with a messy CLI output!
// Happy automating!
func buildWorkflow(wg *sync.WaitGroup) *automa.WorkflowBuilder {
	// wait group to wait for all steps to complete
	var wgStep sync.WaitGroup

	// map to hold spinners for each step
	spinners := map[string]*spinner.Spinner{}

	// startSpinner starts a spinner for the given step id
	startSpinner := func(id string, ws *sync.WaitGroup) *spinner.Spinner {
		ws.Add(1)
		s := spinner.New(spinner.CharSets[spinnerCharset], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
		s.Suffix = fmt.Sprintf(" %s", id)
		s.Start()
		spinners[id] = s
		return s
	}

	// stopSpinner stops spinner and prints result
	//
	// This is extracted to a separate function to avoid code duplication
	// between onCompletion and onFailure callbacks below.
	//
	// It stops the spinner for the step, if it exists,
	// and prints the result of the step (success or failure).
	// It also marks the step as done in the wait group,
	// so that we can wait for all steps to complete before exiting
	// the program.
	stopSpinner := func(report *automa.Report, spinners map[string]*spinner.Spinner, ws *sync.WaitGroup) {
		ws.Done()
		if s, exists := spinners[report.Id]; exists {
			s.Stop()
			if report.IsFailed() {
				fmt.Printf("✘ %s\n", report.Id)
			} else {
				fmt.Printf("%s %s\n", greenTick, report.Id)
			}
			delete(spinners, report.Id) // remove spinner from map
		} else {
			fmt.Printf("No spinner found for step: %s\n", report.Id)
		}
	}

	// onPrepare callback to start spinner
	//
	// This will be called when each step is about to start.
	// It will start a spinner for the step and store it in the spinners map.
	// The spinner will be stopped in the onCompletion callback
	//
	// This is important because the spinners run in separate goroutines.
	// We need to start them before the step starts executing,
	// so that we can see the spinner while the step is running.
	onPrepare := func(ctx context.Context, stp automa.Step) (context.Context, error) {
		startSpinner(st.Id(), &wgStep)
		return ctx, nil
	}

	// onCompletion and onFailure callbacks stop spinner and print result
	//
	// This will be called when each step completes.
	// It will stop the spinner and print the result.
	// It will also mark the step as done in the wait group,
	// so that we can wait for all steps to complete before exiting
	// the program.
	//
	// This is important because the spinners run in separate goroutines.
	// We need to wait for them to complete before exiting;
	// Otherwise we may not see the final output of the spinners
	// If the program exits before they complete
	//
	// This is a common pattern when using spinners in CLI applications
	// to ensure that the output is clean and readable
	// and that we don't leave any dangling goroutines running,
	// which can cause issues with resource usage and stability
	// of the application.
	//
	// So always remember to wait for your spinners to complete! Check the OnCompletion callback for the workflow below.
	// Otherwise, you may end up with a messy CLI output!
	//
	onCompletion := func(ctx context.Context, stp automa.Step, report *automa.Report) {
		stopSpinner(report, spinners, &wgStep)
	}
	onFailure := func(ctx context.Context, stp automa.Step, report *automa.Report) {
		stopSpinner(report, spinners, &wgStep)
	}

	// build workflow
	// A workflow can be composed of other workflows as steps.
	// Here we have a main workflow that sets up the local dev environment,
	// and a nested workflow that installs the tools.
	//
	// Each step and workflow has its own prepare, onCompletion, and onFailure callbacks
	// to handle starting and stopping the spinners appropriately.
	//
	// The main workflow will wait for all steps to complete before exiting the program,
	// using the wait group defined above.
	//
	// This ensures that we don't exit the program before all steps are done.
	workflow := automa.NewWorkflowBuilder().
		WithId("setup_local_dev").
		WithPrepare(func(ctx context.Context, stp automa.Step) (context.Context, error) {
			err := os.RemoveAll(setupDir)
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			}

			return ctx, nil
		}).
		Steps(
			setupDirectories().
				WithPrepare(onPrepare).
				WithOnCompletion(onCompletion).
				WithOnFailure(onFailure),
			automa.NewWorkflowBuilder().
				WithId("install_tools").
				Steps(
					installTask("v3.44.1").
						WithPrepare(onPrepare).
						WithOnCompletion(onCompletion).
						WithOnFailure(onFailure),
					installKind("v0.2x.x"). // pass an incorrect version to test failure and rollback
								WithPrepare(onPrepare).
								WithOnCompletion(onCompletion).
								WithOnFailure(onFailure),
				),
		).
		WithRollbackMode(automa.RollbackModeStopOnError).
		WithOnCompletion(func(ctx context.Context, stp automa.Step, report *automa.Report) {
			wgStep.Wait()
			wg.Done() // we mark is done only after all steps are complete
		}).
		WithOnFailure(func(ctx context.Context, stp automa.Step, report *automa.Report) {
			go func() {
				// call wgStep.Wait() for the steps that we started
				for id, sp := range spinners {
					sp.Stop()
					fmt.Printf("✘ %s - stopped because of failure\n", id)
					wgStep.Done()
				}
			}()
			wgStep.Wait()

			wg.Done()
		})

	return workflow
}

// printReport prints the report in YAML format
//
// This is a utility function to print the report in a readable format.
// It uses the yaml package to marshal the report struct to YAML format.
// This is useful for debugging and understanding the result of the workflow.
func printReport(msg string, report *automa.Report) string {
	out, err := yaml.Marshal(report)
	if err != nil {
		log.Fatalf("Error marshalling report to YAML: %v", err)
	}
	fmt.Printf("\n%s\n", msg)
	fmt.Println("--------------------------------------------------------")
	fmt.Printf("%s\n", out)

	if report.IsFailed() {
		fmt.Printf("%s\n", report.Error)
		fmt.Println("\n--------------------------------------------------------")
		fmt.Printf("✘ Workflow failed at step: %s", report.Id)
		fmt.Println("\n--------------------------------------------------------")
	} else {
		fmt.Println("\n--------------------------------------------------------")
		fmt.Printf("%s Workflow completed successfully!\n", greenTick)
		fmt.Println("\n--------------------------------------------------------")
	}

	return string(out)
}

func main() {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		log.Fatalf("This setup script only supports linux and darwin, current OS: %s", runtime.GOOS)
	}

	// wait group to wait for workflow to complete
	var wg sync.WaitGroup

	// build workflow
	workflow := buildWorkflow(&wg)

	// Execute the workflow in a separate goroutine and wait for completion.
	// Use sync.WaitGroup to ensure main waits for workflow to finish.
	reportCh := make(chan *automa.Report, 1)
	wg.Add(1)
	go func() {
		// we don't do wg.Done() here, because we need to wait for all steps to complete
		// before exiting the program. This is handled in the OnCompletion callback of the workflow.
		// See buildWorkflow function above for details.
		fmt.Println("Starting workflow...")
		report := automa.RunWorkflow(context.Background(), workflow)
		fmt.Println("Finished workflow...")

		// If the workflow preparation fails, no steps will be executed,
		// so we need to mark the wait group as done here.
		//
		// This ensures that we don't leave the wait group hanging
		// if the workflow fails to start.
		if report.IsFailed() && report.Action == automa.ActionPrepare {
			wg.Done()
		}

		reportCh <- report
	}()
	wg.Wait()

	report := <-reportCh
	printReport("\nWorkflow Report", report)
}
