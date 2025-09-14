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
	return automa_steps.NewMkdirStep(id, dirs, 0755,
		automa.WithOnRollback(func(ctx context.Context) (*automa.Report, error) {
			_, err := os.Stat(SetupDir)
			if err != nil {
				if os.IsNotExist(err) {
					// directory does not exist, nothing to rollback
					return &automa.Report{
						Status: automa.StatusSuccess,
						Detail: "Directory /tmp/test-automa does not exist, nothing to rollback",
					}, nil
				}
				return nil, err
			}
			err = os.RemoveAll(SetupDir)
			if err != nil {
				return nil, err
			}
			return &automa.Report{
				Status: automa.StatusSuccess,
				Detail: "Rolled back creation of /tmp/test-automa",
			}, nil
		}))
}

func NewInstallTaskStep(id, version string) automa.Builder {
	installCmd := strings.TrimSpace(fmt.Sprintf(`
	if ! command -v %s/bin/task &> /dev/null; then
		curl -sL https://taskfile.dev/install.sh | sh -s -- -d -b %s/bin %s
		chmod +x %s/bin/task
	fi`, SetupDir, SetupDir, version, SetupDir))
	return automa_steps.NewBashScriptStep(id, []string{installCmd}, "", automa.WithOnRollback(func(ctx context.Context) (*automa.Report, error) {
		p := path.Join(SetupDir, "bin", "task")
		_, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				// task not installed, nothing to rollback
				return &automa.Report{
					Status: automa.StatusSuccess,
					Detail: "Task not installed, nothing to rollback",
				}, nil
			}

			return nil, err
		}
		err = os.Remove(p)
		if err != nil {
			return nil, err
		}
		return &automa.Report{
			Status: automa.StatusSuccess,
			Detail: "Rolled back installation of task",
		}, nil
	}))
}

func NewInstallKindStep(id, version string) automa.Builder {
	installCmd := strings.TrimSpace(fmt.Sprintf(`
	if ! command -v %s/bin/kind &> /dev/null; then
		curl -sL https://kind.sigs.k8s.io/dl/%s/kind-linux-amd64 -o %s/bin/kind 
		chmod +x %s/bin/kind
	fi`, SetupDir, version, SetupDir, SetupDir))
	return automa_steps.NewBashScriptStep(id, []string{installCmd}, "",
		automa.WithOnRollback(func(ctx context.Context) (*automa.Report, error) {
			p := path.Join(SetupDir, "bin", "kind")
			_, err := os.Stat(p)
			if err != nil {
				if os.IsNotExist(err) {
					// kind not installed, nothing to rollback
					return &automa.Report{
						Status: automa.StatusSuccess,
						Detail: "Kind not installed, nothing to rollback",
					}, nil
				}

				return nil, err
			}

			err = os.Remove(p)
			if err != nil {
				return nil, err
			}

			return &automa.Report{
				Status: automa.StatusSuccess,
				Detail: "Rolled back installation of kind",
			}, nil
		}))
}
