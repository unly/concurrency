package concurrency

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	err1 = errors.New("err1")
	err2 = errors.New("err2")
)

func TestTaskPool_AwaitAll(t *testing.T) {
	t.Run("wait for all tasks", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return err1
		})
		task2 := NewTask(func(_ context.Context) error {
			return err2
		})

		_, err := AwaitAll(context.TODO(), []*Task{task1, task2})

		assert.Error(t, err)
		assert.ErrorIs(t, err, err1)
		assert.ErrorIs(t, err, err2)
	})

	t.Run("no errors", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			return nil
		})

		res, err := AwaitAll(context.TODO(), []*Task{task1, task2})

		assert.NoError(t, err)
		err, ok := res.GetResult(task1)
		assert.True(t, ok)
		assert.NoError(t, err)
		err, ok = res.GetResult(task2)
		assert.True(t, ok)
		assert.NoError(t, err)
	})

	t.Run("with timeout", func(t *testing.T) {
		done := make(chan struct{})
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			<-done
			return nil
		})

		res, err := NewTaskPool(WithTimeout(time.Nanosecond)).AwaitAll(context.TODO(), []*Task{task1, task2})
		close(done)

		assert.ErrorIs(t, err, ErrTimeout)
		<-res.GetPromise(task1)
		<-res.GetPromise(task2)
	})

	t.Run("with max concurrency", func(t *testing.T) {
		var counter atomic.Int32
		task1 := NewTask(func(_ context.Context) error {
			assert.Equal(t, int32(1), counter.Add(1))
			counter.Add(-1)
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			assert.Equal(t, int32(1), counter.Add(1))
			counter.Add(-1)
			return nil
		})

		_, err := NewTaskPool(WithMaxConcurrency(1)).AwaitAll(context.TODO(), []*Task{task1, task2})

		assert.NoError(t, err)
		assert.Equal(t, int32(0), counter.Load())
	})

	t.Run("order of tasks", func(t *testing.T) {
		var order []int
		task1 := NewTask(func(_ context.Context) error {
			order = append(order, 1)
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			order = append(order, 2)
			return nil
		}, task1)

		_, err := NewTaskPool().AwaitAll(context.TODO(), []*Task{task2, task1})

		assert.NoError(t, err)
		assert.Len(t, order, 2)
		assert.Equal(t, 1, order[0])
		assert.Equal(t, 2, order[1])
	})

	t.Run("task not provided", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			return nil
		}, task1)

		_, err := NewTaskPool().AwaitAll(context.TODO(), []*Task{task2})

		assert.NoError(t, err)
	})

	t.Run("no tasks", func(t *testing.T) {
		_, err := AwaitAll(context.TODO(), nil)

		assert.NoError(t, err)
	})

	t.Run("panic in task", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			panic(42)
		})
		task2 := NewTask(func(_ context.Context) error {
			return nil
		})

		_, err := AwaitAll(context.TODO(), []*Task{task1, task2})

		assert.Error(t, err)
		var panicErr *PanicError
		assert.ErrorAs(t, err, &panicErr)
		assert.Equal(t, 42, panicErr.Val)
		assert.NotEmpty(t, panicErr.Stack)
	})

	t.Run("transitive tasks", func(t *testing.T) {
		var order []int
		c := NewTask(func(_ context.Context) error {
			order = append(order, 3)
			return nil
		})
		b := NewTask(func(_ context.Context) error {
			order = append(order, 2)
			return nil
		}, c)
		a := NewTask(func(_ context.Context) error {
			order = append(order, 1)
			return nil
		}, b)

		_, err := AwaitAll(context.Background(), []*Task{a})

		assert.NoError(t, err)
		assert.Len(t, order, 3)
		assert.Equal(t, 3, order[0])
		assert.Equal(t, 2, order[1])
		assert.Equal(t, 1, order[2])
	})
}

func TestTaskPool_FirstAwait(t *testing.T) {
	t.Run("wait for all tasks", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return err1
		})
		task2 := NewTask(func(_ context.Context) error {
			return err2
		})

		_, err := CancelFirstError(context.TODO(), []*Task{task1, task2})

		assert.False(t, errors.Is(err, err1) && errors.Is(err, err2))
		assert.True(t, errors.Is(err, err1) || errors.Is(err, err2))
	})

	t.Run("don't start tasks if already aborted", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return err1
		})
		task2 := NewTask(func(_ context.Context) error {
			t.Fail()
			return nil
		}, task1)
		task3 := NewTask(func(_ context.Context) error {
			t.Fail()
			return err2
		}, task2)

		res, err := CancelFirstError(context.TODO(), []*Task{task1, task2, task3})

		assert.ErrorIs(t, err, err1)
		err, ok := res.GetResult(task2)
		assert.True(t, ok)
		assert.Equal(t, err, context.Canceled)
		err, ok = res.GetResult(task3)
		assert.True(t, ok)
		assert.Equal(t, err, context.Canceled)
	})

	t.Run("no errors", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			return nil
		})

		_, err := CancelFirstError(context.TODO(), []*Task{task1, task2})

		assert.NoError(t, err)
	})

	t.Run("with timeout", func(t *testing.T) {
		done := make(chan struct{})
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			<-done
			return nil
		})

		res, err := NewTaskPool(WithTimeout(time.Nanosecond)).CancelFirstError(context.TODO(), []*Task{task1, task2})
		close(done)

		assert.ErrorIs(t, err, ErrTimeout)
		<-res.GetPromise(task1)
		<-res.GetPromise(task2)
	})

	t.Run("with max concurrency", func(t *testing.T) {
		var counter atomic.Int32
		task1 := NewTask(func(_ context.Context) error {
			assert.Equal(t, int32(1), counter.Add(1))
			counter.Add(-1)
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			assert.Equal(t, int32(1), counter.Add(1))
			counter.Add(-1)
			return nil
		})

		_, err := NewTaskPool(WithMaxConcurrency(1)).CancelFirstError(context.TODO(), []*Task{task1, task2})

		assert.NoError(t, err)
		assert.Equal(t, int32(0), counter.Load())
	})

	t.Run("order of tasks", func(t *testing.T) {
		var order []int
		task1 := NewTask(func(_ context.Context) error {
			order = append(order, 1)
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			order = append(order, 2)
			return nil
		}, task1)

		_, err := CancelFirstError(context.TODO(), []*Task{task2, task1})

		assert.Nil(t, err)
		assert.Len(t, order, 2)
		assert.Equal(t, 1, order[0])
		assert.Equal(t, 2, order[1])
	})

	t.Run("no tasks", func(t *testing.T) {
		_, err := CancelFirstError(context.TODO(), nil)

		assert.NoError(t, err)
	})

	t.Run("panic in task", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			panic(42)
		})
		task2 := NewTask(func(_ context.Context) error {
			return nil
		})

		_, err := CancelFirstError(context.TODO(), []*Task{task1, task2})

		assert.Error(t, err)
		var panicErr *PanicError
		assert.ErrorAs(t, err, &panicErr)
		assert.Equal(t, 42, panicErr.Val)
		assert.NotEmpty(t, panicErr.Stack)
	})

	t.Run("double reference", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			return nil
		}, task1, task1)

		_, err := AwaitAll(context.TODO(), []*Task{task1, task2})

		assert.NoError(t, err)
	})
}

func TestTaskPool_First(t *testing.T) {
	t.Run("return directly after first error", func(t *testing.T) {
		done := make(chan struct{})
		task1 := NewTask(func(_ context.Context) error {
			return err1
		})
		task2 := NewTask(func(_ context.Context) error {
			<-done
			return err2
		})

		res, err := AbortFirstError(context.TODO(), []*Task{task1, task2})
		p := res.GetPromise(task2)
		select {
		case <-p:
			t.Fail()
		default:
			// should not be ready yet
		}

		close(done)

		assert.ErrorIs(t, err, err1)
		assert.Equal(t, err2, <-p)
	})

	t.Run("no errors", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			return nil
		})

		_, err := AbortFirstError(context.TODO(), []*Task{task1, task2})

		assert.NoError(t, err)
	})

	t.Run("with timeout", func(t *testing.T) {
		done := make(chan struct{})
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			<-done
			return nil
		})

		res, err := NewTaskPool(WithTimeout(time.Nanosecond)).AbortFirstError(context.TODO(), []*Task{task1, task2})
		p := res.GetPromise(task2)
		select {
		case <-p:
			t.Fail()
		default:
			// should not be ready yet
		}

		close(done)

		assert.ErrorIs(t, err, ErrTimeout)
		<-p
	})

	t.Run("with max concurrency", func(t *testing.T) {
		var counter atomic.Int32
		task1 := NewTask(func(_ context.Context) error {
			assert.Equal(t, int32(1), counter.Add(1))
			counter.Add(-1)
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			assert.Equal(t, int32(1), counter.Add(1))
			counter.Add(-1)
			return nil
		})

		_, err := NewTaskPool(WithMaxConcurrency(1)).AbortFirstError(context.TODO(), []*Task{task1, task2})

		assert.NoError(t, err)
		assert.Equal(t, int32(0), counter.Load())
	})

	t.Run("order of tasks", func(t *testing.T) {
		var order []int
		task1 := NewTask(func(_ context.Context) error {
			order = append(order, 1)
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			order = append(order, 2)
			return nil
		}, task1)

		_, err := AbortFirstError(context.TODO(), []*Task{task2, task1})

		assert.NoError(t, err)
		assert.Len(t, order, 2)
		assert.Equal(t, 1, order[0])
		assert.Equal(t, 2, order[1])
	})

	t.Run("no tasks", func(t *testing.T) {
		_, err := AbortFirstError(context.TODO(), nil)

		assert.NoError(t, err)
	})

	t.Run("panic in task", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			panic(42)
		})
		task2 := NewTask(func(_ context.Context) error {
			return nil
		})

		_, err := AbortFirstError(context.TODO(), []*Task{task1, task2})

		assert.Error(t, err)
		var panicErr *PanicError
		assert.ErrorAs(t, err, &panicErr)
		assert.Equal(t, 42, panicErr.Val)
		assert.NotEmpty(t, panicErr.Stack)
	})
}

func TestTaskPool_Run(t *testing.T) {
	t.Run("cyclic dependencies", func(t *testing.T) {
		nop := func(_ context.Context) error { return nil }
		task1 := NewTask(nop)
		task2 := NewTask(nop, task1)
		task3 := NewTask(nop, task2)
		task1.DependsOn = []*Task{task3}

		_, err := Run(context.TODO(), AwaitAllTasks{}, []*Task{task1, task2, task3})

		assert.ErrorIs(t, err, ErrCyclicDependencies)
	})

	t.Run("cyclic identity", func(t *testing.T) {
		nop := func(_ context.Context) error { return nil }
		task1 := NewTask(nop)
		task1.DependsOn = []*Task{task1}

		_, err := Run(context.TODO(), AwaitAllTasks{}, []*Task{task1})

		assert.ErrorIs(t, err, ErrCyclicDependencies)
	})
}
