package automa_steps

import (
	"context"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"

	"github.com/automa-saga/automa"
	"github.com/stretchr/testify/assert"
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
