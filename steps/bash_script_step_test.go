package steps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBashScriptStep_Run_Success(t *testing.T) {
	step := &BashScriptStep{
		Cmd: []string{"echo hello"},
	}
	if err := step.Run(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestBashScriptStep_Run_Failure(t *testing.T) {
	step := &BashScriptStep{
		Cmd: []string{"exit 1"},
	}
	err := step.Run()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestBashScriptStep_Run_WorkingDir(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	step := &BashScriptStep{
		Cmd:        []string{"echo hi > test.txt"},
		WorkingDir: tmpDir,
	}
	if err := step.Run(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if _, err := os.Stat(testFile); err != nil {
		t.Fatalf("expected file %s to exist, got error: %v", testFile, err)
	}
}
