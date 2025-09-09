package main

import (
	"fmt"
	"github.com/automa-saga/automa"
	"strings"
)

func NewInstallTaskStep(id, version string) automa.Builder {
	installCmd := strings.TrimSpace(fmt.Sprintf(`
	if ! command -v task &> /dev/null; then
		curl -sL https://taskfile.dev/install.sh | sh -s -- -d -b /usr/local/bin %s
	fi`, version))
	return automa.NewBashScriptStep(id, []string{installCmd}, "")
}

func NewInstallKubectlStep(id, version string) automa.Builder {
	installCmd := strings.TrimSpace(fmt.Sprintf(`
	if ! command -v kubectl &> /dev/null; then
		curl -LO "https://dl.k8s.io/release/%s/bin/linux/amd64/kubectl"
		install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
		rm kubectl
	fi`, version))
	return automa.NewBashScriptStep(id, []string{installCmd}, "")
}

func NewInstallKindStep(id, version string) automa.Builder {
	installCmd := strings.TrimSpace(fmt.Sprintf(`
	if ! command -v kind &> /dev/null; then
		curl -Lo ./kind https://kind.sigs.k8s.io/dl/%s/kind-linux-amd64
		chmod +x ./kind
		mv ./kind /usr/local/bin/kind
	fi`, version))
	return automa.NewBashScriptStep(id, []string{installCmd}, "")
}
