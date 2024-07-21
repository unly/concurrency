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

func TestPanicError_As(t *testing.T) {
	t.Run("other panic error", func(t *testing.T) {
		err := PanicError{
			Val: 42,
		}

		var p PanicError
		assert.True(t, err.As(&p))
		assert.Equal(t, 42, p.Val)
	})

	t.Run("no panic error", func(t *testing.T) {
		err := PanicError{
			Val: 42,
		}

		e := assert.AnError
		assert.False(t, err.As(&e))
	})
}

func TestResult_Error(t *testing.T) {
	t.Run("empty result", func(t *testing.T) {
		err := &Result{}

		assert.Empty(t, err.Error())
	})

	t.Run("with underlying error", func(t *testing.T) {
		err := &Result{
			Combined: assert.AnError,
		}

		assert.Equal(t, assert.AnError.Error(), err.Error())
	})
}

func TestResult_Unwrap(t *testing.T) {
	t.Run("empty result", func(t *testing.T) {
		err := &Result{}

		assert.Nil(t, err.Unwrap())
	})

	t.Run("with underlying error", func(t *testing.T) {
		err := &Result{
			Combined: assert.AnError,
		}

		assert.ErrorIs(t, err, assert.AnError)
	})
}

func TestResult_GetResult(t *testing.T) {
	t.Run("empty result", func(t *testing.T) {
		err := &Result{}

		assert.Nil(t, err.GetResult(&Task{}))
	})

	t.Run("empty result", func(t *testing.T) {
		task1 := &Task{}
		task2 := &Task{}
		err := &Result{
			Errors: map[*Task]error{
				task1: assert.AnError,
			},
		}

		assert.Nil(t, err.GetResult(task2))
	})

	t.Run("error for task", func(t *testing.T) {
		task1 := &Task{}
		err := &Result{
			Errors: map[*Task]error{
				task1: assert.AnError,
			},
		}

		assert.Equal(t, assert.AnError, err.GetResult(task1))
	})
}
