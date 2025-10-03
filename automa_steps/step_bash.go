package automa_steps

import (
	"bytes"
	"context"
	"github.com/automa-saga/automa"
	"github.com/rs/zerolog"
	"os/exec"
	"unicode/utf8"
)

// RunBashScript executes a list of bash scripts in the specified working directory.
// It captures and logs the output of each command if a logger is provided.
// If any command fails, it returns an error immediately.
func RunBashScript(scripts []string, workDir string, logger *zerolog.Logger) error {
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

		if logger != nil && len(output) > 0 && utf8.Valid(output) {
			logger.Info().Msgf("command output: %s", string(output))
		}

		if err != nil {
			return automa.StepExecutionError.New("command failed: %s\nerror: %v", script, err)
		}

		if logger != nil {
			logger.Info().Msgf("command succeeded: %s", script)
		}
	}

	return nil
}

// NewBashScriptStep creates a new step that executes a list of bash scripts in the specified working directory.
// Caller can optionally provide Rollback, onPrepare, completion functions via opts.
// Note, any execute function provided in opts will be overridden.
// The step returns a success report if all scripts execute successfully, otherwise it returns an error report.
func NewBashScriptStep(id string, scripts []string, workDir string, opts ...automa.StepOption) automa.Builder {
	sb := automa.NewStepBuilder(id, opts...)

	// Define the execute function to run the bash scripts.
	// Note, it overrides any execute function provided in opts.
	sb.execute = func(ctx context.Context) (*automa.Report, error) {
		err := RunBashScript(scripts, workDir, sb.Logger())
		if err != nil {
			return nil, err
		}

		return automa.SuccessReport(sb.Id()), nil
	}

	return sb
}
