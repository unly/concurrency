# concurrency

Simple helper to run functions concurrently using individual goroutines and wait for the outcomes.

## How to use

Each function is wrapped in a task with potential dependencies.
A task can return an error to indicate any issues within and a potential aborting of other tasks executed.
Other variables and out results can be returned via shared variables such as the `r` variable in the example below.

```go
var r io.Reader
task1 := concurrency.NewTask(func(ctx context.Context) error {
	// do something 
	r = strings.NewReader("some outcome")
	return nil
})

task2 := concurrency.NewTask(func(ctx context.Context) error {
	// do something 
	return nil
})

// task3 depends on task1 and task2 to complete first
task3 := concurrency.NewTask(func(ctx context.Context) error {
	// read from shared memory variable r 
	data, err := io.ReadAll(r)
	// do something
	return errors.New("something went wrong")
}, task1, task2)
```

These tasks can be run concurrently taking into account the given dependencies.
All tasks are executed the outcome stored and returned back to the caller.
If all tasks run without an error `nil` is returned.

```go
res := concurrency.AwaitAll(context.Background(), task1, task2, task3)
if res != nil {
	res.Error() // returns overall error message
	errTask1 := res.GetResult(task1) // error outcome for task1
	
	// res can be used as error
	return res
}
// all tasks are executed without an error
```

Besides the classic waiting for all tasks to run and finish, there are options to abort in case of an error.
The two options below abort in the first error received.

```go
// aborts after the first error received
res := concurrency.First(context.Background(), task1, task2, task3)

// aborts after the first error received, waits for all goroutines to finish
res := concurrency.FirstAwait(context.Background(), task1, task2, task3)
```
