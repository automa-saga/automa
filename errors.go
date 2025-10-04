package automa

import "github.com/joomcode/errorx"

var (
	ErrNamespace   = errorx.NewNamespace("automa")
	StepIdProperty = errorx.RegisterProperty("step_id")

	IllegalArgument   = ErrNamespace.NewType("illegal_argument")
	StepNotFound      = IllegalArgument.NewSubtype("step_not_found", errorx.NotFound())
	StepAlreadyExists = IllegalArgument.NewSubtype("step_already_exists", errorx.Duplicate())

	StepExecutionError = ErrNamespace.NewType("step_execution_error")
)
