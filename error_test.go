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
