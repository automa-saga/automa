package automa

import (
	"fmt"
	"time"
)

type stepReport struct {
	id        string
	action    ActionType
	startTime time.Time
	endTime   time.Time
	status    Status
	err       error
	message   string
	metadata  map[string]string // optional metadata for additional information
}

func (sr *stepReport) Id() string {
	return sr.id
}

func (sr *stepReport) Action() ActionType {
	return sr.action
}

func (sr *stepReport) StartTime() time.Time {
	return sr.startTime
}

func (sr *stepReport) EndTime() time.Time {
	return sr.endTime
}

func (sr *stepReport) Status() Status {
	return sr.status
}

func (sr *stepReport) Error() error {
	return sr.err
}

func (sr *stepReport) Message() string {
	if sr.message != "" {
		return sr.message
	}
	if sr.err != nil {
		return sr.err.Error()
	}
	return ""
}

func (sr *stepReport) Metadata() map[string]string {
	if sr.metadata == nil {
		return make(map[string]string) // return an empty map if metadata is not set
	}
	return sr.metadata
}

type StepReportOption func(*stepReport)

func WithMetadata(metadata map[string]string) StepReportOption {
	return func(sr *stepReport) {
		sr.metadata = metadata
	}
}

func WithMessage(msg string) StepReportOption {
	return func(sr *stepReport) {
		sr.message = msg
	}
}

func WithError(err error) StepReportOption {
	return func(sr *stepReport) {
		sr.err = err
		if sr.message == "" {
			sr.message = fmt.Sprintf("%s: execution failed: %v", sr.id, err)
		}
	}
}

func WithStatus(status Status) StepReportOption {
	return func(sr *stepReport) {
		sr.status = status
		switch status {
		case StatusSuccess:
			if sr.message == "" {
				sr.message = fmt.Sprintf("%s: execution successful", sr.id)
			}
		case StatusFailed:
			if sr.message == "" && sr.err != nil {
				sr.message = fmt.Sprintf("%s: execution failed: %v", sr.id, sr.err)
			}
		case StatusSkipped:
			if sr.message == "" {
				sr.message = fmt.Sprintf("%s: execution skipped", sr.id)
			}
		default:
			sr.message = fmt.Sprintf("%s: execution status is %s", sr.id, status)
		}
	}
}

func WithStartTime(startTime time.Time) StepReportOption {
	return func(sr *stepReport) {
		sr.startTime = startTime
		if sr.endTime.IsZero() {
			sr.endTime = startTime // ensure end time is set if not already
		}
	}
}

func WithEndTime(endTime time.Time) StepReportOption {
	return func(sr *stepReport) {
		sr.endTime = endTime
		if sr.startTime.IsZero() {
			sr.startTime = endTime // ensure start time is set if not already
		}
	}
}

func NewStepReport(id string, action ActionType, opts ...StepReportOption) Report {
	r := &stepReport{
		id:        id,
		action:    action,
		startTime: time.Now(),
		endTime:   time.Now(),
		status:    StatusSuccess,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// StepSuccessReport constructs a success report with options
func StepSuccessReport(id string, action ActionType, opts ...StepReportOption) Report {
	opts = append(opts, WithStatus(StatusSuccess), WithEndTime(time.Now()))
	return NewStepReport(id, action, opts...)
}

// StepFailureReport constructs a failure report with options
func StepFailureReport(id string, action ActionType, opts ...StepReportOption) Report {
	opts = append(opts, WithStatus(StatusFailed), WithEndTime(time.Now()))
	return NewStepReport(id, action, opts...)
}

// StepSkippedReport constructs a skipped report with options
func StepSkippedReport(id string, action ActionType, opts ...StepReportOption) Report {
	opts = append(opts, WithStatus(StatusSkipped), WithEndTime(time.Now()))
	return NewStepReport(id, action, opts...)
}
