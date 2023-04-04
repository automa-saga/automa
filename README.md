# Automa
![test](https://github.com/automa-saga/automa/actions/workflows/test.yaml/badge.svg)
[![Commitizen friendly](https://img.shields.io/badge/commitizen-friendly-brightgreen.svg)](http://commitizen.github.io/cz-cli/)
[![semantic-release: go](https://img.shields.io/badge/semantic--release-go?logo=semantic-release)](https://github.com/semantic-release/semantic-release)
[![codecov](https://codecov.io/gh/automa-saga/automa/branch/master/graph/badge.svg?token=DMRN5J6TJW)](https://codecov.io/gh/automa-saga/automa)

Automa is a Saga Workflow Engine. It is designed to be used for sequential and transactional business processes. It 
implements the choreography Saga pattern. The difference with the traditional
[choreography pattern](https://learn.microsoft.com/en-us/azure/architecture/reference-architectures/saga/saga) is that 
this does not use a centralized message broker, rather each step calls the next step on success or undo previous on 
error. 

The name `automa` is derived from the word `automate`.

All steps are executed sequentially in the Automa Workflow. On success, it moves forward sequentially and on error it moves
backward sequentially. Note that some steps cannot be rollback in reality, for example if an email has been sent. In that
case, some form of compensating behaviour should be implemented, for example, it should send another compensating email 
in order to void the notification email that was sent before.

Apart from Saga workflow pattern, Automa also supports generating a report of the execution for every step in the workflow. 
A report data model can be found in file [reports.go](https://github.com/automa-saga/automa/blob/master/reports.go). 
Developers need to populate a Report object in every `Run` and `Rollback` method as shown in the example. 

## Usage

1. Add dependency using `go get -u "github.com/automa-saga/automa"`.
2. Implement workflow steps (implements `automa.AtomicStep` interface) using the pattern as below:
```go

// MyStep1 is an example AtomicStep that does not implement AtomicStep interface directly and uses the default 
// implementation provided by automa.Step 
type MyStep1 struct {
    automa.Step
    params map[string]string // define any parameter data model as required
}

// run implements the SagaRun method interface to leverage default Run control logic that is implemented in automa.Step
// Note if not provided, Run action will be marked as SKIPPED
func (s *MyStep1) run(ctx context.Context) (skipped bool, err error) {
    // perform run action
    // use params or cache as needed

	// if error happens, just return the error as below to trigger rollback
	return false, err
	
    // if this action needs to be skipped because of a condition, invoke next step using
	return true, nil
}

// rollback implements the SagaRollback method interface to leverage default Rollback control logic that is implemented in automa.Step
// Note this is optional and if not provided, Rollback action will be marked as SKIPPED
func (s *MyStep1) rollback(ctx context.Context) (skipped bool, err error) {
    // perform rollback action
    // use params or cache as needed

    // if error happens, just return the error 
    return false, err

    // if this action needs to be skipped because of a condition, return: 
    return true, nil
}

// MyStep2 is an example AtomicStep that implements the Run and Rollback method interface directly
type MyStep2 struct {
	automa.Step
	params map[string]string // define any parameter data model as required
}

// Run implements the automa.AtomicStep interface
func (s *MyStep2) Run(ctx context.Context, prevSuccess *Success) (*WorkflowReport, error) {
	report := NewStepReport(s.ID, RunAction)
	
	// perform run action with custom logic
	// use params or cache as needed.
	// extra control logic that is not already provided in the default automa.Step.Run
	
	// if error happens, invoke rollback using below
	// return s.Rollback(ctx, NewFailedRun(ctx, prevSuccess, err, report))
	
	// if this action needs to be skipped because of a condition, invoke next Run using..
	// return s.SkippedRun(ctx, prevSuccess, report) 
	
	return s.RunNext(ctx, prevSuccess, report)
}

// Rollback implements the automa.AtomicStep interface
func (s *MyStep2) Rollback(ctx context.Context, prevFailure *Failure) (*WorkflowReport, error) {
	report := NewStepReport(s.ID, RollbackAction)
	
	// perform rollback action.
	// use params or local cache as needed.
	// extra control logic that is not already provided in the default automa.Step.Rollback
	
	// if error happens, invoke previous rollback using below
	// return s.FailedRollback(ctx, prevFailure, err, report)
	
	// if this action needs to be skipped because of a condition, invoke ..
	// return s.SkippedRollback(ctx, prevFailure, report) 
	
	return s.RollbackPrev(ctx, prevFailure, report)
}
```

3. Then create and run a workflow using `automa.StepRegistry` as below:
```go

func buildWorkflow1(ctx context.Context, params map[string]string) automa.Workflow {
    workflowID := "workflow-1"
	
    step1 := &MyStep1 {
        Step:  Step{ID: "step_1"}, // underscore separated string is suitable for IDs

        // add parameters as needed
        params: params
    }
	step1.RegisterSaga(step1.run, step1.rollback) // we need to register so that default Run and Rollback logic can be used

    step2 := &MyStep2 {
        Step:  Step{ID: "step_2"},

        // add parameters as needed
        params: params
    }
	// Note: No need to invoke step2.RegisterSaga() since MyStep2 implements the AtomicStep interface

    // pass custom zap logger if necessary
    registry := automa.NewStepRegistry(zap.NewNop()).RegisterSteps(map[string]AtomicStep{
        step1.GetID(): step1,
        step2.GetID(): step2,
    })

    // prepare the sequence of steps
    // Note that you may create different workflows from the same registry if needed.
    // However, if the same registry is being reused, ensure each step clears its local cache (if it has any)
    // before executing its action as necessary.
    workflow1Steps := StepIDs{
        step1.GetID(),
        step2.GetID(),
    }
	
    workflow := registry.BuildWorkflow(workflowID, workflow1Steps)
    return workflow
}

func runWorkflow(ctx context.Context) error {
    params := map[string]string{} // add params as necessary
	
    workflow := buildWorkflow1(ctx, params)
    defer workflow1.End(ctx)

    report, err := workflow.Start(context.Background())
    if err != nil {
        return err
    }

    // do something with the report if necessary 
    // 'report' can be exported as YAML or JSON. See examples directory.

    return nil
	
}
```

See an [example](https://github.com/automa-saga/automa/blob/master/example/example.go) in the example directory. 

## Development
 - `make test` runs the tests. 
 - In order to build example, do `cd examples && go build`. Then the example can be then run using `./examples/example`.

## Contribution
Any feedback, comment and contributions are very much welcome. 

Developers are encouraged to adopt the usual open source development practices with a PR and sign-off as well as 
verified signed commits. Developers are also encouraged to use [commitizen](https://commitizen-tools.github.io/commitizen/) 
for commits messages.

Please note the PR will be squashed merge to master with commitizen format for the PR title. So even if commitizen is not
used for individual commits in the PR, the repository maintainer are requested to ensure that the PR title follows 
commitizen format before squash-merging the PR.

For beginners use [this](https://github.com/firstcontributions/first-contributions) guide as a start.
