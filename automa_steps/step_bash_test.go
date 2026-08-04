package automa_steps

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/automa-saga/automa"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunBashScript_Success(t *testing.T) {
	output, err := RunBashScript([]string{"echo hello", "echo world"}, "")
	assert.NoError(t, err)
	assert.Contains(t, output, "hello")
	assert.Contains(t, output, "world")
}

func TestRunBashScript_CommandError(t *testing.T) {
	output, err := RunBashScript([]string{"exit 1"}, "")
	assert.Error(t, err)
	assert.Empty(t, output)
	assert.Contains(t, err.Error(), "command failed")
}

func TestRunBashScript_EmptyScripts(t *testing.T) {
	output, err := RunBashScript([]string{}, "")
	assert.Error(t, err)
	assert.Empty(t, output)
	assert.Contains(t, err.Error(), "no scripts provided")
}

func TestRunBashScriptContext_CancelledContextStopsCommand(t *testing.T) {
	// A cancelled context must terminate the running command rather than let it
	// sleep to completion; exec.CommandContext kills the process on ctx.Done().
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := RunBashScriptContext(ctx, []string{"sleep 5"}, "")
	elapsed := time.Since(start)

	require.Error(t, err, "a context-cancelled command must fail, not run to completion")
	assert.Less(t, elapsed, 4*time.Second, "command should be killed shortly after ctx expiry, not after the full sleep")
}

func TestRunBashScript_WorkDir(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	script := "echo foo > test.txt"
	_, err := RunBashScript([]string{script}, tmpDir)
	assert.NoError(t, err)

	// Check file was created in tmpDir
	data, err := os.ReadFile(testFile)
	assert.NoError(t, err)
	assert.Equal(t, "foo\n", string(data))
}

func TestNewBashScriptStep_Success(t *testing.T) {
	step, err := BashScriptStep("bash1", []string{"echo ok"}, "").Build()
	assert.NoError(t, err)

	report := step.Execute(context.Background())
	assert.Equal(t, automa.StatusSuccess, report.Status)
	assert.Equal(t, "bash1", report.Id)
	assert.Contains(t, report.Metadata["output"], "ok")
}

func TestNewBashScriptStep_Failure(t *testing.T) {
	step, err := BashScriptStep("bash2", []string{"exit 2"}, "").Build()
	assert.NoError(t, err)
	report := step.Execute(context.Background())
	assert.Equal(t, automa.StatusFailed, report.Status)
	assert.Equal(t, "bash2", report.Id)
	assert.True(t, report.HasError())
	assert.Contains(t, report.Error.Error(), "command failed")
}

func TestNewBashScriptStep_EmptyScripts(t *testing.T) {
	step, err := BashScriptStep("bash3", []string{}, "").Build()
	require.NoError(t, err)

	report := step.Execute(context.Background())
	assert.Equal(t, automa.StatusFailed, report.Status)
	assert.Equal(t, "bash3", report.Id)
	assert.True(t, report.HasError())
	assert.Contains(t, report.Error.Error(), "no scripts provided")
}
