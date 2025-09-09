package automa

import (
	"time"
)

type Report struct {
	Id          string
	Type        TypeReport        `yaml:"type" json:"type"` // step or workflow report
	StartTime   time.Time         `yaml:"StartTime" json:"StartTime"`
	EndTime     time.Time         `yaml:"EndTime" json:"EndTime"`
	Status      TypeStatus        `yaml:"TypeStatus" json:"TypeStatus"`
	Error       error             `yaml:"error" json:"error"`             // error during execution, if any
	Metadata    map[string]string `yaml:"metadata" json:"metadata"`       // optional Metadata for additional information
	StepReports []*Report         `yaml:"stepReports" json:"stepReports"` // optional, only for workflow report
	Rollback    *Report           `yaml:"rollback" json:"rollback"`       // optional rollback report
}

type ReportOption func(*Report)

func WithReports(reports ...*Report) ReportOption {
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

func WithReportType(t TypeReport) ReportOption {
	return func(sr *Report) {
		sr.Type = t
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

func NewReport(id string, opts ...ReportOption) *Report {
	r := &Report{
		Id:        id,
		Type:      StepReport,
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
func StepSkippedReport(id string, action TypeAction, opts ...ReportOption) *Report {
	opts = append(opts, WithStatus(StatusSkipped), WithEndTime(time.Now()))
	return NewReport(id, opts...)
}
