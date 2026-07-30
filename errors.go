package automa

import "github.com/joomcode/errorx"

var (
	ErrNamespace   = errorx.NewNamespace("automa")
	StepIdProperty = errorx.RegisterProperty("step_id")

	IllegalArgument   = ErrNamespace.NewType("illegal_argument")
	StepNotFound      = IllegalArgument.NewSubtype("step_not_found", errorx.NotFound())
	StepAlreadyExists = IllegalArgument.NewSubtype("step_already_exists", errorx.Duplicate())

	StepExecutionError = ErrNamespace.NewType("step_execution_error")

	// JournalError and its subtypes cover durability journal failures
	// (durability-spec §3, §6). Each subtype maps to a distinct fail-loudly case:
	// a journal that cannot be decoded, a schema version this build does not
	// support, or a topology/mode disagreement with the supplied definition.
	JournalError              = ErrNamespace.NewType("journal_error")
	JournalCorrupt            = JournalError.NewSubtype("corrupt")
	JournalUnsupportedVersion = JournalError.NewSubtype("unsupported_version")
	JournalTopologyMismatch   = JournalError.NewSubtype("topology_mismatch")
)
