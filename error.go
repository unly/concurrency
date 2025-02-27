package concurrency

import (
	"errors"
	"fmt"
)

type PanicError struct {
	Val   any
	Stack []byte
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("panic: %v", e.Val)
}

func newResult() *ResultError {
	return &ResultError{
		Errors: make(map[*Task]error),
	}
}

// ResultError contains a map including all errors occurred based on
// their respective Task.
type ResultError struct {
	// Errors map of all errors occurred
	Errors map[*Task]error
	// Err is the overall error used for the Error method
	Err error
}

func (r *ResultError) GetResult(t *Task) error {
	return r.Errors[t]
}

func (r *ResultError) Error() string {
	if r.Err == nil {
		return ""
	}

	return r.Err.Error()
}

func (r *ResultError) Unwrap() []error {
	errs := make([]error, 0, len(r.Errors)+1)
	for _, v := range r.Errors {
		errs = append(errs, v)
	}
	if r.Err != nil {
		errs = append(errs, r.Err)
	}
	return errs
}

func (r *ResultError) build() *ResultError {
	if r.Err != nil {
		return r
	}

	if len(r.Errors) == 0 {
		return nil
	}

	errs := make([]error, 0, len(r.Errors))
	for _, v := range r.Errors {
		errs = append(errs, v)
	}
	r.Err = errors.Join(errs...)
	return r
}
