package concurrency

import (
	"fmt"
)

// PanicError raised if a panic occurred during the execution of
// a given Task. Contains the basic debugging information.
type PanicError struct {
	// Val return from the recover function
	Val any
	// Stack trace from debug.Stack to track the root cause
	Stack []byte
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("panic: %v", e.Val)
}
