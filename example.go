package automa

import (
	"context"
	"time"
)

type Report interface {
	Id() string
	StartTime() time.Time
	EndTime() time.Time
	Status() string
}

type Step interface {
	Id() string
	Prepare(ctx context.Context) (context.Context, error)
	Execute(ctx context.Context) (Report, error)
	OnSuccess(ctx context.Context)
	OnRollback(ctx context.Context) (Report, error)
}

func StepSuccessReport(opts ...Option) (Report, error) {

}
// StepSuccessReport(WithMetadata(map[string]string{})), error
// StepFailureReport(err error, WithMetadata(map[string]string{})) error
// StepSkippedReport(WithMetadata(map[string]string{})) error


w1, err := NewWorkFlowBuilder("workflow1").
AddStep(NewStep("step1", bl.doSomething, WithPrepare(bl.prepare), WithRollback(bl.doRollback))).
AddStep(NewStep("step2", bl.doSomething2, WithRollback(bl.doRollback2))).
Build()

sr := NewStepRegistry()
err := sr.AddSteps(NewStep("step1", bl.doSomething, WithRollback(bl.doRollback)))


w1 := NewWorkFlowBuilder("workflow1").
WithStepRegistry(sr).
WithWorkflowRegistry(wr).
WithSteps(stepIDs ...string).
WithWorkflows(workflowIDs ...string).
Build()

w2, err := NewWorkFlowBuilder().
WithId("workflow2").
WithLogger(logger).
RollbackMode("FAIL_ON_ERR"). // or "CONTINUE_ON_ERR" WARN_OR_ERR, IGNORE_ERR
AddSteps(s1, s2, sr.Of("wf1", "wf2"), s3, sr.Of("wf3"), sr.Of("s4", "s5")).
Build()
