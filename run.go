package concurrency

import (
	"context"
	"fmt"
	"runtime/debug"
	"slices"
	"sync"
)

func newRun(tp *TaskPool, mode mode, ctx context.Context, tasks []*Task) *run {
	ctx, cancel := context.WithCancel(ctx)
	next := make(chan *Task, len(tasks))
	return &run{
		pool:     tp,
		ctx:      ctx,
		cancel:   cancel,
		mode:     mode,
		result:   newResult(),
		tasks:    tasks,
		outcomes: make(chan outcome, len(tasks)),
		next:     next,
		closeNext: sync.OnceFunc(func() {
			close(next)
		}),
		counter:      createSemaphoreChannel(tp, tasks),
		dependsOn:    make(map[*Task][]*Task),
		revDependsOn: make(map[*Task][]*Task),
	}
}

func createSemaphoreChannel(tp *TaskPool, tasks []*Task) chan struct{} {
	maxConcurrency := tp.config.MaxConcurrency
	if maxConcurrency == 0 {
		maxConcurrency = uint32(len(tasks))
	}

	return make(chan struct{}, maxConcurrency)
}

type run struct {
	pool         *TaskPool
	ctx          context.Context
	cancel       context.CancelFunc
	result       *Result
	mode         mode
	tasks        []*Task
	outcomes     chan outcome
	next         chan *Task
	closeNext    func()
	counter      chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
	dependsOn    map[*Task][]*Task
	revDependsOn map[*Task][]*Task
}

func (r *run) run() *Result {
	defer r.cancel()

	go r.start()

	timeout := r.pool.timeout()
	for {
		select {
		case <-timeout:
			r.result.setError(ErrTimeout)
			return r.result.build()
		case out, open := <-r.outcomes:
			if !open {
				return r.result.build()
			}

			if r.handleOutcome(out) {
				return r.result.build()
			}
		}
	}
}

func (r *run) handleOutcome(out outcome) (abort bool) {
	if out.err == nil {
		return false
	}

	r.result.Errors[out.task] = out.err

	switch r.mode {
	case modeWaitAll:
		// nothing to do
	case modeAbortFirst:
		abort = true
		r.cancel()
		r.result.setError(fmt.Errorf("first error: %w", out.err))
	case modeAbortFirstWait:
		r.cancel()
		r.result.setError(fmt.Errorf("first error: %w", out.err))
	}

	return abort
}

func (r *run) start() {
	defer close(r.outcomes)
	defer close(r.counter)

	if len(r.tasks) == 0 {
		return
	}

	// build dependencies and reversed dependencies
	r.buildDependencies()

	for task := range r.next {
		r.counter <- struct{}{}
		r.wg.Add(1)

		select {
		case <-r.ctx.Done():
			// don't start tasks if already aborted
			r.outcomes <- outcome{
				task: task,
				err:  r.ctx.Err(),
			}
			r.finishTask(task)
		default:
			go r.runTask(task)
		}
	}

	r.wg.Wait()
}

func (r *run) runTask(task *Task) {
	defer func() {
		val := recover()
		if val != nil {
			r.outcomes <- outcome{
				task: task,
				err: &PanicError{
					Val:   val,
					Stack: debug.Stack(),
				},
			}
		}
		r.finishTask(task)
	}()

	r.outcomes <- outcome{
		task: task,
		err:  task.Func(r.ctx),
	}
}

func (r *run) buildDependencies() {
	r.mu.Lock()
	defer r.mu.Unlock()
	// TODO: cyclic check
	for _, task := range r.tasks {
		if len(task.DependsOn) == 0 {
			r.next <- task
			continue
		}

		r.dependsOn[task] = task.DependsOn
		for _, depends := range task.DependsOn {
			r.revDependsOn[depends] = append(r.revDependsOn[depends], task)
		}
	}
}

func (r *run) finishTask(t *Task) {
	defer r.wg.Done()
	<-r.counter

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, dependsOnTask := range r.revDependsOn[t] {
		r.dependsOn[dependsOnTask] = slices.DeleteFunc(r.dependsOn[dependsOnTask], func(other *Task) bool {
			return other == t
		})
		if len(r.dependsOn[dependsOnTask]) == 0 {
			r.next <- dependsOnTask
			delete(r.dependsOn, dependsOnTask)
		}
	}
	delete(r.revDependsOn, t)
	// close the next task channel if there are no dependent tasks left
	if len(r.dependsOn) == 0 {
		r.closeNext()
	}
}

type outcome struct {
	task *Task
	err  error
}
