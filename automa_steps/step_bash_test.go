package automa_steps

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/automa-saga/automa"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestRunBashScript_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	err := RunBashScript([]string{"echo hello"}, "", &logger)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "command output: hello")
	assert.Contains(t, buf.String(), "command succeeded: echo hello")
}

func TestRunBashScript_Failure(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	err := RunBashScript([]string{"exit 1"}, "", &logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command failed: exit 1")
}

func TestRunBashScript_NoLogger(t *testing.T) {
	err := RunBashScript([]string{"echo no-log"}, "", nil)
	assert.NoError(t, err)
}

func TestRunBashScript_InvalidUTF8(t *testing.T) {
	// Simulate a command that outputs invalid UTF-8 (using printf with hex bytes)
	// This test is platform dependent, so we check that it does not panic or log
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	// Output invalid UTF-8: \xff
	err := RunBashScript([]string{"printf '\\xff'"}, "", &logger)
	assert.NoError(t, err)
	// Should not log output
	assert.NotContains(t, buf.String(), "command output")
}

func TestNewBashScriptStep_Success(t *testing.T) {
	step := NewBashScriptStep("bash-step", []string{"echo test"}, "")
	assert.Equal(t, "bash-step", step.Id())
	s, err := step.Build()
	assert.NoError(t, err)
	assert.NotNil(t, s)
	report, err := s.Execute(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, automa.StatusSuccess, report.Status)
}

func TestNewBashScriptStep_Failure(t *testing.T) {
	step := NewBashScriptStep("fail-step", []string{"exit 2"}, "")
	s, err := step.Build()
	assert.NoError(t, err)
	assert.NotNil(t, s)
	report, err := s.Execute(context.Background())
	assert.Error(t, err)
	assert.Nil(t, report)
}

func TestNewBashScriptStep_LoggerOption(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	step := NewBashScriptStep("log-step", []string{"echo log"}, "", automa.WithLogger(logger))
	s, err := step.Build()
	assert.NoError(t, err)
	_, err = s.Execute(context.Background())
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "command output: log")
}

func TestNewBashScriptStep_OverridesOnExecute(t *testing.T) {
	// Provide a custom OnExecute that should be overridden
	opt := automa.WithOnExecute(func(ctx context.Context) (*automa.Report, error) {
		return nil, errors.New("should not be called")
	})
	step := NewBashScriptStep("override-step", []string{"echo override"}, "", opt)
	s, err := step.Build()
	assert.NoError(t, err)
	report, err := s.Execute(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, report)
}

func TestRunBashScript_WorkDir(t *testing.T) {
	// Create a temp dir and file, then list it
	dir := t.TempDir()
	file := dir + "/foo.txt"
	err := os.WriteFile(file, []byte("bar"), 0644)
	assert.NoError(t, err)
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	err = RunBashScript([]string{"ls foo.txt"}, dir, &logger)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "foo.txt")
}
