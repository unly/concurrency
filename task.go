package concurrency

import (
	"context"
)

// Func actual function to execute concurrently. Should react
// to the potential cancellation of the given context. Returned
// errors will be routed to the used Controller.
type Func func(ctx context.Context) error

type Task struct {
	Func      Func
	DependsOn []*Task
}

func NewTask(fn Func, dependsOn ...*Task) *Task {
	return &Task{
		Func:      fn,
		DependsOn: dependsOn,
	}
}
