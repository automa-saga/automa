package automa

import "github.com/joomcode/errorx"

var (
	ErrNamespace = errorx.NewNamespace("automa")

	StepNotFound          = ErrNamespace.NewType("step_not_found", errorx.NotFound())
	StepAlreadyExists     = ErrNamespace.NewType("step_already_exists", errorx.Duplicate())
	WorkflowNotFound      = ErrNamespace.NewType("workflow_not_found", errorx.NotFound())
	WorkflowAlreadyExists = ErrNamespace.NewType("workflow_already_exists", errorx.Duplicate())

	WorkflowExecutionError          = ErrNamespace.NewType("workflow_execution_error")
	WorkflowExecutionTimeout        = WorkflowExecutionError.NewSubtype("workflow_execution_timeout", errorx.Timeout())
	WorkflowExecutionAborted        = WorkflowExecutionError.NewSubtype("workflow_execution_aborted")
	WorkflowExecutionFailed         = WorkflowExecutionError.NewSubtype("workflow_execution_failed")
	WorkflowExecutionSkipped        = WorkflowExecutionError.NewSubtype("workflow_execution_skipped")
	WorkflowExecutionNotStarted     = WorkflowExecutionError.NewSubtype("workflow_execution_not_started")
	WorkflowExecutionAlreadyStarted = WorkflowExecutionError.NewSubtype("workflow_execution_already_started")

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
