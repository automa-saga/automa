package automa

// Status defines the execution status of the AtomicStep
type Status string

const (
	StatusSuccess Status = "SUCCESS"
	StatusFailed  Status = "FAILED"
	StatusSkipped Status = "SKIPPED"
)
