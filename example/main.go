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

type SkippedStep struct {
	automa.Step
	cache  map[string][]byte
	logger *zap.Logger
}

type Step3 struct {
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

func (s *SkippedStep) Run(ctx context.Context, prevSuccess *automa.Success) (automa.Reports, error) {
	s.logger.Sugar().Debugf("RUN SKIPPED - %q", s.ID)
	s.cache["rollbackMsg"] = []byte(fmt.Sprintf("ROLLBACK SKIPPED - %q", s.ID))
	return s.SkippedRun(ctx, prevSuccess, nil)
}

func (s *SkippedStep) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.Reports, error) {
	report := automa.StartReport(s.ID)
	s.logger.Sugar().Debug(string(s.cache["rollbackMsg"]))
	return s.SkippedRollback(ctx, prevFailure, report)
}

func (s *Step3) Run(ctx context.Context, prevSuccess *automa.Success) (automa.Reports, error) {
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

func (s *Step3) Rollback(ctx context.Context, prevFailure *automa.Failure) (automa.Reports, error) {
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

	first := &Step1{
		Step:   automa.Step{ID: "Step1"},
		cache:  map[string][]byte{},
		logger: logger,
	}

	step2 := &Step2{
		Step:   automa.Step{ID: "Step2"},
		cache:  map[string][]byte{},
		logger: logger,
	}

	skippedState := &SkippedStep{
		Step:   automa.Step{ID: "Step3"},
		cache:  map[string][]byte{},
		logger: logger,
	}

	last := &Step3{
		Step:   automa.Step{ID: "Step4"},
		cache:  map[string][]byte{},
		logger: logger,
	}

	workflow := automa.NewWorkflow(automa.WithSteps(first, step2, skippedState, last), automa.WithLogger(logger))
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
