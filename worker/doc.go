// Package worker provides a simple goroutine pool for background task
// execution.
//
// Create a pool with a fixed number of workers, submit tasks, and wait for
// completion:
//
//	pool := worker.NewPool(4)
//	for _, item := range items {
//	    item := item
//	    pool.Submit(func() { process(item) })
//	}
//	pool.WaitAll()
//
// [Pool.Submit] queues a [Task] for execution by one of the pool's workers.
// [Pool.WaitAll] blocks until every submitted task has finished.
package worker
