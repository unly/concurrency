package concurrency

import (
	"errors"
	"fmt"
)

type PanicError struct {
	Val   any
	Stack []byte
}

func (e PanicError) Error() string {
	return fmt.Sprintf("panic: %v", e.Val)
}

func (e PanicError) As(v any) bool {
	other, ok := v.(*PanicError)
	if !ok {
		return false
	}

	other.Val = e.Val
	other.Stack = e.Stack
	return true
}

func newResult() *Result {
	return &Result{
		Errors: make(map[*Task]error),
	}
}

type Result struct {
	Errors   map[*Task]error
	Combined error
}

func (r *Result) GetResult(t *Task) error {
	return r.Errors[t]
}

func (r *Result) Error() string {
	if r.Combined == nil {
		return ""
	}

	return r.Combined.Error()
}

func (r *Result) Unwrap() error {
	return r.Combined
}

func (r *Result) setError(err error) {
	if r.Combined == nil {
		r.Combined = err
	}
}

func (r *Result) build() *Result {
	if r.Combined != nil {
		return r
	}

	if len(r.Errors) > 0 {
		errs := make([]error, 0, len(r.Errors))
		for _, v := range r.Errors {
			errs = append(errs, v)
		}
		r.Combined = errors.Join(errs...)
		return r
	}

	return nil
}
