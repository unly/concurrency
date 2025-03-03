package concurrency

import (
	"context"
	"fmt"
	"runtime/debug"
	"slices"
	"sync"
	"time"
)

func newRun(ctx context.Context, tp *TaskPool, ctrl Controller, tasks []*Task) *run {
	ctx, cancel := context.WithCancel(ctx)
	next := make(chan *Task, len(tasks))
	return &run{
		pool:     tp,
		ctx:      ctx,
		cancel:   cancel,
		ctrl:     ctrl,
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

func createTimeoutChannel(tp *TaskPool) <-chan time.Time {
	if tp.config.Timeout == 0 {
		return nil
	}

	return time.After(tp.config.Timeout)
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
	result       *ResultError
	ctrl         Controller
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

func (r *run) run() *ResultError {
	defer r.cancel()

	go r.start()

	timeout := createTimeoutChannel(r.pool)
	for {
		select {
		case <-timeout:
			r.result.Err = ErrTimeout
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

func (r *run) handleOutcome(out outcome) bool {
	if out.task == nil {
		r.result.Err = out.err
		return true
	}

	r.finishTask(out.task)

	if out.err == nil {
		return false
	}

	r.result.Errors[out.task] = out.err

	abort := r.ctrl.EarlyAbort(r.ctx, out.task, out.err)
	if abort || r.ctrl.Cancel(r.ctx, out.task, out.err) {
		r.cancel()
		r.result.Err = fmt.Errorf("canceled after: %w", out.err)
	}

	return abort
}

func (r *run) start() {
	defer close(r.outcomes)
	defer close(r.counter)

	if len(r.tasks) == 0 {
		r.closeNext()
		return
	}

	// build dependencies and reversed dependencies
	err := r.prepare()
	if err != nil {
		r.outcomes <- outcome{
			err: err,
		}
		return
	}

	r.wg.Add(len(r.tasks))
	for task := range r.next {
		r.counter <- struct{}{}

		select {
		case <-r.ctx.Done():
			// don't start tasks if already aborted
			r.outcomes <- outcome{
				task: task,
				err:  r.ctx.Err(),
			}
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
	}()

	r.outcomes <- outcome{
		task: task,
		err:  task.Func(r.ctx),
	}
}

func (r *run) prepare() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// cyclic check
	if cyclic(r.tasks) {
		return ErrCyclicDependencies
	}

	// build dependencies
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

	return nil
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

func cyclic(tasks []*Task) bool {
	var stack []*Task
	var pop *Task
	seen := make(map[*Task]struct{})

	for _, task := range tasks {
		stack = append(stack, task.DependsOn...)
		clear(seen)

		for len(stack) > 0 {
			pop = stack[0]
			if pop == task {
				return true
			}

			stack = stack[1:]
			if _, ok := seen[pop]; ok {
				continue
			}
			seen[pop] = struct{}{}
			stack = append(stack, pop.DependsOn...)
		}
	}

	return false
}
