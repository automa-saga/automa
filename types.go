package automa

type TypeAction uint8

const (
	ActionExecute  TypeAction = 1 // "execute"
	ActionRollback TypeAction = 2 // "rollback"
)

type TypeRollbackMode uint8

const (
	RollbackModeContinueOnError TypeRollbackMode = 1 // "continue" // continue rolling back previous steps even if one fails
	RollbackModeStopOnError     TypeRollbackMode = 2 // "stop"     // stop rolling back previous steps on first failure
)

type TypeReport uint8

const (
	StepReport     TypeReport = 1
	WorkflowReport TypeReport = 2
)

type TypeStatus uint8

const (
	StatusSuccess TypeStatus = 1 // "success"
	StatusFailed  TypeStatus = 2 // "failed"
	StatusSkipped TypeStatus = 3 //"skipped"
)
