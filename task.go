package concurrency

import (
	"context"
)

// Func actual function to execute concurrently. Should react
// to the given context.
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
