package concurrency

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTask(t *testing.T) {
	t.Run("without dependencies", func(t *testing.T) {
		task := NewTask(func(_ context.Context) error {
			return assert.AnError
		})

		got := task.Func(context.TODO())

		assert.ErrorIs(t, got, assert.AnError)
		assert.Empty(t, task.DependsOn)
	})

	t.Run("with dependencies", func(t *testing.T) {
		task1 := NewTask(func(_ context.Context) error {
			return nil
		})

		task2 := NewTask(func(_ context.Context) error {
			return nil
		}, task1, task1)

		assert.Len(t, task2.DependsOn, 2)
		assert.Equal(t, task1, task2.DependsOn[0])
		assert.Equal(t, task1, task2.DependsOn[1])
	})
}
