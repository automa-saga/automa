package automa

import (
	"time"
)

type Report struct {
	Id          string
	Type        TypeReport        `yaml:"type" json:"type"`
	StartTime   time.Time         `yaml:"StartTime" json:"StartTime"`
	EndTime     time.Time         `yaml:"EndTime" json:"EndTime"`
	Status      TypeStatus        `yaml:"Status" json:"Status"`
	Error       error             `yaml:"error" json:"error"`
	StepReports []*Report         `yaml:"stepReports" json:"stepReports"`
	Metadata    map[string]string `yaml:"metadata" json:"metadata"` // optional Metadata for additional information
	Rollback    *RollbackReport   `yaml:"rollback" json:"rollback"` // optional rollback report
}

type RollbackReport struct {
	StartTime   time.Time  `yaml:"StartTime" json:"StartTime"`
	EndTime     time.Time  `yaml:"EndTime" json:"EndTime"`
	Status      TypeStatus `yaml:"Status" json:"Status"`
	Error       error      `yaml:"error" json:"error"` // error during rollback, if any
	StepReports []*Report  `yaml:"stepReports" json:"stepReports"`
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

func WithRollbackReport(rollback *RollbackReport) ReportOption {
	return func(sr *Report) {
		sr.Rollback = rollback
	}
}

func WithReportType(reportType TypeReport) ReportOption {
	return func(sr *Report) {
		sr.Type = reportType
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
		Type:      StepReportType,
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
