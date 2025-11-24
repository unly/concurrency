package concurrency

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResult_Err(t *testing.T) {
	t.Run("empty result", func(t *testing.T) {
		r := &Result{}

		assert.Nil(t, r.Err())
	})

	t.Run("with underlying error", func(t *testing.T) {
		r := &Result{
			err: assert.AnError,
		}

		assert.Equal(t, assert.AnError, r.Err())
	})

	t.Run("with multiple errors", func(t *testing.T) {
		r := &Result{
			errors: map[*Task]error{
				&Task{}: err1,
				&Task{}: err2,
			},
		}

		err := r.Err()

		assert.ErrorIs(t, err, err1)
		assert.ErrorIs(t, err, err2)
	})
}

func TestResult_GetResult(t *testing.T) {
	t.Run("empty result", func(t *testing.T) {
		r := &Result{}

		err, ok := r.GetResult(&Task{})
		assert.False(t, ok)
		assert.Nil(t, err)
	})

	t.Run("empty result", func(t *testing.T) {
		task1 := &Task{}
		task2 := &Task{}
		r := &Result{
			errors: map[*Task]error{
				task1: assert.AnError,
			},
		}

		err, ok := r.GetResult(task2)
		assert.False(t, ok)
		assert.Nil(t, err)
	})

	t.Run("error for task", func(t *testing.T) {
		task1 := &Task{}
		r := &Result{
			errors: map[*Task]error{
				task1: assert.AnError,
			},
		}

		err, ok := r.GetResult(task1)
		assert.True(t, ok)
		assert.Equal(t, assert.AnError, err)
	})
}

func TestResult_GetPromise(t *testing.T) {
	t.Run("already existing error", func(t *testing.T) {
		task1 := &Task{}
		r := &Result{
			errors: map[*Task]error{
				task1: assert.AnError,
			},
		}

		p := r.GetPromise(task1)
		err := <-p
		assert.Equal(t, assert.AnError, err)
	})

	t.Run("waiting for result", func(t *testing.T) {
		task1 := &Task{}
		r := &Result{
			errors:   make(map[*Task]error),
			promises: make(map[*Task][]chan<- error),
		}
		ch1 := r.GetPromise(task1)
		ch2 := r.GetPromise(task1)
		r.setResult(task1, assert.AnError)

		err := <-ch1
		assert.Equal(t, assert.AnError, err)
		err = <-ch2
		assert.Equal(t, assert.AnError, err)
	})
}
