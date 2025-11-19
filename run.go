package concurrency

import (
	"context"
	"fmt"
	"maps"
	"runtime/debug"
	"sync"
	"time"

	"github.com/unly/go-collections"
)

func newRun(ctx context.Context, tp *TaskPool, ctrl Controller, tasks []*Task) *run {
	ctx, cancel := context.WithCancel(ctx)

	return &run{
		pool:           tp,
		ctx:            ctx,
		cancel:         cancel,
		ctrl:           ctrl,
		result:         newResult(),
		tasks:          tasks,
		startCompleted: make(chan struct{}),
		completed:      make(chan struct{}),
		dependsOn:      make(map[*Task]*collections.Set[*Task]),
		revDependsOn:   make(map[*Task]*collections.Set[*Task]),
	}
}

func createSemaphoreChannel(tp *TaskPool, total int) chan struct{} {
	maxConcurrency := int(tp.config.MaxConcurrency)
	if maxConcurrency == 0 {
		maxConcurrency = total
	}

	return make(chan struct{}, maxConcurrency)
}

type run struct {
	pool           *TaskPool
	ctx            context.Context
	cancel         context.CancelFunc
	result         *Result
	ctrl           Controller
	tasks          []*Task
	outcomes       chan outcome
	next           chan *Task
	closeNextOnce  sync.Once
	counter        chan struct{}
	startCompleted chan struct{}
	completed      chan struct{}
	closeCompleted sync.Once
	wg             sync.WaitGroup
	mu             sync.Mutex
	dependsOn      map[*Task]*collections.Set[*Task]
	revDependsOn   map[*Task]*collections.Set[*Task]
}

func (r *run) run() (*Result, error) {
	defer r.cancel()

	go r.start()

	var timeout <-chan time.Time
	if r.pool.config.Timeout > 0 {
		timer := time.NewTimer(r.pool.config.Timeout)
		defer timer.Stop()
		timeout = timer.C
	}

	select {
	case <-timeout:
		r.result.setError(ErrTimeout)
		r.cancel()
	case <-r.completed:
		// nothing to do
	}

	return r.result, r.result.Err()
}

func (r *run) handleOutcome(out outcome) {
	defer r.wg.Done()
	defer r.finishTask(out.task)

	r.result.setResult(out.task, out.err)

	if out.err == nil {
		return
	}

	abort := r.ctrl.EarlyAbort(r.ctx, out.task, out.err)
	if abort || r.ctrl.Cancel(r.ctx, out.task, out.err) {
		r.cancel()
		r.result.setError(fmt.Errorf("canceled after: %w", out.err))
	}
	if abort {
		r.abort()
	}
}

func (r *run) start() {
	defer close(r.startCompleted)
	// iterate graph and check cyclic dependencies
	err := r.iterateGraph()
	if err != nil {
		r.result.setError(err)
		r.abort()
		return
	}

	total := len(r.tasks)
	if total == 0 {
		r.abort()
		return
	}

	r.outcomes = make(chan outcome, total)
	r.next = make(chan *Task, total)
	r.counter = createSemaphoreChannel(r.pool, total)
	defer close(r.outcomes)
	defer close(r.counter)

	go r.listen()

	// build dependencies and reversed dependencies
	r.prepare()

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
		default:
			go r.runTask(task)
		}
	}

	r.wg.Wait()
}

func (r *run) listen() {
	defer r.abort()

	for out := range r.outcomes {
		r.handleOutcome(out)
	}

	<-r.startCompleted
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

func (r *run) prepare() {
	r.mu.Lock()
	var ready []*Task
	// build dependencies
	for _, task := range r.tasks {
		if len(task.DependsOn) == 0 {
			ready = append(ready, task)
			continue
		}

		s := &collections.Set[*Task]{}
		s.Add(task.DependsOn...)
		r.dependsOn[task] = s
		for _, depends := range task.DependsOn {
			if _, ok := r.revDependsOn[depends]; !ok {
				r.revDependsOn[depends] = &collections.Set[*Task]{}
			}
			r.revDependsOn[depends].Add(task)
		}
	}
	r.mu.Unlock()

	for _, t := range ready {
		r.sendNext(t)
	}
}

func (r *run) finishTask(t *Task) {
	<-r.counter

	r.mu.Lock()
	revDependsOn, ok := r.revDependsOn[t]
	var ready []*Task
	if ok {
		for dependsOnTask := range revDependsOn.Values() {
			r.dependsOn[dependsOnTask].Delete(t)
			if r.dependsOn[dependsOnTask].Size() == 0 {
				ready = append(ready, dependsOnTask)
				delete(r.dependsOn, dependsOnTask)
			}
		}
		delete(r.revDependsOn, t)
	}
	completed := len(r.dependsOn) == 0
	r.mu.Unlock()

	for _, rt := range ready {
		r.sendNext(rt)
	}

	if completed {
		r.closeNext()
	}
}

func (r *run) sendNext(task *Task) {
	r.next <- task
}

func (r *run) closeNext() {
	r.closeNextOnce.Do(func() {
		close(r.next)
	})
}

func (r *run) abort() {
	r.closeCompleted.Do(func() {
		close(r.completed)
	})
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

func (r *run) iterateGraph() error {
	seen := make(map[*Task]uint8)

	for _, task := range r.tasks {
		if seen[task] == unvisited {
			if hasCycle(task, seen) {
				return ErrCyclicDependencies
			}
		}
	}

	r.tasks = collections.Collect(maps.Keys(seen), len(seen))

	return nil
}

func hasCycle(task *Task, seen map[*Task]uint8) bool {
	seen[task] = visiting

	for _, dependency := range task.DependsOn {
		if seen[dependency] == visiting {
			return true
		}

		if seen[dependency] == unvisited && hasCycle(dependency, seen) {
			return true
		}
	}

	seen[task] = visited
	return false
}
