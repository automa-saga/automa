package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/automa-saga/automa"
)

// TestResumableSetup_CrashThenResume drives the example the way the CLI does:
// a first run that crashes after write_config, then a resume that completes.
// It asserts the resume skips the already-completed step and finishes the rest.
func TestResumableSetup_CrashThenResume(t *testing.T) {
	workDir := t.TempDir()
	journalPath := filepath.Join(t.TempDir(), "setup.journal")

	// Simulate the crash as a panic (recovered here) instead of os.Exit, so it
	// does not kill the test process. Restore the default afterward.
	original := crashFn
	t.Cleanup(func() { crashFn = original })
	crashFn = func() { panic("simulated crash") }

	// First run: crash right after write_config.
	func() {
		defer func() { _ = recover() }()
		wb := buildWorkflow(workDir, stepConfig)
		_ = automa.ResumeWorkflow(context.Background(), wb, journalPath)
	}()

	assertExists(t, filepath.Join(workDir, "config.yaml"), "write_config ran its side effect before the crash")
	assertMissing(t, filepath.Join(workDir, "DONE"), "finalize must not have run before the crash")

	// Second run: no crash → resume to completion.
	crashFn = func() {}
	report := automa.ResumeWorkflow(context.Background(), buildWorkflow(workDir, ""), journalPath)
	if report.IsFailed() {
		t.Fatalf("resume should succeed, got: %v", report.Error)
	}

	assertExists(t, filepath.Join(workDir, "DONE"), "resume must run the remaining steps to completion")
}

func assertExists(t *testing.T, path, why string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist (%s): %v", path, why, err)
	}
}

func assertMissing(t *testing.T, path, why string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s to be absent (%s)", path, why)
	}
}
