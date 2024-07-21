package concurrency

import (
	"context"
	"errors"
	"time"
)

var ErrTimeout = errors.New("timeout")

type mode uint8

const (
	modeAbortFirst mode = 1 + iota
	modeAbortFirstWait
	modeWaitAll
)

type Option func(cfg *TaskPoolConfig)

type TaskPoolConfig struct {
	MaxConcurrency uint32
	Timeout        time.Duration
}

func WithMaxConcurrency(maxConcurrency uint32) Option {
	return func(cfg *TaskPoolConfig) {
		cfg.MaxConcurrency = maxConcurrency
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(cfg *TaskPoolConfig) {
		cfg.Timeout = timeout
	}
}

var defaultPool = NewTaskPool()

func AwaitAll(ctx context.Context, tasks []*Task) *Result {
	return defaultPool.AwaitAll(ctx, tasks)
}

func First(ctx context.Context, tasks []*Task) *Result {
	return defaultPool.First(ctx, tasks)
}

func FirstAwait(ctx context.Context, tasks []*Task) *Result {
	return defaultPool.FirstAwait(ctx, tasks)
}

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

func (tp *TaskPool) AwaitAll(ctx context.Context, tasks []*Task) *Result {
	return newRun(tp, modeWaitAll, ctx, tasks).run()
}

func (tp *TaskPool) First(ctx context.Context, tasks []*Task) *Result {
	return newRun(tp, modeAbortFirst, ctx, tasks).run()
}

func (tp *TaskPool) FirstAwait(ctx context.Context, tasks []*Task) *Result {
	return newRun(tp, modeAbortFirstWait, ctx, tasks).run()
}

func (tp *TaskPool) timeout() <-chan time.Time {
	if tp.config.Timeout == 0 {
		return nil
	}

	return time.After(tp.config.Timeout)
}
