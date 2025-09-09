package automa

import (
	"context"
	"github.com/rs/zerolog"
	"os/exec"
	"strings"
)

// RunBashScript executes a list of bash scripts in the specified working directory.
// It logs the output of each script execution using the provided logger(if any).
// If any script fails, it returns an error with details about the failure.
func RunBashScript(scripts []string, workDir string, logger *zerolog.Logger) error {
	for _, script := range scripts {
		c := exec.Command("bash", "-c", script)
		if workDir != "" {
			c.Dir = workDir
		}

		output, err := c.CombinedOutput()
		if err != nil {
			return StepExecutionError.New("command failed: %s\nerror: %v\noutput: %s",
				script, err, strings.TrimSpace(string(output)))
		}

		if logger != nil {
			logger.Info().Msgf("command succeeded: %s\noutput: %s", script, strings.TrimSpace(string(output)))
		}
	}

	return nil
}

// NewBashScriptStep creates a new step that executes a list of bash scripts in the specified working directory.
// Caller can optionally provide OnRollback, OnPrepare, OnSuccess functions via opts.
// Note, any OnExecute function provided in opts will be overridden.
// The step returns a success report if all scripts execute successfully, otherwise it returns an error report.
func NewBashScriptStep(id string, scripts []string, workDir string, opts ...StepOption) Builder {
	sb := NewStepBuilder(id, opts...)

	// Define the OnExecute function to run the bash scripts.
	// Note, it overrides any OnExecute function provided in opts.
	sb.OnExecute = func(ctx context.Context) (*Report, error) {
		err := RunBashScript(scripts, workDir, sb.Logger)
		if err != nil {
			return nil, err
		}

		return StepSuccessReport(sb.Id()), nil
	}

	return sb
}
