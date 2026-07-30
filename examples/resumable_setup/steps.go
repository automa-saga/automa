package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/automa-saga/automa"
)

// Step IDs for the resumable provisioning workflow.
const (
	stepWorkdir  = "create_workdir"
	stepConfig   = "write_config"
	stepDatabase = "provision_database"
	stepFinalize = "finalize"
)

// crashFn simulates a hard process crash (power loss / kill -9). In the CLI it
// is os.Exit; tests override it so they can observe resume without killing the
// test process. A hard crash is the interesting case for durability: it stops
// the process AFTER a step's side effect but before the commit record, leaving
// the journal with that step `started`.
var crashFn = func() { os.Exit(1) }

// fileStep returns a step that idempotently creates a marker file under dir.
//
// Idempotency is the durability author contract (durability-spec §7): running a
// step twice must equal running it once. Here that means checking whether the
// marker already exists and doing nothing if so — which is exactly what makes it
// safe for resume to re-run a step that was `started` but never confirmed
// `completed` before a crash.
//
// If crashAfter equals this step's ID, the step simulates a crash right after
// writing its file (before returning), so the journal is left mid-workflow.
func fileStep(id, dir, filename, crashAfter string) *automa.StepBuilder {
	target := filepath.Join(dir, filename)
	return automa.NewStepBuilder().WithId(id).
		WithExecute(func(ctx context.Context, stp automa.Step) *automa.Report {
			if _, err := os.Stat(target); err == nil {
				fmt.Printf("  = %s (already done; skipping side effect)\n", id)
				return automa.SuccessReport(stp)
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return automa.FailureReport(stp, automa.WithError(err))
			}
			if err := os.WriteFile(target, []byte(id+"\n"), 0o644); err != nil {
				return automa.FailureReport(stp, automa.WithError(err))
			}
			fmt.Printf("  + %s\n", id)
			if crashAfter == id {
				fmt.Printf("  ! simulating a crash right after %s\n", id)
				crashFn()
			}
			return automa.SuccessReport(stp)
		}).
		WithRollback(func(ctx context.Context, stp automa.Step) *automa.Report {
			_ = os.Remove(target)
			fmt.Printf("  - rolled back %s\n", id)
			return automa.SuccessReport(stp)
		})
}

// buildWorkflow assembles the provisioning workflow. crashAfter names a step to
// crash after (empty on the resume run).
func buildWorkflow(dir, crashAfter string) *automa.WorkflowBuilder {
	return automa.NewWorkflowBuilder().
		WithId("resumable_setup").
		WithExecutionMode(automa.RollbackOnError).
		Steps(
			fileStep(stepWorkdir, dir, ".workdir", crashAfter),
			fileStep(stepConfig, dir, "config.yaml", crashAfter),
			fileStep(stepDatabase, dir, "database.id", crashAfter),
			fileStep(stepFinalize, dir, "DONE", crashAfter),
		)
}
