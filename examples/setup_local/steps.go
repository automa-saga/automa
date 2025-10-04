package main

import (
	"context"
	"fmt"
	"github.com/automa-saga/automa"
	"github.com/automa-saga/automa/automa_steps"
	"os"
	"path"
	"strings"
)

const (
	setupDir          = "/tmp/test-automa"
	setupDirStepId    = "setup_directory"
	installTaskStepId = "install_task"
	installKindStepId = "install_kind"
)

func setupDirectories() *automa.StepBuilder {
	dirs := []string{
		path.Join(setupDir, "bin"),
		path.Join(setupDir, "config"),
		path.Join(setupDir, "data"),
	}
	script := fmt.Sprintf("mkdir -p %s", strings.Join(dirs, " "))
	return automa_steps.BashScriptStep(setupDirStepId, []string{script}, "").
		WithRollback(func(ctx context.Context) *automa.Report {
			_, err := os.Stat(setupDir)
			if err != nil {
				if os.IsNotExist(err) {
					return automa.StepSuccessReport(setupDirStepId, automa.WithActionType(automa.ActionRollback))
				}
				return automa.StepFailureReport(setupDirStepId, automa.WithActionType(automa.ActionRollback), automa.WithError(err))
			}
			err = os.RemoveAll(setupDir)
			if err != nil {
				return automa.StepFailureReport(setupDirStepId, automa.WithActionType(automa.ActionRollback), automa.WithError(err))
			}
			return automa.StepSuccessReport(setupDirStepId, automa.WithActionType(automa.ActionRollback))
		})
}

func installTask(version string) *automa.StepBuilder {
	installCmd := strings.TrimSpace(fmt.Sprintf(`
if ! command -v %[1]s/bin/task &> /dev/null; then
  curl -sL https://taskfile.dev/install.sh | sh -s -- -d -b %[1]s/bin %[2]s
  if [ ! -s %[1]s/bin/task ] || [ $(stat -f%%z %[1]s/bin/task) -lt 1000000 ]; then
    echo "Download failed or file too small"
    rm -f %[1]s/bin/task
    exit 1
  fi
  chmod +x %[1]s/bin/task
fi`, setupDir, version))
	return automa_steps.BashScriptStep(installTaskStepId, []string{installCmd}, "").
		WithRollback(func(ctx context.Context) *automa.Report {
			p := path.Join(setupDir, "bin", "task")
			_, err := os.Stat(p)
			if err != nil {
				if os.IsNotExist(err) {
					return automa.StepSuccessReport(installTaskStepId, automa.WithActionType(automa.ActionRollback))
				}
				return automa.StepFailureReport(installTaskStepId, automa.WithActionType(automa.ActionRollback), automa.WithError(err))
			}
			err = os.Remove(p)
			if err != nil {
				return automa.StepFailureReport(installTaskStepId, automa.WithActionType(automa.ActionRollback), automa.WithError(err))
			}
			return automa.StepSuccessReport(installTaskStepId, automa.WithActionType(automa.ActionRollback))
		})
}

func installKind(version string) *automa.StepBuilder {
	installCmd := strings.TrimSpace(fmt.Sprintf(`
if ! command -v %[1]s/bin/kind &> /dev/null; then
  curl -sL https://kind.sigs.k8s.io/dl/%[2]s/kind-linux-amd64 -o %[1]s/bin/kind
  if [ ! -s %[1]s/bin/kind ] || [ $(stat -f%%z %[1]s/bin/kind) -lt 1000000 ]; then
    echo "Download failed or file too small"
    rm -f %[1]s/bin/kind
    exit 1
  fi
  chmod +x %[1]s/bin/kind
fi`, setupDir, version))
	return automa_steps.BashScriptStep(installKindStepId, []string{installCmd}, "").
		WithRollback(func(ctx context.Context) *automa.Report {
			p := path.Join(setupDir, "bin", "kind")
			_, err := os.Stat(p)
			if err != nil {
				if os.IsNotExist(err) {
					return automa.StepSuccessReport(installKindStepId, automa.WithActionType(automa.ActionRollback))
				}
				return automa.StepFailureReport(installKindStepId, automa.WithActionType(automa.ActionRollback), automa.WithError(err))
			}
			err = os.Remove(p)
			if err != nil {
				return automa.StepFailureReport(installKindStepId, automa.WithActionType(automa.ActionRollback), automa.WithError(err))
			}
			return automa.StepSuccessReport(installKindStepId, automa.WithActionType(automa.ActionRollback))
		})
}
