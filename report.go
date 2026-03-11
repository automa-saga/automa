package automa

import (
	"encoding/json"
	"time"
)

// Report describes the outcome of a single step or an entire workflow
// execution. It is the primary observability primitive in automa: every
// Execute, Rollback, and workflow run produces a Report that the caller can
// inspect, log, or persist.
//
// # Structure
//
// A step report contains a flat set of fields describing that step's outcome.
// A workflow report nests the per-step reports inside StepReports, and if
// rollback was triggered, the rollback outcomes are nested inside the
// Rollback field of the report for the step that failed.
//
//	workflow report
//	├── StepReports[0]  — step "provision"  (Execute)
//	├── StepReports[1]  — step "configure"  (Execute, failed)
//	│       └── Rollback
//	│               ├── StepReports[0]  — step "configure"  (Rollback)
//	│               └── StepReports[1]  — step "provision"  (Rollback)
//	└── …
//
// # Serialization
//
// Report implements json.Marshaler and yaml.Marshaler. The error field is
// serialized as a string (its Error() message) because the error interface is
// not directly marshallable. The field names follow camelCase JSON conventions.
//
// # Immutability
//
// Reports produced by SuccessReport, FailureReport, and SkippedReport are
// considered immutable after creation. Use Clone to obtain a deep copy when
// the report needs to be stored or passed to an asynchronous callback.
type Report struct {
	// Id is the step or workflow ID that produced this report.
	Id string

	// IsWorkflow is true when this report was produced by a Workflow rather
	// than a plain Step.
	IsWorkflow bool

	// Action identifies which lifecycle phase produced this report:
	// ActionPrepare, ActionExecute, or ActionRollback.
	Action TypeAction

	// Status is the outcome of the phase: StatusSuccess, StatusFailed, or
	// StatusSkipped.
	Status TypeStatus

	// StartTime is the wall-clock time at which the phase began.
	StartTime time.Time

	// EndTime is the wall-clock time at which the phase completed.
	EndTime time.Time

	// Detail is an optional human-readable message providing additional context
	// about the outcome (e.g. why a step was skipped).
	Detail string

	// Error holds the error returned by the step if the phase failed. It is
	// nil for successful and skipped reports. During JSON/YAML marshaling this
	// field is serialized as the string produced by Error().
	Error error

	// Metadata is an optional map of string key-value pairs that a step can
	// populate with structured diagnostic information (e.g. resource IDs,
	// configuration values, counts). It is serialized as a JSON/YAML object.
	Metadata StringMap

	// StepReports contains the per-step reports for a workflow report, in
	// execution order. It is nil for plain step reports.
	StepReports []*Report

	// Rollback holds the rollback sub-report tree when rollback was triggered.
	// For a workflow report this contains the rollback outcomes for all steps
	// that were rolled back. It is nil when no rollback occurred.
	Rollback *Report

	// ExecutionMode records the workflow's execution mode at the time the
	// report was produced. This provides context when replaying or auditing
	// a workflow run.
	ExecutionMode TypeMode

	// RollbackMode records the workflow's rollback mode at the time the
	// report was produced.
	RollbackMode TypeMode
}

// marshalReport is the wire-format struct used by MarshalJSON and MarshalYAML.
// It mirrors Report but represents Error as a plain string (error is not
// directly marshallable) and uses struct tags to control field names and
// omitempty behaviour.
type marshalReport struct {
	Id            string     `yaml:"id,omitempty" json:"id,omitempty"` // rollback report does not need id
	IsWorkflow    bool       `yaml:"isWorkflow" json:"isWorkflow"`
	Action        TypeAction `yaml:"action,omitempty" json:"action,omitempty"`
	Status        TypeStatus `yaml:"status,omitempty" json:"status,omitempty"`
	StartTime     time.Time  `yaml:"startTime,omitempty" json:"startTime,omitempty"`
	EndTime       time.Time  `yaml:"endTime,omitempty" json:"endTime,omitempty"`
	Detail        string     `yaml:"detail,omitempty" json:"detail,omitempty"`
	Error         string     `yaml:"error,omitempty" json:"error,omitempty"`
	Metadata      StringMap  `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	StepReports   []*Report  `yaml:"stepReports,omitempty" json:"steps,omitempty"`
	Rollback      *Report    `yaml:"rollback,omitempty" json:"rollback,omitempty"`
	ExecutionMode TypeMode   `yaml:"executionMode,omitempty" json:"executionMode,omitempty"`
	RollbackMode  TypeMode   `yaml:"rollbackMode,omitempty" json:"rollbackMode,omitempty"`
}

// HasError reports whether the report carries a non-nil error. A report can
// have a failed status without an error (e.g. when the step set the status
// explicitly), so callers that need to distinguish these cases should check
// both IsFailed and HasError.
func (r *Report) HasError() bool {
	return r.Error != nil
}

// IsSuccess reports whether the phase completed successfully with no error.
// Both conditions must hold: Status == StatusSuccess AND Error == nil.
// A report with Status == StatusSuccess but a non-nil Error is not considered
// successful.
func (r *Report) IsSuccess() bool {
	return r.Status == StatusSuccess && !r.HasError()
}

// IsFailed reports whether the phase failed. The report is considered failed
// when either Status == StatusFailed OR a non-nil Error is present, whichever
// comes first. This means a step that sets a success status but also attaches
// an error is still treated as failed.
func (r *Report) IsFailed() bool {
	return r.Status == StatusFailed || r.HasError()
}

// IsSkipped reports whether the phase was skipped. A skipped report indicates
// that no work was performed (e.g. the step had no execute function, or the
// step's logic determined that the operation was already complete).
func (r *Report) IsSkipped() bool {
	return r.Status == StatusSkipped
}

// Duration returns the elapsed time between StartTime and EndTime. It returns
// zero for reports where both timestamps are equal (e.g. when only one of the
// two was set).
func (r *Report) Duration() time.Duration {
	return r.EndTime.Sub(r.StartTime)
}

// Clone returns a deep copy of the report. The copy is safe to modify without
// affecting the original. Specifically:
//   - All scalar fields (Id, Status, Action, …) are copied by value.
//   - Error is copied by reference (errors are immutable by convention).
//   - Metadata is deep-copied into a new StringMap.
//   - StepReports is deep-copied by recursively cloning each child.
//   - Rollback is deep-copied by recursively cloning the sub-report.
//
// Clone on a nil receiver returns nil.
func (r *Report) Clone() *Report {
	if r == nil {
		return nil
	}
	clone := &Report{
		Id:            r.Id,
		IsWorkflow:    r.IsWorkflow,
		Action:        r.Action,
		Status:        r.Status,
		StartTime:     r.StartTime,
		EndTime:       r.EndTime,
		Detail:        r.Detail,
		Error:         r.Error,
		RollbackMode:  r.RollbackMode,
		ExecutionMode: r.ExecutionMode,
	}

	if r.Metadata != nil {
		clone.Metadata = make(StringMap, len(r.Metadata))
		for k, v := range r.Metadata {
			clone.Metadata[k] = v
		}
	}

	if r.StepReports != nil {
		clone.StepReports = make([]*Report, 0, len(r.StepReports))
		for _, sr := range r.StepReports {
			clone.StepReports = append(clone.StepReports, sr.Clone())
		}
	}

	if r.Rollback != nil {
		clone.Rollback = r.Rollback.Clone()
	}

	return clone
}

// ReportOption is a functional option applied to a Report by the NewReport,
// SuccessReport, FailureReport, and SkippedReport constructors. Options are
// applied in order after the report's default fields are set, so a later
// option can override an earlier one.
type ReportOption func(*Report)

// WithStepReports appends one or more child reports to the report's
// StepReports slice. If the slice is nil it is allocated first. This option is
// used by the workflow to attach per-step execution reports to the workflow's
// final report.
func WithStepReports(reports ...*Report) ReportOption {
	return func(sr *Report) {
		if sr.StepReports == nil {
			sr.StepReports = make([]*Report, 0, len(reports))
		}
		sr.StepReports = append(sr.StepReports, reports...)
	}
}

// WithRollbackReport sets the Rollback field of the report to the given
// sub-report. This is used by the workflow to attach the rollback outcome
// tree to the report of the step (or workflow) that triggered rollback.
func WithRollbackReport(rollback *Report) ReportOption {
	return func(sr *Report) {
		sr.Rollback = rollback
	}
}

// WithMetadata sets the report's Metadata field to the given StringMap,
// replacing any previously set metadata. Use this option to attach structured
// diagnostic information (resource IDs, counts, configuration values) to a
// report.
func WithMetadata(metadata StringMap) ReportOption {
	return func(sr *Report) {
		sr.Metadata = metadata
	}
}

// WithError sets the report's Error field. The report's Status is not changed
// by this option; set the status explicitly with WithStatus or use the
// FailureReport constructor which sets both StatusFailed and an error in one
// call.
func WithError(err error) ReportOption {
	return func(sr *Report) {
		sr.Error = err
	}
}

// WithStatus sets the report's Status field to the given TypeStatus. This
// option is applied after the constructor's default status, so it can override
// the default (e.g. to mark a report as Skipped inside a custom execute
// function).
func WithStatus(status TypeStatus) ReportOption {
	return func(sr *Report) {
		sr.Status = status
	}
}

// WithStartTime sets the report's StartTime field. If EndTime has not been set
// yet (is zero), it is also set to startTime so that Duration() never returns
// a negative value for reports that only record a start.
func WithStartTime(startTime time.Time) ReportOption {
	return func(sr *Report) {
		sr.StartTime = startTime
		if sr.EndTime.IsZero() {
			sr.EndTime = startTime // ensure end time is set if not already
		}
	}
}

// WithEndTime sets the report's EndTime field. If StartTime has not been set
// yet (is zero), it is also set to endTime so that Duration() never returns a
// negative value for reports that only record an end.
func WithEndTime(endTime time.Time) ReportOption {
	return func(sr *Report) {
		sr.EndTime = endTime
		if sr.StartTime.IsZero() {
			sr.StartTime = endTime // ensure start time is set if not already
		}
	}
}

// WithDetail sets the report's Detail field to a human-readable message that
// provides additional context about the outcome. Useful for skipped reports
// where a brief explanation of why the step was skipped aids observability.
func WithDetail(detail string) ReportOption {
	return func(sr *Report) {
		sr.Detail = detail
	}
}

// WithReport merges selected fields from an existing report into the target
// report. Only non-zero/non-nil fields in the source are applied; existing
// non-zero fields in the target are overwritten. This is used by defaultStep
// to promote user-returned report details (Detail, Metadata, Error, etc.)
// into the final canonically-constructed report while ensuring the ActionType
// and timing fields are always set by the framework.
//
// Fields merged (when non-zero/non-nil in source):
//   - Detail, Action, StartTime, EndTime, Status, Error
//   - Metadata (merged key-by-key into the target map)
//   - StepReports (appended)
//   - Rollback (replaced)
//   - ExecutionMode and RollbackMode (always copied)
func WithReport(report *Report) ReportOption {
	return func(sr *Report) {
		if report == nil {
			return
		}
		if report.Detail != "" {
			sr.Detail = report.Detail
		}
		if report.Action != 0 {
			sr.Action = report.Action
		}
		if !report.StartTime.IsZero() {
			sr.StartTime = report.StartTime
		}
		if !report.EndTime.IsZero() {
			sr.EndTime = report.EndTime
		}
		if report.Status != 0 {
			sr.Status = report.Status
		}
		if report.Error != nil {
			sr.Error = report.Error
		}
		if report.Metadata != nil {
			if sr.Metadata == nil {
				sr.Metadata = make(StringMap)
			}
			for k, v := range report.Metadata {
				sr.Metadata[k] = v
			}
		}
		if report.StepReports != nil {
			if sr.StepReports == nil {
				sr.StepReports = make([]*Report, 0, len(report.StepReports))
			}
			sr.StepReports = append(sr.StepReports, report.StepReports...)
		}
		if report.Rollback != nil {
			sr.Rollback = report.Rollback
		}
		sr.ExecutionMode = report.ExecutionMode
		sr.RollbackMode = report.RollbackMode
	}
}

// WithActionType sets the report's Action field to the given TypeAction. This
// is used by the framework to record which lifecycle phase (Prepare, Execute,
// Rollback) produced the report.
func WithActionType(actionType TypeAction) ReportOption {
	return func(sr *Report) {
		sr.Action = actionType
	}
}

// WithExecutionMode sets the report's ExecutionMode field. This records the
// workflow's execution policy at the time the report was produced and is used
// for audit and replay purposes.
func WithExecutionMode(mode TypeMode) ReportOption {
	return func(sr *Report) {
		sr.ExecutionMode = mode
	}
}

// WithRollbackMode sets the report's RollbackMode field. This records the
// workflow's rollback policy at the time the report was produced.
func WithRollbackMode(mode TypeMode) ReportOption {
	return func(sr *Report) {
		sr.RollbackMode = mode
	}
}

// WithWorkflow copies the IsWorkflow flag, ExecutionMode, and RollbackMode
// from a workflow into the report. This is called by the workflow's Execute
// and Rollback methods to stamp the workflow's configuration onto its final
// report. It is a no-op when w is nil.
func WithWorkflow(w *workflow) ReportOption {
	return func(sr *Report) {
		if w != nil {
			sr.IsWorkflow = IsWorkflow(w)
			sr.ExecutionMode = w.executionMode
			sr.RollbackMode = w.rollbackMode
		}
	}
}

// WithIsWorkflow sets the report's IsWorkflow field. When true, readers of the
// report know it was produced by a Workflow rather than a plain Step, which
// affects how StepReports is interpreted.
func WithIsWorkflow(isWorkflow bool) ReportOption {
	return func(sr *Report) {
		sr.IsWorkflow = isWorkflow
	}
}

// NewReport creates a new Report with the given id and default field values:
//   - StartTime and EndTime are both set to time.Now().
//   - Status defaults to StatusSuccess.
//   - ExecutionMode defaults to StopOnError.
//   - RollbackMode defaults to ContinueOnError.
//
// All provided opts are applied in order after the defaults, so options can
// override any default field. Prefer the typed constructors (SuccessReport,
// FailureReport, SkippedReport) over calling NewReport directly.
func NewReport(id string, opts ...ReportOption) *Report {
	r := &Report{
		Id:            id,
		StartTime:     time.Now(),
		EndTime:       time.Now(),
		Status:        StatusSuccess,
		ExecutionMode: StopOnError,
		RollbackMode:  ContinueOnError,
	}

	for _, opt := range opts {
		opt(r)
	}
	return r
}

// StepSuccessReport creates a success report identified by a plain string id
// rather than a Step. It sets IsWorkflow=false, Status=StatusSuccess, and
// EndTime=time.Now(). Additional opts are applied after these defaults.
//
// Use this constructor in tests or in situations where a Step is not available
// but a success report needs to be produced for a named step.
func StepSuccessReport(id string, opts ...ReportOption) *Report {
	opts = append(opts, WithIsWorkflow(false), WithStatus(StatusSuccess), WithEndTime(time.Now()))
	return NewReport(id, opts...)
}

// StepFailureReport creates a failure report identified by a plain string id.
// It sets IsWorkflow=false, Status=StatusFailed, and EndTime=time.Now().
// Additional opts are applied after these defaults.
func StepFailureReport(id string, opts ...ReportOption) *Report {
	opts = append(opts, WithIsWorkflow(false), WithStatus(StatusFailed), WithEndTime(time.Now()))
	return NewReport(id, opts...)
}

// StepSkippedReport creates a skipped report identified by a plain string id.
// It sets IsWorkflow=false, Status=StatusSkipped, and EndTime=time.Now().
// Additional opts are applied after these defaults.
func StepSkippedReport(id string, opts ...ReportOption) *Report {
	opts = append(opts, WithIsWorkflow(false), WithStatus(StatusSkipped), WithEndTime(time.Now()))
	return NewReport(id, opts...)
}

// SuccessReport creates a success report for the given Step. It sets
// IsWorkflow to reflect whether s is a Workflow, Status=StatusSuccess, and
// EndTime=time.Now(). Additional opts are applied after these defaults.
//
// This is the primary constructor for step success outcomes inside an
// ExecuteFunc or RollbackFunc:
//
//	return automa.SuccessReport(stp)
//	return automa.SuccessReport(stp, automa.WithMetadata(meta))
func SuccessReport(s Step, opts ...ReportOption) *Report {
	opts = append(opts, WithIsWorkflow(IsWorkflow(s)), WithStatus(StatusSuccess), WithEndTime(time.Now()))
	return NewReport(s.Id(), opts...)
}

// FailureReport creates a failure report for the given Step. It sets
// IsWorkflow to reflect whether s is a Workflow, Status=StatusFailed, and
// EndTime=time.Now(). Additional opts are applied after these defaults.
//
// Attach an error with WithError to record the root cause:
//
//	return automa.FailureReport(stp, automa.WithError(err))
func FailureReport(s Step, opts ...ReportOption) *Report {
	opts = append(opts, WithIsWorkflow(IsWorkflow(s)), WithStatus(StatusFailed), WithEndTime(time.Now()))
	return NewReport(s.Id(), opts...)
}

// SkippedReport creates a skipped report for the given Step. It sets
// IsWorkflow to reflect whether s is a Workflow, Status=StatusSkipped, and
// EndTime=time.Now(). Additional opts are applied after these defaults.
//
// Use this when a step determines at runtime that its work is unnecessary:
//
//	return automa.SkippedReport(stp, automa.WithDetail("already provisioned"))
func SkippedReport(s Step, opts ...ReportOption) *Report {
	opts = append(opts, WithIsWorkflow(IsWorkflow(s)), WithStatus(StatusSkipped), WithEndTime(time.Now()))
	return NewReport(s.Id(), opts...)
}

// MarshalJSON implements json.Marshaler. It serializes the Report into a JSON
// object using the marshalReport wire format. The Error field is serialized as
// its Error() string (or omitted when nil). The step ID is included when
// non-empty; rollback sub-reports typically omit the ID via the omitempty tag.
func (r *Report) MarshalJSON() ([]byte, error) {
	m := marshalReport{
		Id:            r.Id,
		Action:        r.Action,
		IsWorkflow:    r.IsWorkflow,
		Status:        r.Status,
		StartTime:     r.StartTime,
		EndTime:       r.EndTime,
		Detail:        r.Detail,
		Metadata:      r.Metadata,
		StepReports:   r.StepReports,
		Rollback:      r.Rollback,
		ExecutionMode: r.ExecutionMode,
		RollbackMode:  r.RollbackMode,
	}

	if r.Id != "" {
		m.Id = r.Id
	}

	if r.Error != nil {
		m.Error = r.Error.Error()
	}
	return json.Marshal(m)
}

// MarshalYAML implements yaml.Marshaler. It serializes the Report into a YAML
// mapping using the marshalReport wire format. The Error field is serialized
// as its Error() string (or omitted when nil). Field names follow the yaml
// struct tags on marshalReport.
func (r *Report) MarshalYAML() (interface{}, error) {
	m := marshalReport{
		Id:            r.Id,
		IsWorkflow:    r.IsWorkflow,
		Action:        r.Action,
		Status:        r.Status,
		StartTime:     r.StartTime,
		EndTime:       r.EndTime,
		Detail:        r.Detail,
		Metadata:      r.Metadata,
		StepReports:   r.StepReports,
		Rollback:      r.Rollback,
		ExecutionMode: r.ExecutionMode,
		RollbackMode:  r.RollbackMode,
	}
	if r.Error != nil {
		m.Error = r.Error.Error()
	}
	return m, nil
}
