package automa

// Status defines the execution status of the Step
type Status string

const (
	StatusSuccess   Status = "SUCCESS"
	StatusFailed    Status = "FAILED"
	StatusSkipped   Status = "SKIPPED"
	StatusUndefined Status = "UNDEFINED"
)
