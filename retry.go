package retry

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Worker is a function that performs work and returns no error when succeeds.
type Worker func(ctx context.Context) (interface{}, error)

// Result is what is returned from the api for any worker function call.
type Result struct {
	Value interface{}
	Err   error
}

// Error informs that a cancellation took place before the worker
// function returned successfully.
type Error struct {
	errWork error
	since   time.Duration
}

// Error implements the error interface and returns information about
// the timeout error.
func (err *Error) Error() string {
	if err.errWork != nil {
		return fmt.Sprintf("context cancelled after %v : %s", err.since, err.errWork)
	}
	return fmt.Sprintf("context cancelled after %v", err.since)
}

// Unwrap returns the context error, if any
func (err *Error) Unwrap() error {
	return err.errWork
}

// Constants that represent the max goroutines to use.
const (
	MaxGoroutines = 0
)

// Func calls the worker function every retry interval until the worker
// function succeeds or the context times out.
func Func(ctx context.Context, retryInterval time.Duration, worker Worker) Result {
	var retry *time.Timer
	start := time.Now()

	if ctx.Err() != nil {
		return Result{Err: &Error{errWork: nil, since: time.Since(start)}}
	}

	for {
		value, err := worker(ctx)
		if err == nil {
			return Result{Value: value}
		}

		if ctx.Err() != nil {
			return Result{Err: &Error{errWork: err, since: time.Since(start)}}
		}

		if retry == nil {
			retry = time.NewTimer(retryInterval)
		}

		select {
		case <-ctx.Done():
			retry.Stop()
			return Result{Err: &Error{errWork: err, since: time.Since(start)}}
		case <-retry.C:
			retry.Reset(retryInterval)
		}
	}
}

// All calls all the worker functions every retry interval until the worker
// functions succeeds or the context times out. maxGs represents the number
// of goroutines to run simultaneously to execute all the worker functions.
func All(ctx context.Context, retryInterval time.Duration, workers map[string]Worker, maxGs int) map[string]Result {
	results := make(map[string]Result)

	switch {
	case maxGs <= 0 || maxGs >= len(workers):
		for result := range workMap(ctx, retryInterval, workers) {
			results[result.name] = result.Result
		}
	default:
		for result := range workPool(ctx, retryInterval, workers, maxGs) {
			results[result.name] = result.Result
		}
	}

	return results
}

// First calls all the worker functions every retry interval until the worker
// functions succeeds or the context times out. Once the first worker function
// succeeds, this function will return that result. maxGs represents the number
// of goroutines to run simultaneously to execute all the worker functions.
func First(ctx context.Context, retryInterval time.Duration, workers map[string]Worker, maxGs int) Result {
	start := time.Now()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	switch {
	case maxGs <= 0 || maxGs >= len(workers):
		for result := range workMap(ctx, retryInterval, workers) {
			if result.Result.Err != nil {
				continue
			}
			return result.Result
		}
	default:
		for result := range workPool(ctx, retryInterval, workers, maxGs) {
			if result.Result.Err != nil {
				continue
			}
			return result.Result
		}
	}

	return Result{Err: &Error{errWork: errors.New("all worker functions failed"), since: time.Since(start)}}
}

// namedResult provides support to match a result to a goroutine that
// performed the work.
type namedResult struct {
	name string
	Result
}

// workMap calls the map of worker functions every retry interval until the
// worker function succeeds or the context times out. As worker functions
// complete, their results are signaled over the channel for processing.
func workMap(ctx context.Context, retryInterval time.Duration, workers map[string]Worker) <-chan namedResult {
	g := len(workers)
	results := make(chan namedResult, g)

	go func() {
		var wg sync.WaitGroup
		wg.Add(g)
		for name, worker := range workers {
			name, worker := name, worker
			go func() {
				defer wg.Done()
				result := Func(ctx, retryInterval, worker)
				results <- namedResult{name: name, Result: result}
			}()
		}
		wg.Wait()
		close(results)
	}()

	return results
}

// workPool calls the map of worker functions every retry interval until the
// worker function succeeds or the context times out. As worker functions
// complete, their results are signaled over the channel for processing. Instead
// of running each worker in a separate goroutine, the worker functions are
// executed from a pool of goroutines.
func workPool(ctx context.Context, retryInterval time.Duration, workers map[string]Worker, concurrency int) <-chan namedResult {
	g := concurrency
	results := make(chan namedResult, g)

	var wg sync.WaitGroup
	wg.Add(g)

	type namedWorker struct {
		name   string
		worker Worker
	}
	input := make(chan namedWorker, g)

	for i := 0; i < g; i++ {
		go func() {
			defer wg.Done()
			for nw := range input {
				result := Func(ctx, retryInterval, nw.worker)
				results <- namedResult{name: nw.name, Result: result}
			}
		}()
	}

	go func() {
		for name, worker := range workers {
			input <- namedWorker{name, worker}
		}
		close(input)
		wg.Wait()
		close(results)
	}()

	return results
}
