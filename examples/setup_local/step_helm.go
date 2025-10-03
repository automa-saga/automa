package main

import (
	"context"
	"fmt"
	"github.com/automa-saga/automa"
	"github.com/automa-saga/automa/automa_steps"
	"strings"
)

// NewInstallHelmStep creates a step to install Helm if it's not already installed.
// It assumes a Linux environment with curl available.
// On rollback, it uninstalls Helm if the helm binary is found.
func NewInstallHelmStep(id string, version string, opts ...automa.StepOption) automa.Builder {
	installCmd := strings.TrimSpace(fmt.Sprintf(`
	if ! command -v helm &> /dev/null; then
		curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
		chmod 700 get_helm.sh
		./get_helm.sh -v %s
		rm get_helm.sh
	fi`, version))

	// add Rollback to uninstall Helm if installation was performed
	newOpts := append([]automa.StepOption{}, opts...)
	newOpts = append(opts, automa.WithOnRollback(func(ctx context.Context) (*automa.Report, error) {
		rollbackCmd := strings.TrimSpace(`
			if command -v helm &> /dev/null; then 
				rm -rf $(which helm); 
			fi
		`)
		err := automa_steps.RunBashScript([]string{rollbackCmd}, "", nil)
		if err != nil {
			return nil, err
		}
		return automa.StepSuccessReport(id), nil
	}))

	return automa_steps.NewBashScriptStep(id, []string{installCmd}, "", newOpts...)
}

// NewUninstallHelmStep creates a step to uninstall Helm.
func NewUninstallHelmStep(id string, opts ...automa.StepOption) automa.Builder {
	uninstallCmd := strings.TrimSpace(`
		if command -v helm &> /dev/null; then 
			rm -rf $(which helm); 
		fi
	`)
	return automa_steps.NewBashScriptStep(id, []string{uninstallCmd}, "", opts...)
}

// NewHelmRepoAddStep creates a step to add a Helm repo and update it.
func NewHelmRepoAddStep(id, repo, url string, opts ...automa.StepOption) automa.Builder {
	scripts := []string{
		fmt.Sprintf("helm repo add %s %s", repo, url),
		"helm repo update",
	}
	return automa_steps.NewBashScriptStep(id, scripts, "", opts...)
}

// NewHelmInstallStep creates a step to install a Helm chart and sets up rollback to uninstall on failure.
func NewHelmInstallStep(id, repo, chart, releaseName, namespace string, args []string, opts ...automa.StepOption) automa.Builder {
	argStr := strings.TrimSpace(strings.Join(args, " "))
	cmd := fmt.Sprintf("helm install %s %s/%s --namespace %s %s", releaseName, repo, chart, namespace, argStr)

	// Copy opts to avoid mutating the input slice
	newOpts := append([]automa.StepOption{}, opts...)
	newOpts = append(newOpts, automa.WithOnRollback(func(ctx context.Context) (*automa.Report, error) {
		rollbackCmd := fmt.Sprintf("helm uninstall %s --namespace %s", releaseName, namespace)
		err := automa_steps.RunBashScript([]string{rollbackCmd}, "", nil)
		if err != nil {
			return nil, err
		}
		return automa.StepSuccessReport(id), nil
	}))

	return automa_steps.NewBashScriptStep(id, []string{cmd}, "", newOpts...)
}

// NewHelmUpgradeStep creates a step to upgrade a Helm chart and sets up rollback to revert to the previous version on failure.
func NewHelmUpgradeStep(id, repo, chart, releaseName, namespace string, args []string, opts ...automa.StepOption) automa.Builder {
	argStr := strings.TrimSpace(strings.Join(args, " "))
	cmd := fmt.Sprintf("helm upgrade %s %s/%s --namespace %s %s", releaseName, repo, chart, namespace, argStr)

	// Copy opts to avoid mutating the input slice
	newOpts := append([]automa.StepOption{}, opts...)
	newOpts = append(newOpts, automa.WithOnRollback(func(ctx context.Context) (*automa.Report, error) {
		rollbackCmd := fmt.Sprintf("helm rollback %s 1 --namespace %s", releaseName, namespace)
		err := automa_steps.RunBashScript([]string{rollbackCmd}, "", nil)
		if err != nil {
			return nil, err
		}
		return automa.StepSuccessReport(id), nil
	}))

	return automa_steps.NewBashScriptStep(id, []string{cmd}, "", newOpts...)
}

// NewHelmUninstallStep creates a step to uninstall a Helm release.
func NewHelmUninstallStep(id, releaseName, namespace string, opts ...automa.StepOption) automa.Builder {
	cmd := fmt.Sprintf("helm uninstall %s --namespace %s", releaseName, namespace)
	return automa_steps.NewBashScriptStep(id, []string{cmd}, "", opts...)
}

// NewHelmListStep creates a step to list Helm releases in a namespace.
func NewHelmListStep(id, namespace string, opts ...automa.StepOption) automa.Builder {
	cmd := fmt.Sprintf("helm list --namespace %s", namespace)
	return automa_steps.NewBashScriptStep(id, []string{cmd}, "", opts...)
}
