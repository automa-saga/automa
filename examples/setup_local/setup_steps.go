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

const SetupDir = "/tmp/test-automa"

func NewSetupSteps(id string) automa.Builder {
	dirs := []string{
		path.Join(SetupDir, "bin"),
		path.Join(SetupDir, "config"),
		path.Join(SetupDir, "data"),
	}
	script := fmt.Sprintf("mkdir -p %s", strings.Join(dirs, " "))
	return automa_steps.NewBashScriptStep(id, []string{script}, "").
		WithRollback(func(ctx context.Context) *automa.Report {
			_, err := os.Stat(SetupDir)
			if err != nil {
				if os.IsNotExist(err) {
					return automa.StepSuccessReport(id, automa.WithActionType(automa.ActionRollback))
				}
				return automa.StepFailureReport(id, automa.WithActionType(automa.ActionRollback), automa.WithError(err))
			}
			err = os.RemoveAll(SetupDir)
			if err != nil {
				return automa.StepFailureReport(id, automa.WithActionType(automa.ActionRollback), automa.WithError(err))
			}
			return automa.StepSuccessReport(id, automa.WithActionType(automa.ActionRollback))
		})
}

func NewInstallTaskStep(id, version string) automa.Builder {
	installCmd := strings.TrimSpace(fmt.Sprintf(`
if ! command -v %s/bin/task &> /dev/null; then
	curl -sL https://taskfile.dev/install.sh | sh -s -- -d -b %s/bin %s
	chmod +x %s/bin/task
fi`, SetupDir, SetupDir, version, SetupDir))
	return automa_steps.NewBashScriptStep(id, []string{installCmd}, "").
		WithRollback(func(ctx context.Context) *automa.Report {
			p := path.Join(SetupDir, "bin", "task")
			_, err := os.Stat(p)
			if err != nil {
				if os.IsNotExist(err) {
					return automa.StepSuccessReport(id, automa.WithActionType(automa.ActionRollback))
				}
				return automa.StepFailureReport(id, automa.WithActionType(automa.ActionRollback), automa.WithError(err))
			}
			err = os.Remove(p)
			if err != nil {
				return automa.StepFailureReport(id, automa.WithError(err))
			}
			return automa.StepSuccessReport(id, automa.WithActionType(automa.ActionRollback))
		})
}

func NewInstallKindStep(id, version string) automa.Builder {
	installCmd := strings.TrimSpace(fmt.Sprintf(`
if ! command -v %s/bin/kind &> /dev/null; then
 curl -sL https://kind.sigs.k8s.io/dl/%s/kind-linux-amd64 -o %s/bin/kind
 chmod +x %s/bin/kind
fi`, SetupDir, version, SetupDir, SetupDir))
	return automa_steps.NewBashScriptStep(id, []string{installCmd}, "").
		WithRollback(func(ctx context.Context) *automa.Report {
			p := path.Join(SetupDir, "bin", "kind")
			_, err := os.Stat(p)
			if err != nil {
				if os.IsNotExist(err) {
					return automa.StepSuccessReport(id, automa.WithActionType(automa.ActionRollback))
				}
				return automa.StepFailureReport(id, automa.WithActionType(automa.ActionRollback), automa.WithError(err))
			}
			err = os.Remove(p)
			if err != nil {
				return automa.StepFailureReport(id, automa.WithActionType(automa.ActionRollback), automa.WithError(err))
			}
			return automa.StepSuccessReport(id, automa.WithActionType(automa.ActionRollback))
		})
}
