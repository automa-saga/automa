package main

import (
	"context"
	"fmt"
	"github.com/cockroachdb/errors"
	"github.com/leninmehedy/automa"
	"go.uber.org/zap"
)

type Step1 struct {
	automa.Step
	cache  map[string][]byte
	logger *zap.Logger
}

type Step2 struct {
	automa.Step
	cache  map[string][]byte
	logger *zap.Logger
}

// Step3 is an example where Run or Rollback can be skipped based on conditions if required
type Step3 struct {
	automa.Step
	cache  map[string][]byte
	logger *zap.Logger
}

type Step4 struct {
	automa.Step
	cache  map[string][]byte
	logger *zap.Logger
}

func (s *Step1) Run(ctx context.Context, prevSuccess *automa.Success) (automa.Reports, error) {
	report := automa.StartReport(s.ID)
	s.logger.Sugar().Debugf("RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID)) // store something in the cache for rollback
	return s.RunNext(ctx, prevSuccess, report)
}

func (s *Step1) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.Reports, error) {
	report := automa.StartReport(s.ID)
	s.logger.Sugar().Debug(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *Step2) Run(ctx context.Context, prevSuccess *automa.Success) (automa.Reports, error) {
	s.logger.Sugar().Debugf("RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))

	return s.RunNext(ctx, prevSuccess, nil)
}

func (s *Step2) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.Reports, error) {
	report := automa.StartReport(s.ID)
	s.logger.Sugar().Debug(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func (s *Step3) Run(ctx context.Context, prevSuccess *automa.Success) (automa.Reports, error) {
	s.logger.Sugar().Debugf("RUN SKIPPED - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK SKIPPED - %q", s.ID))
	return s.SkippedRun(ctx, prevSuccess, nil)
}

func (s *Step3) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.Reports, error) {
	report := automa.StartReport(s.ID)
	s.logger.Sugar().Debug(string(s.cache["rollbackMsg"]))
	return s.SkippedRollback(ctx, prevFailure, report)
}

func (s *Step4) Run(ctx context.Context, prevSuccess *automa.Success) (automa.Reports, error) {
	report := automa.StartReport(s.ID)

	s.logger.Sugar().Debugf("RUN - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK - %q", s.ID))

	// trigger rollback on error
	err := errors.New("error running step 3")
	report.Error = errors.EncodeError(ctx, err)
	if err != nil {
		return s.Rollback(ctx, automa.NewRollbackTrigger(prevSuccess, err, report))
	}

	return s.RunNext(ctx, prevSuccess, report)
}

func (s *Step4) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.Reports, error) {
	report := automa.StartReport(s.ID)
	s.logger.Sugar().Debug(string(s.cache["rollbackMsg"]))
	return s.RollbackPrev(ctx, prevFailure, report)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	step1 := &Step1{
		Step:   automa.Step{ID: "Step1"},
		cache:  map[string][]byte{},
		logger: logger,
	}

	step2 := &Step2{
		Step:   automa.Step{ID: "Step2"},
		cache:  map[string][]byte{},
		logger: logger,
	}

	step3 := &Step3{
		Step:   automa.Step{ID: "Invalid Step"},
		cache:  map[string][]byte{},
		logger: logger,
	}

	step4 := &Step4{
		Step:   automa.Step{ID: "Step4"},
		cache:  map[string][]byte{},
		logger: logger,
	}

	workflow := automa.NewWorkflow(automa.WithSteps(step1, step2, step3, step4), automa.WithLogger(logger))
	defer workflow.End(ctx)

	reports, _ := workflow.Start(ctx) // ignore the error since it is an example with forced error

	logger.Debug("* ----------------------------- *")
	logger.Debug("*        Execution Report       *")
	logger.Debug("* ----------------------------- *")
	for _, report := range reports { // note the items in the map is not ordered
		logger.Sugar().Debugf("*\t%s: %s\t\t*", report.StepID, report.Status)
	}
	logger.Debug("* ----------------------------- *")
}
