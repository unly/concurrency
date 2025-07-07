package concurrency

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/unly/go-collections"
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
		dependsOn:    make(map[*Task]*collections.Set[*Task]),
		revDependsOn: make(map[*Task]*collections.Set[*Task]),
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
	dependsOn    map[*Task]*collections.Set[*Task]
	revDependsOn map[*Task]*collections.Set[*Task]
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

		var s collections.Set[*Task]
		s.Add(task.DependsOn...)
		r.dependsOn[task] = &s
		for _, depends := range task.DependsOn {
			if _, ok := r.revDependsOn[depends]; !ok {
				r.revDependsOn[depends] = &collections.Set[*Task]{}
			}
			r.revDependsOn[depends].Add(task)
		}
	}

	return nil
}

func (r *run) finishTask(t *Task) {
	defer r.wg.Done()
	<-r.counter

	r.mu.Lock()
	defer r.mu.Unlock()
	revDependsOn, ok := r.revDependsOn[t]
	if ok {
		for dependsOnTask := range revDependsOn.Values() {
			r.dependsOn[dependsOnTask].Delete(t)
			if r.dependsOn[dependsOnTask].Size() == 0 {
				r.next <- dependsOnTask
				delete(r.dependsOn, dependsOnTask)
			}
		}
		delete(r.revDependsOn, t)
	}
	// close the next task channel if there are no dependent tasks left
	if len(r.dependsOn) == 0 {
		r.closeNext()
	}
}

type outcome struct {
	task *Task
	err  error
}

const (
	unvisited uint8 = iota
	visiting
	visited
)

func cyclic(tasks []*Task) bool {
	visited := make(map[*Task]uint8)

	for _, task := range tasks {
		if visited[task] == unvisited {
			if hasCycle(task, visited) {
				return true
			}
		}
	}

	return false
}

func hasCycle(task *Task, visitedMap map[*Task]uint8) bool {
	visitedMap[task] = visiting

	for _, dependency := range task.DependsOn {
		if visitedMap[dependency] == visiting {
			return true
		}

		if visitedMap[dependency] == unvisited && hasCycle(dependency, visitedMap) {
			return true
		}
	}

	visitedMap[task] = visited
	return false
}
