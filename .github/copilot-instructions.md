# Copilot Coding Agent Onboarding Instructions

## High Level Details

**Repository Purpose:**  
This repository implements the `automa` framework for orchestrating and executing automated steps, with support for step composition, reporting, error handling, and extensibility. It is designed for use in automation pipelines and workflows.

**Project Type & Size:**  
- Language: Go (Golang)
- Frameworks: Standard library, some custom packages (see `automa/`)
- Typical repo size: Small to medium (dozens of source files)
- Target runtime: Go 1.25.0+ (see `go.mod` for exact version)

## Build & Validation Instructions

**Environment Setup:**  
- Always ensure Go 1.21 or newer is installed (`go version`).
- Run `go mod tidy` to ensure dependencies are up to date before building or testing.

**Bootstrap:**  
- No explicit bootstrap script.  
- Always run `go mod tidy` after cloning or pulling changes.

**Build:**  
- Build the project with:  
  `go build ./...`  
- If you encounter missing dependencies, run `go mod tidy` and retry.

**Test:**  
- Run all tests with:  
  `go test ./...`  
- For verbose output:  
  `go test -v ./...`  
- If tests fail due to missing dependencies, run `go mod tidy` first.

**Lint:**  
- Lint with:  
  `golangci-lint run`  
- If `golangci-lint` is not installed, install via:  
  `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`

**Run:**  
- There is no main executable; usage is via library import or tests.

**Validation:**  
- Always run `go test ./...` and `golangci-lint run` before submitting changes.
- GitHub Actions CI runs tests and lint on push and pull request.  
- Do not submit code that fails either step.

**Common Issues & Workarounds:**  
- If you see dependency errors, always run `go mod tidy`.
- If linting fails, fix issues before submitting.
- If tests fail due to environment, check Go version and dependencies.

## Project Layout & Architecture

**Key Directories & Files:**  
- `automa/`: Core framework code (step orchestration, reporting, etc.)
- `automa_steps/`: Step implementations (e.g., bash script execution)
- `go.mod`, `go.sum`: Dependency management
- `.github/workflows/`: GitHub Actions CI configuration
- `.golangci.yml`: Linter configuration

**Configuration Files:**  
- Lint: `.golangci.yml`
- CI: `.github/workflows/ci.yml` (runs `go test` and `golangci-lint`)
- No custom build scripts; use standard Go commands.

**Validation Pipelines:**  
- On push/PR, GitHub Actions runs:
  - `go test ./...`
  - `golangci-lint run`
- All code must pass these checks.

**Explicit Validation Steps:**  
- Always run `go test ./...` and `golangci-lint run` locally before pushing.
- Ensure `go mod tidy` is run after dependency changes.

**Dependencies:**  
- All dependencies are managed via Go modules.
- No hidden or non-obvious dependencies.

## File Inventory

**Repo Root Files:**  
- `README.md`: Project overview and usage.
- `go.mod`, `go.sum`: Go module files.
- `.golangci.yml`: Linter config.
- `.github/`: GitHub workflows and instructions.

**Key Source Files:**  
- `automa/report.go`: Report struct and logic.
- `automa/step.go`: Step orchestration.
- `automa_steps/step_bash.go`: Bash script step implementation.

**Next Level Down:**  
- `automa/`: All core framework code.
- `automa_steps/`: Step implementations.

**README.md Summary:**  
- Describes the purpose, usage, and examples for the `automa` framework.
- Outlines how to define and execute steps.

## Agent Instructions

- Trust these instructions for build, test, lint, and validation steps.
- Only perform additional search if information here is incomplete or in error.
- Always run `go mod tidy` before build/test/lint.
- Always validate with `go test ./...` and `golangci-lint run`.
- Ensure changes do not break CI or linting.