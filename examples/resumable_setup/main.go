// Command resumable_setup is a runnable demonstration of automa's crash-recovery
// durability. It provisions a few marker files under a work directory, journaling
// its progress. Crash it partway through and re-run it: it resumes from where it
// stopped instead of starting over.
//
// Usage:
//
//	# First run: crash right after the "write_config" step.
//	go run ./examples/resumable_setup -crash-at=write_config
//
//	# Re-run with no crash: it resumes — completed steps are skipped, the rest run.
//	go run ./examples/resumable_setup
//
// The single call site is ResumeWorkflow: with no journal yet it starts fresh
// and creates one; with an existing journal it continues from the recorded point.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/automa-saga/automa"
)

func main() {
	var journalPath, workDir, crashAt string
	flag.StringVar(&journalPath, "journal", "/tmp/automa-resumable/setup.journal", "journal file path")
	flag.StringVar(&workDir, "work", "/tmp/automa-resumable/work", "work directory the steps provision")
	flag.StringVar(&crashAt, "crash-at", "", "step ID to simulate a crash after (leave empty on the resume run)")
	flag.Parse()

	if err := os.MkdirAll(filepath.Dir(journalPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "cannot create journal directory: %v\n", err)
		os.Exit(1)
	}

	wb := buildWorkflow(workDir, crashAt)

	fmt.Printf("Running resumable_setup (journal=%s, work=%s)\n", journalPath, workDir)
	if _, err := os.Stat(journalPath); err == nil {
		fmt.Println("Found an existing journal — resuming.")
	} else {
		fmt.Println("No journal yet — starting fresh.")
	}

	// ResumeWorkflow is the single entry point for both the first run and every
	// resume: a missing journal is a normal start (durability-spec §6.2).
	report := automa.ResumeWorkflow(context.Background(), wb, journalPath)
	if report.IsFailed() {
		fmt.Printf("\n✘ workflow failed: %v\n", report.Error)
		os.Exit(1)
	}

	fmt.Printf("\n✔ workflow completed. Journal: %s\n", journalPath)
	fmt.Println("(Run again and it is a safe no-op: the journal is `done`.)")
}
