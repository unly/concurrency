package concurrency

import (
	"context"
)

// Controller the functions are called in case a Task returns an error. Will
// be ignored for nil errors.
type Controller interface {
	// EarlyAbort calculates if the entire execution should be aborted. Returns
	// directly to the caller if returned true. Also cancels all pending tasks.
	EarlyAbort(ctx context.Context, task *Task, err error) bool
	// Cancel cancels the execution of all tasks. Not called if EarlyAbort
	// returned true.
	Cancel(ctx context.Context, task *Task, err error) bool
}

type AwaitAllTasks struct{}

func (AwaitAllTasks) EarlyAbort(_ context.Context, _ *Task, _ error) bool {
	return false
}

func (AwaitAllTasks) Cancel(_ context.Context, _ *Task, _ error) bool {
	return false
}

type FirstErrorAbort struct{}

func (FirstErrorAbort) EarlyAbort(_ context.Context, _ *Task, _ error) bool {
	return true
}

func (FirstErrorAbort) Cancel(_ context.Context, _ *Task, _ error) bool {
	return true
}

type FirstErrorCancelWait struct{}

func (FirstErrorCancelWait) EarlyAbort(_ context.Context, _ *Task, _ error) bool {
	return false
}

func (FirstErrorCancelWait) Cancel(_ context.Context, _ *Task, _ error) bool {
	return true
}
