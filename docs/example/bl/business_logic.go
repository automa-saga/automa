package bl

import (
	"context"
	"fmt"
	"github.com/automa-saga/automa"
	"github.com/joomcode/errorx"
)

func Step1Execute(ctx context.Context) (automa.Report, error) {
	// Simulate some work
	fmt.Println("Executing step 1")
	return automa.StepSuccessReport("step1", automa.ActionExecute), nil
}

func Step1Rollback(ctx context.Context) (automa.Report, error) {
	// Simulate rollback work
	fmt.Println("Rolling back step 1")
	return automa.StepSuccessReport("step1", automa.ActionRollback, automa.WithMessage("rollback was successful")), nil
}

func Step2Execute(ctx context.Context) (automa.Report, error) {
	// Simulate some work
	fmt.Println("Executing step 2")
	return automa.StepSuccessReport("step2", automa.ActionExecute), nil
}

func Step3Execute(ctx context.Context) (automa.Report, error) {
	// Simulate some work
	fmt.Println("step3 execution failed")
	return nil, errorx.IllegalState.New("step3 execution failed")
}

func Step3Rollback(ctx context.Context) (automa.Report, error) {
	// Simulate rollback work
	fmt.Println("Rolling back step 3")
	return automa.StepSuccessReport("step3", automa.ActionRollback, automa.WithMessage("rollback was successful")), nil
}
