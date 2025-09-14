package automa

import "github.com/joomcode/errorx"

var (
	ErrNamespace = errorx.NewNamespace("automa")

	IllegalArgument      = ErrNamespace.NewType("illegal_argument")
	StepNotFound         = IllegalArgument.NewSubtype("step_not_found", errorx.NotFound())
	StepAlreadyExists    = IllegalArgument.NewSubtype("step_already_exists", errorx.Duplicate())
	RegistryNotProvided  = IllegalArgument.NewSubtype("registry_not_provided", errorx.NotFound())
	StepIdsNotProvided   = IllegalArgument.NewSubtype("step_ids_not_provided", errorx.NotFound())
	StepValidationFailed = IllegalArgument.NewSubtype("step_validation_failed")
	ErrInvalidReportType = IllegalArgument.NewSubtype("invalid_report_type", errorx.NotFound())

	StepExecutionError          = ErrNamespace.NewType("step_execution_error")
	StepExecutionFailed         = StepExecutionError.NewSubtype("step_execution_failed")
	StepExecutionSkipped        = StepExecutionError.NewSubtype("step_execution_skipped")
	StepExecutionAborted        = StepExecutionError.NewSubtype("step_execution_aborted")
	StepExecutionTimeout        = StepExecutionError.NewSubtype("step_execution_timeout", errorx.Timeout())
	StepExecutionNotStarted     = StepExecutionError.NewSubtype("step_execution_not_started")
	StepExecutionAlreadyStarted = StepExecutionError.NewSubtype("step_execution_already_started")
	StepRollbackFailed          = StepExecutionError.NewSubtype("step_rollback_failed")
	StepRollbackSkipped         = StepExecutionError.NewSubtype("step_rollback_skipped")
	StepRollbackAborted         = StepExecutionError.NewSubtype("step_rollback_aborted")
	StepRollbackTimeout         = StepExecutionError.NewSubtype("step_rollback_timeout", errorx.Timeout())
	StepRollbackNotStarted      = StepExecutionError.NewSubtype("step_rollback_not_started")
	StepRollbackAlreadyStarted  = StepExecutionError.NewSubtype("step_rollback_already_started")
)
