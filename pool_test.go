package concurrency

import (
	"context"
	"errors"
	"sync"
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

		res := NewTaskPool().AwaitAll(context.TODO(), task1, task2)

		assert.ErrorIs(t, res, err1)
		assert.ErrorIs(t, res, err2)
	})

	t.Run("no errors", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			return nil
		})

		res := NewTaskPool().AwaitAll(context.TODO(), task1, task2)

		assert.Nil(t, res)
	})

	t.Run("with timeout", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			time.Sleep(time.Millisecond)
			return nil
		})

		res := NewTaskPool(WithTimeout(time.Nanosecond)).AwaitAll(context.TODO(), task1, task2)

		assert.ErrorIs(t, res, ErrTimeout)
	})

	t.Run("with max concurrency", func(t *testing.T) {
		var order []int
		task1 := NewTask(func(_ context.Context) error {
			time.Sleep(50 * time.Millisecond)
			order = append(order, 1)
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			order = append(order, 2)
			return nil
		})

		res := NewTaskPool(WithMaxConcurrency(1)).AwaitAll(context.TODO(), task1, task2)

		assert.Nil(t, res)
		assert.Len(t, order, 2)
		assert.Equal(t, 1, order[0])
		assert.Equal(t, 2, order[1])
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

		res := NewTaskPool().AwaitAll(context.TODO(), task2, task1)

		assert.Nil(t, res)
		assert.Len(t, order, 2)
		assert.Equal(t, 1, order[0])
		assert.Equal(t, 2, order[1])
	})

	t.Run("no tasks", func(t *testing.T) {
		res := NewTaskPool().AwaitAll(context.TODO())

		assert.Nil(t, res)
	})

	t.Run("panic in task", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			panic(42)
		})
		task2 := NewTask(func(_ context.Context) error {
			return nil
		})

		res := NewTaskPool().AwaitAll(context.TODO(), task1, task2)

		assert.Error(t, res)
		var panicErr PanicError
		assert.ErrorAs(t, res, &panicErr)
		assert.Equal(t, 42, panicErr.Val)
		assert.NotEmpty(t, panicErr.Stack)
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

		res := NewTaskPool().FirstAwait(context.TODO(), task1, task2)

		assert.True(t, (errors.Is(res, err1) && !errors.Is(res, err2)) || (errors.Is(res, err2) && !errors.Is(res, err1)))
	})

	t.Run("don't start tasks if already aborted", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return err1
		})
		task2 := NewTask(func(_ context.Context) error {
			time.Sleep(time.Millisecond)
			return nil
		}, task1)
		task3 := NewTask(func(_ context.Context) error {
			t.Fatal()
			return err2
		}, task2)

		res := NewTaskPool().FirstAwait(context.TODO(), task1, task2, task3)

		assert.ErrorIs(t, res, err1)
		assert.Equal(t, context.Canceled, res.GetResult(task3))
	})

	t.Run("no errors", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			return nil
		})

		res := NewTaskPool().FirstAwait(context.TODO(), task1, task2)

		assert.Nil(t, res)
	})

	t.Run("with timeout", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			time.Sleep(time.Millisecond)
			return nil
		})

		res := NewTaskPool(WithTimeout(time.Nanosecond)).FirstAwait(context.TODO(), task1, task2)

		assert.ErrorIs(t, res, ErrTimeout)
	})

	t.Run("with max concurrency", func(t *testing.T) {
		var order []int
		task1 := NewTask(func(_ context.Context) error {
			time.Sleep(50 * time.Millisecond)
			order = append(order, 1)
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			order = append(order, 2)
			return nil
		})

		res := NewTaskPool(WithMaxConcurrency(1)).FirstAwait(context.TODO(), task1, task2)

		assert.Nil(t, res)
		assert.Len(t, order, 2)
		assert.Equal(t, 1, order[0])
		assert.Equal(t, 2, order[1])
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

		res := NewTaskPool().FirstAwait(context.TODO(), task2, task1)

		assert.Nil(t, res)
		assert.Len(t, order, 2)
		assert.Equal(t, 1, order[0])
		assert.Equal(t, 2, order[1])
	})

	t.Run("no tasks", func(t *testing.T) {
		res := NewTaskPool().FirstAwait(context.TODO())

		assert.Nil(t, res)
	})

	t.Run("panic in task", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			panic(42)
		})
		task2 := NewTask(func(_ context.Context) error {
			return nil
		})

		res := NewTaskPool().FirstAwait(context.TODO(), task1, task2)

		assert.Error(t, res)
		var panicErr PanicError
		assert.ErrorAs(t, res, &panicErr)
		assert.Equal(t, 42, panicErr.Val)
		assert.NotEmpty(t, panicErr.Stack)
	})
}

func TestTaskPool_First(t *testing.T) {
	t.Run("return directly after first error", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(1)
		task1 := NewTask(func(_ context.Context) error {
			return err1
		})
		task2 := NewTask(func(_ context.Context) error {
			defer wg.Done()
			time.Sleep(50 * time.Millisecond)
			return err2
		})

		start := time.Now()
		res := NewTaskPool().First(context.TODO(), task1, task2)
		elapsed := time.Since(start)

		assert.ErrorIs(t, res, err1)
		assert.Less(t, elapsed, 50*time.Millisecond)
		wg.Wait()
	})

	t.Run("no errors", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			return nil
		})

		res := NewTaskPool().First(context.TODO(), task1, task2)

		assert.Nil(t, res)
	})

	t.Run("with timeout", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			time.Sleep(time.Millisecond)
			return nil
		})

		res := NewTaskPool(WithTimeout(time.Nanosecond)).First(context.TODO(), task1, task2)

		assert.ErrorIs(t, res, ErrTimeout)
	})

	t.Run("with max concurrency", func(t *testing.T) {
		var order []int
		task1 := NewTask(func(_ context.Context) error {
			time.Sleep(50 * time.Millisecond)
			order = append(order, 1)
			return nil
		})
		task2 := NewTask(func(_ context.Context) error {
			order = append(order, 2)
			return nil
		})

		res := NewTaskPool(WithMaxConcurrency(1)).First(context.TODO(), task1, task2)

		assert.Nil(t, res)
		assert.Len(t, order, 2)
		assert.Equal(t, 1, order[0])
		assert.Equal(t, 2, order[1])
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

		res := NewTaskPool().First(context.TODO(), task2, task1)

		assert.Nil(t, res)
		assert.Len(t, order, 2)
		assert.Equal(t, 1, order[0])
		assert.Equal(t, 2, order[1])
	})

	t.Run("no tasks", func(t *testing.T) {
		res := NewTaskPool().First(context.TODO())

		assert.Nil(t, res)
	})

	t.Run("panic in task", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			panic(42)
		})
		task2 := NewTask(func(_ context.Context) error {
			return nil
		})

		res := NewTaskPool().First(context.TODO(), task1, task2)

		assert.Error(t, res)
		var panicErr PanicError
		assert.ErrorAs(t, res, &panicErr)
		assert.Equal(t, 42, panicErr.Val)
		assert.NotEmpty(t, panicErr.Stack)
	})
}
