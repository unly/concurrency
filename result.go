package concurrency

import (
	"errors"
	"sync"
)

func newResult() *Result {
	return &Result{
		errors:   make(map[*Task]error),
		promises: make(map[*Task][]chan<- error),
	}
}

// Result contains a map including all errors occurred based on
// their respective Task.
type Result struct {
	// errors map of all outcomes occurred, via GetResult
	errors map[*Task]error
	// promises waiting for not finished tasks, via GetPromise
	promises map[*Task][]chan<- error
	// err is the overall error used for the Err method
	err error
	// sealed is set if the run ended without executing any task. Pending
	// and future promises resolve with err instead of waiting.
	sealed bool

	mu sync.Mutex
}

// GetResult returns the outcome error for the given Task.
// The boolean identified indicates if the result has been
// updated already.
func (r *Result) GetResult(t *Task) (error, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	err, ok := r.errors[t]
	return err, ok
}

// GetPromise returns a read only channel of size 1. If the
// outcome is already available the error is in that channel
// otherwise it will be written once available. If the run
// failed before executing any task (e.g. cyclic dependencies)
// the channel holds the error of the run.
func (r *Result) GetPromise(t *Task) <-chan error {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan error, 1)
	err, ok := r.errors[t]
	if ok {
		ch <- err
		close(ch)
		return ch
	}

	if r.sealed {
		ch <- r.err
		close(ch)
		return ch
	}

	r.promises[t] = append(r.promises[t], ch)

	return ch
}

// Err returns a combined error of the individual errors
// occurred based on the respective controller used.
func (r *Result) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err != nil {
		return r.err
	}

	if len(r.errors) == 0 {
		return nil
	}

	errs := make([]error, 0, len(r.errors))
	for _, v := range r.errors {
		errs = append(errs, v)
	}
	r.err = errors.Join(errs...)
	return r.err
}

func (r *Result) setError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err == nil {
		r.err = err
	}
}

// seal marks the run as finished without executing any task and
// resolves all pending promises with err.
func (r *Result) seal(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.err == nil {
		r.err = err
	}
	r.sealed = true

	for t, chs := range r.promises {
		for _, ch := range chs {
			ch <- r.err
			close(ch)
		}
		delete(r.promises, t)
	}
}

func (r *Result) setResult(t *Task, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.errors[t] = err
	for _, ch := range r.promises[t] {
		ch <- err
		close(ch)
	}

	delete(r.promises, t)
}
