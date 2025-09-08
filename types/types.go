package types

type Action uint8

const (
	ActionExecute  Action = 1 // "execute"
	ActionRollback Action = 2 // "rollback"
)

type RollbackMode uint8

const (
	RollbackModeContinueOnError RollbackMode = 1 // "continue" // continue rolling back previous steps even if one fails
	RollbackModeStopOnError     RollbackMode = 2 // "stop"     // stop rolling back previous steps on first failure
)

type Report uint8

const (
	StepReport     Report = 1
	WorkflowReport Report = 2
)

type Status uint8

const (
	StatusSuccess Status = 1 // "success"
	StatusFailed  Status = 2 // "failed"
	StatusSkipped Status = 3 //"skipped"
)
