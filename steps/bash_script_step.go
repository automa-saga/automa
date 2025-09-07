package steps

import (
	"fmt"
	"os/exec"
	"strings"
)

type BashScriptStep struct {
	Cmd        []string `yaml:"run" json:"run"`
	WorkingDir string   `yaml:"working_dir" json:"working_dir"`
}

func (b *BashScriptStep) Name() string {
	return "BashScriptStep"
}

func (b *BashScriptStep) Run() error {
	for _, cmdStr := range b.Cmd {
		cmd := exec.Command("bash", "-c", cmdStr)
		if b.WorkingDir != "" {
			cmd.Dir = b.WorkingDir
		}
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("command failed: %s\nerror: %v\noutput: %s", cmdStr, err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}
