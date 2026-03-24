package worker_test

import (
	"fmt"
	"sync/atomic"

	"github.com/luanguimaraesla/garlic/worker"
)

func ExamplePool() {
	pool := worker.NewPool(2)

	var count atomic.Int32
	for i := 0; i < 5; i++ {
		pool.Submit(func() {
			count.Add(1)
		})
	}
	pool.WaitAll()

	fmt.Println(count.Load())
	// Output:
	// 5
}
