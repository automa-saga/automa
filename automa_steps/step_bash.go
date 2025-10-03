package automa_steps

import (
	"bytes"
	"context"
	"github.com/automa-saga/automa"
	"os/exec"
)

// RunBashScript executes a list of bash scripts in the specified working directory.
// It captures and logs the output of each command if a logger is provided.
// If any command fails, it returns an error immediately.
func RunBashScript(scripts []string, workDir string) (string, error) {
	var outputs bytes.Buffer // To capture combined output of all scripts

	if len(scripts) == 0 {
		return "", automa.StepExecutionError.New("no scripts provided")
	}

	for _, script := range scripts {
		c := exec.Command("bash", "-c", script)
		if workDir != "" {
			c.Dir = workDir
		}

		var out bytes.Buffer
		c.Stdout = &out
		c.Stderr = &out

		err := c.Run()
		output := out.Bytes()

		if err != nil {
			return outputs.String(), automa.StepExecutionError.New("command failed: %s\nerror: %v", script, err)
		}

		outputs.Write(output)
	}

	return outputs.String(), nil
}

// BashScriptStep creates a new step builder that executes a list of bash scripts in the specified working directory.
// The returned StepBuilder can be further configured via method chaining (e.g., to add rollback, onPrepare, or completion functions).
// The step will return a success report if all scripts execute successfully, otherwise it returns an error report.
func BashScriptStep(id string, scripts []string, workDir string) *automa.StepBuilder {
	return automa.NewStepBuilder().
		WithId(id).
		WithExecute(func(ctx context.Context) *automa.Report {
			output, err := RunBashScript(scripts, workDir)
			if err != nil {
				return automa.StepFailureReport(id, automa.WithError(err))
			}

			return automa.StepSuccessReport(id, automa.WithMetadata(map[string]string{
				"output": output,
			}))
		})
}
