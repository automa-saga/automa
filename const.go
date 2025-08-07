package automa

import "github.com/rs/zerolog"

var nolog = zerolog.Nop()

const (
	KeyPrevResult = "PREV_RESULT"
	KeyLogger     = "LOGGER"

	StatusSuccess     = "SUCCESS"
	StatusFailed      = "FAILED"
	StatusSkipped     = "SKIPPED"
	StatusInitialized = "INITIALIZED"

	RunAction      = "RUN"
	RollbackAction = "ROLLBACK"
)
