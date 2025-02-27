package concurrency

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPanicError_Error(t *testing.T) {
	t.Run("sample error", func(t *testing.T) {
		err := PanicError{
			Val:   42,
			Stack: debug.Stack(),
		}

		assert.Equal(t, "panic: 42", err.Error())
	})

	t.Run("empty error", func(t *testing.T) {
		err := PanicError{}

		assert.Equal(t, "panic: <nil>", err.Error())
	})
}

func TestResult_Error(t *testing.T) {
	t.Run("empty result", func(t *testing.T) {
		err := &ResultError{}

		assert.Empty(t, err.Error())
	})

	t.Run("with underlying error", func(t *testing.T) {
		err := &ResultError{
			Err: assert.AnError,
		}

		assert.Equal(t, assert.AnError.Error(), err.Error())
	})
}

func TestResult_Unwrap(t *testing.T) {
	t.Run("empty result", func(t *testing.T) {
		err := &ResultError{}

		assert.Len(t, err.Unwrap(), 0)
	})

	t.Run("with underlying error", func(t *testing.T) {
		err := &ResultError{
			Err: assert.AnError,
		}

		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("map of errors", func(t *testing.T) {
		err := &ResultError{
			Errors: map[*Task]error{
				NewTask(nil): assert.AnError,
				NewTask(nil): assert.AnError,
				NewTask(nil): assert.AnError,
			},
		}

		assert.Len(t, err.Unwrap(), 3)
	})
}

func TestResult_GetResult(t *testing.T) {
	t.Run("empty result", func(t *testing.T) {
		err := &ResultError{}

		assert.Nil(t, err.GetResult(&Task{}))
	})

	t.Run("empty result", func(t *testing.T) {
		task1 := &Task{}
		task2 := &Task{}
		err := &ResultError{
			Errors: map[*Task]error{
				task1: assert.AnError,
			},
		}

		assert.Nil(t, err.GetResult(task2))
	})

	t.Run("error for task", func(t *testing.T) {
		task1 := &Task{}
		err := &ResultError{
			Errors: map[*Task]error{
				task1: assert.AnError,
			},
		}

		assert.Equal(t, assert.AnError, err.GetResult(task1))
	})
}
