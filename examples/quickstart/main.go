// Command quickstart is the smallest complete automa workflow: two steps with
// compensating rollbacks, run to completion, with the resulting report printed
// as JSON.
//
// The second step fails, so automa halts and compensates the already-executed
// steps in reverse order (the saga pattern). Run it with:
//
//	go run ./examples/quickstart
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/automa-saga/automa"
)

func main() {
	// A step is defined by its Execute (the work) and an optional Rollback (the
	// compensating action run if a later step fails).
	reserveInventory := automa.NewStepBuilder().
		WithId("reserve_inventory").
		WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
			fmt.Println("→ reserving inventory")
			return automa.SuccessReport(stp)
		}).
		WithRollback(func(ctx context.Context, stp automa.Step) *automa.Report {
			fmt.Println("← releasing inventory")
			return automa.SuccessReport(stp)
		})

	chargeCard := automa.NewStepBuilder().
		WithId("charge_card").
		WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
			fmt.Println("→ charging card")
			// Simulate a failure to trigger the saga rollback.
			return automa.FailureReport(stp, automa.WithError(fmt.Errorf("card declined")))
		}).
		WithRollback(func(ctx context.Context, stp automa.Step) *automa.Report {
			fmt.Println("← refunding card")
			return automa.SuccessReport(stp)
		})

	// A workflow is an ordered list of steps. RollbackOnError compensates
	// executed steps in reverse order when any step fails.
	wb := automa.NewWorkflowBuilder().
		WithId("checkout").
		WithExecutionMode(automa.RollbackOnError).
		Steps(reserveInventory, chargeCard)

	report := automa.RunWorkflow(context.Background(), wb)

	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))

	if report.IsFailed() {
		os.Exit(1)
	}
}
