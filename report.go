package automa

import (
	"encoding/json"
	"time"
)

type Report struct {
	Id          string
	Action      TypeAction
	Status      TypeStatus
	StartTime   time.Time
	EndTime     time.Time
	Detail      string
	Error       error
	Metadata    map[string]string
	StepReports []*Report
	Rollback    *Report
}

type marshalReport struct {
	Id          string            `yaml:"id,omitempty" json:"id,omitempty"` // rollback report does not need id
	Action      TypeAction        `yaml:"action,omitempty" json:"action,omitempty"`
	Status      TypeStatus        `yaml:"status,omitempty" json:"status,omitempty"`
	StartTime   time.Time         `yaml:"startTime,omitempty" json:"startTime,omitempty"`
	EndTime     time.Time         `yaml:"endTime,omitempty" json:"endTime,omitempty"`
	Detail      string            `yaml:"detail,omitempty" json:"detail,omitempty"`
	Error       string            `yaml:"error,omitempty" json:"error,omitempty"`
	Metadata    map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	StepReports []*Report         `yaml:"stepReports,omitempty" json:"steps,omitempty"`
	Rollback    *Report           `yaml:"rollback,omitempty" json:"rollback,omitempty"`
}

type ReportOption func(*Report)

func WithStepReports(reports ...*Report) ReportOption {
	return func(sr *Report) {
		if sr.StepReports == nil {
			sr.StepReports = make([]*Report, 0, len(reports))
		}
		sr.StepReports = append(sr.StepReports, reports...)
	}
}

func WithRollbackReport(rollback *Report) ReportOption {
	return func(sr *Report) {
		sr.Rollback = rollback
	}
}

func WithMetadata(metadata map[string]string) ReportOption {
	return func(sr *Report) {
		sr.Metadata = metadata
	}
}

func WithError(err error) ReportOption {
	return func(sr *Report) {
		sr.Error = err
	}
}

func WithStatus(status TypeStatus) ReportOption {
	return func(sr *Report) {
		sr.Status = status
	}
}

func WithStartTime(startTime time.Time) ReportOption {
	return func(sr *Report) {
		sr.StartTime = startTime
		if sr.EndTime.IsZero() {
			sr.EndTime = startTime // ensure end time is set if not already
		}
	}
}

func WithEndTime(endTime time.Time) ReportOption {
	return func(sr *Report) {
		sr.EndTime = endTime
		if sr.StartTime.IsZero() {
			sr.StartTime = endTime // ensure start time is set if not already
		}
	}
}

func WithDetail(detail string) ReportOption {
	return func(sr *Report) {
		sr.Detail = detail
	}
}

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
				sr.Metadata = make(map[string]string)
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
	}
}

func WithActionType(actionType TypeAction) ReportOption {
	return func(sr *Report) {
		sr.Action = actionType
	}
}

func NewReport(id string, opts ...ReportOption) *Report {
	r := &Report{
		Id:        id,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Status:    StatusSuccess,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// StepSuccessReport constructs a success report with options
func StepSuccessReport(id string, opts ...ReportOption) *Report {
	opts = append(opts, WithStatus(StatusSuccess), WithEndTime(time.Now()))
	return NewReport(id, opts...)
}

// StepFailureReport constructs a failure report with options
func StepFailureReport(id string, opts ...ReportOption) *Report {
	opts = append(opts, WithStatus(StatusFailed), WithEndTime(time.Now()))
	return NewReport(id, opts...)
}

// StepSkippedReport constructs a skipped report with options
func StepSkippedReport(id string, opts ...ReportOption) *Report {
	opts = append(opts, WithStatus(StatusSkipped), WithEndTime(time.Now()))
	return NewReport(id, opts...)
}

func (r *Report) MarshalJSON() ([]byte, error) {
	m := marshalReport{
		Action:      r.Action,
		Status:      r.Status,
		StartTime:   r.StartTime,
		EndTime:     r.EndTime,
		Detail:      r.Detail,
		Metadata:    r.Metadata,
		StepReports: r.StepReports,
		Rollback:    r.Rollback,
	}

	if r.Id != "" {
		m.Id = r.Id
	}

	if r.Error != nil {
		m.Error = r.Error.Error()
	}
	return json.Marshal(m)
}

func (r *Report) MarshalYAML() (interface{}, error) {
	m := marshalReport{
		Id:          r.Id,
		Action:      r.Action,
		Status:      r.Status,
		StartTime:   r.StartTime,
		EndTime:     r.EndTime,
		Detail:      r.Detail,
		Metadata:    r.Metadata,
		StepReports: r.StepReports,
		Rollback:    r.Rollback,
	}
	if r.Error != nil {
		m.Error = r.Error.Error()
	}
	return m, nil
}
