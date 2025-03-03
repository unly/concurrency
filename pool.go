package concurrency

import (
	"context"
	"errors"
	"time"
)

// ErrTimeout returned if WithTimeout set timeout is exceeded
var ErrTimeout = errors.New("timeout")

type Option func(cfg *TaskPoolConfig)

type TaskPoolConfig struct {
	// MaxConcurrency number of parallel goroutines spawned from this
	// TaskPool at the same time.
	// Defaults to 0.
	MaxConcurrency uint32
	// Timeout the overall timeout for the executions of the TaskPool.
	// Defaults to 0.
	Timeout time.Duration
}

// WithMaxConcurrency controls the number of spawned goroutines at the
// same time. Will hold tasks back if the max limit is reached and waits
// for tasks to finish. A value of 0 represents no limit.
func WithMaxConcurrency(maxConcurrency uint32) Option {
	return func(cfg *TaskPoolConfig) {
		cfg.MaxConcurrency = maxConcurrency
	}
}

// WithTimeout sets a timeout for the overall execution of all tasks.
// Once this exceeds, context for all tasks gets canceled and no new
// tasks will be scheduled. However, active goroutines will continue
// to run as long as the task exits.
// The functions will return ErrTimeout.
func WithTimeout(timeout time.Duration) Option {
	return func(cfg *TaskPoolConfig) {
		cfg.Timeout = timeout
	}
}

var defaultPool = NewTaskPool()

func AwaitAll(ctx context.Context, tasks []*Task) *ResultError {
	return defaultPool.AwaitAll(ctx, tasks)
}

func AbortFirstError(ctx context.Context, tasks []*Task) *ResultError {
	return defaultPool.AbortFirstError(ctx, tasks)
}

func CancelFistError(ctx context.Context, tasks []*Task) *ResultError {
	return defaultPool.CancelFistError(ctx, tasks)
}

func Run(ctx context.Context, ctrl Controller, tasks []*Task) *ResultError {
	return defaultPool.Run(ctx, ctrl, tasks)
}

// NewTaskPool creates a new custom TaskPool with the given options.
func NewTaskPool(options ...Option) *TaskPool {
	tp := &TaskPool{}
	for _, option := range options {
		option(&tp.config)
	}

	return tp
}

type TaskPool struct {
	config TaskPoolConfig
}

// AwaitAll runs all provided tasks and returns a potential ResultError
// holding the individual outcomes of the tasks.
func (tp *TaskPool) AwaitAll(ctx context.Context, tasks []*Task) *ResultError {
	return tp.Run(ctx, AwaitAllTasks{}, tasks)
}

// AbortFirstError aborts and returns back to the caller after the first
// error returned from any of the tasks. Started tasks run in the background
// until they return from their closure.
func (tp *TaskPool) AbortFirstError(ctx context.Context, tasks []*Task) *ResultError {
	return tp.Run(ctx, FirstErrorAbort{}, tasks)
}

// CancelFistError cancels all pending and running tasks after the first
// error returned from any of tasks. In comparison with AbortFirstError,
// this method waits for all tasks to finish before retuning to the caller.
func (tp *TaskPool) CancelFistError(ctx context.Context, tasks []*Task) *ResultError {
	return tp.Run(ctx, FirstErrorCancelWait{}, tasks)
}

// Run runs the given list of tasks using the given Controller for aborting
// and stopping logics.
func (tp *TaskPool) Run(ctx context.Context, ctrl Controller, tasks []*Task) *ResultError {
	return newRun(ctx, tp, ctrl, tasks).run()
}
