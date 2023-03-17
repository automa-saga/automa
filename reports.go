package automa

import (
	"github.com/cockroachdb/errors"
	"time"
)

type Status string

func (r *Report) End(prevReports Reports, status Status) Reports {
	r.EndTime = time.Now()

	clone := Reports{}
	for key, val := range prevReports {
		clone[key] = val
	}

	clone[r.StepID] = r

	return clone
}

func StartReport(id string) *Report {
	return &Report{
		StepID:    id,
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Status:    StatusFailed,
		Error:     errors.EncodedError{},
		metadata:  map[string][]byte{},
	}
}

type Reports map[string]*Report

type Report struct {
	StepID    string
	StartTime time.Time
	EndTime   time.Time
	Status    Status
	Error     errors.EncodedError
	metadata  map[string][]byte
}
