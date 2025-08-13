package automa

import "context"

// basicStep implements Step interfaces.
// This is the default Step implementation that is meant to be stateless. For stateful steps, you can implement your
// custom-step Builder.
// It can be used to create steps with custom prepare, execute, onSuccess, and onRollback functions.
type basicStep struct {
	id         string
	prepare    PrepareFunc
	execute    ExecuteFunc
	onSuccess  OnSuccessFunc
	onRollback RollbackFunc
}

func (s *basicStep) Id() string {
	return s.id
}

func (s *basicStep) Prepare(ctx context.Context) (context.Context, error) {
	if s.prepare != nil {
		return s.prepare(ctx)
	}
	return ctx, nil
}

func (s *basicStep) Execute(ctx context.Context) (Report, error) {
	if s.execute != nil {
		return s.execute(ctx)
	}
	return StepSkippedReport(s.id, ActionExecute, WithMessage("no execute function defined")), nil
}

func (s *basicStep) OnCompletion(ctx context.Context, report Report) {
	if s.onSuccess != nil {
		s.onSuccess(ctx, report)
	}
}

func (s *basicStep) OnRollback(ctx context.Context) (Report, error) {
	if s.onRollback != nil {
		return s.onRollback(ctx)
	}
	return StepSkippedReport(s.id, ActionRollback, WithMessage("no rollback function defined")), nil
}
